package app

import (
	"context"
	"fmt"
	"log/slog"
	"syscall"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

// cancelGracePeriod is how long CancelRun waits after SIGTERM before
// escalating to SIGKILL on the process group. 5 seconds matches the
// TB-48 acceptance and gives a well-behaved agent a chance to flush.
const cancelGracePeriod = 5 * time.Second

// CancelRun terminates an in-flight run and writes the cancelled record
// in the order required by TB-48 so that an inopportune GUI crash can
// never lose the cancellation. The order is:
//
//  1. Mark activeRun.Cancelled = true (under ar.mu). The post-run handler
//     in runGoroutine sees this and skips its own finished/Wails/edit
//     writes, leaving the cancel writes uncontested.
//  2. SIGTERM the leader process; wait up to cancelGracePeriod for the
//     runner goroutine to finish (Done closes). Then SIGKILL the pgid if
//     it's still alive — the leader's process group catches every child.
//  3. JSONL `finished{status: cancelled, reason: "user cancelled"}`.
//  4. Wails `agent:run-finished{status: cancelled}` so the frontend
//     updates without waiting on a disk re-read.
//  5. `tb edit --agent-status cancelled`. This is LAST: if the GUI dies
//     between step 4 and step 5, the JSONL is durable and M5's
//     stale-recovery will reconcile by setting AgentStatus: cancelled
//     rather than `failed`.
//
// Idempotent — calling CancelRun after a run completes returns
// ErrNotRunning cleanly.
func (s *AgentService) CancelRun(ctx context.Context, id string) error {
	if s.board == nil {
		return ErrNoBoard
	}
	c := s.board.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	boardDir, err := s.board.resolveBoardDir(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	ar, ok := s.active[id]
	s.mu.Unlock()
	if !ok {
		return ErrNotRunning
	}

	// Step 1 — mark before kill so the post-run handler defers to us.
	ar.markCancelled()

	// Step 2 — SIGTERM + grace + SIGKILL on pgid.
	if err := killActiveRun(ar); err != nil {
		// Even if the signal fails (process already dead), continue
		// through the rest of the protocol — the goal is to leave the
		// task in `cancelled` state regardless of the kernel's view.
		slog.Warn("agent: cancel signal failed", "task", id, "err", err)
	}

	// Step 3 — JSONL finished{cancelled}.
	if err := agent.AppendEvent(boardDir, id, agent.Event{
		TS:       time.Now().UTC().Format(time.RFC3339),
		RunID:    ar.RunID,
		TaskID:   ar.TaskID,
		Event:    agent.EvFinished,
		Status:   agent.StatusCancelled,
		ExitCode: -1,
		Reason:   "user cancelled",
	}); err != nil {
		slog.Warn("agent: cancel JSONL append failed", "task", id, "err", err)
	}

	// Step 4 — Wails emit.
	s.emit("agent:run-finished", map[string]any{
		"run_id":    ar.RunID,
		"task_id":   ar.TaskID,
		"status":    string(agent.StatusCancelled),
		"exit_code": -1,
		"reason":    "user cancelled",
	})

	// Step 5 — AgentStatus: cancelled. Done last so a crash between 4 and
	// 5 leaves the durable JSONL record for M5's stale-recovery.
	if err := c.Edit(context.Background(), id, cli.EditInput{AgentStatus: "cancelled"}); err != nil {
		// Surface the error — the JSONL is durable but the metadata
		// field is now stale. The caller can decide whether to retry.
		return fmt.Errorf("CancelRun: write AgentStatus cancelled: %w", err)
	}

	// Drop the in-flight entry — a second CancelRun should return
	// ErrNotRunning idempotently.
	s.mu.Lock()
	delete(s.active, id)
	s.mu.Unlock()
	return nil
}

// killActiveRun delivers SIGTERM and (after a grace period) SIGKILL to
// the run's process group. Reads ar fields under ar.mu so a racing
// OnStarted callback doesn't see torn values.
func killActiveRun(ar *activeRun) error {
	ar.mu.Lock()
	pid := ar.Pid
	pgid := ar.Pgid
	ar.mu.Unlock()

	// In rare cases the runner hasn't returned from cmd.Start yet (the
	// OnStarted callback hasn't fired). Cancel the runner's context too
	// so a never-started cmd doesn't sit in limbo.
	if ar.Cancel != nil {
		ar.Cancel()
	}

	if pid > 0 {
		// Step 2a — SIGTERM the leader. Allows the runner goroutine to
		// observe cmd.Wait returning, write its return, and close Done.
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			slog.Warn("agent: SIGTERM failed", "pid", pid, "err", err)
		}
	}

	// Step 2b — wait up to cancelGracePeriod for the goroutine to wrap
	// up. Done is closed by runGoroutine's deferred close in agent_run.go.
	select {
	case <-ar.Done:
		return nil
	case <-time.After(cancelGracePeriod):
	}

	if pgid > 0 {
		// Step 2c — SIGKILL the pgid (negative pid). Cascades to every
		// child the agent itself may have spawned (Setpgid=true on
		// cmd.SysProcAttr makes the agent the group leader). pgid==0
		// means the runner couldn't verify Setpgid took effect (see
		// runExternal); in that case we fall back to SIGKILL'ing the
		// leader only.
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			slog.Warn("agent: SIGKILL pgid failed", "pgid", pgid, "err", err)
		}
	} else if pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			slog.Warn("agent: SIGKILL pid fallback failed", "pid", pid, "err", err)
		}
	}
	// Wait without bound for Done — at this point the kernel has killed
	// the process, so cmd.Wait can't be far behind.
	<-ar.Done
	return nil
}
