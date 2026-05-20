// Package app holds Wails services exposed to the frontend.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"tools/tb-gui/internal/cli"
)

// Task mirrors the JSON contract emitted by `tb ls --json` (see
// cli/json_output.go in the CLI module). The JSON tags drive both the Wails
// binding generator and the on-the-wire shape.
type Task struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	Priority string   `json:"priority"`
	Size     string   `json:"size"`
	Module   string   `json:"module"`
	Tags     []string `json:"tags"`
	Branch   string   `json:"branch"`
	// ReviewRef is the branch/PR/commit/worktree reviewers should inspect.
	// Required (non-placeholder) when the task enters code-review (TB-235).
	ReviewRef   string `json:"reviewRef"`
	Parent      string `json:"parent"`
	Status      string `json:"status"`
	FilePath    string `json:"filePath"`
	Agent       string `json:"agent"`
	AgentStatus string `json:"agentStatus"`
	// AgentResumable is true when the latest run has a captured session id
	// and the task is in a terminal state that permits Resume.
	AgentResumable bool `json:"agentResumable"`
	// TB-237: per-mode attribution. Each pair mirrors the (Agent,
	// AgentStatus) shape but scoped to one kanban action; the legacy pair
	// continues to reflect the most recent run.
	GroomedBy       string `json:"groomedBy"`
	GroomStatus     string `json:"groomStatus"`
	ImplementedBy   string `json:"implementedBy"`
	ImplementStatus string `json:"implementStatus"`
	ReviewedBy      string `json:"reviewedBy"`
	ReviewStatus    string `json:"reviewStatus"`
}

// BoardSnapshot is the read-only view the frontend renders as a kanban.
// Tasks are pre-bucketed server-side so the frontend doesn't have to know
// about the status taxonomy. Columns are in canonical kanban order:
// backlog → ready → in-progress → code-review → done (+ archive on demand).
//
// All slices are always non-nil so the JSON encoder emits `[]` not `null`;
// Archive stays empty until LoadBoard is called in `all` mode.
//
// WipLimits / WipCounts / WipEnforcement come from `tb board --json` so the
// frontend can render `(n/m)` badges without re-counting. Missing entries
// in WipLimits mean the column has no limit configured. Maps are always
// non-nil for clean JSON ({} not null).
type BoardSnapshot struct {
	Backlog        []Task         `json:"backlog"`
	Ready          []Task         `json:"ready"`
	InProgress     []Task         `json:"inProgress"`
	CodeReview     []Task         `json:"codeReview"`
	Done           []Task         `json:"done"`
	Archive        []Task         `json:"archive"`
	WipLimits      map[string]int `json:"wipLimits"`
	WipCounts      map[string]int `json:"wipCounts"`
	WipEnforcement string         `json:"wipEnforcement"`
}

// boardWIPSnapshot is the slice of `tb board --json` LoadBoardWithMode
// reads to populate the WIP fields. The full board snapshot has many
// other fields we don't need; this struct only decodes the WIP metadata.
type boardWIPSnapshot struct {
	WipLimits      map[string]int `json:"wipLimits"`
	WipCounts      map[string]int `json:"wipCounts"`
	WipEnforcement string         `json:"wipEnforcement"`
}

// StatusMode selects which directories LoadBoard surfaces. "active" is
// backlog + ready + in-progress + code-review + done (everything the kanban
// board shows by default). "all" adds the archive bucket.
type StatusMode string

const (
	StatusModeActive StatusMode = "active"
	StatusModeAll    StatusMode = "all"
)

// TaskDetail is what GetTask returns: full metadata plus the raw markdown
// body (the frontend's TaskDrawer renders it).
type TaskDetail struct {
	Metadata Task   `json:"metadata"`
	Body     string `json:"body"`
}

// ErrNoBoard is returned by service methods when no project root has been
// selected yet. The frontend treats this as "show the folder picker", not
// "fail loudly".
var ErrNoBoard = errors.New("no board selected")

// ErrNotFound is returned by GetTask when the requested ID isn't on disk.
var ErrNotFound = errors.New("task not found")

