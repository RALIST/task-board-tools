# TB-179: GUI: enqueue auto-implement candidates from daemon

**Type:** feature
**Priority:** P0
**Size:** M
**Module:** gui
**Tags:** auto-implement,daemon,agent
**ImplementedBy:** claude
**ImplementStatus:** success
**ReviewRef:** TB-179+TB-233 ship in next commit
**Branch:** —
**Parent:** TB-177

## Goal

Teach the GUI daemon to enqueue safe implementation-mode runs for committed `ready` tasks that match the saved auto-implement query.

## Context

- The existing daemon in `gui/internal/daemon/daemon.go` activates after `SettingsService.OpenBoard`, scans queued tasks, dedupes active work, and calls `AgentService.RunQueuedAgentSync`.
- `BoardService.Triage()` shells out to `tb triage --json`; auto-groom owns backlog tasks that still need grooming, while auto-implement starts only from `ready`.
- `AgentService` already owns implement-mode JSONL/log/Wails lifecycle in `gui/app/agent_run.go`; auto-implement should reuse that lifecycle rather than adding a parallel runner.
- Auto-implement settings and query parsing come from **TB-178**.

**Constraints / non-goals**

- Depends on **TB-178**.
- Only ready tasks are eligible. Backlog, in-progress, code-review, done, archive, and triage-reported/stale tasks are always skipped even when the query matches.
- Only tasks with blank `AgentStatus` are eligible for automatic first runs; queued, running, success, failed, cancelled, interrupted, lost, and needs-user statuses are not retried automatically.
- Use the task's assigned `Agent` when present; otherwise use the configured `default_agent` as the effective runner.
- Move the selected task to in-progress through the canonical board path before or at run start; respect existing daemon worker limits, active-run dedupe, cancellation, restart recovery, JSONL/log placement, and Wails run events.
- Respect the epic ordering gate from TB-267 before applying the review-failed priority boost from TB-233.
- Do not run groom mode and do not write task files directly from frontend code.

## Review Target

- gui/app/auto_implement.go (new): AutoImplementCoordinator parallel to AutoGroomCoordinator. ready-column candidate scan, query+triage+epic-order+active-run+blank-AgentStatus gates, TB-233 sort (priority desc, review-failed first within priority bucket, numeric id asc), canonical tb pull → RunAgent pipeline, diagnostic events on every failure path (pull-failed, run-failed, epic-order-skip, needs-default-agent, needs-query).
- gui/app/auto_implement_test.go (new): 16 tests covering disabled, no-default emit, matching enqueues, query mismatch, backlog skipped, non-blank AgentStatus skipped, assigned-agent preserved, default-agent fallback, dedupe across rapid scans, epic-order blocked, review-failed first within priority, P1 plain beats P2 review-failed, no review-failed preserves id order, Activate kicks initial scan, Deactivate stops further work, WIP-blocked pull skips with diagnostic, RunAgent active-run path skipped, Status reflects state.
- gui/adapters.go: boardActivator extended with autoImplement field; SetDefaultAgent notifies both coordinators; new NotifyAutoImplementEnabled / NotifyAutoImplementQueryChanged forwarding.
- gui/main.go: constructs AutoImplementCoordinator, plumbs into TeeEmitter chain, late-binds settings, registers as Wails service.
- gui/frontend/bindings/tools/tb-gui/app/autoimplementcoordinator.ts: regenerated bindings for Status().

## Related Tasks

- **TB-177** — parent epic.
- **TB-178** — prerequisite settings/query storage.
- **TB-180** — frontend visibility and manual controls.
- **TB-5** — existing daemon pickup/recovery/shutdown lifecycle.
- **TB-172** — sibling auto-groom epic that should stay separate from implement-mode automation.
- **TB-233** — review-failed priority boost within the eligible ready pool.
- **TB-234** — prerequisite status gate for daemon/manual implement runs.
- **TB-267** — epic child ordering gate.
- **TB-268** — review-failed handoff state clearing needed for retry eligibility.

## Acceptance Criteria

- [ ] With auto-implement disabled, daemon activation, watcher events, and board reloads never enqueue implementation runs automatically.
- [ ] With auto-implement enabled, a valid query, and `default_agent=codex` or `claude`, daemon activation scans ready tasks and enqueues exactly the tasks that match the query and have blank `AgentStatus`.
- [ ] An auto-started task is moved from ready to in-progress with the canonical pull/move path before implementation work begins; if WIP strict mode blocks the move, no runner starts and the skip is visible.
- [ ] Watcher-driven updates enqueue a newly eligible matching task once, and active-set/durable status checks prevent duplicate runs across rapid file events and app restart.
- [ ] Backlog tasks returned by `BoardService.Triage()` are skipped regardless of query match, with test coverage for a task missing acceptance criteria or module.
- [ ] Assigned-agent tasks run with their assigned agent; unassigned eligible tasks run with the default agent and emit queued/started/finished JSONL events with `mode=implement`.
- [ ] Failed, cancelled, success, interrupted, lost, needs-user, queued, and running tasks are not auto-retried unless a future task explicitly defines a retry policy; ready `review-failed` tasks with blank `AgentStatus` remain eligible.
- [ ] Integration-style Go tests use a fake runner/board to cover disabled, no-default, query mismatch, backlog skip, ready eligible task, assigned-agent, default-agent fallback, duplicate-event dedupe, WIP-blocked move, epic-order blocked task, review-failed eligible task, and restart scan behavior.
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
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited implemented-by=claude, implement-status=success, reviewref=TB-179+TB-233 ship in next commit
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited review-target
- 2026-05-20: Done

