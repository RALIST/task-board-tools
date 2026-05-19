# TB-244: Periodic re-recovery for stale agent runs

**Type:** improvement
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** agent,daemon,recovery,reliability
**Branch:** —

## Goal

Periodically re-run agent recovery so a task whose process dies between daemon restarts gets its `AgentStatus` reconciled without requiring a daemon restart or manual `tb edit --agent-status`.

## Acceptance Criteria

- [ ] Add a periodic recovery tick (e.g. every 60s, configurable) that iterates tasks with `AgentStatus=running` whose `RunID` is not in `AgentService.active` (i.e. not tracked by this daemon instance) and applies the same rules as `RecoveryService.RecoverStale` in `gui/app/agent_recovery.go`.
- [ ] Live PIDs are still skipped — the tick must not touch a run that the current daemon is actively streaming. The in-memory `s.active` map is the source of truth for "this daemon owns it".
- [ ] Dead PID + no `finished` event + captured `session_id` → mark `interrupted` (matches `RecoverStale`); dead PID + no `session_id` → mark `failed`.
- [ ] Emit `agent:run-finished` so any open drawer updates without a manual refresh, identical to startup-time recovery.
- [ ] Add a setting / config knob to disable the tick (default on); document it under `docs/ARCHITECTURE.md` agent recovery section.
- [ ] Unit test: simulated stale task with dead PID gets reconciled by the tick without restarting `RecoveryService`.
- [ ] `cd gui && go test ./...` passes.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance

