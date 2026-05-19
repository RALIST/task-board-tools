# TB-174: GUI: auto-groom triage tasks via groom-mode daemon runs

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** auto-groom,daemon,triage,groom
**ReviewRef:** f262ec8
**Branch:** —
**Parent:** TB-172

## Goal

Add an auto-groom coordinator that queues groom-mode agent runs for backlog tasks returned by `tb triage --json` when auto-groom is enabled and a default agent is configured, then promotes successfully groomed tasks to `ready` when they pass the triage gate.

## Context

- `gui/app/board_service.go` exposes `BoardService.Triage()` over `tb triage --json`; it is cached and invalidated by watcher events from TB-70/TB-71.
- `gui/app/agent_run.go` exposes `AgentService.GroomTask(ctx, id)`, which writes a queued JSONL event with `mode=groom`; `RunQueuedAgentSync` must see that mode or daemon pickup defaults to `implement` for CLI-originated queues.
- `gui/internal/daemon` already scans and picks up tasks with `AgentStatus=queued` and non-empty `Agent`; auto-groom should reuse that lifecycle instead of creating a second runner path.
- `cli/triage.go` can flag `no priority`, `no module`, `no goal`, `no acceptance criteria`, and `auto-created by scan`, so automation needs a repeat guard for tasks that still appear in triage after one groom pass.

**Constraints / non-goals**

- Only backlog tasks returned by `BoardService.Triage()` are candidates. Do not queue in-progress, done, or archive tasks.
- Auto-groom is opt-in: if `auto_groom_enabled` is false, this task must not enqueue anything.
- A valid `default_agent` (`claude` or `codex`) is required before any automatic enqueue. When a triaged task has no `Agent`, persist `Agent=<default_agent>` before queueing; do not silently overwrite an explicit task-level agent.
- The groom agent prompt still forbids moving columns. The coordinator may move backlog -> ready with `tb ready <ID>` after a successful groom only when the task no longer appears in triage.
- Do not change task status outside the existing agent lifecycle except the explicit post-groom `tb ready` promotion. Do not bypass the existing JSONL/log/Wails event contracts.
- Prevent repeat loops: an unchanged task must not be auto-groomed over and over just because `tb triage` still reports a reason after a successful auto-groom pass.

## Acceptance Criteria

- [ ] Auto-groom runs on the same lifecycle hooks that make sense for automation: after board activation, after enabling the preference, after triage-relevant watcher invalidation (`board:reloaded` and `task:updated:<id>`), and on a deferred timer when a task's settle window expires — without adding per-card shell-outs.
- [ ] With `auto_groom_enabled=true` and `default_agent=claude|codex`, a backlog task returned by `BoardService.Triage()` and not already queued/running gets queued for a groom run: `Agent` is set from `default_agent` only when absent, `AgentStatus` becomes `queued`, JSONL/Wails queued data carries `mode=groom` plus a `triage_hash`, and daemon execution uses `GroomingDecorator` rather than the implement prompt.
- [ ] A settle window is enforced: a task is only eligible for auto-groom once `now - max(task_mtime, created_at) ≥ auto_groom_settle_minutes`. Tasks inside the window are skipped with `reason="settle"` and a single deferred rescan is armed at `eligibleAt`; any subsequent edit/attachment that bumps mtime re-arms the timer; `auto_groom_settle_minutes=0` disables the window.
- [ ] After a daemon-owned groom run finishes successfully, the coordinator re-checks `BoardService.Triage()`; if the task is no longer reported, it runs `tb ready <ID>` via `BoardService.ReadyTask` so the task lands in the canonical commitment column. On promotion failure a single `auto-groom:promote-failed` Wails event is emitted; no retry loop.
- [ ] If the task still appears in triage after a successful groom, it stays in backlog and the coordinator emits a single `auto-groom:guarded-skip` event and records a guarded skip. Subsequent scans skip silently via the `triage_hash` dedupe.
- [ ] With `auto_groom_enabled=false`, no automatic enqueue happens on activation, watcher events, or deferred ticks; the existing manual Groom button remains the only grooming entry point.
- [ ] With `default_agent=none` or an unreadable preference, no task metadata or JSONL is written. `auto-groom:needs-default-agent` is emitted **only on state transition** into the no-default state; its companion `auto-groom:default-agent-cleared` is emitted exactly once when the user fixes the setting. Scans in steady-state do not spam either event.
- [ ] Deduplication is durable: `gui/internal/agent/state.go` `Event` carries an optional `triage_hash` field written on `mode=groom` `queued`/`finished` events; `LastGroomTriageHash(boardDir, taskID)` reads the most-recent successful groom hash from `.agent-state.jsonl`. Queued/running/active tasks are skipped, duplicate watcher events do not enqueue duplicate runs, and an unchanged task with a completed auto-groom attempt for the same triage-reason fingerprint is not auto-groomed again until the task changes or the user manually clicks Groom.
- [ ] Backend tests cover startup scan, watcher-triggered scan, disabled preference, missing default agent (with edge-triggered emission asserted), state-cleared transition, settle window (skip, deferred-tick fire, re-arm on edit, opt-out at 0), task with/without existing `Agent`, queued/running skip, duplicate event coalescing, `mode=groom` preservation through daemon pickup, post-groom promotion to ready, and post-groom still-needs-triage skip.
- [ ] Verification passes: `cd gui && go test ./... -race` (include race/fake-daemon coverage if new concurrent state is introduced).

## Related Tasks

- **TB-5** — Existing daemon pickup, active-set dedup, recovery, and shutdown lifecycle.
- **TB-6** — Existing manual groom flow and triage highlighting epic.
- **TB-67** — `AgentService.GroomTask` lifecycle this should reuse.
- **TB-68** — Daemon pickup must preserve `mode=groom` when JSONL queued metadata exists.
- **TB-70** — `BoardService.Triage()` cache/invalidation source.
- **TB-88** — Triage-unavailable behavior for stale CLI binaries must remain advisory.
- **TB-173** — Provides the persisted `auto_groom_enabled` switch.
- **TB-175** — Surfaces user-visible state and fallback behavior.
- **TB-266** — Shared daemon reconciliation should use the same safe post-groom promotion rule.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited reviewref=f262ec8
- 2026-05-20: Submitted to code-review

