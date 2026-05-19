# TB-268: Review-failed handoff clears retry-blocking agent state

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** workflow
**Tags:** code-review,review-failed,automation,agent-status
**ImplementedBy:** claude
**ImplementStatus:** success
**ReviewRef:** TB-268 ships in next commit
**Branch:** —
**Parent:** TB-177

## Goal

Ensure a failed review that returns to ready with findings does not leave AgentStatus success/failed behind in a way that blocks auto-implement retry pickup.

## Context

- Auto-implement eligibility in TB-179/TB-233 expects retryable ready tasks to have blank `AgentStatus`.
- Review-mode agents can successfully execute `tb review --fail`, which moves the task to ready and tags `review-failed`, then the runner may still record terminal `AgentStatus=success` for the review run.
- That single-cursor status can prevent the next implement run from being auto-enqueued even though the kanban state clearly says "ready for rework."
- TB-237 added per-mode fields, so the review run can still preserve `ReviewedBy` / `ReviewStatus` while clearing the generic scheduling cursor.

## Constraints / Non-goals

- Preserve review run history and per-mode review attribution.
- Do not clear `needs-user`, `cancelled`, or unresolved `interrupted`.
- Do not make `review-failed` a new `AgentStatus`.
- The fix must work whether the failure handoff is initiated by manual CLI, manual GUI review, or an auto-review daemon run.
- Do not loosen auto-implement's skip of genuinely failed/cancelled/interrupted implementation runs.

## Acceptance Criteria

- [ ] `tb review --fail` and/or the review-mode terminal recording path leaves a ready `review-failed` task with blank generic `AgentStatus` after the failure handoff is complete.
- [ ] The task still records review attribution/history: JSONL contains the review run, and per-mode `ReviewedBy` / `ReviewStatus` remain meaningful.
- [ ] Auto-implement candidate selection sees the resulting ready `review-failed` task as eligible when it otherwise matches query, triage, epic-order, and WIP gates.
- [ ] Manual CLI fail path covered: `tb review --fail` from code-review writes findings, moves to ready, adds `review-failed`, and does not leave retry-blocking generic status.
- [ ] Review-agent fail path covered: a review-mode runner that invokes `tb review --fail` and exits 0 does not rewrite generic `AgentStatus=success` onto the ready task afterward.
- [ ] Non-fail review success path still records terminal status normally for tasks that move to done.
- [ ] Tests cover manual fail, daemon review fail, auto-review fail if TB-264 exists, and preservation of `needs-user`/`cancelled`/`interrupted`.
- [ ] Verification includes `cd cli && go test ./...` and `cd gui && go test ./...`.

## Review Target

- cli/review.go reviewWriteFailMetadata — clears generic AgentStatus inside the same board lock.
- cli/review_test.go TestReviewFailClearsAgentStatusForRetry — covers the manual CLI path.
- gui/app/agent_run.go recordTerminal — new TB-268 carve-out (next to the existing needs-user one) that actively clears AgentStatus when a review-mode run finishes success and the task is now in ready + review-failed. Per-mode ReviewedBy/ReviewStatus still written.
- gui/app/agent_run.go tasksContainsTag — small helper.
- gui/app/agent_run_test.go — three new tests:
  - TestRecordTerminalPreservesBlankAgentStatusOnReviewFailHandoff
  - TestRecordTerminalClearsLingeringAgentStatusOnReviewFailHandoff (alternate-path coverage)
  - TestRecordTerminalNeedsUserBeatsReviewFailHandoff (precedence guard)
- TestRecordTerminalReviewSuccessWithoutFailHandoffWritesAgentStatus (negative case).

## Related Tasks

- **TB-177** — Auto-implement retry pickup depends on blank scheduling state.
- **TB-179** — Auto-implement candidate selector.
- **TB-194** — Existing code-review workflow.
- **TB-199** — Original review-failed marker and fail flow.
- **TB-233** — Review-failed priority boost.
- **TB-237** — Per-mode attribution fields that preserve review history after generic status clear.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited implemented-by=claude, implement-status=success, reviewref=TB-268 ships in next commit
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited review-target
- 2026-05-20: Done