// BoardService exposes read-only board operations to the frontend.
//
// The active CLI client is swappable so SettingsService can re-target the
// service when the user picks a new board, without rebuilding the service or
// restarting the daemon.
type BoardService struct {
	mu       sync.RWMutex
	client   *cli.Client
	boardDir string // populated by SetBoardDir; used by direct-write paths
	openFile attachmentOpener

	loadMu       sync.Mutex
	loadInFlight map[string]*boardLoadCall

	triageMu    sync.RWMutex
	triageCache map[string][]string
	triageGen   uint64
}

type boardLoadCall struct {
	done chan struct{}
	snap BoardSnapshot
	err  error
}

// NewBoardService returns a service with no client attached. The caller (a
// SettingsService at startup) must call setClient before LoadBoard/GetTask
// return useful data.
func NewBoardService() *BoardService { return &BoardService{} }

// ServiceName satisfies the Wails service contract.
func (b *BoardService) ServiceName() string { return "BoardService" }

// setClient atomically swaps the active CLI client. Passing nil clears it
// (LoadBoard then returns ErrNoBoard).
func (b *BoardService) setClient(c *cli.Client) {
	b.triageMu.Lock()
	defer b.triageMu.Unlock()

	b.mu.Lock()
	b.client = c
	b.mu.Unlock()

	b.triageCache = nil
	b.triageGen++
}

// LoadBoard returns the active task set (backlog + ready + in-progress +
// code-review + done), pre-bucketed by status. Equivalent to
// LoadBoardWithMode(ctx, "active").
func (b *BoardService) LoadBoard(ctx context.Context) (BoardSnapshot, error) {
	return b.LoadBoardWithMode(ctx, string(StatusModeActive))
}

// LoadBoardWithMode is the archive-aware variant. mode = "active" returns
// backlog + ready + in-progress + code-review + done; mode = "all" also
// populates Snapshot.Archive. Any other value is treated as "active" so
// unknown modes from the frontend degrade safely.
func (b *BoardService) LoadBoardWithMode(ctx context.Context, mode string) (BoardSnapshot, error) {
	c := b.snapshot()
	if c == nil {
		return BoardSnapshot{}, ErrNoBoard
	}

	statusArg := "active"
	if StatusMode(mode) == StatusModeAll {
		statusArg = "all"
	}

	key := fmt.Sprintf("%p:%s", c, statusArg)
	return b.singleBoardLoad(ctx, key, func() (BoardSnapshot, error) {
		return b.loadBoardSnapshot(ctx, c, statusArg)
	})
}

func (b *BoardService) singleBoardLoad(
	ctx context.Context,
	key string,
	load func() (BoardSnapshot, error),
) (BoardSnapshot, error) {
	b.loadMu.Lock()
	if b.loadInFlight == nil {
		b.loadInFlight = make(map[string]*boardLoadCall)
	}
	if call := b.loadInFlight[key]; call != nil {
		done := call.done
		b.loadMu.Unlock()
		select {
		case <-done:
			return call.snap, call.err
		case <-ctx.Done():
			return BoardSnapshot{}, ctx.Err()
		}
	}

	call := &boardLoadCall{done: make(chan struct{})}
	b.loadInFlight[key] = call
	b.loadMu.Unlock()

	call.snap, call.err = load()

	b.loadMu.Lock()
	delete(b.loadInFlight, key)
	close(call.done)
	b.loadMu.Unlock()

	return call.snap, call.err
}

