package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
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
	board   *BoardService
	agent   *AgentService
	logger  *slog.Logger
	liveFn  pidLivenessFunc
}

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
		board:  board,
		agent:  agentSvc,
		logger: logger.With("component", "recovery"),
		liveFn: liveFn,
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

	snap, err := r.board.LoadBoard(ctx)
	if err != nil {
		return fmt.Errorf("recovery: load board: %w", err)
	}

	candidates := make([]Task, 0)
	for _, bucket := range [][]Task{snap.Backlog, snap.InProgress, snap.Done} {
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
		return nil
	}

	r.logger.Info("recovery: stale running; marking failed",
		"task", t.ID, "run", latest.RunID, "pid", latest.PID, "agent", expectedAgent)
	return r.markFailed(ctx, c, boardDir, t, latest.RunID, "stale after restart")
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

// runRecoveryView is the slice of run state recovery cares about. The
// fields are derived from the JSONL events for the latest run_id.
type runRecoveryView struct {
	RunID        string
	AgentName    string
	PID          int
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
		case agent.EvStarted:
			if ev.PID > 0 {
				v.PID = ev.PID
			}
			if ev.Agent != "" {
				v.AgentName = ev.Agent
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
