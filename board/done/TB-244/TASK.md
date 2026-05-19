# TB-244: Periodic re-recovery for stale agent runs

**Type:** improvement
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** agent,daemon,recovery,reliability
**Agent:** codex
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**Branch:** —

## Goal

Periodically re-run agent recovery so a task whose process dies between daemon restarts gets its `AgentStatus` reconciled without requiring a daemon restart or manual `tb edit --agent-status`.

## Acceptance Criteria

- [x] Add a periodic recovery tick (60s default, test/config override) that iterates tasks with `AgentStatus=running` whose `RunID` is not in `AgentService.active` and applies the same rules as `RecoveryService.RecoverStale`.
- [x] Live PIDs are still skipped — the tick does not touch a run that the current daemon is actively streaming. The in-memory `AgentService.active` map is the source of truth for "this daemon owns it".
- [x] Dead PID + no `finished` event + captured `session_id` → mark `interrupted`; dead PID + no `session_id` → mark `failed`.
- [x] Emit `agent:run-finished` so any open drawer updates without a manual refresh, identical to startup-time recovery.
- [x] Add a setting / config knob to disable the tick (default on); documented under `docs/ARCHITECTURE.md` agent recovery section and applied to the live daemon immediately.
- [x] Unit test: simulated stale task with dead PID gets reconciled by the tick without restarting `RecoveryService`.
- [x] `cd gui && go test ./...` passes.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Implementation complete — daemon now runs a default-on stale recovery ticker using `RecoverStaleUntracked`, skips currently-owned active runs, emits terminal events, and exposes a live settings toggle.
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-19: Done

