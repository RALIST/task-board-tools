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
)

// Status is the terminal disposition of a run, written into the `finished`
// event's `status` field and (in lockstep) into the task's `AgentStatus`
// metadata.
type Status string

const (
	StatusSuccess   Status = "success"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Event is the union-shape every JSONL line decodes into. Fields are
// optional per event name; the writer encodes via json.Marshal so empty
// strings disappear from the output (omitempty). Every event carries
// `task_id` so a future log-trawler does not need a cross-file index.
//
// Schema (locked here; the documentation table in TB-47 cross-references):
//
//	queued   {ts, run_id, task_id, event:"queued",   agent, mode}
//	started  {ts, run_id, task_id, event:"started",  agent, mode, pid}
//	stdout   {ts, run_id, task_id, event:"stdout",   mode, line}
//	stderr   {ts, run_id, task_id, event:"stderr",   mode, line}
//	finished {ts, run_id, task_id, event:"finished", agent, mode, status, exit_code, reason?}
type Event struct {
	TS       string    `json:"ts"`
	RunID    string    `json:"run_id"`
	TaskID   string    `json:"task_id"`
	Event    EventName `json:"event"`
	Agent    string    `json:"agent,omitempty"`
	Mode     string    `json:"mode,omitempty"`
	PID      int       `json:"pid,omitempty"`
	Line     string    `json:"line,omitempty"`
	Status   Status    `json:"status,omitempty"`
	ExitCode int       `json:"exit_code,omitempty"`
	Reason   string    `json:"reason,omitempty"`
}

// ErrBoardDirMissing is returned by AppendEvent / NewLogWriter when the
// parent board directory does not exist. We deliberately do NOT auto-create
// it — a missing board dir at this stage means a wiring bug, not a
// first-run case.
var ErrBoardDirMissing = errors.New("board directory does not exist")

// agentStateDir / agentLogsDir are the sub-paths under boardDir that the
// agent subsystem owns. The CLI never touches them (see
// docs/ARCHITECTURE.md "On-disk layout").
const (
	agentStateDir = ".agent-state"
	agentLogsDir  = ".agent-logs"
)

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

// AppendEvent serialises ev as one JSON line and appends it to
// `<boardDir>/.agent-state/<taskID>.jsonl`. The file is created with
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

	stateDir := filepath.Join(boardDir, agentStateDir)
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

	path := filepath.Join(stateDir, taskID+".jsonl")
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

// NewLogWriter returns an io.WriteCloser pointed at
// `<boardDir>/.agent-logs/<taskID>/<runID>.log`, creating the directory if
// missing. Caller is responsible for Close() after the run completes.
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
	dir := filepath.Join(boardDir, agentLogsDir, taskID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("NewLogWriter: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, runID+".log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("NewLogWriter: open %s: %w", path, err)
	}
	return f, nil
}

// LogPath returns the canonical absolute path for a run's log file. It does
// NOT check whether the file exists; the GUI uses this for past-run links
// and renders an empty pane when the read fails.
func LogPath(boardDir, taskID, runID string) string {
	return filepath.Join(boardDir, agentLogsDir, taskID, runID+".log")
}

// StatePath returns the canonical absolute path for a task's JSONL run
// history. Same lifetime rules as LogPath — may not exist yet.
func StatePath(boardDir, taskID string) string {
	return filepath.Join(boardDir, agentStateDir, taskID+".jsonl")
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
