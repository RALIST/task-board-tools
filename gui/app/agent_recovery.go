package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
	"tools/tb-gui/internal/daemon"
)

// pidLivenessFunc is the seam tests use to substitute pidAlive without
// pulling in the daemon package's real probe. Production wiring sets
// this from gui/main.go; daemon.pidAlive is not exported (it stays in
// gui/internal/daemon).
type pidLivenessFunc func(pid int, expectedAgent string) bool

// RecoveryService implements daemon.Recovery on top of AgentService. It
// resolves the task's JSONL run history for each AgentStatus=running task,
// checks whether the most recent run has a finished event, and reconciles
// the task markdown via `tb edit --agent-status …`. File-form tasks use the
// legacy board-root agent state; folder-form tasks use task-local state.
//
// The two reconciliation outcomes:
//
//   - JSONL has finished{status: cancelled} for the latest run → set
//     AgentStatus=cancelled (TB-61 carve-out — never overwrite a
//     user-initiated cancel).
//   - JSONL has no finished event AND the started PID is dead per
//     pidAlive → append synthetic finished{failed, reason: "stale
//     after restart"} and set AgentStatus=failed.
//   - JSONL has no finished event AND the PID is alive → leave the
//     task alone. M5 does not re-attach to live runs.
type RecoveryService struct {
	board  *BoardService
	agent  *AgentService
	logger *slog.Logger
	liveFn pidLivenessFunc

	monitorMu           sync.Mutex
	monitors            map[recoveredRunKey]recoveredRunMonitor
	monitorPollInterval time.Duration
}

const recoveredRunMonitorPollIntervalDefault = time.Second
const orphanedProcessExitedReason = "orphaned process exited after restart"

// NewRecoveryService returns a Recovery wrapper.
func NewRecoveryService(board *BoardService, agentSvc *AgentService, liveFn pidLivenessFunc, logger *slog.Logger) *RecoveryService {
	if logger == nil {
		logger = slog.Default()
	}
	if liveFn == nil {
		// Default: always alive (don't recover) — production wiring
		// supplies the real probe. The conservative default avoids
		// over-recovery in tests that forget to inject.
		liveFn = func(int, string) bool { return true }
	}
	return &RecoveryService{
		board:               board,
		agent:               agentSvc,
		logger:              logger.With("component", "recovery"),
		liveFn:              liveFn,
		monitors:            make(map[recoveredRunKey]recoveredRunMonitor),
		monitorPollInterval: recoveredRunMonitorPollIntervalDefault,
	}
}

// Compile-time assertion that RecoveryService implements daemon.Recovery.
var _ daemon.Recovery = (*RecoveryService)(nil)

// RecoverStale is the entry point the daemon calls during Activate.
// Iterates every AgentStatus=running task and applies the reconciliation
// rules above.
func (r *RecoveryService) RecoverStale(ctx context.Context, boardDir string) error {
	if r.board == nil {
		return errors.New("recovery: no board service")
	}
	c := r.board.snapshot()
	if c == nil {
		return errors.New("recovery: no CLI client")
	}

	// "all" mode includes archive tasks alongside the active buckets.
	// LoadBoard (active) was scoped to backlog/in-progress/done in M2, so
	// a closed task with a dangling running run would slip past recovery.
	snap, err := r.board.LoadBoardWithMode(ctx, string(StatusModeAll))
	if err != nil {
		return fmt.Errorf("recovery: load board: %w", err)
	}

	candidates := make([]Task, 0)
	// All six status buckets are scanned. An earlier version only walked
	// Backlog/InProgress/Done, which missed tasks that the agent moved into
	// code-review (TB-194) — or ready (canonical kanban), or that the user
	// archived — while a daemon-tracked run was still in flight. After a
	// daemon restart those tasks would stay at AgentStatus=running
	// indefinitely, blocking Run/Groom in the drawer (taskHasActiveRun
	// gating) until manually cleared.
	for _, bucket := range [][]Task{snap.Backlog, snap.Ready, snap.InProgress, snap.CodeReview, snap.Done, snap.Archive} {
		for _, t := range bucket {
			if t.AgentStatus == "running" {
				candidates = append(candidates, t)
			}
		}
	}

	for _, t := range candidates {
		if err := r.recoverOne(ctx, c, boardDir, t); err != nil {
			r.logger.Warn("recovery: task failed; continuing", "task", t.ID, "err", err)
		}
	}
	return nil
}

