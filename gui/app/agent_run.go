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
	switch mode {
	case agent.ModeGroom:
		return agent.NewGroomingDecorator(runner, promptVarsFromDetail(detail))
	case agent.ModeReview:
		return agent.NewReviewDecorator(runner, promptVarsFromDetail(detail))
	case agent.ModeResume:
		return agent.NewResumeDecorator(runner)
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

// effectiveMode returns the mode that the per-mode attribution pair should
// target for ar's run. For a fresh run that's just ar.Mode; for a resume
// run it's the parent's originating mode (TB-237: resume updates the
// originating action's pair, never a fourth slot).
func effectiveMode(ar *activeRun) agent.Mode {
	m := agent.Mode(ar.Mode)
	if m == agent.ModeResume {
		if pm := agent.Mode(ar.ParentMode); pm != "" && pm != agent.ModeResume {
			return pm
		}
		return agent.ModeImplement
	}
	return m
}

// applyPerModeAttribution copies the (agent, status) pair into the
// per-mode attribution fields on edit that match mode. The legacy
// AgentStatus / Agent fields are not touched here — callers set those
// directly. A run mode outside the three kanban actions is a no-op so a
// future mode addition cannot accidentally overwrite an unrelated pair.
//
// Each per-mode pair reflects the most recent terminal state for THAT
// action — same "latest wins" semantics as the legacy pair, just scoped.
// So a cancelled resume of a previously-successful implement writes
// implement-status=cancelled; the per-mode pair is not a "last-success"
// sticky. needs-user is the one exception: recordTerminal's carve-out
// gates this call on shouldWriteStatus, so an agent-set needs-user is
// preserved on AgentStatus and the per-mode pair retains its prior
// terminal value (the per-mode enum has no needs-user slot by design).
func applyPerModeAttribution(edit *cli.EditInput, mode agent.Mode, agentName, status string) {
	switch mode {
	case agent.ModeGroom:
		edit.GroomedBy = agentName
		edit.GroomStatus = status
	case agent.ModeImplement:
		edit.ImplementedBy = agentName
		edit.ImplementStatus = status
	case agent.ModeReview:
		edit.ReviewedBy = agentName
		edit.ReviewStatus = status
	}
}

func runMethodName(mode agent.Mode) string {
	switch mode {
	case agent.ModeGroom:
		return "GroomTask"
	case agent.ModeReview:
		return "ReviewTask"
	case agent.ModeResume:
		return "ResumeAgent"
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
	return s.startAgentRun(ctx, id, agent.ModeImplement, "")
}

// GroomTask kicks off a new run for the given task in grooming mode. It
// intentionally reuses the same lifecycle as RunAgent; only the queued mode
// and runner decorator differ. Manual groom runs from the drawer record no
// triage hash; use StartGroomWithTriageHash for auto-groom paths that need
// the durable dedupe fingerprint persisted on the run's JSONL events.
func (s *AgentService) GroomTask(ctx context.Context, id string) (string, error) {
	return s.startAgentRun(ctx, id, agent.ModeGroom, "")
}

// StartGroomWithTriageHash is the auto-groom entry point (TB-174): it
// queues a `mode=groom` run AND attaches the provided triage hash to the
// `queued`/`finished` JSONL events so a subsequent scan can dedupe an
// unchanged task across daemon restarts. Empty hash falls back to the
// manual GroomTask semantics; the AC mandates a real hash from the
// coordinator, so callers should pre-compute it.
func (s *AgentService) StartGroomWithTriageHash(ctx context.Context, id, triageHash string) (string, error) {
	return s.startAgentRun(ctx, id, agent.ModeGroom, triageHash)
}

// ReviewTask kicks off a code-review run for the given task. Same lifecycle
// as RunAgent / GroomTask; the review prompt instructs the agent to read the
// implementation referenced by `## Review Target`, write actionable findings
// via `tb review --findings`, and use `tb review --fail` when rework is
// required. Review runs do NOT edit implementation files.
func (s *AgentService) ReviewTask(ctx context.Context, id string) (string, error) {
	return s.startAgentRun(ctx, id, agent.ModeReview, "")
}

// ResumeAgent continues the most recent interrupted agent session for
// the task (TB-130). Reads the persisted SessionID + cwd + env from the
// parent run's JSONL session event, validates that the task is in
// `interrupted` status (resume from finished runs is documented as a
// follow-up, NOT M1 scope), then launches a new run via the same
// runGoroutine pipeline used by RunAgent. Returns ErrNotResumable when
// the latest run has no captured session id; ErrCannotResume when the
// task's AgentStatus is not `interrupted`.
//
// The queued JSONL event for a resume run carries `resumed_from`
// (parent session id) and `resumed_from_run` (parent RunID) so the
// frontend's "resumed from r_xxxx" chip has a target. For Claude the
// runner appends `-r <uuid>` so the agent CLI reuses the same session;
// for Codex the runner uses `codex exec --json resume <uuid> <prompt>`
// and a new id flows through the OnSessionID callback (TB-139).
func (s *AgentService) ResumeAgent(ctx context.Context, id string) (string, error) {
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
	if detail.Metadata.AgentStatus == "needs-user" {
		// TB-182: don't silently resume into a task that needs user input.
		return "", ErrNeedsUserAttention
	}
	if detail.Metadata.AgentStatus != "interrupted" {
		return "", fmt.Errorf("%w: AgentStatus is %q (want \"interrupted\")",
			ErrCannotResume, detail.Metadata.AgentStatus)
	}
	candidate, ok, err := resumableSessionID(boardDir, id)
	if err != nil {
		return "", fmt.Errorf("ResumeAgent: resumableSessionID: %w", err)
	}
	if !ok {
		return "", ErrNotResumable
	}

	agentName := strings.ToLower(strings.TrimSpace(detail.Metadata.Agent))
	if agentName == "" {
		return "", ErrNoAgent
	}
	runner, err := s.runnerFor(agentName)
	if err != nil {
		return "", err
	}
	runner = runnerForMode(runner, agent.ModeResume, detail)

	runID := agent.GenerateRunID()
	now := time.Now().UTC().Format(time.RFC3339)

	runCtx, cancel := context.WithCancel(context.Background())
	envSlice := envSliceFromMap(candidate.Env)
	parentMode := candidate.Mode
	if parentMode == "" || parentMode == agent.ModeResume {
		// "No recursive resume" (TB-130 spec §5): a chained resume should
		// never appear in practice, but guard defensively so the per-mode
		// update still lands on a real action slot.
		parentMode = agent.ModeImplement
	}
	ar := &activeRun{
		RunID:      runID,
		TaskID:     id,
		Agent:      agentName,
		Mode:       agent.ModeResume.String(),
		ParentMode: parentMode.String(),
		Cancel:     cancel,
		Done:       make(chan struct{}),
		SessionID:  candidate.SessionID,
		Cwd:        candidate.Cwd,
		Env:        envSlice,
	}

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

	// queued JSONL — carries the resume linkage so the UI chip + future
	// recovery can trace the chain.
	if err := agent.AppendEvent(boardDir, id, agent.Event{
		TS:             now,
		RunID:          runID,
		TaskID:         id,
		Event:          agent.EvQueued,
		Agent:          agentName,
		Mode:           agent.ModeResume.String(),
		ResumedFrom:    candidate.SessionID,
		ResumedFromRun: candidate.RunID,
	}); err != nil {
		rollback()
		return "", fmt.Errorf("ResumeAgent: append queued: %w", err)
	}

	s.emit("agent:run-queued", map[string]any{
		"run_id":           runID,
		"task_id":          id,
		"agent":            agentName,
		"mode":             agent.ModeResume.String(),
		"resumed_from":     candidate.SessionID,
		"resumed_from_run": candidate.RunID,
	})

	if err := c.Edit(ctx, id, cli.EditInput{AgentStatus: "queued"}); err != nil {
		rollback()
		return "", fmt.Errorf("ResumeAgent: AgentStatus queued: %w", err)
	}

	go s.runGoroutine(runCtx, runner, c, ar, boardDir, detail)
	return runID, nil
}

// envSliceFromMap converts a TB_-prefixed env map (as persisted in the
// session JSONL event) into the KEY=VALUE slice RunInput.Env / exec.Cmd
// Env expect.
func envSliceFromMap(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

func (s *AgentService) startAgentRun(ctx context.Context, id string, mode agent.Mode, triageHash string) (string, error) {
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
	case "needs-user":
		// TB-182: the previous agent run stopped because the task needs a
		// user decision. The `## User Attention` section captures the ask;
		// users resolve it via `tb edit --agent-status none` before a new
		// run can start.
		return "", ErrNeedsUserAttention
	}

	runID := agent.GenerateRunID()
	now := time.Now().UTC().Format(time.RFC3339)

	// Pre-build activeRun outside the lock; its Done channel must exist
	// before the runner goroutine starts.
	runCtx, cancel := context.WithCancel(context.Background())
	ar := &activeRun{
		RunID:      runID,
		TaskID:     id,
		Agent:      agentName,
		Mode:       mode.String(),
		Cancel:     cancel,
		Done:       make(chan struct{}),
		TriageHash: triageHash,
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

	// Step 1 — JSONL queued. TriageHash is set only by the auto-groom
	// coordinator path (TB-174); manual groom/implement/review runs leave
	// it empty so the on-disk format stays stable via omitempty.
	if err := agent.AppendEvent(boardDir, id, agent.Event{
		TS:         now,
		RunID:      runID,
		TaskID:     id,
		Event:      agent.EvQueued,
		Agent:      agentName,
		Mode:       mode.String(),
		TriageHash: triageHash,
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
	qr, err := findQueuedRun(boardDir, id)
	if errors.Is(err, ErrNoQueuedRun) {
		qr = queuedRun{RunID: agent.GenerateRunID(), Mode: agent.ModeImplement}
		now := time.Now().UTC().Format(time.RFC3339)
		if err := agent.AppendEvent(boardDir, id, agent.Event{
			TS:     now,
			RunID:  qr.RunID,
			TaskID: id,
			Event:  agent.EvQueued,
			Agent:  agentName,
			Mode:   qr.Mode.String(),
		}); err != nil {
			return "", fmt.Errorf("RunQueuedAgentSync: append synthetic queued: %w", err)
		}
		s.emit("agent:run-queued", map[string]any{
			"run_id":  qr.RunID,
			"task_id": id,
			"agent":   agentName,
			"mode":    qr.Mode.String(),
		})
	} else if err != nil {
		return "", fmt.Errorf("RunQueuedAgentSync: find queued run: %w", err)
	}
	runner = runnerForMode(runner, qr.Mode, detail)

	// TB-130 BLOCKER fix: a daemon replay of a `mode:"resume"` queued run
	// MUST rehydrate the parent session id + cwd + env, otherwise the
	// runner would silently launch as a fresh implement (claude gets no
	// `-r <uuid>`, codex gets `exec --json <prompt>` without resume).
	// Resume turns into "start over" with no error surfaced to the user.
	var (
		resumeSessionID string
		resumeCwd       string
		resumeEnv       []string
		parentMode      agent.Mode
	)
	if qr.Mode == agent.ModeResume {
		if qr.ResumedFrom == "" || qr.ResumedFromRun == "" {
			return "", fmt.Errorf("RunQueuedAgentSync: resume queued event missing resume linkage (run=%s)", qr.RunID)
		}
		cwd, envMap, ok := runSessionContext(boardDir, id, qr.ResumedFromRun)
		if !ok {
			return "", fmt.Errorf("RunQueuedAgentSync: parent run %s has no session event — cannot rehydrate resume", qr.ResumedFromRun)
		}
		resumeSessionID = qr.ResumedFrom
		resumeCwd = cwd
		resumeEnv = envSliceFromMap(envMap)
		// TB-237: look up the parent run's originating mode so the per-mode
		// pair update at terminal lands on the right action. Fall back to
		// ModeImplement when the parent JSONL is too old to carry mode.
		if m, ok := runModeFor(boardDir, id, qr.ResumedFromRun); ok {
			parentMode = m
		}
		if parentMode == "" || parentMode == agent.ModeResume {
			parentMode = agent.ModeImplement
		}
	}

	// Do not derive the runner context directly from ctx: if daemon
	// shutdown closes ctx first, the runner can return context.Canceled
	// before the watcher below marks the active run as cancelled. Keep the
	// cancel ordering explicit so the terminal record is cancelled, not
	// failed{context canceled}.
	runCtx, cancel := context.WithCancel(context.Background())
	ar := &activeRun{
		RunID:      qr.RunID,
		TaskID:     id,
		Agent:      agentName,
		Mode:       qr.Mode.String(),
		ParentMode: parentMode.String(),
		Cancel:     cancel,
		Done:       make(chan struct{}),
		SessionID:  resumeSessionID,
		Cwd:        resumeCwd,
		Env:        resumeEnv,
		TriageHash: qr.TriageHash,
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

// hasActiveRunID reports whether AgentService is tracking the specific
// (taskID, runID) pair. ListRuns uses this to distinguish a "detached"
// queued/running JSONL entry (no goroutine attached) from one that
// AgentService is actively managing.
func (s *AgentService) hasActiveRunID(taskID, runID string) bool {
	s.mu.Lock()
	ar, ok := s.active[taskID]
	s.mu.Unlock()
	return ok && ar != nil && ar.RunID == runID
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
	defer ar.closeDone()

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
	// TB-130 session-write hook reads these. ResumeAgent (TB-138)
	// populates ar.Cwd / ar.Env from the parent run's session event so
	// the resume launches in the original execution context — critical
	// for Claude's cwd-keyed session lookup and for TB-114 worktrees.
	projectRoot := c.Cwd()
	if ar.Cwd != "" {
		projectRoot = ar.Cwd
	}
	var runEnv []string
	if len(ar.Env) > 0 {
		runEnv = append(runEnv, ar.Env...)
	}

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
//
// TB-182 carve-out: if the running agent set AgentStatus to `needs-user`
// mid-run (via `tb edit --agent-status needs-user`), we must NOT overwrite
// that with the exit-mapped status. The JSONL `finished` line still gets
// written so run history is intact; the AgentStatus stays `needs-user`
// until the user clears it through the resolution flow.
func (s *AgentService) recordTerminal(c *cli.Client, ar *activeRun, boardDir string, status agent.Status, reason string, exitCode int) {
	ar.finishOnce.Do(func() {
		ts := time.Now().UTC().Format(time.RFC3339)
		if err := agent.AppendEvent(boardDir, ar.TaskID, agent.Event{
			TS:         ts,
			RunID:      ar.RunID,
			TaskID:     ar.TaskID,
			Event:      agent.EvFinished,
			Agent:      ar.Agent,
			Mode:       ar.Mode,
			Status:     status,
			ExitCode:   exitCode,
			Reason:     reason,
			TriageHash: ar.TriageHash,
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

		// Preserve an agent-set `needs-user` over the exit-mapped status.
		// We re-read AgentStatus from disk via BoardService.GetTask so the
		// check sees whatever the in-flight agent wrote through tb edit.
		//
		// Scope: only the exit-mapped statuses (success / failed) are
		// gated. User-explicit `cancelled` and recovery-driven `interrupted`
		// still write through — explicit human or recovery intent wins
		// over an agent's needs-user marker.
		shouldWriteStatus := true
		if (status == agent.StatusSuccess || status == agent.StatusFailed) && s.board != nil {
			if latest, err := s.board.GetTask(context.Background(), ar.TaskID); err == nil {
				if latest.Metadata.AgentStatus == "needs-user" {
					shouldWriteStatus = false
					slog.Info("agent: preserving needs-user AgentStatus over exit status",
						"task", ar.TaskID, "run", ar.RunID, "exitStatus", status)
				}
			} else {
				slog.Warn("agent: needs-user carve-out: GetTask failed; falling back to writing exit status",
					"task", ar.TaskID, "run", ar.RunID, "err", err)
			}
		}
		if shouldWriteStatus {
			// TB-237: the per-mode pair is written under the same gate as
			// the legacy AgentStatus so the needs-user carve-out also
			// covers per-mode (matches AC "needs-user stays a single-
			// cursor status … no per-mode needs-user fields").
			edit := cli.EditInput{AgentStatus: string(status)}
			applyPerModeAttribution(&edit, effectiveMode(ar), ar.Agent, string(status))
			if err := c.Edit(context.Background(), ar.TaskID, edit); err != nil {
				slog.Warn("agent: AgentStatus write failed", "task", ar.TaskID, "run", ar.RunID, "status", status, "err", err)
			}
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
