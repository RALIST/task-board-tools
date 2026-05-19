package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// EventName is the closed set of event names that may appear in a JSONL
// run-history file. Anything outside this set is a bug — callers should not
// invent new event types ad-hoc; extend the set here AND the Wails event
// mapping in agent_service.go in lockstep.
type EventName string

const (
	EvQueued   EventName = "queued"
	EvStarted  EventName = "started"
	EvStdout   EventName = "stdout"
	EvStderr   EventName = "stderr"
	EvFinished EventName = "finished"
	// EvSession captures the agent-side conversation id immediately
	// after EvStarted (TB-130). One per run, always preceded by EvStarted
	// so recovery's pidAlive cross-check stays meaningful.
	EvSession EventName = "session"
)

// Status is the terminal disposition of a run, written into the `finished`
// event's `status` field and (in lockstep) into the task's `AgentStatus`
// metadata.
//
// StatusInterrupted is the recovery-initiated terminal state: the daemon
// crashed mid-run with a captured SessionID. The user can choose to
// Resume the previous session. By convention nothing manual writes
// `interrupted` — only `RecoverStale` does (mirror of how nothing manual
// writes `cancelled` from outside the cancel path).
type Status string

const (
	StatusSuccess     Status = "success"
	StatusFailed      Status = "failed"
	StatusCancelled   Status = "cancelled"
	StatusInterrupted Status = "interrupted"
)

