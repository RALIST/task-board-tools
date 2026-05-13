package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

// agentTimeoutDefault is the M4 default deadline for a single run. The
// architecture doc names 30 minutes as the working timeout; this is a const
// rather than a setting so M4 stays focused and M7 can plumb it through
// SettingsService when the time comes.
const agentTimeoutDefault = 30 * time.Minute

// agentBinary is the strategy point where AgentService picks a Runner for
// a given agent name. Lives here (not on the struct) so tests can swap it.
//
// Keeping the factory in one place also means a future GroomingDecorator
// (M6) gets injected uniformly across both runners.
var defaultRunnerFactory = func(name string) (agent.Runner, error) {
	switch name {
	case AgentClaude:
		return agent.NewClaudeRunner(), nil
	case AgentCodex:
		return agent.NewCodexRunner(), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrAgentNotSupported, name)
	}
}

// runnerFactory is the seam tests override. Tests substitute a stub
// implementation here to avoid spawning real `claude` / `codex` processes.
type runnerFactory func(name string) (agent.Runner, error)

// runnerFor selects the Runner used by the current AgentService. Defaults
// to defaultRunnerFactory; tests override via SetRunnerFactoryForTesting.
func (s *AgentService) runnerFor(name string) (agent.Runner, error) {
	f := s.factory
	if f == nil {
		f = defaultRunnerFactory
	}
	return f(name)
}

// setRunnerFactory swaps the Runner factory. Unexported so the Wails
// binding generator doesn't surface it to the frontend; tests reach it
// via the test-only setRunnerFactoryForTest helper.
func (s *AgentService) setRunnerFactory(f runnerFactory) {
	s.factory = f
}

// RunAgent kicks off a new run for the given task. See TB-47 for the
// step-by-step contract; the short version:
//
//	(synchronous)
//	1. Validate state (agent assigned, no run in progress)
//	2. Append JSONL queued + emit Wails agent:run-queued
//	3. Set AgentStatus: queued, then running (two tb edit calls)
//	4. Register activeRun under s.mu and return run_id
//	(goroutine)
//	5. Spawn Runner with OnStarted callback (sync inside the runner)
//	6. Stream stdout/stderr to JSONL + log file + Wails events
//	7. Post-run handler: writes finished record unless Cancelled was set
//	   by TB-48; closes activeRun.Done either way
func (s *AgentService) RunAgent(ctx context.Context, id string) (string, error) {
	if s.board == nil {
		return "", ErrNoBoard
	}
	c := s.board.snapshot()
	if c == nil {
		return "", ErrNoBoard
	}
	boardDir, err := s.board.resolveBoardDir(ctx)
	if err != nil {
		return "", err
	}

	detail, err := s.board.GetTask(ctx, id)
	if err != nil {
		return "", err
	}
	agentName := strings.ToLower(strings.TrimSpace(detail.Metadata.Agent))
	if agentName == "" {
		return "", ErrNoAgent
	}
	runner, err := s.runnerFor(agentName)
	if err != nil {
		return "", err
	}
	switch detail.Metadata.AgentStatus {
	case "queued", "running":
		return "", ErrAlreadyRunning
	}

	// Don't allow two concurrent in-flight maps for the same task. This
	// also covers the gap between the AgentStatus check above and the map
	// insert below — two RunAgent goroutines racing for the same task.
	s.mu.Lock()
	if _, busy := s.active[id]; busy {
		s.mu.Unlock()
		return "", ErrAlreadyRunning
	}

	runID := agent.GenerateRunID()
	now := time.Now().UTC().Format(time.RFC3339)

	// Step 1 — JSONL queued. Done while holding s.mu so the in-flight
	// table observation and the JSONL event stay in causal order.
	if err := agent.AppendEvent(boardDir, id, agent.Event{
		TS:     now,
		RunID:  runID,
		TaskID: id,
		Event:  agent.EvQueued,
		Agent:  agentName,
		Mode:   agent.ModeImplement.String(),
	}); err != nil {
		s.mu.Unlock()
		return "", fmt.Errorf("RunAgent: append queued: %w", err)
	}

	// Step 2 — Wails queued.
	s.emit("agent:run-queued", map[string]any{
		"run_id":  runID,
		"task_id": id,
		"agent":   agentName,
		"mode":    string(agent.ModeImplement),
	})

	// Step 3 — AgentStatus: queued. Synchronous tb edit so a frontend
	// re-render after RunAgent's return sees the right state. In M4 the
	// gap between queued and running is ~10ms; M5 widens it once the
	// daemon owns the spawn.
	if err := c.Edit(ctx, id, cli.EditInput{AgentStatus: "queued"}); err != nil {
		s.mu.Unlock()
		return "", fmt.Errorf("RunAgent: AgentStatus queued: %w", err)
	}

	// Step 4 — register activeRun. The context lives until the runner
	// goroutine's post-run handler runs to completion.
	runCtx, cancel := context.WithCancel(context.Background())
	ar := &activeRun{
		RunID:  runID,
		TaskID: id,
		Agent:  agentName,
		Mode:   agent.ModeImplement.String(),
		Cancel: cancel,
		Done:   make(chan struct{}),
	}
	s.active[id] = ar
	s.mu.Unlock()

	// Step 4b — kick off the run.
	go s.runGoroutine(runCtx, runner, c, ar, boardDir, detail)

	return runID, nil
}