// recoverOne reconciles a single task. The outer loop never aborts on
// per-task failures — one stale task should not block the others.
func (r *RecoveryService) recoverOne(ctx context.Context, c *cli.Client, boardDir string, t Task) error {
	latest, ok, err := readLatestRun(boardDir, t.ID)
	if err != nil {
		return fmt.Errorf("read JSONL: %w", err)
	}
	if !ok {
		r.logger.Warn("recovery: AgentStatus=running but no JSONL run; flipping to failed",
			"task", t.ID)
		return r.markFailed(ctx, c, boardDir, t, "", "running without JSONL")
	}

	// TB-61 carve-out: latest event for the latest run is finished{cancelled}.
	// Reconcile to cancelled, never failed.
	if latest.LastFinished != nil && latest.LastFinished.Status == agent.StatusCancelled {
		r.logger.Info("recovery: cancelled JSONL carve-out", "task", t.ID, "run", latest.RunID)
		if err := c.Edit(ctx, t.ID, cli.EditInput{AgentStatus: "cancelled"}); err != nil {
			return fmt.Errorf("edit cancelled: %w", err)
		}
		r.emitFinished(latest.RunID, t.ID, string(agent.StatusCancelled), latest.LastFinished.ExitCode, latest.LastFinished.Reason)
		return nil
	}

	// JSONL finished naturally (success/failed) but the .md never got
	// updated — sync the status.
	if latest.LastFinished != nil {
		status := string(latest.LastFinished.Status)
		r.logger.Info("recovery: JSONL finished but AgentStatus stale; syncing",
			"task", t.ID, "run", latest.RunID, "status", status)
		if err := c.Edit(ctx, t.ID, cli.EditInput{AgentStatus: status}); err != nil {
			return fmt.Errorf("edit %s: %w", status, err)
		}
		r.emitFinished(latest.RunID, t.ID, status, latest.LastFinished.ExitCode, latest.LastFinished.Reason)
		return nil
	}

	// No finished record. Decide via pidAlive whether the process is
	// still going.
	expectedAgent := latest.AgentName
	if expectedAgent == "" {
		// As a last resort fall back to the task's .md Agent field.
		expectedAgent = t.Agent
	}

	if latest.PID > 0 && r.liveFn(latest.PID, expectedAgent) {
		r.logger.Info("recovery: live PID detected; skipping",
			"task", t.ID, "run", latest.RunID, "pid", latest.PID, "agent", expectedAgent)
		// TB-176: install a stub activeRun so the GUI's Cancel button can
		// signal the orphaned process group. The monitor below will close
		// the stub's Done channel when the PID exits, so killActiveRun's
		// wait unblocks regardless of whether cancel or natural exit wins.
		stub := newRecoveredStubRun(r.logger, latest.RunID, t.ID, expectedAgent, latest.Mode, latest.PID)
		if r.agent != nil {
			r.agent.adoptRecoveredRun(t.ID, stub)
		}
		r.registerRecoveredRunMonitor(c, boardDir, t, latest.RunID, latest.PID, expectedAgent)
		return nil
	}

	// TB-137: dead PID + captured SessionID → `interrupted`, not
	// `failed`. The user can choose to Resume from the agent CLI's own
	// session log; falling through to `failed` would discard that
	// option. Without a SessionID resume isn't possible so the existing
	// `failed` branch (and reason) stays unchanged. The cancelled
	// carve-out above already short-circuits before we reach this
	// branch — a user-cancelled run with a SessionID stays `cancelled`.
	if latest.SessionID != "" {
		r.logger.Info("recovery: stale running with session; marking interrupted",
			"task", t.ID, "run", latest.RunID, "pid", latest.PID,
			"agent", expectedAgent, "session", latest.SessionID)
		return r.markInterrupted(ctx, c, boardDir, t, latest.RunID, latest.SessionID, "interrupted by daemon restart")
	}

	r.logger.Info("recovery: stale running; marking failed",
		"task", t.ID, "run", latest.RunID, "pid", latest.PID, "agent", expectedAgent)
	return r.markFailed(ctx, c, boardDir, t, latest.RunID, "stale after restart")
}

