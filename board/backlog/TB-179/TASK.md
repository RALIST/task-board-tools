# TB-179: GUI: enqueue auto-implement candidates from daemon

**Type:** feature
**Priority:** P0
**Size:** M
**Module:** gui
**Tags:** auto-implement,daemon,agent
**Branch:** —
**Parent:** TB-177

## Goal

Teach the GUI daemon to enqueue safe implementation-mode runs for groomed backlog tasks that match the saved auto-implement query.

## Context

- The existing daemon in `gui/internal/daemon/daemon.go` activates after `SettingsService.OpenBoard`, scans queued tasks, dedupes active work, and calls `AgentService.RunQueuedAgentSync`.
- `BoardService.Triage()` shells out to `tb triage --json`; tasks present in that map are not groomed and must never be auto-implemented.
- `AgentService` already owns implement-mode JSONL/log/Wails lifecycle in `gui/app/agent_run.go`; auto-implement should reuse that lifecycle rather than adding a parallel runner.
- Auto-implement settings and query parsing come from **TB-178**.

**Constraints / non-goals**

- Depends on **TB-178**.
- Only backlog tasks are eligible. In-progress, done, archive, and triage-reported tasks are always skipped even when the query matches.
- Only tasks with blank `AgentStatus` are eligible for automatic first runs; queued, running, success, failed, and cancelled statuses are not retried automatically.
- Use the task's assigned `Agent` when present; otherwise use the configured `default_agent` as the effective runner.
- Respect existing daemon worker limits, active-run dedupe, cancellation, restart recovery, JSONL/log placement, and Wails run events.
- Do not run groom mode and do not write task files directly from frontend code.

## Related Tasks

- **TB-177** — parent epic.
- **TB-178** — prerequisite settings/query storage.
- **TB-180** — frontend visibility and manual controls.
- **TB-5** — existing daemon pickup/recovery/shutdown lifecycle.
- **TB-172** — sibling auto-groom epic that should stay separate from implement-mode automation.

## Acceptance Criteria

- [ ] With auto-implement disabled, daemon activation, watcher events, and board reloads never enqueue implementation runs automatically.
- [ ] With auto-implement enabled, a valid query, and `default_agent=codex` or `claude`, daemon activation scans backlog tasks and enqueues exactly the groomed tasks that match the query and have blank `AgentStatus`.
- [ ] Watcher-driven updates enqueue a newly eligible matching task once, and active-set/durable status checks prevent duplicate runs across rapid file events and app restart.
- [ ] Tasks returned by `BoardService.Triage()` are skipped regardless of query match, with test coverage for a task missing acceptance criteria or module.
- [ ] Assigned-agent tasks run with their assigned agent; unassigned eligible tasks run with the default agent and emit queued/started/finished JSONL events with `mode=implement`.
- [ ] Failed, cancelled, success, queued, and running tasks are not auto-retried unless a future task explicitly defines a retry policy.
- [ ] Integration-style Go tests use a fake runner/board to cover disabled, no-default, query mismatch, ungroomed skip, assigned-agent, default-agent fallback, duplicate-event dedupe, and restart scan behavior.
- [ ] Verification includes `cd gui && go test ./...`.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-19: Moved to in-progress
- 2026-05-19: Moved to backlog
- 2026-05-19: Moved to code-review
- 2026-05-19: Moved to done
- 2026-05-19: Moved to backlog

