package app

import (
	"context"
	"errors"
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
	if ar.Client != nil {
		c = ar.Client
	}
	if ar.BoardDir != "" {
		boardDir = ar.BoardDir
	}

	if !ar.verifyPIDIdentity() {
		return fmt.Errorf("%w: task %s run %s", ErrRunIdentityMismatch, id, ar.RunID)
	}

	// Step 1 — mark before kill so the post-run handler defers to us.
	ar.markCancelled()

	// Step 2 — SIGTERM + grace + SIGKILL on pgid.
	if err := killActiveRun(ar, boardDir, "user cancelled"); err != nil {
		// Even if the signal fails (process already dead), continue
		// through the rest of the protocol — the goal is to leave the
		// task in `cancelled` state regardless of the kernel's view.
		slog.Warn("agent: cancel signal failed", "task", id, "err", err)
	}

	// Steps 3-5 — JSONL finished{cancelled} + Wails emit + tb edit.
	// The shared helper guards against a racing daemon-shutdown caller
	// via ar.finishOnce; the AgentStatus write happens LAST so a crash
	// between the JSONL append and the edit leaves a durable cancel
	// intent for M5 stale-recovery to reconcile.
	if err := s.finishCancelled(c, ar, boardDir, "user cancelled"); err != nil {
		return fmt.Errorf("CancelRun: %w", err)
	}
	return nil
}

// CancelRunsForCurrentBoard terminalizes active runs that were started on the
// currently-open board. Board switching calls this before rebinding BoardService
// so old-board automation and manual runs cannot leak into the next board.
func (s *AgentService) CancelRunsForCurrentBoard(ctx context.Context, reason string) error {
	if s.board == nil {
		return nil
	}
	c := s.board.snapshot()
	if c == nil {
		return nil
	}
	boardDir, err := s.board.resolveBoardDir(ctx)
	if err != nil {
		if errors.Is(err, ErrNoBoard) {
			return nil
		}
		return err
	}
	return s.cancelRunsForBoard(c, boardDir, reason)
}

func (s *AgentService) cancelRunsForBoard(defaultClient *cli.Client, boardDir, reason string) error {
	s.mu.Lock()
	runs := make([]*activeRun, 0, len(s.active))
	for _, ar := range s.active {
		if ar.BoardDir == boardDir {
			runs = append(runs, ar)
		}
	}
	s.mu.Unlock()

	var joined error
	for _, ar := range runs {
		client := ar.Client
		if client == nil {
			client = defaultClient
		}
		if client == nil {
			joined = errors.Join(joined, ErrNoBoard)
			continue
		}
		if !ar.verifyPIDIdentity() {
			joined = errors.Join(joined, fmt.Errorf("%w: task %s run %s", ErrRunIdentityMismatch, ar.TaskID, ar.RunID))
			continue
		}
		ar.markCancelled()
		if err := killActiveRun(ar, boardDir, reason); err != nil {
			slog.Warn("agent: board-scoped cancel signal failed", "task", ar.TaskID, "run", ar.RunID, "reason", reason, "err", err)
		}
		if err := s.finishCancelled(client, ar, boardDir, reason); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// killActiveRun delivers SIGTERM and (after a grace period) SIGKILL to
// the run's process group. Reads ar fields under ar.mu so a racing
// OnStarted callback doesn't see torn values.
func killActiveRun(ar *activeRun, boardDir string, reason string) error {
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
		} else if err == nil {
			appendCleanupAudit(boardDir, ar, pid, "SIGTERM", "pid", reason)
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
		} else if err == nil {
			appendCleanupAudit(boardDir, ar, pgid, "SIGKILL", "pgid", reason)
		}
	} else if pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			slog.Warn("agent: SIGKILL pid fallback failed", "pid", pid, "err", err)
		} else if err == nil {
			appendCleanupAudit(boardDir, ar, pid, "SIGKILL", "pid", reason)
		}
	}
	if ar.isRecovered() {
		ar.closeDone()
	}
	// Wait without bound for Done — at this point the kernel has killed
	// the process, so cmd.Wait can't be far behind.
	<-ar.Done
	return nil
}

func appendCleanupAudit(boardDir string, ar *activeRun, pid int, signal, target, reason string) {
	if boardDir == "" || ar == nil || pid <= 0 {
		return
	}
	if err := agent.AppendEvent(boardDir, ar.TaskID, agent.Event{
		TS:     time.Now().UTC().Format(time.RFC3339),
		RunID:  ar.RunID,
		TaskID: ar.TaskID,
		Event:  agent.EvCleanup,
		Agent:  ar.Agent,
		Mode:   ar.Mode,
		PID:    pid,
		Signal: signal,
		Target: target,
		Reason: reason,
	}); err != nil {
		slog.Warn("agent: cleanup audit write failed", "task", ar.TaskID, "run", ar.RunID, "signal", signal, "target", target, "err", err)
	}
}