// Event is the union-shape every JSONL line decodes into. Fields are
// optional per event name; the writer encodes via json.Marshal so empty
// strings disappear from the output (omitempty). Every event carries
// `task_id` so a future log-trawler does not need a cross-file index.
//
// Schema (locked here; the documentation table in TB-47 cross-references):
//
//	queued   {ts, run_id, task_id, event:"queued",   agent, mode, resumed_from?, resumed_from_run?}
//	started  {ts, run_id, task_id, event:"started",  agent, mode, pid}
//	session  {ts, run_id, task_id, event:"session",  session_id, pid, cwd, run_env}
//	stdout   {ts, run_id, task_id, event:"stdout",   mode, line}
//	stderr   {ts, run_id, task_id, event:"stderr",   mode, line}
//	finished {ts, run_id, task_id, event:"finished", agent, mode, status, exit_code, reason?}
//
// TB-130 added SessionID/ResumedFrom/ResumedFromRun/Cwd/RunEnv for the
// resume flow. RunEnv is the on-disk allowlisted replay of the env vars
// the run was launched with — filtered to `TB_`-prefixed keys only by
// FilterTBEnv so credential vars (ANTHROPIC_API_KEY etc.) never land in
// a JSONL log file.
type Event struct {
	TS             string            `json:"ts"`
	RunID          string            `json:"run_id"`
	TaskID         string            `json:"task_id"`
	Event          EventName         `json:"event"`
	Agent          string            `json:"agent,omitempty"`
	Mode           string            `json:"mode,omitempty"`
	PID            int               `json:"pid,omitempty"`
	Line           string            `json:"line,omitempty"`
	Status         Status            `json:"status,omitempty"`
	ExitCode       int               `json:"exit_code,omitempty"`
	Reason         string            `json:"reason,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	ResumedFrom    string            `json:"resumed_from,omitempty"`
	ResumedFromRun string            `json:"resumed_from_run,omitempty"`
	Cwd            string            `json:"cwd,omitempty"`
	RunEnv         map[string]string `json:"run_env,omitempty"`
}

// FilterTBEnv reduces a `KEY=VALUE` env slice (RunInput.Env shape) to a
// map of TB-prefixed keys only. Keys without a `TB_` prefix are dropped
// on disk: JSONL log files live unencrypted, so persisting
// credential-bearing vars (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`,
// `OAUTH_*`, etc.) would leak them. Returns nil when no key qualifies so
// the `omitempty` JSON tag keeps the wire format clean.
func FilterTBEnv(env []string) map[string]string {
	var out map[string]string
	for _, kv := range env {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		key := kv[:i]
		if !strings.HasPrefix(key, "TB_") {
			continue
		}
		if out == nil {
			out = make(map[string]string, 4)
		}
		out[key] = kv[i+1:]
	}
	return out
}

// ErrBoardDirMissing is returned by AppendEvent / NewLogWriter when the
// parent board directory does not exist. We deliberately do NOT auto-create
// it — a missing board dir at this stage means a wiring bug, not a
// first-run case.
var ErrBoardDirMissing = errors.New("board directory does not exist")

// agentStateDir / agentLogsDir are the legacy board-root sub-paths for
// file-form tasks. Folder-form tasks own task-local files with the same names
// defined in docs/ARCHITECTURE.md "Folder-form tasks".
const (
	agentStateDir       = ".agent-state"
	agentLogsDir        = ".agent-logs"
	folderTaskFileName  = "TASK.md"
	folderTaskStateFile = ".agent-state.jsonl"
)

var taskStatusDirs = []string{"backlog", "in-progress", "code-review", "done", "archive"}

// ArtifactLayout names the on-disk storage form that owns a task's agent
// artifacts.
type ArtifactLayout string

const (
	ArtifactLayoutFile   ArtifactLayout = "file"
	ArtifactLayoutFolder ArtifactLayout = "folder"
)

// ArtifactPaths is the resolved state/log location for one task. File-form
// tasks keep the legacy board-root paths; folder-form tasks use task-local
// paths inside <status>/<ID>/.
type ArtifactPaths struct {
	Layout    ArtifactLayout
	StatePath string
	LogDir    string
	TaskDir   string
}

// LogPath returns the absolute log-file path for runID under the resolved
// layout. It does not check whether the file exists.
func (p ArtifactPaths) LogPath(runID string) string {
	return filepath.Join(p.LogDir, runID+".log")
}

// ResolveArtifactPaths returns the canonical state/log paths for taskID.
// Resolution follows the folder-task contract: any existing
// <status>/<ID>/TASK.md wins and owns task-local artifacts; otherwise the
// legacy board-root file-task layout is used.
func ResolveArtifactPaths(boardDir, taskID string) (ArtifactPaths, error) {
	if taskID == "" {
		return ArtifactPaths{}, errors.New("ResolveArtifactPaths: empty taskID")
	}
	if err := requireBoardDir(boardDir); err != nil {
		return ArtifactPaths{}, err
	}
	return resolveArtifactPaths(boardDir, taskID)
}

// taskMutexes serialises AppendEvent calls per task so two goroutines
// writing into the same .jsonl file never produce interleaved bytes. POSIX
// guarantees small O_APPEND writes are atomic, but JSON lines can exceed
// PIPE_BUF on agents that spew long stdout lines — the mutex is the belt
// while POSIX is the braces.
//
// One entry per task ID; never garbage-collected (the working set is
// bounded by the number of distinct tasks per process, which is small).
var taskMutexes sync.Map // map[string]*sync.Mutex

func taskMutex(taskID string) *sync.Mutex {
	if m, ok := taskMutexes.Load(taskID); ok {
		return m.(*sync.Mutex)
	}
	m, _ := taskMutexes.LoadOrStore(taskID, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// AppendEvent serialises ev as one JSON line and appends it to the task's
// resolved JSONL state file. The file is created with
// O_APPEND|O_CREATE; each write is fsync'd before close so a crash between
// `Sync` and process exit cannot lose the event.
//
// Concurrent calls for the same taskID are safe — a per-task mutex
// serialises writers; calls for different taskIDs run in parallel.
func AppendEvent(boardDir, taskID string, ev Event) error {
	if taskID == "" {
		return errors.New("AppendEvent: empty taskID")
	}
	if err := requireBoardDir(boardDir); err != nil {
		return err
	}

	paths, err := resolveArtifactPaths(boardDir, taskID)
	if err != nil {
		return fmt.Errorf("AppendEvent: resolve paths: %w", err)
	}
	stateDir := filepath.Dir(paths.StatePath)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("AppendEvent: mkdir state dir: %w", err)
	}

	// Fill in defaults the caller is likely to forget — the on-disk format
	// is the source of truth so making the writer enforce it keeps callers
	// honest.
	if ev.TaskID == "" {
		ev.TaskID = taskID
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("AppendEvent: marshal: %w", err)
	}
	line = append(line, '\n')

	mu := taskMutex(taskID)
	mu.Lock()
	defer mu.Unlock()

	path := paths.StatePath
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("AppendEvent: open %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("AppendEvent: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("AppendEvent: fsync: %w", err)
	}
	return nil
}

// requireBoardDir verifies that boardDir exists AND is a directory before
// any MkdirAll-on-a-subpath call has a chance to silently create it. Using
// os.Open + Stat on the *handle* means the subsequent MkdirAll can race
// only with a deletion (which is acceptable — we return the OS error then),
// not with a creation that would defeat the "don't auto-create boardDir"
// contract.
func requireBoardDir(boardDir string) error {
	f, err := os.Open(boardDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrBoardDirMissing, boardDir)
		}
		return fmt.Errorf("open board dir: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat board dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrBoardDirMissing, boardDir)
	}
	return nil
}

// NewLogWriter returns an io.WriteCloser pointed at the task's resolved
// per-run log path, creating the directory if missing. Caller is responsible
// for Close() after the run completes.
//
// The log file is a separate sink from the JSONL stream so the GUI can
// re-render past-run output without parsing 10MB of JSON. Both sinks
// receive the same lines from AgentService (see TB-47 step 6).
func NewLogWriter(boardDir, taskID, runID string) (io.WriteCloser, error) {
	if taskID == "" || runID == "" {
		return nil, errors.New("NewLogWriter: empty taskID or runID")
	}
	if err := requireBoardDir(boardDir); err != nil {
		return nil, err
	}
	paths, err := resolveArtifactPaths(boardDir, taskID)
	if err != nil {
		return nil, fmt.Errorf("NewLogWriter: resolve paths: %w", err)
	}
	dir := paths.LogDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("NewLogWriter: mkdir %s: %w", dir, err)
	}
	path := paths.LogPath(runID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("NewLogWriter: open %s: %w", path, err)
	}
	return f, nil
}