// markInterrupted appends synthetic finished{interrupted} JSONL for
// the given run_id and sets AgentStatus=interrupted via tb edit.
// Mirror of markFailed but with the TB-130 interrupted status — the
// user can then click Resume to continue the captured session. The
// validator (cli/task.go validAgentStatuses) was widened in TB-131
// to accept "interrupted" via the same `tb edit --agent-status` path
// every other status goes through. sessionID is included in the Wails
// emit so the frontend can display resume state without re-reading
// JSONL (TB-130 review NIT).
func (r *RecoveryService) markInterrupted(ctx context.Context, c *cli.Client, boardDir string, t Task, runID, sessionID, reason string) error {
	if runID == "" {
		runID = agent.GenerateRunID()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := agent.AppendEvent(boardDir, t.ID, agent.Event{
		TS:       now,
		RunID:    runID,
		TaskID:   t.ID,
		Event:    agent.EvFinished,
		Status:   agent.StatusInterrupted,
		ExitCode: -1,
		Reason:   reason,
	}); err != nil {
		return fmt.Errorf("append finished: %w", err)
	}
	if err := c.Edit(ctx, t.ID, cli.EditInput{AgentStatus: "interrupted"}); err != nil {
		return fmt.Errorf("edit interrupted: %w", err)
	}
	r.emitFinishedWithSession(runID, t.ID, string(agent.StatusInterrupted), -1, reason, sessionID)
	return nil
}

// emitFinishedWithSession mirrors emitFinished but adds a session_id
// to the Wails payload (TB-130). Kept as a separate helper so the
// existing markFailed call sites stay unchanged.
func (r *RecoveryService) emitFinishedWithSession(runID, taskID, status string, exitCode int, reason, sessionID string) {
	if r.agent == nil {
		return
	}
	payload := map[string]any{
		"run_id":    runID,
		"task_id":   taskID,
		"status":    status,
		"exit_code": exitCode,
		"reason":    reason,
	}
	if sessionID != "" {
		payload["session_id"] = sessionID
	}
	r.agent.emit("agent:run-finished", payload)
}

// markFailed appends synthetic finished{failed} JSONL for the given
// run_id and sets AgentStatus=failed via tb edit. Emits the same
// Wails event the normal post-run handler would so any open drawer
// updates without a manual refresh.
func (r *RecoveryService) markFailed(ctx context.Context, c *cli.Client, boardDir string, t Task, runID, reason string) error {
	// If runID is empty (no JSONL at all) we synthesise a fresh one so
	// the finished record has a stable key for the GUI.
	if runID == "" {
		runID = agent.GenerateRunID()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := agent.AppendEvent(boardDir, t.ID, agent.Event{
		TS:       now,
		RunID:    runID,
		TaskID:   t.ID,
		Event:    agent.EvFinished,
		Status:   agent.StatusFailed,
		ExitCode: -1,
		Reason:   reason,
	}); err != nil {
		return fmt.Errorf("append finished: %w", err)
	}
	if err := c.Edit(ctx, t.ID, cli.EditInput{AgentStatus: "failed"}); err != nil {
		return fmt.Errorf("edit failed: %w", err)
	}
	r.emitFinished(runID, t.ID, string(agent.StatusFailed), -1, reason)
	return nil
}

func (r *RecoveryService) emitFinished(runID, taskID, status string, exitCode int, reason string) {
	if r.agent == nil {
		return
	}
	r.agent.emit("agent:run-finished", map[string]any{
		"run_id":    runID,
		"task_id":   taskID,
		"status":    status,
		"exit_code": exitCode,
		"reason":    reason,
	})
}

// newRecoveredStubRun builds an activeRun for an orphaned PID that
// recovery decided to leave alone (TB-176). Pgid is derived from the
// PID via syscall.Getpgid so killActiveRun's SIGKILL on the negative
// pgid still cascades to grandchildren; if the lookup fails the stub
// keeps Pgid=0 and the kill path falls back to signalling the leader
// directly — losing any grandchildren the orphan may have spawned.
//
// The stub carries enough Agent/Mode context for recordTerminal to
// write a per-mode-correct finished line when CancelRun (or the
// monitor on natural exit) reaches that branch.
func newRecoveredStubRun(logger *slog.Logger, runID, taskID, agentName string, mode agent.Mode, pid int) *activeRun {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		if logger != nil {
			logger.Warn("recovery: syscall.Getpgid failed; cancel will signal leader PID only",
				"task", taskID, "run", runID, "pid", pid, "err", err)
		}
		pgid = 0
	}
	modeStr := mode.String()
	if modeStr == "" {
		modeStr = agent.ModeImplement.String()
	}
	return &activeRun{
		RunID:  runID,
		TaskID: taskID,
		Agent:  agentName,
		Mode:   modeStr,
		Pid:    pid,
		Pgid:   pgid,
		Done:   make(chan struct{}),
	}
}

type recoveredRunKey struct {
	boardDir string
	taskID   string
	runID    string
}

type recoveredRunMonitor struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

func (r *RecoveryService) registerRecoveredRunMonitor(c *cli.Client, boardDir string, t Task, runID string, pid int, expectedAgent string) {
	if c == nil || boardDir == "" || t.ID == "" || runID == "" || pid <= 0 {
		return
	}
	key := recoveredRunKey{boardDir: boardDir, taskID: t.ID, runID: runID}

	r.monitorMu.Lock()
	if r.monitors == nil {
		r.monitors = make(map[recoveredRunKey]recoveredRunMonitor)
	}
	if _, exists := r.monitors[key]; exists {
		r.monitorMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.monitors[key] = recoveredRunMonitor{cancel: cancel, done: done}
	interval := r.monitorPollInterval
	if interval <= 0 {
		interval = recoveredRunMonitorPollIntervalDefault
	}
	r.monitorMu.Unlock()

	go func() {
		defer close(done)
		r.monitorRecoveredRun(ctx, key, c, t, pid, expectedAgent, interval)
	}()
}

func (r *RecoveryService) monitorRecoveredRun(ctx context.Context, key recoveredRunKey, c *cli.Client, t Task, pid int, expectedAgent string, interval time.Duration) {
	defer r.unregisterRecoveredRunMonitor(key)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			done, err := r.pollRecoveredRunMonitor(ctx, c, key.boardDir, t, key.runID, pid, expectedAgent)
			if err != nil {
				r.logger.Warn("recovery: recovered-run monitor poll failed; retrying",
					"task", key.taskID, "run", key.runID, "err", err)
				continue
			}
			if done {
				return
			}
		}
	}
}