// runGoroutine owns steps 5–7 of the lifecycle. It is invoked from
// RunAgent and never directly.
//
// Note on cancellation order: the AgentStatus: running write happens inside
// OnStarted (after cmd.Start succeeds), not here at the top of the
// goroutine. That removes the race where CancelRun could fire between this
// goroutine starting and the running write, and the running write would
// then overwrite the cancelled write that CancelRun is about to do. By
// gating on OnStarted we guarantee the process actually started before any
// running write hits disk; OnStarted itself also short-circuits if
// Cancelled is already set (cancel-before-start).
func (s *AgentService) runGoroutine(ctx context.Context, runner agent.Runner, c *cli.Client, ar *activeRun, boardDir string, detail TaskDetail) {
	defer close(ar.Done)

	if ar.wasCancelled() {
		// Cancel fired between RunAgent's return and now — don't even
		// spawn. The cancel handler will close Done via our defer and
		// owns all the cancel-path writes.
		return
	}

	// Open the per-run log file. The writer is the third fan-out of every
	// agent line (alongside JSONL + Wails); if the log file fails to open,
	// the run continues — the JSONL stream is the source of truth.
	logWriter, logErr := agent.NewLogWriter(boardDir, ar.TaskID, ar.RunID)
	if logErr != nil {
		slog.Warn("agent: open log file failed; continuing without log file", "task", ar.TaskID, "run", ar.RunID, "err", logErr)
	}
	defer func() {
		if logWriter != nil {
			_ = logWriter.Close()
		}
	}()

	// Wrap stdout/stderr so every line fans out to (JSONL, log file, Wails).
	stdoutSink := s.newLineSink(boardDir, ar, logWriter, "stdout")
	stderrSink := s.newLineSink(boardDir, ar, logWriter, "stderr")

	prompt := agent.RenderPrompt(agent.PromptImplement, agent.PromptVars{
		TaskID:    detail.Metadata.ID,
		TaskTitle: detail.Metadata.Title,
		TaskBody:  detail.Body,
	})

	in := agent.RunInput{
		TaskID:      ar.TaskID,
		Mode:        agent.ModeImplement,
		Prompt:      prompt,
		ProjectRoot: c.Cwd(),
		Timeout:     agentTimeoutDefault,
		Stdout:      stdoutSink,
		Stderr:      stderrSink,
		OnStarted: func(pid, pgid int) {
			// Hold ar.mu across the cancelled-check and the pid/pgid
			// write so a racing CancelRun observes a consistent activeRun
			// before reading Pid/Pgid for its kill cascade.
			ar.mu.Lock()
			cancelled := ar.Cancelled
			ar.Pid = pid
			ar.Pgid = pgid
			ar.mu.Unlock()
			if cancelled {
				// Cancel fired before the process started. Don't write
				// `running` (it would race the cancelled write); cancel
				// path owns the AgentStatus, JSONL, and Wails events.
				// We still recorded Pid/Pgid so killActiveRun can deliver
				// SIGTERM to the now-running leader.
				return
			}

			// JSONL started + Wails started + AgentStatus running.
			// AgentStatus is written here (rather than at the top of
			// runGoroutine) so it tracks the moment the process actually
			// started, AND so cancel-before-start can never lose the
			// race against this write.
			if err := c.Edit(context.Background(), ar.TaskID, cli.EditInput{AgentStatus: "running"}); err != nil {
				slog.Warn("agent: AgentStatus running failed", "task", ar.TaskID, "run", ar.RunID, "err", err)
			}
			ts := time.Now().UTC().Format(time.RFC3339)
			if err := agent.AppendEvent(boardDir, ar.TaskID, agent.Event{
				TS:     ts,
				RunID:  ar.RunID,
				TaskID: ar.TaskID,
				Event:  agent.EvStarted,
				PID:    pid,
			}); err != nil {
				slog.Warn("agent: append started failed", "task", ar.TaskID, "run", ar.RunID, "err", err)
			}
			s.emit("agent:run-started", map[string]any{
				"run_id":  ar.RunID,
				"task_id": ar.TaskID,
				"agent":   ar.Agent,
				"mode":    ar.Mode,
				"pid":     pid,
			})
		},
	}

	res, runErr := runner.Run(ctx, in)
	s.postRun(c, ar, boardDir, res, runErr)
}

