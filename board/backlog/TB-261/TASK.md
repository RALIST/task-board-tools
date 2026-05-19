# TB-261: Safely clean up orphaned agent processes

**Type:** improvement
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** agent,daemon,recovery,process-safety
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

- [ ] Audit existing recovery/process tracking in TB-242, TB-244, and TB-253; either merge this scope into one of them or document why a separate cleanup command/UI is needed.
- [ ] Cleanup candidates are discovered only from agent JSONL state, not from a global process-age scan.
- [ ] Before terminating, the implementation verifies PID liveness and command identity using the existing `pidAlive`/expected-agent style checks.
- [ ] Processes with a terminal `finished` event are not killed unless a separate ownership bug proves a child process was orphaned from that run.
- [ ] User-cancelled runs use the existing CancelRun/daemon shutdown semantics; this task does not invent a second cancellation meaning.
- [ ] Recovery-interrupted resumable runs are not killed automatically if the process is still alive and owned; the UI should prefer resume/cancel choices.
- [ ] Any cleanup action writes an auditable JSONL/log event naming task ID, run ID, PID, signal, reason, and whether the process group or single PID was targeted.
- [ ] Tests or manual smoke cover: owned stale process killed, unrelated old `claude`/`codex` process ignored, command-name mismatch ignored, done-task orphan handling, and cancelled/interrupted preservation.
- [ ] Verification includes `cd gui && go test ./...` if implemented in GUI/daemon code.

## Related Tasks

- **TB-242** — Agent runner stdout/process reliability.
- **TB-244** — Periodic re-recovery for stale agent runs.
- **TB-253** — Run history stale/concurrent running state.

## Attachments

## Log

- 2026-05-19: Created
