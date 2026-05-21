# TB-320: GUI: make startup grace header a live countdown

**Type:** bug
**Priority:** P2
**Size:** S
**Module:** gui/frontend
**Tags:** startup,automation,ui,grace
**Agent:** codex
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** 30daff0
**AgentStatus:** success
**Branch:** —

## Goal

Header currently shows the configured automation startup grace (for example Grace 30s) as a persistent badge. That reads like a countdown, but it does not decrement or disappear when the grace window ends.

## Context

Observed on 2026-05-21 in the GUI header: `Grace 30s` remained visible as a static pill. The frontend currently renders `$preferencesStore.automationStartupGraceSeconds`, which is only the configured preference, not live board-activation state.

Backend startup grace timers exist for daemon/coordinator activation. The frontend needs a live active-grace signal/countdown, or the header should avoid countdown-like wording when no active grace window is running.

Related work: TB-301 added startup grace behavior and the first static header pill.

## Constraints

- Do not remove the persisted Settings control for `automation_startup_grace_seconds`.
- Do not show a permanent countdown-looking badge from a static preference value.
- Board switch/open must reset the displayed grace state for the newly active board.
- Setting value `0` means no grace indicator.
- Keep manual Run/Groom/Review and Settings controls visible during grace.

## Acceptance Criteria

- [ ] Header shows a startup-grace indicator only while an activation grace window is active, then hides automatically when the window expires.
- [ ] When shown, the indicator decrements at least once per second or otherwise clearly communicates remaining time.
- [ ] Opening/switching boards restarts/cancels the indicator for the correct board; an old board's countdown cannot keep running after switch.
- [ ] With `automation_startup_grace_seconds = 0`, no grace countdown is shown and existing immediate behavior remains.
- [ ] Tests cover initial display, ticking/decrement, expiry/hide, board switch cancellation, and zero-delay behavior.
- [ ] Verification includes `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- --run`.

## Review Target

branch: main
commit: 30daff0
base review ref: b1ced3d

Summary:
- Existing-board startup now routes through refreshPersistedBoardOnStartup, so startStartupGraceForBoard runs before the initial board refresh waits.
- The live countdown store remains responsible for zero-delay suppression, ticking, expiry, and board-switch cancellation.
- Review finding at b1ced3d is addressed by commits 51708ec and 30daff0; no extra production change was needed in this pass.

Verification:
- cd gui/frontend && npm test -- --run src/lib/boardSwitch.test.ts src/lib/stores/startupGrace.test.ts
- cd gui/frontend && npm run check
- cd gui/frontend && npm test -- --run
- cd gui/frontend && npm run lint

## Review Findings

Previous blocking finding at b1ced3d is addressed in 30daff0: existing-board mount now calls refreshPersistedBoardOnStartup(projectRoot, startStartupGraceForBoard, refresh), which starts grace before the initial board refresh awaits. Countdown/tick/expiry/switch/zero-delay behavior remains covered by startupGrace store tests.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Edited agent=codex
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-21: Moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited review-target
- 2026-05-21: Edited reviewref=a6e3660
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited agentstatus=none, reviewed-by=codex, review-status=success
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited review-target
- 2026-05-21: Edited implemented-by=codex, implement-status=success, reviewref=479891e
- 2026-05-21: Submitted to code-review
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited review-findings
- 2026-05-21: Cleared review-failed marker on resubmit
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited tags=startup,automation,ui,grace, reviewed-by=none, review-status=none
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited agentstatus=none, reviewed-by=codex, review-status=success
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited tags=startup,automation,ui,grace, agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=none, review-status=none, reviewref=499e324
- 2026-05-21: Edited review-target
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited reviewref=b1ced3d
- 2026-05-21: Edited review-target
- 2026-05-21: Edited review-findings
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited agentstatus=none, reviewed-by=codex, review-status=success
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited review-target
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=none, review-status=none, reviewref=30daff0
- 2026-05-21: Submitted to code-review
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited review-findings
- 2026-05-21: Edited agentstatus=success, reviewed-by=none, review-status=none, reviewref=30daff0
- 2026-05-21: Cleared review-failed marker on resubmit
- 2026-05-21: Submitted to code-review