// LogPath returns the canonical absolute path for a run's log file. It does
// NOT check whether the file exists; the GUI uses this for past-run links
// and renders an empty pane when the read fails. If path resolution itself
// fails, the legacy file-task path is returned as a safe display fallback.
func LogPath(boardDir, taskID, runID string) string {
	paths, err := resolveArtifactPaths(boardDir, taskID)
	if err == nil {
		return paths.LogPath(runID)
	}
	return legacyLogPath(boardDir, taskID, runID)
}

// StatePath returns the canonical absolute path for a task's JSONL run
// history. Same lifetime rules as LogPath — may not exist yet.
func StatePath(boardDir, taskID string) string {
	paths, err := resolveArtifactPaths(boardDir, taskID)
	if err == nil {
		return paths.StatePath
	}
	return legacyStatePath(boardDir, taskID)
}

func resolveArtifactPaths(boardDir, taskID string) (ArtifactPaths, error) {
	// Uppercase the ID to match the peer resolvers (resolveTaskDir,
	// findTaskFile). All production callers happen to source IDs from
	// already-normalized output (tb show --json), so this is latent — but a
	// lowercase ID slipping in here used to silently fall through every status
	// dir and synthesise legacy paths whose basename diverged from the
	// canonical uppercase task file. Normalising eliminates the parallel
	// universe.
	taskID = strings.ToUpper(strings.TrimSpace(taskID))
	for _, status := range taskStatusDirs {
		taskDir := filepath.Join(boardDir, status, taskID)
		taskPath := filepath.Join(taskDir, folderTaskFileName)
		info, err := os.Stat(taskPath)
		if err == nil {
			if info.IsDir() {
				return ArtifactPaths{}, fmt.Errorf("folder-form task %s is a directory, expected markdown file", taskPath)
			}
			return ArtifactPaths{
				Layout:    ArtifactLayoutFolder,
				StatePath: filepath.Join(taskDir, folderTaskStateFile),
				LogDir:    filepath.Join(taskDir, agentLogsDir),
				TaskDir:   taskDir,
			}, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return ArtifactPaths{}, fmt.Errorf("stat folder-form task %s: %w", taskPath, err)
		}
	}

	for _, status := range taskStatusDirs {
		taskPath := filepath.Join(boardDir, status, taskID+".md")
		info, err := os.Stat(taskPath)
		if err == nil {
			if info.IsDir() {
				return ArtifactPaths{}, fmt.Errorf("file-form task %s is a directory, expected markdown file", taskPath)
			}
			return legacyArtifactPaths(boardDir, taskID), nil
		}
		if err != nil && !os.IsNotExist(err) {
			return ArtifactPaths{}, fmt.Errorf("stat file-form task %s: %w", taskPath, err)
		}
	}

	// Preserve the pre-folder-task behavior for callers that are creating or
	// inspecting historical artifacts when the task file is absent.
	return legacyArtifactPaths(boardDir, taskID), nil
}

