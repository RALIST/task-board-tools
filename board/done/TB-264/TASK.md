# TB-264: GUI: enqueue code-review tasks for review-mode daemon runs

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** auto-review,daemon,agent
**ReviewRef:** local-workspace-diff
**Branch:** —
**Parent:** TB-262

## Goal

Teach the daemon to enqueue eligible code-review tasks as review-mode runs when auto-review is enabled, using existing review prompt/lifecycle/cancellation/recovery paths.

## Context

- `gui/internal/daemon/daemon.go` already owns startup scan, watcher-driven enqueue, active-set dedupe, worker limits, and stale recovery.
- `gui/app/agent_run.go` already supports review mode from TB-198. Daemon pickup must preserve `mode=review` exactly as it preserves groom mode.
- `code-review` is the only input column for auto-review. Review failures return to `ready` through `tb review --fail`; review passes move to `done`.

## Constraints / Non-goals

- Depends on TB-263.
- Only `code-review` tasks are eligible.
- A task that just arrived from implementation may have generic `AgentStatus=success`; that must not block first review pickup. Use review-specific durable state/attempt metadata rather than treating generic success as "already reviewed."
- Require a concrete review target before queueing. `ReviewRef` is the machine-readable minimum; `## Review Target` is useful reviewer prose and should be passed to the prompt when present.
- Skip tasks with `AgentStatus` `queued`, `running`, `cancelled`, `interrupted`, or `needs-user`.
- Avoid duplicate review runs across activation scan, watcher bursts, and restart. Dedupe should be keyed to the reviewed target (for example task ID + ReviewRef + review-target/finding fingerprint), not only in-memory active state.
- Do not infer review pass/fail from free-form text. The review agent records the decision using managed CLI commands; TB-266 may reconcile only objective board markers.
- Do not change implement or groom candidate selection here.

## Acceptance Criteria

- [x] With auto-review disabled, daemon activation and watcher events never enqueue review runs automatically.
- [x] With auto-review enabled and `default_agent=claude|codex`, daemon activation scans `code-review` and queues eligible tasks as `mode=review` runs.
- [x] Watcher-driven updates enqueue a newly eligible code-review task once; active-set and durable status checks prevent duplicate runs from rapid file events and app restart.
- [x] A code-review task with `AgentStatus=success` from the preceding implement run is still eligible for its first review run.
- [x] A code-review task missing `ReviewRef` is not auto-reviewed; the daemon records a visible skip or `needs-user` handoff instead of launching a doomed review run. If `ReviewRef` exists but `## Review Target` prose is missing, pass the metadata as the machine target and surface the missing prose as advisory.
- [x] A code-review task already reviewed for the same ReviewRef/review target is not re-enqueued repeatedly on every watcher reload; changing ReviewRef or resubmitting after rework makes it eligible again.
- [x] Explicit task `Agent` is respected when present; otherwise the configured default agent is persisted before queueing.
- [x] Review JSONL queued/started/finished events carry `mode=review`, and `RunQueuedAgentSync` uses `ReviewDecorator`.
- [x] Tasks in `ready`, `backlog`, `in-progress`, `done`, or `archive` are skipped even if assigned and otherwise matching.
- [x] Tasks with `AgentStatus=needs-user` are skipped until the user clears the status.
- [x] Integration-style Go tests cover disabled, no-default, eligible code-review task, implement-success generic status, missing ReviewRef skip, ReviewRef-without-prose advisory, assigned-agent override, default-agent fallback, duplicate-event durable dedupe, wrong-column skip, needs-user skip, changed ReviewRef retry, and restart scan.
- [x] Verification includes `cd gui && go test ./...`.

## Review Target

local-workspace-diff: GUI auto-review coordinator and review-mode enqueue changes for TB-264.

## Review Findings

No CRITICAL issues found. No MAJOR issues found.

Verification:
- cd gui && go test ./...
- git diff --check

## Related Tasks

- **TB-262** — Parent epic.
- **TB-263** — Provides the persisted enablement setting.
- **TB-265** — Frontend feedback for queued/skipped/reviewed tasks.
- **TB-198** — Existing review-mode agent lifecycle.
- **TB-266** — Cross-stage reconciliation of objective board markers.
- **TB-272** — Managed pass flow that gives auto-review a deterministic successful terminal action.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited reviewref=local-workspace-diff
- 2026-05-21: Edited review-target
- 2026-05-21: Submitted to code-review
- 2026-05-21: Passed code review

