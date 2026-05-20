package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"tools/tb-gui/internal/cli"
)

// Supported agent identifiers. The empty string and the literal "none" both
// mean "clear the field"; everything else is rejected up front so the CLI
// never sees garbage.
const (
	AgentClaude = "claude"
	AgentCodex  = "codex"
	AgentNone   = "none"
)

// ErrAgentNotSupported is returned by AssignAgent when the caller passes an
// agent name we don't know how to spawn. Mapped on the wire to a non-blocking
// toast in the GUI.
var ErrAgentNotSupported = errors.New("agent not supported (allowed: claude, codex, none)")

// ErrNoAgent is returned by RunAgent / GroomTask when the task has no agent
// assigned. The drawer hides Run/Cancel in that case, so this is the
// belt-and-braces server-side check.
var ErrNoAgent = errors.New("no agent assigned to task")

// ErrAlreadyRunning is returned by RunAgent when an active run for the task
// is already tracked in AgentService.active. The frontend disables Run when
// AgentStatus ∈ {queued, running}, so this only fires for racing callers.
var ErrAlreadyRunning = errors.New("agent run already in progress")

// ErrNotRunning is returned by CancelRun when there's no in-flight run for
// the task. Idempotent semantics — the second cancel on a finished run
// returns this cleanly.
var ErrNotRunning = errors.New("no agent run in progress")

// ErrRunIdentityMismatch is returned by CancelRun when a recovered orphan's
// PID no longer matches the agent identity recorded at recovery time.
var ErrRunIdentityMismatch = errors.New("agent run identity mismatch")

// ErrCannotResume is returned by ResumeAgent when the latest run has no
// captured session id, or when the task is not in a terminal status that
// may intentionally resume a captured session.
var ErrCannotResume = errors.New("task has no captured terminal session to resume")

// ErrNotResumable is kept as a compatibility alias for older callers that
// checked the pre-TB-252 no-session error name.
var ErrNotResumable = ErrCannotResume

// ErrNeedsUserAttention is returned by RunAgent / GroomTask / ResumeAgent
// when the task's AgentStatus is `needs-user` (TB-182). The agent stopped
// mid-run because a human decision is required; the user must read the
// task's `## User Attention` section, then clear the status via
// `tb edit --agent-status none` before any new run can start.
var ErrNeedsUserAttention = errors.New("task is waiting for user input (AgentStatus: needs-user); clear --agent-status to retry")

// Emitter is the contract AgentService needs to forward Wails events to the
// frontend. *application.App.Event satisfies it in production; tests pass an
// in-memory implementation. Defined narrowly here (Name + payload only) so
// the service is decoupled from Wails3 plumbing.
type Emitter interface {
	Emit(name string, data ...any)
}

// TimeoutProvider returns the deadline for a single agent run. Production
// wiring can late-bind SettingsService through a closure so a preference
// change is observed by the next run without reconstructing AgentService.
type TimeoutProvider func() time.Duration

// AgentService coordinates agent assignment and (in later subtasks) run
// orchestration. It composes the CLI client from BoardService for `tb edit`
// calls and a separate Emitter for Wails events. Its active-run table is
// kept here so CancelRun (TB-48) can mutate live state without reaching
// across packages.
type AgentService struct {
	board           *BoardService
	emitter         Emitter
	timeoutProvider TimeoutProvider

	// factory is the Runner selector. Nil in production (defaultRunnerFactory
	// applies); tests override via SetRunnerFactoryForTesting.
	factory runnerFactory

	// active holds in-flight runs keyed by task ID. Populated by RunAgent,
	// drained by the post-run handler or CancelRun. Guarded by mu.
	mu     sync.Mutex
	active map[string]*activeRun
}

// AgentServiceOptions configures NewAgentService.
type AgentServiceOptions struct {
	Board           *BoardService
	Emitter         Emitter
	TimeoutProvider TimeoutProvider
}

// NewAgentService returns a ready service. Emitter is allowed to be nil
// during tests; production wiring always passes the app's event bus.
func NewAgentService(opts AgentServiceOptions) *AgentService {
	return &AgentService{
		board:           opts.Board,
		emitter:         opts.Emitter,
		timeoutProvider: opts.TimeoutProvider,
		active:          make(map[string]*activeRun),
	}
}

