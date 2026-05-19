# TB-233: Auto-implement priority: rank review-failed backlog tasks first

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** auto-implement,review-failed,epic-tb194
**Agent:** claude
**AgentStatus:** interrupted
**Branch:** —
**Parent:** TB-177

## Goal

When TB-179 ships the auto-implement candidate selector in the GUI daemon, break ties at the same priority by ranking eligible backlog tasks tagged `review-failed` ahead of other eligible backlog tasks. This is the ordering follow-up TB-199 deferred so that failed-review rework returns to an agent before fresh backlog work.

## Context

- Parent epic for the candidate selector: TB-177; the selector itself lives in TB-179 (currently in `backlog/TB-179/TASK.md`).
- Origin requirement: TB-199 (done) — "If auto-implement candidate selection from TB-177/TB-179 exists, eligible review-failed tasks are prioritized ahead of other matching backlog tasks at the same priority; if it is not merged yet, this task adds a focused follow-up or test fixture documenting the required ordering."
- Eligibility predicate already exists at `gui/internal/daemon/daemon.go` (`isReadyForDaemon` / `IsAutomationEligible`); this task only changes the *order* in which eligible candidates are picked, not the predicate.
- The `review-failed` marker is a tag on backlog tasks written by `tb review --fail` (TB-199); it is already exposed in CLI JSON and GUI cards.

## Constraints / Non-goals

- Do not change the eligibility rules from TB-179 (groomed, blank `AgentStatus`, query match, backlog only). This task only affects sort order among already-eligible tasks.
- Do not change ordering across priorities — P0 always beats P1, etc. The `review-failed` boost only applies *within the same priority bucket*.
- Do not introduce a new column, status, or `AgentStatus` value. `review-failed` stays a tag.
- Do not add a new CLI surface or settings toggle; this is a daemon-internal ordering rule.
- If TB-179 lands with this rule already implemented, close this task as duplicate rather than re-implementing.

## Acceptance Criteria

- [ ] In the daemon's auto-implement candidate selector (added by TB-179 in `gui/internal/daemon/`), eligible backlog tasks at the same priority are ordered with `review-failed`-tagged tasks first; tasks without the tag follow using the existing secondary key (oldest-first or whatever TB-179 chooses).
- [ ] Ordering across priorities is unchanged: a P1 non-`review-failed` task still ranks ahead of a P2 `review-failed` task.
- [ ] Unit test fixture in the daemon candidate-selection tests covers: (a) two eligible P2 backlog tasks where the `review-failed`-tagged one is selected first; (b) P1 plain task vs P2 `review-failed` — the P1 still wins; (c) no `review-failed` candidates — existing ordering is preserved.
- [ ] The `review-failed` boost only applies to backlog tasks; tasks in other statuses are not promoted (and are already filtered by the eligibility predicate, so this is enforced by reuse, not by a second check).
- [ ] Tag matching is case-sensitive on the literal `review-failed` value written by `tb review --fail` (TB-199); no normalization or alias logic is introduced.
- [ ] Verification: `cd gui && go test ./...` passes.
- [ ] If TB-179 has not been picked up yet when this task starts, either (a) implement the ordering rule and tests as part of TB-179 and close this task as merged-into-TB-179, or (b) leave a passing test fixture in the daemon test file that asserts the desired ordering against a small in-package helper, so the rule lands as soon as TB-179 wires the selector. Choose one and note it in the Log.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=interrupted