func (r *RecoveryService) unregisterRecoveredRunMonitor(key recoveredRunKey) {
	r.monitorMu.Lock()
	delete(r.monitors, key)
	r.monitorMu.Unlock()
}

func (r *RecoveryService) stopRecoveredRunMonitors() {
	r.monitorMu.Lock()
	monitors := make([]recoveredRunMonitor, 0, len(r.monitors))
	for _, monitor := range r.monitors {
		monitors = append(monitors, monitor)
		monitor.cancel()
	}
	r.monitorMu.Unlock()

	for _, monitor := range monitors {
		<-monitor.done
	}
}

func (r *RecoveryService) pollRecoveredRunMonitor(ctx context.Context, c *cli.Client, boardDir string, t Task, runID string, pid int, expectedAgent string) (bool, error) {
	latest, ok, err := readLatestRun(boardDir, t.ID)
	if err != nil {
		return false, fmt.Errorf("read JSONL: %w", err)
	}
	if !ok {
		return true, r.markFailed(ctx, c, boardDir, t, runID, orphanedProcessExitedReason)
	}
	if latest.RunID != runID {
		// A different run has surfaced for this task while a stub was
		// installed for runID. RunAgent's ErrAlreadyRunning gate should
		// prevent this; if it ever fires, it indicates someone is
		// mutating JSONL out-of-band or the gate was bypassed. Log
		// loudly so the anomaly is visible in production logs.
		r.logger.Info("recovery: monitor observed run id change; stopping",
			"task", t.ID, "stub_run", runID, "latest_run", latest.RunID)
		return true, nil
	}
	if latest.LastFinished != nil {
		return true, r.syncFinishedStatus(ctx, c, t, latest)
	}

	probePID := latest.PID
	if probePID <= 0 {
		probePID = pid
	}
	probeAgent := latest.AgentName
	if probeAgent == "" {
		probeAgent = expectedAgent
	}
	if probePID > 0 && r.liveFn(probePID, probeAgent) {
		return false, nil
	}
	return true, r.reconcileOrphanExit(ctx, c, boardDir, t, runID)
}

