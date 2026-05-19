# TB-234: Daemon should not auto-pick up tasks in code-review

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** daemon,code-review,epic-tb194,regression
**Agent:** claude
**AgentStatus:** interrupted
**Branch:** —

## Goal

Gate every agent-run entry point (daemon auto-pickup, manual `RunAgent`, `ResumeAgent`) on the task's status column so reassigning an `Agent` to a `code-review` / `done` / `archive` task does not spawn an implement-mode run that regresses already-reviewed or finished work.

## Context

- This is a prerequisite for the staged autonomous flow. Auto-implement must not start implement runs from `code-review`, `done`, or `archive`, while auto-review must still be able to run review mode on `code-review`.
- Mode-specific entry points need different column gates: groom belongs to backlog/ready, implement belongs to in-progress (or the daemon-owned ready -> in-progress transition), and review belongs to code-review.
- The current daemon read model needs status awareness before auto-groom/auto-implement/auto-review candidate selectors can be trusted.

## Acceptance Criteria

- [ ] `daemon.AgentTask` carries the task's status column (string in the canonical set: `backlog`, `ready`, `in-progress`, `code-review`, `done`, `archive`). `gui/adapters.go` populates it from `Task.Status` in both `ListActive` and `GetTask` paths.
- [ ] `isReadyForDaemon` (and therefore `EnqueueIfReady`, `scanQueued`, `RescanActive`) returns false when `Status` is `code-review`, `done`, or `archive` even if `AgentStatus == "queued"` and `Agent != ""`. Document the rejected set in the function comment.
- [ ] `IsAutomationEligible` inherits the same status gate so auto-groom (TB-174) and auto-implement (TB-179) loops do not pick these tasks up either.
- [ ] `AgentService.RunAgent` (implement mode, via `startAgentRun(ctx, id, agent.ModeImplement)`) rejects tasks whose status is `backlog`, `ready`, `code-review`, `done`, or `archive` unless the caller is the auto-implement coordinator that first moves ready -> in-progress. The rejection happens before any JSONL/state mutation.
- [ ] `GroomTask` accepts backlog/ready only and rejects code-review/done/archive before any JSONL/state mutation.
- [ ] `ReviewTask` accepts code-review only and rejects backlog/ready/in-progress/done/archive before any JSONL/state mutation.
- [ ] `AgentService.ResumeAgent` rejects when status is `done` or `archive` (resume implies the run was previously running, which only happens in active columns). The existing `interrupted` AgentStatus gate is preserved.
- [ ] Errors surface cleanly to the GUI: the typed error maps to a user-visible toast / drawer message identifying the column instead of failing mid-run. No partial state is written (no JSONL `queued` event, no AgentStatus flip).
- [ ] Unit tests in `gui/internal/daemon` cover: code-review + queued + agent → not enqueued; done + queued + agent → not enqueued; backlog/ready/in-progress + queued + agent → enqueued. Existing happy-path tests still pass.
- [ ] Unit tests in `gui/app` cover: `RunAgent` against backlog/ready/code-review returns the typed error and leaves AgentStatus untouched; `RunAgent` against an in-progress task still succeeds; `GroomTask` wrong-column rejection; `ReviewTask` wrong-column rejection; `ResumeAgent` against a done task returns the typed error.
- [ ] Daemon watcher path: emit a `daemon: skip wrong column` (or similar) Info/Warn log when a queued task is skipped purely because of its status, so misuse is debuggable.
- [ ] Docs: update `docs/ARCHITECTURE.md` (daemon eligibility section) and `board/CONVENTIONS.md` agent-lifecycle table if needed to describe the column gate.
- [ ] Manual smoke: `tb assign TB-XXX claude` on a task already in `code-review` does not spawn an implement run; the GUI shows a clear refusal and the AgentStatus stays at `queued` until a human clears it (or the gate refuses earlier — pick one, document it).

## Related Tasks

- **TB-177** — Auto-implement depends on the status gate.
- **TB-179** — Candidate selector must consume the gated predicate.
- **TB-262** — Auto-review must remain allowed for code-review tasks in review mode.
- **TB-266** — Reconciliation must respect the same column gates.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited priority=P2, tags=daemon,code-review,epic-tb194,regression
- 2026-05-19: Edited agentstatus=interrupted
