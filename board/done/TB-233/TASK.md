# TB-233: Auto-implement priority: rank review-failed ready tasks first

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** auto-implement,review-failed,epic-tb194
**Agent:** claude
**AgentStatus:** interrupted
**ImplementedBy:** claude
**ImplementStatus:** success
**ReviewRef:** TB-233 ships as part of TB-179 commit
**Branch:** —
**Parent:** TB-177

## Goal

When TB-179 ships the auto-implement candidate selector in the GUI daemon, break ties at the same priority by ranking eligible tasks tagged `review-failed` ahead of other eligible tasks. This is the ordering follow-up TB-199 deferred so that failed-review rework returns to an agent before fresh work. Post-M10 (TB-239) the canonical home of `review-failed` tasks is the `ready` column, so this task and TB-179 both target `ready` as the candidate pool (legacy `backlog` tasks still carrying the tag are accepted by `tb review --submit` for backwards compatibility, but new auto-implement candidates flow through `ready`).

## Context

- Parent epic for the candidate selector: TB-177; the selector itself lives in TB-179 (currently in `backlog/TB-179/TASK.md`). TB-179's "backlog only" eligibility wording predates M10 (TB-239) and should read "ready only" by the time it ships; this task assumes that update.
- Origin requirement: TB-199 (done) — "If auto-implement candidate selection from TB-177/TB-179 exists, eligible review-failed tasks are prioritized ahead of other matching tasks at the same priority; if it is not merged yet, this task adds a focused follow-up or test fixture documenting the required ordering."
- Kanban realignment: M10 (TB-239) moved `tb review --fail` to land tasks in `ready/` instead of `backlog/`. The `review-failed` tag still rides along; the column changed, not the marker.
- Eligibility predicate already exists at `gui/internal/daemon/daemon.go` (`isReadyForDaemon` / `IsAutomationEligible`); this task only changes the *order* in which eligible candidates are picked, not the predicate.
- The `review-failed` marker is a tag on `ready` tasks written by `tb review --fail` (TB-199 + TB-239); it is already exposed in CLI JSON and GUI cards.

## Constraints / Non-goals

- Do not change the eligibility rules from TB-179 (blank `AgentStatus`, query match, source column = `ready` post-M10, and epic-order gate from TB-267). This task only affects sort order among already-eligible tasks.
- Do not change ordering across priorities — P0 always beats P1, etc. The `review-failed` boost only applies *within the same priority bucket*.
- Do not introduce a new column, status, or `AgentStatus` value. `review-failed` stays a tag.
- Do not add a new CLI surface or settings toggle; this is a daemon-internal ordering rule.
- If TB-179 lands with this rule already implemented, close this task as duplicate rather than re-implementing.

## Acceptance Criteria

- [ ] In the daemon's auto-implement candidate selector (added by TB-179 in `gui/internal/daemon/`), eligible `ready` tasks at the same priority are ordered with `review-failed`-tagged tasks first; tasks without the tag follow using the existing secondary key (oldest-first or whatever TB-179 chooses).
- [ ] The boost is applied after hard eligibility gates, including TB-267's epic child ordering. A later child tagged `review-failed` never bypasses an unfinished earlier sibling.
- [ ] Ordering across priorities is unchanged: a P1 non-`review-failed` task still ranks ahead of a P2 `review-failed` task.
- [ ] Unit test fixture in the daemon candidate-selection tests covers: (a) two eligible P2 `ready` tasks where the `review-failed`-tagged one is selected first; (b) P1 plain task vs P2 `review-failed` — the P1 still wins; (c) no `review-failed` candidates — existing ordering is preserved.
- [ ] The `review-failed` boost only applies to tasks in the auto-implement candidate column (`ready` post-M10); tasks in other statuses are not promoted (and are already filtered by the eligibility predicate, so this is enforced by reuse, not by a second check).
- [ ] Tag matching is case-sensitive on the literal `review-failed` value written by `tb review --fail` (TB-199 + TB-239); no normalization or alias logic is introduced.
- [ ] Verification: `cd gui && go test ./...` passes.
- [ ] If TB-179 has not been picked up yet when this task starts, either (a) implement the ordering rule and tests as part of TB-179 and close this task as merged-into-TB-179, or (b) leave a passing test fixture in the daemon test file that asserts the desired ordering against a small in-package helper, so the rule lands as soon as TB-179 wires the selector. Choose one and note it in the Log.

## Review Target

Merged into TB-179 candidate selector per AC's "if TB-179 has not been picked up yet, implement the ordering rule and tests as part of TB-179 and close this task as merged-into-TB-179". The sort comparator in gui/app/auto_implement.go ranks review-failed first within the same priority bucket, with three dedicated tests in auto_implement_test.go covering: TB-200 review-failed beats TB-100 plain at same priority; P1 plain beats P2 review-failed (priority bucket trumps tag); no-review-failed pool preserves id ascending order.

## Related Tasks

- **TB-177** — Parent auto-implement epic.
- **TB-179** — Candidate selector this ordering extends.
- **TB-267** — Prerequisite epic child ordering gate.
- **TB-268** — Ensures review-failed ready tasks have blank retryable agent state.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited — rewrote scope from "backlog tasks" to "ready tasks" to reflect M10 (TB-239) canonical kanban; `tb review --fail` now lands in `ready/` not `backlog/`.
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited implemented-by=claude, implement-status=success, reviewref=TB-233 ships as part of TB-179 commit
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited review-target
- 2026-05-20: Done