// reconcileOrphanExit closes out the stub activeRun (if one is
// registered) and writes the terminal record for an orphaned PID that
// has just been observed dead. Routing through ar.finishOnce when a
// stub exists keeps the cancel/exit race honest: whichever path
// arrives first owns the terminal line, and the other observes a
// no-op. With no stub (legacy path / adopt-failed) we still call
// markFailed so behaviour matches the pre-TB-176 contract.
func (r *RecoveryService) reconcileOrphanExit(ctx context.Context, c *cli.Client, boardDir string, t Task, runID string) error {
	var ar *activeRun
	if r.agent != nil {
		ar = r.agent.getActiveRun(t.ID)
	}
	if ar == nil || ar.RunID != runID {
		// No stub (or it belongs to a newer run). Preserve the legacy
		// markFailed path that pre-TB-176 callers relied on.
		return r.markFailed(ctx, c, boardDir, t, runID, orphanedProcessExitedReason)
	}

	// Unblock any CancelRun waiter on Done first. If CancelRun owns the
	// terminal record we then short-circuit — recordTerminal's finishOnce
	// gate would no-op anyway, but skipping the call avoids a stray
	// Wails emit when the cancel path's own finishCancelled is about to
	// fire one with the correct status. removeActiveRun drops the entry
	// only when it still matches our stub.
	ar.closeDone()
	if ar.wasCancelled() {
		// CancelRun goroutine is in flight and will call finishCancelled;
		// nothing left for the monitor to write here.
		r.logger.Info("recovery: monitor observed cancelled orphan; deferring terminal to CancelRun",
			"task", t.ID, "run", runID, "pid", ar.Pid)
		return nil
	}
	if r.agent != nil {
		// recordTerminal deletes s.active inside its finishOnce body,
		// so this is the producer-of-terminal-record path. The follow-
		// up removeActiveRun is a defensive no-op for the case where a
		// future divergence between recordTerminal and this branch
		// stops deleting; today both call sites agree.
		r.agent.recordTerminal(c, ar, boardDir, agent.StatusFailed, orphanedProcessExitedReason, -1)
		r.agent.removeActiveRun(t.ID, ar)
	}
	return nil
}

func (r *RecoveryService) syncFinishedStatus(ctx context.Context, c *cli.Client, t Task, latest runRecoveryView) error {
	if latest.LastFinished == nil {
		return nil
	}
	status := string(latest.LastFinished.Status)
	if err := c.Edit(ctx, t.ID, cli.EditInput{AgentStatus: status}); err != nil {
		return fmt.Errorf("edit %s: %w", status, err)
	}
	r.emitFinished(latest.RunID, t.ID, status, latest.LastFinished.ExitCode, latest.LastFinished.Reason)
	return nil
}

// runRecoveryView is the slice of run state recovery cares about. The
// fields are derived from the JSONL events for the latest run_id.
type runRecoveryView struct {
	RunID     string
	AgentName string
	PID       int
	SessionID string
	Cwd       string
	RunEnv    map[string]string
	// Mode is the run's mode parsed from the queued JSONL event
	// (TB-237). Empty when the queued event predates mode capture.
	Mode         agent.Mode
	LastFinished *finishedEvent
}

type finishedEvent struct {
	Status   agent.Status
	ExitCode int
	Reason   string
}