// ServiceName satisfies the Wails service contract.
func (s *AgentService) ServiceName() string { return "AgentService" }

func (s *AgentService) timeoutForRun() time.Duration {
	if s.timeoutProvider == nil {
		return defaultAgentTimeoutDuration()
	}
	timeout := s.timeoutProvider()
	if timeout <= 0 {
		return defaultAgentTimeoutDuration()
	}
	return timeout
}

func defaultAgentTimeoutDuration() time.Duration {
	return time.Duration(AgentTimeoutMinutesDefault) * time.Minute
}

// AssignAgent sets the task's `**Agent:**` metadata to the given agent name,
// or clears it when agent is "" / "none". Validation happens in the service
// (not in the CLI client) so the frontend can branch on ErrAgentNotSupported
// without parsing stderr.
//
// The mutation is delegated through `tb edit -a <agent>` (or `-a none` to
// clear). Persistence is end-to-end: after this call returns nil,
// `tb show <id>` reports the new value, watchers see a `task:updated:<id>`
// event, and a process restart preserves the assignment.
func (s *AgentService) AssignAgent(ctx context.Context, id, agent string) error {
	if s.board == nil {
		return ErrNoBoard
	}
	c := s.board.snapshot()
	if c == nil {
		return ErrNoBoard
	}

	norm, err := normalizeAgent(agent)
	if err != nil {
		return err
	}

	// `tb edit` accepts "none" verbatim to clear the field; the CLI's enum
	// validator (cli/edit.go) handles both. We pass "none" rather than ""
	// so an empty Agent field on EditInput continues to mean "skip".
	return c.Edit(ctx, id, cli.EditInput{Agent: norm})
}

// normalizeAgent maps free-form user input to a canonical CLI argument.
// Returns ErrAgentNotSupported for anything outside the allowed set.
func normalizeAgent(agent string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(agent))
	switch trimmed {
	case "", AgentNone:
		return AgentNone, nil
	case AgentClaude, AgentCodex:
		return trimmed, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrAgentNotSupported, agent)
	}
}

// --- activeRun (used by RunAgent/CancelRun in TB-47/TB-48) ---

// activeRun tracks an in-flight agent run. The mutex guards Cancelled
// against the post-run handler; the Done channel lets CancelRun wait for
// the goroutine to exit before declaring success.
//
// Field-level docs live on the embedded type; see TB-47 for the contract.
type activeRun struct {
	RunID  string
	TaskID string
	Agent  string
	Mode   string

	// BoardDir and Client capture the board that owned the run at start.
	// Board switches rebind BoardService to a different client, so terminal
	// cancellation for an old-board run must not look these up again.
	BoardDir string
	Client   *cli.Client

	// ParentMode is the originating action's mode when Mode == ModeResume
	// (TB-237): the per-mode pair we update at terminal mirrors the action
	// the resume is continuing, not "resume" itself. Empty for fresh runs.
	ParentMode string

	// Pid/Pgid are populated by the Runner's OnStarted callback after
	// cmd.Start() succeeds. Pgid is the leader's PID (matches Pid because
	// the runner sets Setpgid=true).
	Pid  int
	Pgid int

	// SessionID is the agent-side conversation id (TB-130). Set by
	// runGoroutine pre-allocate for Claude (TB-135) or filled in by the
	// Codex --json OnSessionID callback (TB-136). Empty here means
	// "session capture not wired for this run" — the post-`started`
	// session-write hook in runGoroutine no-ops when this is empty.
	SessionID string

	// Cwd / Env carry the persisted execution context for a resume run
	// (TB-138). Set by ResumeAgent from the parent run's session event;
	// empty for fresh runs (runGoroutine falls back to the CLI client's
	// cwd and the empty env). When TB-114's worktree mode is on, the
	// parent's Cwd is the worktree path; with worktrees off it's the
	// repo root.
	Cwd string
	Env []string

	// TriageHash is the durable dedupe fingerprint recorded on auto-groom
	// runs (TB-174). It is sha256 hex of the sorted triage reasons that
	// motivated the run. Manual groom runs from the drawer leave it empty;
	// the queued/finished JSONL events skip the `triage_hash` key via
	// omitempty so the on-disk format stays stable for non-auto runs.
	TriageHash string

	// Initiator records the automation owner stamped into JSONL queued events.
	// Empty means a user/manual run.
	Initiator string

	// Cancel cancels the runner's exec context. The runner converts that
	// into a single SIGTERM and exits; CancelRun escalates to SIGKILL via
	// Pgid if the process doesn't go quietly.
	Cancel context.CancelFunc

	// VerifyPID rechecks a recovered orphan's PID/agent identity immediately
	// before CancelRun signals it. Fresh runs leave this nil because this
	// process owns their os.Process handle directly.
	VerifyPID pidLivenessFunc

	// Recovered is true for orphaned processes adopted from JSONL after a
	// GUI/daemon restart. They have no runner goroutine to close Done.
	Recovered bool

	// Cancelled is set by CancelRun before it kills the process, so the
	// post-run handler can skip its own finished/Wails/AgentStatus writes.
	// Guarded by mu.
	Cancelled bool

	// Done is closed by the post-run handler (or the cancel handler) when
	// all post-run writes have completed. CancelRun waits on this so its
	// caller knows the run is fully torn down.
	Done chan struct{}

	// finishOnce gates the cancel-finish helper so a race between user
	// CancelRun and daemon shutdown does not write the JSONL finished
	// line twice. The first caller wins; the second is a no-op.
	finishOnce sync.Once

	// doneOnce ensures Done is closed exactly once across the multiple
	// goroutines that may reach the close point. For runs the
	// AgentService itself spawns, runGoroutine's defer closes Done. For
	// runs adopted by recovery (TB-176), the monitor goroutine closes
	// Done when it observes the orphaned PID exit. Both paths route
	// through closeDone so a defensive double-call is safe.
	doneOnce sync.Once

	mu sync.Mutex
}