func legacyArtifactPaths(boardDir, taskID string) ArtifactPaths {
	return ArtifactPaths{
		Layout:    ArtifactLayoutFile,
		StatePath: legacyStatePath(boardDir, taskID),
		LogDir:    filepath.Join(boardDir, agentLogsDir, taskID),
	}
}

func legacyStatePath(boardDir, taskID string) string {
	return filepath.Join(boardDir, agentStateDir, taskID+".jsonl")
}

func legacyLogPath(boardDir, taskID, runID string) string {
	return filepath.Join(boardDir, agentLogsDir, taskID, runID+".log")
}

// GenerateRunID returns "r_<8 lowercase hex chars>" sourced from
// crypto/rand. Collisions in 32 bits of entropy across a single task's run
// history are practically impossible at the scale of one run per minute;
// across the lifetime of a board (thousands of runs) the birthday-bound is
// still < 0.001%.
func GenerateRunID() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// rand.Read on stdlib is documented to never return an error; if
		// the kernel CSPRNG is broken we have bigger problems. Fall back
		// to a deterministic suffix so the program keeps moving.
		return "r_00000000"
	}
	return "r_" + hex.EncodeToString(buf[:])
}

// GenerateSessionID returns a canonical UUIDv4 string sourced from
// crypto/rand. `claude --session-id` requires a real UUID — anything
// else is rejected silently, which is the worst possible failure mode
// in a fake-runner test. We do NOT reuse GenerateRunID (32-bit hex, not
// UUID-shaped) for this reason. See TB-130.
func GenerateSessionID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Same fallback rationale as GenerateRunID — stdlib documents
		// rand.Read as infallible; if it fails we keep moving with a
		// deterministic id (Claude will then reject it, exposing the
		// outage immediately rather than silently disabling resume).
		return "00000000-0000-4000-8000-000000000000"
	}
	// Version 4 (random) + variant 10x.
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	hex := func(b []byte) string {
		const digits = "0123456789abcdef"
		out := make([]byte, len(b)*2)
		for i, v := range b {
			out[i*2] = digits[v>>4]
			out[i*2+1] = digits[v&0x0f]
		}
		return string(out)
	}
	return hex(buf[0:4]) + "-" + hex(buf[4:6]) + "-" + hex(buf[6:8]) + "-" + hex(buf[8:10]) + "-" + hex(buf[10:16])
}
