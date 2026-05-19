# TB-262: Auto-review

**Type:** feature
**Priority:** P1
**Size:** L
**Module:** gui
**Tags:** auto-review,agent,daemon,settings,epic
**Branch:** —

## Goal

Ship an opt-in auto-review stage: when enabled, the daemon runs review-mode agents on code-review tasks and lets them pass to done or fail back to ready with findings.

## Context

- The staged autonomous flow should be independently toggleable: auto-groom handles backlog -> ready, auto-implement handles ready -> in-progress -> code-review, and auto-review handles code-review -> done or ready.
- TB-194 already shipped the code-review status, review-mode prompt, review findings section, and `tb review --fail` flow.
- Auto-review should reuse the existing review-mode `AgentService` lifecycle, JSONL/log storage, Wails run events, cancellation, stale recovery, and `ReviewDecorator`.
- Human review remains valid. This feature is an opt-in daemon stage, not a replacement for manual code review.

## Constraints / Non-goals

- Auto-review is off by default and controlled independently from auto-groom and auto-implement.
- A valid `default_agent` (`claude` or `codex`) is required before the daemon auto-queues review runs.
- Only tasks in `code-review` are candidates. `ready` tasks tagged `review-failed` are for auto-implement rework, not auto-review.
- Review-mode agents must not edit implementation files. They may only write review board state through managed review commands (`tb review --pass` / `tb review --fail`, with `tb review --findings` retained for notes/manual use).
- Do not introduce a new status or `AgentStatus` value. The kanban result is still `done` for pass, `ready` + `review-failed` for fail.

## Subtasks

- **TB-263** (M) — GUI: persist auto-review setting and controls
- **TB-264** (M) — GUI: enqueue code-review tasks for review-mode daemon runs
- **TB-265** (S) — GUI: surface auto-review state and decisions
- **TB-272** (M) — CLI: add managed review pass flow
## Acceptance Criteria

- [ ] **TB-263** is done: `auto_review_enabled` is persisted, defaulted off, exposed in Settings and a compact board-header control, and validated against the configured default agent.
- [ ] **TB-264** is done: daemon activation and watcher events enqueue eligible `code-review` tasks as `mode=review` runs through the existing agent lifecycle, without implement/groom fallback.
- [ ] **TB-265** is done: users can see auto-review enabled/disabled state, skipped reasons, active review runs, and pass/fail outcomes while still being able to review manually.
- [ ] **TB-272** is done: review agents have a managed pass command symmetrical with `tb review --fail`.
- [ ] PASS path: a review-mode agent records "no blocking findings" through the managed pass flow and moves the task to `done`; daemon bookkeeping does not re-enqueue the finished task.
- [ ] FAIL path: a review-mode agent uses `tb review --fail` so findings are visible, the task returns to `ready`, and the `review-failed` tag is present for auto-implement retry.
- [ ] Disabled path: with `auto_review_enabled=false`, tasks entering `code-review` are never auto-queued for review.
- [ ] No-default path: with auto-review enabled and no valid default agent, no task metadata/JSONL/log files are mutated and the GUI shows an actionable setup message.
- [ ] Verification for the epic includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.
- [ ] Manual test note: exercise human-only review, auto-review pass, auto-review fail, Cancel during a review run, app restart with a queued/running review, and toggling auto-review while tasks are already in code-review.

## Related Tasks

- **TB-172** — Auto-groom stage: backlog -> ready.
- **TB-177** — Auto-implement stage: ready -> in-progress -> code-review.
- **TB-194** — Existing code-review workflow and review-mode agent surface.
- **TB-266** — Cross-stage daemon reconciliation for safe missed moves.
- **TB-268** — Review-failed retry state must not block auto-implement.
- **TB-269** — Docs task for the staged autonomous workflow.
- **TB-270** — Prompt cleanup needed before broad automation.
- **TB-272** — Managed review pass command for deterministic pass semantics.

## Attachments

## Log

- 2026-05-19: Created
