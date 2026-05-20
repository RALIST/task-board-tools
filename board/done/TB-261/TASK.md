# TB-261: Safely clean up orphaned agent processes

**Type:** improvement
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** agent,daemon,recovery,process-safety
**Agent:** codex
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewedBy:** codex
**ReviewStatus:** success
**ReviewRef:** ce50c51
**Branch:** —

## Goal

Provide an explicit, safe cleanup path for orphaned `claude`/`codex` processes using JSONL ownership, PID identity checks, and board state; never kill agent processes by age alone.

## Context

- The app already tracks daemon-owned runs through task-local or board-root `.agent-state*.jsonl` events, including `started` PID, agent name, mode, and terminal `finished` events.
- Periodic/stale recovery distinguishes live vs dead runs and preserves user-cancel / interrupted semantics. A process cleanup feature must build on that ownership trail rather than scanning arbitrary old `claude`/`codex` processes.
- Related active reliability work exists in TB-242, TB-244, and TB-253. This task should either merge into that recovery work or stay as a small explicit cleanup command/UI.

## Constraints / Non-goals

- Never kill by age alone.
- Never kill a `claude`/`codex` process unless it is tied to a known task/run through JSONL ownership and PID identity matches the expected agent command.
- Preserve user intent: `cancelled` is user-initiated, `interrupted` is recovery/resume-related, and `needs-user` is a handoff state.
- Do not kill processes for tasks in `done`/`archive` unless the run has no terminal JSONL event and the ownership checks still prove the process belongs to that task.
- Avoid duplicate cleanup surfaces if TB-242/TB-244/TB-253 already solve the process ownership problem.

## Acceptance Criteria

- [x] Audited the neighboring recovery/process scope from TB-242, TB-244, and TB-253; this task stayed separate for recovered live-run cancellation/cleanup audit behavior, not a global process-age scanner.
- [x] Cleanup candidates are discovered from recovered agent JSONL/run state only.
- [x] Recovered live-run cancellation rechecks PID liveness and expected command identity before terminating.
- [x] Processes with a terminal `finished` event are released from the recovered stub and are not killed.
- [x] User cancellation uses the existing `CancelRun`/daemon shutdown semantics; no second cancellation meaning was introduced.
- [x] Recovery-interrupted resumable runs are not killed automatically; cleanup happens only when the user cancels a recovered live run.
- [x] Cleanup actions write auditable JSONL `cleanup` events naming task ID, run ID, PID, signal, target, and reason before the terminal cancellation event.
- [x] Tests cover owned stale process cleanup, command-name mismatch ignored, terminal-event process ignored, done-task cleanup audit, folder-form cleanup, and cancellation preservation.
- [x] Verification passed: `cd gui && go test ./...`.

## Related Tasks

- **TB-242** — Agent runner stdout/process reliability.
- **TB-244** — Periodic re-recovery for stale agent runs.
- **TB-253** — Run history stale/concurrent running state.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Committed — moved to ready
- 2026-05-20: Edited agent=codex
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=codex, review-status=success, reviewref=ce50c51, acceptance
- 2026-05-20: Done