func (b *BoardService) loadBoardSnapshot(
	ctx context.Context,
	c *cli.Client,
	statusArg string,
) (BoardSnapshot, error) {
	var tasks []Task
	if err := c.RunJSON(ctx, &tasks, "ls", "--json", "--status", statusArg); err != nil {
		return BoardSnapshot{}, boardLoadError(err, statusArg)
	}

	snap := BoardSnapshot{
		Backlog:        make([]Task, 0),
		Ready:          make([]Task, 0),
		InProgress:     make([]Task, 0),
		CodeReview:     make([]Task, 0),
		Done:           make([]Task, 0),
		Archive:        make([]Task, 0),
		WipLimits:      map[string]int{},
		WipCounts:      map[string]int{},
		WipEnforcement: "warn",
	}
	for _, t := range tasks {
		b.populateAgentResumable(ctx, &t)
		switch t.Status {
		case "backlog":
			snap.Backlog = append(snap.Backlog, t)
		case "ready":
			snap.Ready = append(snap.Ready, t)
		case "in-progress":
			snap.InProgress = append(snap.InProgress, t)
		case "code-review":
			snap.CodeReview = append(snap.CodeReview, t)
		case "done":
			snap.Done = append(snap.Done, t)
		case "archive":
			if statusArg != string(StatusModeAll) {
				continue
			}
			snap.Archive = append(snap.Archive, t)
		}
	}

	// Second CLI call: pull the WIP metadata from `tb board --json`. This
	// is cheap (a single re-walk of statusDirs) and keeps the frontend
	// free of separate calls. Failures degrade silently — the buckets
	// above are the authoritative data; the badge just won't render if
	// the second call fails. Backend logs the error so it doesn't go
	// silent in production.
	var wip boardWIPSnapshot
	if err := c.RunJSON(ctx, &wip, "board", "--json"); err == nil {
		if wip.WipLimits != nil {
			snap.WipLimits = wip.WipLimits
		}
		if wip.WipCounts != nil {
			snap.WipCounts = wip.WipCounts
		}
		if wip.WipEnforcement != "" {
			snap.WipEnforcement = wip.WipEnforcement
		}
	} else {
		slog.Default().Warn("board: tb board --json failed; rendering without WIP metadata", "err", err)
	}
	return snap, nil
}

// ListWithFilter shells out to `tb ls --json --status <status>` with the
// AutoImplementFilter serialized into the multi-value flags TB-289 added
// to the CLI. The coordinator uses this for its candidate pool so the
// CLI stays the single source of truth for filter semantics; no
// in-process matching survives in the GUI repo.
func (b *BoardService) ListWithFilter(ctx context.Context, status string, filter AutoImplementFilter) ([]Task, error) {
	c := b.snapshot()
	if c == nil {
		return nil, ErrNoBoard
	}
	if status == "" {
		status = "ready"
	}
	args := append([]string{"ls", "--json", "--status", status}, filter.toLsArgs()...)
	var tasks []Task
	if err := c.RunJSON(ctx, &tasks, args...); err != nil {
		return nil, boardLoadError(err, status)
	}
	return tasks, nil
}

// Triage returns task IDs that need grooming, keyed by ID with the CLI's
// reason strings as the value. The first call shells out to `tb triage --json`;
// subsequent calls return a copy of the cached map until watcher events
// invalidate it.
func (b *BoardService) Triage(ctx context.Context) (map[string][]string, error) {
	for {
		b.triageMu.RLock()
		if b.triageCache != nil {
			out := cloneTriageMap(b.triageCache)
			b.triageMu.RUnlock()
			return out, nil
		}
		gen := b.triageGen
		b.triageMu.RUnlock()

		c := b.snapshot()
		if c == nil {
			return nil, ErrNoBoard
		}

		next, err := b.loadTriage(ctx, c)
		if err != nil {
			return nil, err
		}

		b.triageMu.Lock()
		if b.triageGen != gen {
			b.triageMu.Unlock()
			continue
		}
		b.triageCache = cloneTriageMap(next)
		out := cloneTriageMap(b.triageCache)
		b.triageMu.Unlock()
		return out, nil
	}
}

// GetTask returns metadata + body for a single task. Returns ErrNotFound when
// the ID resolves to nothing.
func (b *BoardService) GetTask(ctx context.Context, id string) (TaskDetail, error) {
	c := b.snapshot()
	if c == nil {
		return TaskDetail{}, ErrNoBoard
	}

	var detail TaskDetail
	if err := c.RunJSON(ctx, &detail, "show", id, "--json"); err != nil {
		if isNotFoundErr(err) {
			return TaskDetail{}, ErrNotFound
		}
		return TaskDetail{}, err
	}
	b.populateAgentResumable(ctx, &detail.Metadata)
	return detail, nil
}

