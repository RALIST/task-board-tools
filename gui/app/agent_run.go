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
	"tools/tb-gui/internal/redact"
)

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

func runnerForMode(runner agent.Runner, mode agent.Mode, detail TaskDetail) agent.Runner {
	if mode == agent.ModeGroom {
		return agent.NewGroomingDecorator(runner, promptVarsFromDetail(detail))
	}
	return runner
}

func promptVarsFromDetail(detail TaskDetail) agent.PromptVars {
	return agent.PromptVars{
		TaskID:    detail.Metadata.ID,
		TaskTitle: detail.Metadata.Title,
		TaskBody:  detail.Body,
	}
}

func runMethodName(mode agent.Mode) string {
	if mode == agent.ModeGroom {
		return "GroomTask"
	}
	return "RunAgent"
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
//
// TB-54 narrowed s.mu to guard only active map insert/delete: the JSONL
// queued write, Wails emit, and tb edit run outside the mutex with
// rollback semantics on failure (entry is removed if a setup step errors
// out before the runner goroutine is spawned).
func (s *AgentService) RunAgent(ctx context.Context, id string) (string, error) {
	return s.startAgentRun(ctx, id, agent.ModeImplement)
}

// GroomTask kicks off a new run for the given task in grooming mode. It
// intentionally reuses the same lifecycle as RunAgent; only the queued mode
// and runner decorator differ.
func (s *AgentService) GroomTask(ctx context.Context, id string) (string, error) {
	return s.startAgentRun(ctx, id, agent.ModeGroom)
}

func (s *AgentService) startAgentRun(ctx context.Context, id string, mode agent.Mode) (string, error) {
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
	runner = runnerForMode(runner, mode, detail)
	switch detail.Metadata.AgentStatus {
	case "queued", "running":
		return "", ErrAlreadyRunning
	}

	runID := agent.GenerateRunID()
	now := time.Now().UTC().Format(time.RFC3339)

	// Pre-build activeRun outside the lock; its Done channel must exist
	// before the runner goroutine starts.
	runCtx, cancel := context.WithCancel(context.Background())
	ar := &activeRun{
		RunID:  runID,
		TaskID: id,
		Agent:  agentName,
		Mode:   mode.String(),
		Cancel: cancel,
		Done:   make(chan struct{}),
	}

	// Insert placeholder under s.mu only — the rest is I/O outside the
	// mutex. On any I/O failure before the goroutine launches we roll
	// back the map entry.
	s.mu.Lock()
	if _, busy := s.active[id]; busy {
		s.mu.Unlock()
		cancel()
		return "", ErrAlreadyRunning
	}
	s.active[id] = ar
	s.mu.Unlock()

	rollback := func() {
		s.mu.Lock()
		delete(s.active, id)
		s.mu.Unlock()
		cancel()
	}

	// Step 1 — JSONL queued.
	if err := agent.AppendEvent(boardDir, id, agent.Event{
		TS:     now,
		RunID:  runID,
		TaskID: id,
		Event:  agent.EvQueued,
		Agent:  agentName,
		Mode:   mode.String(),
	}); err != nil {
		rollback()
		return "", fmt.Errorf("%s: append queued: %w", runMethodName(mode), err)
	}

	// Step 2 — Wails queued.
	s.emit("agent:run-queued", map[string]any{
		"run_id":  runID,
		"task_id": id,
		"agent":   agentName,
		"mode":    mode.String(),
	})

	// Step 3 — AgentStatus: queued. Synchronous tb edit so a frontend
	// re-render after RunAgent's return sees the right state.
	if err := c.Edit(ctx, id, cli.EditInput{AgentStatus: "queued"}); err != nil {
		rollback()
		return "", fmt.Errorf("%s: AgentStatus queued: %w", runMethodName(mode), err)
	}

	// Step 4 — kick off the run.
	go s.runGoroutine(runCtx, runner, c, ar, boardDir, detail)

	return runID, nil
}

// RunQueuedAgentSync is the daemon-only blocking executor for a task that
// is already AgentStatus=queued (typically because RunAgent was called or
// because the CLI flipped the field externally). Unlike RunAgent it:
//
//   - accepts queued/running tasks without rejecting them,
//   - uses the caller-supplied ctx as the runner ctx parent so that
//     daemon shutdown cancellation reaches exec.CommandContext,
//   - blocks until the run reaches terminal status, returning
//     ("success" | "failed" | "cancelled", nil) on success and the
//     setup error otherwise.
//
// The function does NOT write a fresh "queued" JSONL event — the caller
// (RunAgent or the CLI) already did. It records `started` with `pid` AND
// `agent` (TB-60 needs the latter for the pidAlive cross-check), spawns
// the runner, and finalises through the same postRun / finishCancelled
// paths as the manual M4 flow.
func (s *AgentService) RunQueuedAgentSync(ctx context.Context, id string) (string, error) {
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
	if detail.Metadata.AgentStatus != "queued" {
		return "", fmt.Errorf("RunQueuedAgentSync: %q is not queued (got %q)", id, detail.Metadata.AgentStatus)
	}

	// Two queue sources to reconcile:
	//   - Drawer "Run" button via RunAgent (writes a queued JSONL event)
	//   - CLI: `tb edit X --agent-status queued` (no JSONL trail because
	//     the CLI doesn't know about the JSONL schema)
	// When findQueuedRun returns ErrNoQueuedRun, the daemon owns the
	// queued lifecycle: synthesise a fresh run_id + JSONL queued event +
	// agent:run-queued emit so the frontend's run history surfaces it.
	runID, mode, err := findQueuedRun(boardDir, id)
	if errors.Is(err, ErrNoQueuedRun) {
		runID = agent.GenerateRunID()
		mode = agent.ModeImplement
		now := time.Now().UTC().Format(time.RFC3339)
		if err := agent.AppendEvent(boardDir, id, agent.Event{
			TS:     now,
			RunID:  runID,
			TaskID: id,
			Event:  agent.EvQueued,
			Agent:  agentName,
			Mode:   mode.String(),
		}); err != nil {
			return "", fmt.Errorf("RunQueuedAgentSync: append synthetic queued: %w", err)
		}
		s.emit("agent:run-queued", map[string]any{
			"run_id":  runID,
			"task_id": id,
			"agent":   agentName,
			"mode":    mode.String(),
		})
	} else if err != nil {
		return "", fmt.Errorf("RunQueuedAgentSync: find queued run: %w", err)
	}
	runner = runnerForMode(runner, mode, detail)

	// Do not derive the runner context directly from ctx: if daemon
	// shutdown closes ctx first, the runner can return context.Canceled
	// before the watcher below marks the active run as cancelled. Keep the
	// cancel ordering explicit so the terminal record is cancelled, not
	// failed{context canceled}.
	runCtx, cancel := context.WithCancel(context.Background())
	ar := &activeRun{
		RunID:  runID,
		TaskID: id,
		Agent:  agentName,
		Mode:   mode.String(),
		Cancel: cancel,
		Done:   make(chan struct{}),
	}

	s.mu.Lock()
	if _, busy := s.active[id]; busy {
		s.mu.Unlock()
		cancel()
		return "", ErrAlreadyRunning
	}
	s.active[id] = ar
	s.mu.Unlock()

	// Watch the parent ctx: if it cancels before the runner exits (i.e.
	// daemon shutdown), mark the run as cancelled BEFORE the runner
	// returns so postRun defers to finishCancelled.
	ctxCancelled := make(chan struct{})
	// Defer close so even a panic in runGoroutine releases the watcher
	// goroutine. Without this, an unexpected panic in the run body
	// would leak the ctx-watcher goroutine waiting on ctx.Done().
	defer close(ctxCancelled)
	go func() {
		select {
		case <-ctx.Done():
			ar.markCancelled()
			killActiveRun(ar)
		case <-ctxCancelled:
		}
	}()

	// Block on the run. runGoroutine is the same body the M4 manual path
	// uses; it closes ar.Done when finished and calls postRun (which
	// no-ops if ar was cancelled).
	s.runGoroutine(runCtx, runner, c, ar, boardDir, detail)

	// If we got cancelled mid-flight (shutdown), record the
	// finished{cancelled} line and AgentStatus.
	if ar.wasCancelled() {
		// CancelRun may also be racing finishCancelled. The helper is
		// idempotent via ar.finishOnce.
		_ = s.finishCancelled(c, ar, boardDir, "shutdown")
		return "cancelled", nil
	}

	// Re-read AgentStatus from disk — postRun wrote it.
	final, err := s.board.GetTask(context.Background(), id)
	if err != nil {
		return "", err
	}
	return final.Metadata.AgentStatus, nil
}

// HasActiveRun reports whether AgentService is tracking an in-flight run
// for the given task. The daemon's active-set dedup (TB-55) cross-checks
// this so a manual UI run is never duplicated by the daemon.
func (s *AgentService) HasActiveRun(taskID string) bool {
	s.mu.Lock()
	_, ok := s.active[taskID]
	s.mu.Unlock()
	return ok
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

	// TB-130 Claude pre-allocation: generate a UUIDv4 BEFORE spawning so
	// the agent CLI uses the same id we record in JSONL, even if the
	// daemon crashes mid-run. Codex doesn't accept a pre-allocated id;
	// its session capture goes through the OnSessionID callback wired by
	// TB-136. ResumeAgent (TB-138) supplies SessionID itself, so a
	// resume run keeps its parent's id.
	if ar.Agent == AgentClaude && ar.SessionID == "" {
		ar.SessionID = agent.GenerateSessionID()
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

	prompt := agent.RenderPrompt(agent.PromptImplement, promptVarsFromDetail(detail))
	timeout := s.timeoutForRun()
	// Capture cwd/env in locals so the OnStarted closure can reference
	// them without observing the partially-constructed RunInput literal.
	// TB-130 session-write hook reads these; future TB-138 resume runs
	// will override projectRoot/runEnv before this point.
	projectRoot := c.Cwd()
	var runEnv []string

	in := agent.RunInput{
		TaskID:      ar.TaskID,
		Mode:        agent.Mode(ar.Mode),
		Prompt:      prompt,
		ProjectRoot: projectRoot,
		Env:         runEnv,
		SessionID:   ar.SessionID,
		// The started JSONL schema has no timeout field yet; the effective
		// deadline is carried to the runner here.
		Timeout: timeout,
		Stdout:  stdoutSink,
		Stderr:  stderrSink,
		// TB-136: codex --json emits the session id mid-stream. The
		// translator (codex_stream.go) parses it out and invokes this
		// callback exactly once per run on the stream-reader goroutine
		// — record the id on activeRun and write the matching session
		// JSONL event. For Claude this stays nil; pre-allocation
		// (TB-135) and the post-`started` write in OnStarted already
		// cover that path.
		OnSessionID: func(sessionID string) {
			if sessionID == "" {
				return
			}
			ar.mu.Lock()
			ar.SessionID = sessionID
			pid := ar.Pid
			ar.mu.Unlock()
			if err := agent.AppendEvent(boardDir, ar.TaskID, agent.Event{
				TS:        time.Now().UTC().Format(time.RFC3339),
				RunID:     ar.RunID,
				TaskID:    ar.TaskID,
				Event:     agent.EvSession,
				SessionID: sessionID,
				PID:       pid,
				Cwd:       projectRoot,
				RunEnv:    agent.FilterTBEnv(runEnv),
			}); err != nil {
				slog.Warn("agent: append session (OnSessionID) failed",
					"task", ar.TaskID, "run", ar.RunID, "err", err)
			}
		},
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
			// TB-54 schema change: `agent` is recorded on `started` so TB-60's
			// pidAlive cross-check has an unambiguous source of the expected
			// command name. Older JSONL files (pre-M5) may not have it; the
			// recovery reader falls back to the queued event's `agent`.
			if err := agent.AppendEvent(boardDir, ar.TaskID, agent.Event{
				TS:     ts,
				RunID:  ar.RunID,
				TaskID: ar.TaskID,
				Event:  agent.EvStarted,
				Agent:  ar.Agent,
				Mode:   ar.Mode,
				PID:    pid,
			}); err != nil {
				slog.Warn("agent: append started failed", "task", ar.TaskID, "run", ar.RunID, "err", err)
			}
			// TB-130: capture the agent-side session id immediately AFTER
			// `started` so recovery can rely on PID durability before any
			// session metadata appears on disk. Empty SessionID means
			// session capture is not wired for this run (Claude pre-alloc
			// lands in TB-135, Codex --json callback lands in TB-136); the
			// gate keeps TB-133 a no-op until those wires light up.
			if ar.SessionID != "" {
				if err := agent.AppendEvent(boardDir, ar.TaskID, agent.Event{
					TS:        time.Now().UTC().Format(time.RFC3339),
					RunID:     ar.RunID,
					TaskID:    ar.TaskID,
					Event:     agent.EvSession,
					SessionID: ar.SessionID,
					PID:       pid,
					Cwd:       projectRoot,
					RunEnv:    agent.FilterTBEnv(runEnv),
				}); err != nil {
					slog.Warn("agent: append session failed", "task", ar.TaskID, "run", ar.RunID, "err", err)
				}
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

// postRun writes the terminal record via the shared finishOnce-gated
// helper. The cancel-path writers (`CancelRun`, daemon shutdown) call
// the same gate so whichever caller arrives first owns the on-disk
// terminal record — the others see a no-op.
func (s *AgentService) postRun(c *cli.Client, ar *activeRun, boardDir string, res agent.RunResult, runErr error) {
	if ar.wasCancelled() {
		// A cancel path is in flight — let it own the finished record.
		// The finishOnce gate prevents a double-write even if a race
		// brings us through anyway, but skipping here also avoids
		// emitting a spurious "agent:run-finished{success}" the cancel
		// path would shadow with "cancelled" milliseconds later.
		return
	}
	status, reason, exitCode := mapRunnerOutcome(res, runErr)
	s.recordTerminal(c, ar, boardDir, agent.Status(status), reason, exitCode)
}

// recordTerminal is the one-and-only writer of the terminal JSONL
// `finished` line + Wails emit + `tb edit --agent-status …`. Gated by
// `ar.finishOnce` so any of the three callers — postRun, CancelRun
// (TB-48), daemon shutdown (TB-62) — produces exactly one record per
// activeRun. Subsequent callers observe the no-op.
//
// AgentStatus write happens LAST so a crash between the JSONL line and
// the edit leaves the durable intent for next-start recovery.
func (s *AgentService) recordTerminal(c *cli.Client, ar *activeRun, boardDir string, status agent.Status, reason string, exitCode int) {
	ar.finishOnce.Do(func() {
		ts := time.Now().UTC().Format(time.RFC3339)
		if err := agent.AppendEvent(boardDir, ar.TaskID, agent.Event{
			TS:       ts,
			RunID:    ar.RunID,
			TaskID:   ar.TaskID,
			Event:    agent.EvFinished,
			Agent:    ar.Agent,
			Mode:     ar.Mode,
			Status:   status,
			ExitCode: exitCode,
			Reason:   reason,
		}); err != nil {
			slog.Warn("agent: append finished failed", "task", ar.TaskID, "run", ar.RunID, "err", err)
		}
		s.emit("agent:run-finished", map[string]any{
			"run_id":    ar.RunID,
			"task_id":   ar.TaskID,
			"status":    string(status),
			"exit_code": exitCode,
			"reason":    reason,
			"mode":      ar.Mode,
		})
		if err := c.Edit(context.Background(), ar.TaskID, cli.EditInput{AgentStatus: string(status)}); err != nil {
			slog.Warn("agent: AgentStatus write failed", "task", ar.TaskID, "run", ar.RunID, "status", status, "err", err)
		}
		s.mu.Lock()
		delete(s.active, ar.TaskID)
		s.mu.Unlock()
	})
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
	// Mask credential-like substrings before any sink so the secret never
	// reaches disk (log file), the JSONL state, the Wails event, or the
	// GetRunLog readback. The trailing newline pattern from streamLines is
	// preserved so the log file's line framing is unchanged.
	cleanRedacted := redact.Line(clean)
	suffix := line[len(clean):]

	if l.logFile != nil {
		l.mu.Lock()
		_, _ = l.logFile.Write([]byte(cleanRedacted + suffix))
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
		Mode:   l.ar.Mode,
		Line:   cleanRedacted,
	}); err != nil {
		// Failed JSONL appends are not fatal — drop the event but keep
		// the log file going. The frontend gets the line via Wails.
		slog.Warn("agent: append line event failed", "task", l.ar.TaskID, "stream", l.stream, "err", err)
	}

	l.svc.emit("agent:run-log", map[string]any{
		"run_id":  l.ar.RunID,
		"task_id": l.ar.TaskID,
		"stream":  l.stream,
		"line":    cleanRedacted,
	})

	return len(p), nil
}
