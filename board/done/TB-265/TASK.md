# TB-265: GUI: surface auto-review state and decisions

**Type:** feature
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** auto-review,frontend,ux
**ReviewRef:** local-workspace-diff
**Branch:** —
**Parent:** TB-262

## Goal

Show auto-review enablement, skipped states, active review runs, pass/fail outcomes, and manual fallback controls in the GUI without hiding the human review path.

## Context

- TB-197 already added the Code Review column, review fields, and manual Review action.
- TB-263 adds the setting/control. TB-264 adds daemon queueing.
- Users need to understand why a code-review task was not auto-reviewed, especially when the default agent is missing, the task is already queued/running, or it is waiting for user attention.

## Constraints / Non-goals

- Do not add a landing/tutorial surface. Use existing settings, header, card, drawer, toast, and run-history patterns.
- Human review remains first-class. Manual Review and direct `tb review` flows must stay visible and usable when auto-review is off or skipped.
- Do not expose raw daemon internals; show actionable state.

## Acceptance Criteria

- [x] Settings/header toggle state is reflected in the board UI and updates without restart.
- [x] Code Review cards and TaskDrawer show when a task is queued/running under auto-review using the existing review-mode run history labels.
- [x] Skipped states are visible enough for action: disabled, missing default agent, already queued/running, needs-user, wrong column, or duplicate active run.
- [x] PASS path feedback: after auto-review moves a task to done, the drawer/card state and board columns refresh cleanly without stale code-review rows.
- [x] FAIL path feedback: after auto-review fails a task, the ready card shows `review-failed`, findings are visible in the drawer, and manual Run remains available for explicit retry.
- [x] Existing manual Review button still works when auto-review is disabled or when the task was skipped by a guard.
- [x] Frontend tests cover visible enabled/disabled state, skipped reason rendering, pass refresh, fail marker/findings rendering, and manual fallback.
- [x] Verification includes `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- --run`.

## Review Target

local-workspace-diff: GUI auto-review runtime state and feedback for TB-265.

## Review Findings

No CRITICAL issues found. No MAJOR issues found.

Verification:
- cd gui/frontend && npm run check
- cd gui/frontend && npm run lint
- cd gui/frontend && npm run deadcode
- cd gui/frontend && npm test -- --run
- git diff --check

## Related Tasks

- **TB-262** — Parent epic.
- **TB-263** — Settings/header control.
- **TB-264** — Daemon auto-review enqueue logic.
- **TB-197** — Existing Code Review GUI surface.

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