func (b *BoardService) populateAgentResumable(ctx context.Context, t *Task) {
	_ = ctx
	if t == nil || !isResumeTerminalStatus(t.AgentStatus) {
		if t != nil {
			t.AgentResumable = false
		}
		return
	}

	b.mu.RLock()
	boardDir := b.boardDir
	b.mu.RUnlock()
	if boardDir == "" {
		t.AgentResumable = false
		return
	}

	_, ok, err := resumableSessionID(boardDir, t.ID)
	if err != nil {
		slog.Default().Warn("board: resumable session lookup failed", "task", t.ID, "err", err)
		t.AgentResumable = false
		return
	}
	t.AgentResumable = ok
}

// CreateTaskInput mirrors cli.CreateInput on the wire. Lives here (not as a
// re-export) so the Wails binding generator emits a frontend-friendly type
// without leaking the internal cli package.
type CreateTaskInput struct {
	Title       string `json:"title"`
	Module      string `json:"module"`
	Type        string `json:"type"`
	Priority    string `json:"priority"`
	Size        string `json:"size"`
	Tags        string `json:"tags"` // comma-separated
	Description string `json:"description"`
	Parent      string `json:"parent"`
	Epic        bool   `json:"epic"`
}

// CreateTaskResult is what the frontend gets back: the new task's ID and
// (optionally) the relative path the CLI printed.
type CreateTaskResult struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// CreateTask runs `tb create` with the given input.
func (b *BoardService) CreateTask(ctx context.Context, in CreateTaskInput) (CreateTaskResult, error) {
	c := b.snapshot()
	if c == nil {
		return CreateTaskResult{}, ErrNoBoard
	}
	res, err := c.Create(ctx, cli.CreateInput{
		Title:       in.Title,
		Module:      in.Module,
		Type:        in.Type,
		Priority:    in.Priority,
		Size:        in.Size,
		Tags:        in.Tags,
		Description: in.Description,
		Parent:      in.Parent,
		Epic:        in.Epic,
	})
	if err != nil {
		return CreateTaskResult{}, err
	}
	return CreateTaskResult{ID: res.ID, Path: res.Path}, nil
}

// EditTaskInput names the fields the GUI can change. Empty fields are
// skipped server-side. Pass at least one non-empty value or the CLI returns
// a validation error.
type EditTaskInput struct {
	Priority    string `json:"priority"`
	Type        string `json:"type"`
	Size        string `json:"size"`
	Module      string `json:"module"`
	Tags        string `json:"tags"` // comma-separated, replaces existing
	Agent       string `json:"agent"`
	AgentStatus string `json:"agentStatus"`
	// Title rewrites the H1 header (`# <ID>: <title>`). Empty means "leave
	// unchanged". The CLI rejects whitespace-only values and treats a
	// no-op rename (new == current) as a silent success.
	Title string `json:"title"`
	// ReviewRef sets/clears the **ReviewRef:** metadata line. Empty means
	// "leave unchanged"; "none" (case-insensitive) clears the field. Required
	// for moves into code-review (TB-235).
	ReviewRef string `json:"reviewRef"`
	// TB-237: per-mode attribution pairs. Each pair mirrors the legacy
	// (Agent, AgentStatus) shape but scoped to one kanban action. Empty
	// means "leave unchanged"; "none" clears the line.
	GroomedBy       string `json:"groomedBy"`
	GroomStatus     string `json:"groomStatus"`
	ImplementedBy   string `json:"implementedBy"`
	ImplementStatus string `json:"implementStatus"`
	ReviewedBy      string `json:"reviewedBy"`
	ReviewStatus    string `json:"reviewStatus"`
}

// EditTask runs `tb edit <id> [flags…]`. Returns nil on success.
func (b *BoardService) EditTask(ctx context.Context, id string, in EditTaskInput) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Edit(ctx, id, cli.EditInput{
		Priority:        in.Priority,
		Type:            in.Type,
		Size:            in.Size,
		Module:          in.Module,
		Tags:            in.Tags,
		Agent:           in.Agent,
		AgentStatus:     in.AgentStatus,
		Title:           in.Title,
		ReviewRef:       in.ReviewRef,
		GroomedBy:       in.GroomedBy,
		GroomStatus:     in.GroomStatus,
		ImplementedBy:   in.ImplementedBy,
		ImplementStatus: in.ImplementStatus,
		ReviewedBy:      in.ReviewedBy,
		ReviewStatus:    in.ReviewStatus,
	})
}