// mark... helpers keep the locking patterns explicit at call sites.

func (r *activeRun) markCancelled() {
	r.mu.Lock()
	r.Cancelled = true
	r.mu.Unlock()
}

func (r *activeRun) wasCancelled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Cancelled
}

func (r *activeRun) verifyPIDIdentity() bool {
	r.mu.Lock()
	pid := r.Pid
	agentName := r.Agent
	verify := r.VerifyPID
	r.mu.Unlock()
	if verify == nil || pid <= 0 {
		return true
	}
	return verify(pid, agentName)
}

func (r *activeRun) isRecovered() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Recovered
}

// closeDone closes Done at most once, regardless of how many goroutines
// race here. Used by runGoroutine (deferred) and by the recovered-run
// monitor (TB-176) so a stub activeRun adopted at recovery time can
// unblock killActiveRun's wait when the orphaned PID exits.
func (r *activeRun) closeDone() {
	r.doneOnce.Do(func() { close(r.Done) })
}

// adoptRecoveredRun installs a stub activeRun for an orphaned PID that
// survived a GUI/daemon restart (TB-176). The stub carries enough
// context for `CancelRun` to signal the orphaned process group and for
// `recordTerminal` to write a per-mode-correct finished line — but no
// runner goroutine is attached, so the recovered-run monitor is the
// only producer of ar.Done close events.
//
// Idempotent: if `s.active[taskID]` already holds an entry (a previous
// adopt-or-spawn already ran), the existing entry is returned and the
// stub is dropped. The caller compares the returned pointer to the
// input to decide whether to keep its own references valid.
func (s *AgentService) adoptRecoveredRun(taskID string, stub *activeRun) *activeRun {
	if stub == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.active[taskID]; ok {
		return existing
	}
	s.active[taskID] = stub
	return stub
}

// getActiveRun returns the currently registered activeRun for taskID,
// or nil. The recovered-run monitor uses it to coordinate Done closure
// with CancelRun without re-deriving the stub.
func (s *AgentService) getActiveRun(taskID string) *activeRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active[taskID]
}

// removeActiveRun deletes the active entry for taskID only when the
// stored pointer matches ar — so a stale monitor that fires after a
// fresh RunAgent re-uses the same task does not evict the new run.
func (s *AgentService) removeActiveRun(taskID string, ar *activeRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur, ok := s.active[taskID]; ok && cur == ar {
		delete(s.active, taskID)
	}
}