// postRun writes the terminal record unless CancelRun has flipped
// activeRun.Cancelled — TB-48 owns the cancel-path writes.
func (s *AgentService) postRun(c *cli.Client, ar *activeRun, boardDir string, res agent.RunResult, runErr error) {
	if ar.wasCancelled() {
		// TB-48 has already written (or is about to write) the cancelled
		// record. Leave the in-flight entry alone — CancelRun removes it
		// after its own writes succeed.
		return
	}

	status, reason, exitCode := mapRunnerOutcome(res, runErr)

	ts := time.Now().UTC().Format(time.RFC3339)
	if err := agent.AppendEvent(boardDir, ar.TaskID, agent.Event{
		TS:       ts,
		RunID:    ar.RunID,
		TaskID:   ar.TaskID,
		Event:    agent.EvFinished,
		Status:   agent.Status(status),
		ExitCode: exitCode,
		Reason:   reason,
	}); err != nil {
		slog.Warn("agent: append finished failed", "task", ar.TaskID, "run", ar.RunID, "err", err)
	}
	s.emit("agent:run-finished", map[string]any{
		"run_id":    ar.RunID,
		"task_id":   ar.TaskID,
		"status":    status,
		"exit_code": exitCode,
		"reason":    reason,
	})

	if err := c.Edit(context.Background(), ar.TaskID, cli.EditInput{AgentStatus: status}); err != nil {
		slog.Warn("agent: AgentStatus write failed", "task", ar.TaskID, "run", ar.RunID, "status", status, "err", err)
	}

	s.mu.Lock()
	delete(s.active, ar.TaskID)
	s.mu.Unlock()
}

// mapRunnerOutcome implements the error → status mapping table from TB-47.
// Keeping it as a pure function makes the cancel/timeout/binary-not-found
// branches separately testable.
func mapRunnerOutcome(res agent.RunResult, runErr error) (status, reason string, exitCode int) {
	switch {
	case runErr == nil && res.ExitCode == 0:
		return "success", "", 0
	case runErr == nil && res.ExitCode != 0:
		return "failed", "non-zero exit", res.ExitCode
	case errors.Is(runErr, agent.ErrBinaryNotFound):
		return "failed", "binary not found", -1
	case errors.Is(runErr, agent.ErrTimeout):
		return "failed", "timeout", -1
	case errors.Is(runErr, context.Canceled):
		// Should be intercepted by ar.wasCancelled() before this gets
		// called, but if some other path produces a context.Canceled the
		// safest record is "failed" with the reason surfaced.
		return "failed", runErr.Error(), -1
	default:
		return "failed", runErr.Error(), -1
	}
}

// emit is the Wails event fan-out. nil-safe so tests without an Emitter
// don't blow up.
func (s *AgentService) emit(name string, payload any) {
	if s.emitter == nil {
		return
	}
	s.emitter.Emit(name, payload)
}

// --- line-by-line sink (used for both stdout and stderr) ---

// lineSink is the io.Writer the Runner streams into. The runner's
// streamLines helper writes one full line per Write call (followed by
// '\n'), so lineSink can treat each Write as a single event.
type lineSink struct {
	svc      *AgentService
	boardDir string
	ar       *activeRun
	logFile  io.Writer
	stream   string // "stdout" / "stderr"
	mu       sync.Mutex
}

func (s *AgentService) newLineSink(boardDir string, ar *activeRun, logFile io.Writer, stream string) *lineSink {
	return &lineSink{
		svc:      s,
		boardDir: boardDir,
		ar:       ar,
		logFile:  logFile,
		stream:   stream,
	}
}

func (l *lineSink) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	line := string(p)
	// streamLines passes a trailing '\n' on every line; strip for events
	// but keep for the log file.
	clean := strings.TrimRight(line, "\n")

	if l.logFile != nil {
		l.mu.Lock()
		_, _ = l.logFile.Write(p)
		l.mu.Unlock()
	}

	ev := agent.EvStdout
	if l.stream == "stderr" {
		ev = agent.EvStderr
	}
	if err := agent.AppendEvent(l.boardDir, l.ar.TaskID, agent.Event{
		TS:     time.Now().UTC().Format(time.RFC3339),
		RunID:  l.ar.RunID,
		TaskID: l.ar.TaskID,
		Event:  ev,
		Line:   clean,
	}); err != nil {
		// Failed JSONL appends are not fatal — drop the event but keep
		// the log file going. The frontend gets the line via Wails.
		slog.Warn("agent: append line event failed", "task", l.ar.TaskID, "stream", l.stream, "err", err)
	}

	l.svc.emit("agent:run-log", map[string]any{
		"run_id":  l.ar.RunID,
		"task_id": l.ar.TaskID,
		"stream":  l.stream,
		"line":    clean,
	})

	return len(p), nil
}