// MoveTask runs `tb mv <id> <status>` where status ∈ {backlog, ready, in-progress, code-review, done}.
func (b *BoardService) MoveTask(ctx context.Context, id, status string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Move(ctx, id, status)
}

// ReadyTask runs `tb ready <id>`: promotes a backlog task into the ready
// column (canonical kanban commitment point). The CLI enforces the triage
// gate so a task missing priority or with a placeholder goal is rejected
// here without GUI-side validation.
func (b *BoardService) ReadyTask(ctx context.Context, id string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Ready(ctx, id)
}

// PullNext runs `tb pull` to pull the highest-priority oldest task from
// the ready column into in-progress. Returns nil with no movement when
// the ready column is empty (the CLI emits the stderr hint).
func (b *BoardService) PullNext(ctx context.Context) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Pull(ctx, "")
}

// PullTask runs `tb pull <id>` to pull a specific ready task into
// in-progress.
func (b *BoardService) PullTask(ctx context.Context, id string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Pull(ctx, id)
}

// CloseTask runs `tb close <id>` which archives the task.
func (b *BoardService) CloseTask(ctx context.Context, id string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Close(ctx, id)
}

// Regenerate runs `tb regenerate` to rebuild BOARD.md. Wails-exposed so the
// frontend can offer a "rebuild board" affordance after manual file edits;
// also used after EditTaskBody to bring BOARD.md in sync.
func (b *BoardService) Regenerate(ctx context.Context) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Regenerate(ctx)
}

// SubmitReview runs `tb review --submit <id>`. Moves an in-progress task (or
// a review-failed ready/backlog task) into code-review.
func (b *BoardService) SubmitReview(ctx context.Context, id string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.ReviewSubmit(ctx, id)
}

// SetReviewTarget replaces the ## Review Target section with body.
func (b *BoardService) SetReviewTarget(ctx context.Context, id, body string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.ReviewWriteSection(ctx, id, "target", body)
}

// SetReviewerNotes replaces the ## Reviewer Notes section with body.
func (b *BoardService) SetReviewerNotes(ctx context.Context, id, body string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.ReviewWriteSection(ctx, id, "notes", body)
}

// SetReviewFindings replaces the ## Review Findings section with body.
func (b *BoardService) SetReviewFindings(ctx context.Context, id, body string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.ReviewWriteSection(ctx, id, "findings", body)
}

// FailReview moves a code-review task back to ready with the review-failed
// marker and persists the findings body.
func (b *BoardService) FailReview(ctx context.Context, id, findings string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.ReviewFail(ctx, id, findings)
}

func (b *BoardService) snapshot() *cli.Client {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.client
}

type triageTask struct {
	ID      string   `json:"id"`
	Reasons []string `json:"reasons"`
}

var triageJSONArgs = []string{"triage", "--json"}

func (b *BoardService) loadTriage(ctx context.Context, c *cli.Client) (map[string][]string, error) {
	var rows []triageTask
	stdout, err := c.Run(ctx, triageJSONArgs...)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(stdout)) == 0 {
		return nil, fmt.Errorf("tb %v: empty stdout (expected JSON)", triageJSONArgs)
	}
	if err := json.Unmarshal(stdout, &rows); err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			logTriageUnavailable(c, stdout, err)
			return map[string][]string{}, nil
		}
		return nil, fmt.Errorf("tb %v: decode JSON: %w", triageJSONArgs, err)
	}
	out := make(map[string][]string, len(rows))
	for _, row := range rows {
		if row.ID == "" {
			continue
		}
		out[row.ID] = append([]string(nil), row.Reasons...)
	}
	return out, nil
}

func logTriageUnavailable(c *cli.Client, stdout []byte, decodeErr error) {
	slog.Default().Warn(
		"triage-unavailable: selected CLI must support triage --json",
		"bin", c.BinaryPath(),
		"args", "triage --json",
		"stdout", triageStdoutPreview(stdout),
		"decode_error", decodeErr.Error(),
	)
}