// readLatestRun parses the task's resolved agent state JSONL and returns the
// latest run's reconciled view. `ok==false` when the file is missing or empty.
//
// "Latest" here is the last run_id observed in the file in order. We
// don't sort by timestamp because the writer is append-only and the
// JSONL is the authoritative order. The latest run's `queued` event
// supplies `agent`, the `started` event supplies `pid` (and from M5
// onwards also `agent` — see TB-54); the `finished` event (if any) is
// the terminal record.
func readLatestRun(boardDir, taskID string) (runRecoveryView, bool, error) {
	path := agent.StatePath(boardDir, taskID)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return runRecoveryView{}, false, nil
		}
		return runRecoveryView{}, false, err
	}
	defer f.Close()

	views := map[string]*runRecoveryView{}
	order := []string{}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var ev agent.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.RunID == "" {
			continue
		}
		v, ok := views[ev.RunID]
		if !ok {
			v = &runRecoveryView{RunID: ev.RunID}
			views[ev.RunID] = v
			order = append(order, ev.RunID)
		}
		switch ev.Event {
		case agent.EvQueued:
			if ev.Agent != "" {
				v.AgentName = ev.Agent
			}
			if ev.Mode != "" {
				v.Mode = parseRunMode(ev.Mode)
			}
		case agent.EvStarted:
			if ev.PID > 0 {
				v.PID = ev.PID
			}
			if ev.Agent != "" {
				v.AgentName = ev.Agent
			}
		case agent.EvSession:
			// EvSession is emitted exactly once per run, AFTER EvStarted —
			// it carries the agent-side conversation id, the live PID, and
			// the cwd / TB_ env replay context that ResumeAgent needs to
			// re-launch in the original execution environment.
			if ev.SessionID != "" {
				v.SessionID = ev.SessionID
			}
			if ev.PID > 0 {
				v.PID = ev.PID
			}
			if ev.Cwd != "" {
				v.Cwd = ev.Cwd
			}
			if ev.RunEnv != nil {
				v.RunEnv = ev.RunEnv
			}
		case agent.EvFinished:
			v.LastFinished = &finishedEvent{
				Status:   ev.Status,
				ExitCode: ev.ExitCode,
				Reason:   ev.Reason,
			}
		}
	}

	if len(order) == 0 {
		return runRecoveryView{}, false, nil
	}
	latest := views[order[len(order)-1]]
	return *latest, true, nil
}

// ResumeCandidate is the resolved execution context for resuming an
// interrupted agent run. RunID is what the GUI surfaces as the
// `resumed_from` chip on the new run; SessionID is what the agent CLI
// needs (`claude -r <uuid>` / `codex exec --json resume <uuid>`).
type ResumeCandidate struct {
	SessionID string
	RunID     string
	Cwd       string
	Env       map[string]string
	// Mode is the parent run's originating mode (groom/implement/review).
	// TB-237 uses it so the resume's per-mode pair update lands on the
	// originating action rather than on a non-existent "resume" slot.
	// Defaults to ModeImplement when the parent JSONL is too old to carry it.
	Mode agent.Mode
}

// resumableSessionID returns the resume context for the *latest* run of
// taskID. The lookup is intentionally one-deep — if the most recent run
// failed before capturing a SessionID, resume is disabled and the helper
// returns ok=false. Walking backward to an older successful run would
// resume a stale conversation; the design's "no recursive resume" rule
// (spec § 5) keeps the contract simple and predictable.
//
// Returns ok=false (no error) when:
//   - the JSONL file does not exist yet,
//   - the file exists but holds no runs, or
//   - the latest run has no `session` event.
//
// Returns a non-nil error only on unexpected IO failure.
func resumableSessionID(boardDir, taskID string) (ResumeCandidate, bool, error) {
	view, ok, err := readLatestRun(boardDir, taskID)
	if err != nil {
		return ResumeCandidate{}, false, err
	}
	if !ok {
		return ResumeCandidate{}, false, nil
	}
	if view.SessionID == "" {
		return ResumeCandidate{}, false, nil
	}
	parentMode := view.Mode
	if parentMode == "" {
		parentMode = agent.ModeImplement
	}
	return ResumeCandidate{
		SessionID: view.SessionID,
		RunID:     view.RunID,
		Cwd:       view.Cwd,
		Env:       view.RunEnv,
		Mode:      parentMode,
	}, true, nil
}
