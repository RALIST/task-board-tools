# TB-320: GUI: make startup grace header a live countdown

**Type:** bug
**Priority:** P2
**Size:** S
**Module:** gui/frontend
**Tags:** startup,automation,ui,grace
**Agent:** codex
**AgentStatus:** failed
**ImplementedBy:** codex
**ImplementStatus:** failed
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