func triageStdoutPreview(stdout []byte) string {
	const maxPreviewBytes = 512
	preview := strings.TrimSpace(string(stdout))
	if len(preview) <= maxPreviewBytes {
		return preview
	}
	return preview[:maxPreviewBytes] + "..."
}

func cloneTriageMap(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for id, reasons := range in {
		out[id] = append([]string(nil), reasons...)
	}
	return out
}

func (b *BoardService) clearTriageCache() {
	b.triageMu.Lock()
	b.triageCache = nil
	b.triageGen++
	b.triageMu.Unlock()
}

// BoardWatcherSink is wired to the filesystem watcher so BoardService can keep
// cached derived data in step with direct CLI writes.
type BoardWatcherSink struct {
	board *BoardService
}

func NewBoardWatcherSink(board *BoardService) *BoardWatcherSink {
	return &BoardWatcherSink{board: board}
}

func (s *BoardWatcherSink) Emit(name string, data ...any) {
	if s == nil || s.board == nil {
		return
	}
	switch {
	case name == "board:reloaded":
		s.board.clearTriageCache()
	case strings.HasPrefix(name, "task:updated:"):
		s.board.clearTriageCache()
	}
}

// isNotFoundErr inspects an *ExitError's stderr for the CLI's not-found
// signature. The CLI returns exit code 1 with stderr `error: task FOO not
// found in any directory ...`.
func isNotFoundErr(err error) bool {
	var exit *cli.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	return strings.Contains(exit.Stderr, "not found in any directory")
}

type duplicateCanonicalPathError struct {
	id      string
	pathA   string
	pathB   string
	statusA string
	statusB string
}

func boardLoadError(err error, statusArg string) error {
	dup, ok := parseDuplicateCanonicalPathError(err)
	if !ok {
		return err
	}
	return fmt.Errorf(
		"cannot load %s board: task %s appears in multiple status directories (%s: %s; %s: %s). Move or remove one duplicate task file, then reload.",
		statusArg,
		dup.id,
		dup.statusA,
		dup.pathA,
		dup.statusB,
		dup.pathB,
	)
}

func parseDuplicateCanonicalPathError(err error) (duplicateCanonicalPathError, bool) {
	var exit *cli.ExitError
	if !errors.As(err, &exit) {
		return duplicateCanonicalPathError{}, false
	}
	lines := strings.Split(exit.Stderr, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if dup, ok := parseDuplicateCanonicalPathLine(lines[i]); ok {
			return dup, true
		}
	}
	return duplicateCanonicalPathError{}, false
}

func parseDuplicateCanonicalPathLine(line string) (duplicateCanonicalPathError, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "error: "))
	const middle = " resolves to multiple canonical markdown paths in requested status scope: "
	if !strings.HasPrefix(line, "task ") {
		return duplicateCanonicalPathError{}, false
	}
	rest := strings.TrimPrefix(line, "task ")
	idx := strings.Index(rest, middle)
	if idx == -1 {
		return duplicateCanonicalPathError{}, false
	}
	id := strings.TrimSpace(rest[:idx])
	paths := rest[idx+len(middle):]
	pathSep := strings.LastIndex(paths, " and ")
	if id == "" || pathSep == -1 {
		return duplicateCanonicalPathError{}, false
	}
	pathA := strings.TrimSpace(paths[:pathSep])
	pathB := strings.TrimSpace(paths[pathSep+len(" and "):])
	if pathA == "" || pathB == "" {
		return duplicateCanonicalPathError{}, false
	}
	return duplicateCanonicalPathError{
		id:      id,
		pathA:   pathA,
		pathB:   pathB,
		statusA: statusFromCanonicalTaskPath(pathA),
		statusB: statusFromCanonicalTaskPath(pathB),
	}, true
}

func statusFromCanonicalTaskPath(path string) string {
	clean := filepath.Clean(path)
	if filepath.Base(clean) == "TASK.md" {
		status := filepath.Base(filepath.Dir(filepath.Dir(clean)))
		if status != "." && status != string(filepath.Separator) {
			return status
		}
		return "unknown"
	}
	status := filepath.Base(filepath.Dir(clean))
	if status == "." || status == string(filepath.Separator) {
		return "unknown"
	}
	return status
}
