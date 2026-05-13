// Package app holds Wails services exposed to the frontend.
package app

import (
	"context"
	"errors"
	"strings"
	"sync"

	"tools/tb-gui/internal/cli"
)

// Task mirrors the JSON contract emitted by `tb ls --json` (see
// cli/json_output.go in the CLI module). The JSON tags drive both the Wails
// binding generator and the on-the-wire shape.
type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	Size        string   `json:"size"`
	Module      string   `json:"module"`
	Tags        []string `json:"tags"`
	Branch      string   `json:"branch"`
	Parent      string   `json:"parent"`
	Status      string   `json:"status"`
	FilePath    string   `json:"filePath"`
	Agent       string   `json:"agent"`
	AgentStatus string   `json:"agentStatus"`
}

// BoardSnapshot is the read-only view the frontend renders as a kanban.
// Tasks are pre-bucketed server-side so the frontend doesn't have to know
// about the status taxonomy.
//
// Archive is always non-nil so the JSON encoder emits `[]` not `null`; it
// stays empty until LoadBoard is called in `all` mode.
type BoardSnapshot struct {
	Backlog    []Task `json:"backlog"`
	InProgress []Task `json:"inProgress"`
	Done       []Task `json:"done"`
	Archive    []Task `json:"archive"`
}

// StatusMode selects which directories LoadBoard surfaces. "active" matches
// M2 behavior (backlog + in-progress + done). "all" adds the archive bucket.
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
	b.mu.Lock()
	b.client = c
	b.mu.Unlock()
}

// LoadBoard returns the active task set (backlog + in-progress + done),
// pre-bucketed by status. Equivalent to LoadBoardWithMode(ctx, "active").
func (b *BoardService) LoadBoard(ctx context.Context) (BoardSnapshot, error) {
	return b.LoadBoardWithMode(ctx, string(StatusModeActive))
}

// LoadBoardWithMode is the archive-aware variant. mode = "active" preserves
// M2 behavior; mode = "all" also populates Snapshot.Archive. Any other value
// is treated as "active" so unknown modes from the frontend degrade safely.
func (b *BoardService) LoadBoardWithMode(ctx context.Context, mode string) (BoardSnapshot, error) {
	c := b.snapshot()
	if c == nil {
		return BoardSnapshot{}, ErrNoBoard
	}

	statusArg := "active"
	if StatusMode(mode) == StatusModeAll {
		statusArg = "all"
	}

	var tasks []Task
	if err := c.RunJSON(ctx, &tasks, "ls", "--json", "--status", statusArg); err != nil {
		return BoardSnapshot{}, err
	}

	snap := BoardSnapshot{
		Backlog:    make([]Task, 0),
		InProgress: make([]Task, 0),
		Done:       make([]Task, 0),
		Archive:    make([]Task, 0),
	}
	for _, t := range tasks {
		switch t.Status {
		case "backlog":
			snap.Backlog = append(snap.Backlog, t)
		case "in-progress":
			snap.InProgress = append(snap.InProgress, t)
		case "done":
			snap.Done = append(snap.Done, t)
		case "archive":
			snap.Archive = append(snap.Archive, t)
		}
	}
	return snap, nil
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
	return detail, nil
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
}

// EditTask runs `tb edit <id> [flags…]`. Returns nil on success.
func (b *BoardService) EditTask(ctx context.Context, id string, in EditTaskInput) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Edit(ctx, id, cli.EditInput{
		Priority:    in.Priority,
		Type:        in.Type,
		Size:        in.Size,
		Module:      in.Module,
		Tags:        in.Tags,
		Agent:       in.Agent,
		AgentStatus: in.AgentStatus,
	})
}

// MoveTask runs `tb mv <id> <status>` where status ∈ {backlog, in-progress, done}.
func (b *BoardService) MoveTask(ctx context.Context, id, status string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Move(ctx, id, status)
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

func (b *BoardService) snapshot() *cli.Client {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.client
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
