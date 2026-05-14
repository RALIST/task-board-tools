# TB-199: Workflow: review-failed marker and retry priority

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** agent
**Tags:** epic-tb194,code-review,review-failed,automation
**Branch:** —
**Parent:** TB-194

## Goal

Implement the failed-review handoff: reviewers can send a Code Review task back to backlog with a visible `review-failed` marker and automation can prioritize it for rework.

## Context

- Parent epic: TB-194.
- Original request calls for failed reviews to move back to backlog and be visibly marked/prioritized.
- TB-177/TB-179 own auto-implementation candidate selection; this task should coordinate with that flow when it exists.
- TB-182 defines a separate `needs-user` protocol; `review-failed` is not a user-attention state and should not be modeled as `AgentStatus: needs-user`.

## Constraints / Non-goals

- Use tag `review-failed` as the durable marker for failed review rework.
- Proposed CLI surface: `tb review --fail <ID> -` writes/replaces `## Review Findings` from stdin, moves a `code-review` task to backlog, and adds `review-failed` while preserving existing tags.
- Re-submitting a task to Code Review after fixes should clear `review-failed` automatically or require an explicit clear command; choose one and document it in TB-200. Prefer automatic clear on successful `tb review --submit` if implementation stays simple.
- The marker must be visible in CLI JSON/task listings and in the GUI card/drawer.
- Auto-implement priority should prefer eligible backlog tasks tagged `review-failed` over other eligible tasks with the same saved query and priority when TB-177/TB-179 are present.
- Do not add a new backlog sub-status or column in this task.

## Related Tasks

- **TB-194** - Parent epic.
- **TB-177** - Auto task implementation epic that should prioritize `review-failed` work when enabled.
- **TB-179** - Auto-implement daemon candidate selection integration point.
- **TB-182** - Separate user-attention protocol; keep semantics distinct.
- **TB-195** - Submit flow can clear failed-review marker on resubmit.
- **TB-196** - Findings writer reused by fail flow.
- **TB-197** - GUI visual treatment for failed reviews.

## Acceptance Criteria

- [ ] `tb review --fail <ID> -` accepts only Code Review tasks, writes/replaces `## Review Findings`, moves the task to backlog, adds `review-failed`, and regenerates `BOARD.md` under the board lock.
- [ ] The fail command rejects empty findings and leaves the task unchanged.
- [ ] The fail command preserves existing tags and does not overwrite `Agent`, `AgentStatus`, `Branch`, review target, or reviewer notes.
- [ ] The chosen clear behavior for `review-failed` on resubmit is implemented and covered by tests.
- [ ] CLI JSON/list output exposes `review-failed` via the existing tags field so downstream automation can filter it.
- [ ] GUI cards and the drawer show a clear failed-review marker for backlog tasks tagged `review-failed`.
- [ ] If auto-implement candidate selection from TB-177/TB-179 exists, eligible `review-failed` tasks are prioritized ahead of other matching backlog tasks at the same priority; if it is not merged yet, this task adds a focused follow-up or test fixture documenting the required ordering.
- [ ] Manual test note: fail a Code Review task with findings, confirm it lands in Backlog with the marker and visible findings, then resubmit after editing and confirm the marker clear behavior.
- [ ] Verification includes `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

