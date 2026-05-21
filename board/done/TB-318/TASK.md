# TB-318: GUI: show loading state during board switch

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** gui,board-switch,loading,ux
**GroomedBy:** codex
**GroomStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** 30daff0
**ReviewedBy:** codex
**ReviewStatus:** success
**AgentStatus:** success
**Branch:** —

## Goal

When the GUI opens a different board, make the selected board feel active immediately: update the visible board identity and replace old-board cards with a clear loading state while the new board snapshot loads, so huge boards do not look like the previous board is still selected.

## Context

- `gui/frontend/src/routes/+page.svelte` owns the header `Open board...` flow, recent-board reopen flow, `projectRoot`, `bootStatus`, and `board:opened` / `board:reloaded` handlers.
- `gui/frontend/src/lib/stores/board.ts` owns the current board snapshot, `loaded`, `loadError`, and refresh coalescing. Today a successful refresh leaves the previous snapshot visible until the new snapshot arrives; stale cards are only cleared on refresh failure.
- `gui/app/settings_service.go` commits a board switch only after candidate validation, watcher switch, daemon handoff, and BoardService rebinding, then emits `board:opened` and `board:reloaded`.
- Large-board behavior matters because TB-312 reduced duplicate load/render bursts, but a slow valid switch can still leave old-board cards on screen with little feedback.

## Constraints

- Preserve TB-208/TB-145 safeguards: invalid candidate boards must not commit the switch, stale previous-board cards must not be rendered as the selected board, and actionable load errors must remain visible.
- Keep scope in the GUI board-open/loading path. Do not change markdown task format, CLI board semantics, daemon scheduling, or large-column virtualization.
- The loading state must make old-board content non-authoritative during a switch: users should not be able to confuse previous-board cards for the newly selected board.
- Handle overlapping switch/refresh events deterministically so an older `board:opened`, `board:reloaded`, or refresh completion cannot overwrite a newer selected board state.

## Acceptance Criteria

- [ ] Selecting a valid board from the header picker updates the visible board identity or pending-board label before the new board snapshot finishes loading.
- [ ] While a board switch refresh is in flight, the kanban area shows a clear loading state for the target board and does not display previous-board cards as active data.
- [ ] The same loading behavior applies to recent-board opens and `board:opened` event refreshes, without causing duplicate user-visible refresh flicker.
- [ ] If `OpenBoard` rejects an invalid candidate before commit, the previous board identity and snapshot remain active and the existing actionable error/toast behavior is preserved.
- [ ] If a committed board refresh fails, stale previous-board cards remain hidden or cleared and the load error is visible, preserving TB-145 behavior.
- [ ] Overlapping board switches or delayed refresh completions cannot restore an older board's header, cards, load error, or recents state after a newer switch starts.
- [ ] Tests cover switch-start loading state, stale-card hiding, failed pre-commit rollback, committed refresh failure, and overlapping switch ordering.
- [ ] Verification includes `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run`, and `cd gui && go test ./...` if backend switch code is touched.
- [ ] Manual test note: run the desktop GUI, switch from a small board to a large board such as Writer Studio, confirm the target board/loading state appears immediately instead of old cards staying silently visible, then switch back and confirm the same behavior.

## Review Target

branch: main commit: 30daff0

Summary:
- Preserved the board-switch loading coordinator and stale-card clearing from 479891e.
- Reconciled overlapping direct opens through current coordinator behavior so an older committed open can win after a newer invalid candidate rejects.
- Started persisted-board startup grace before the initial refresh resolves, so launch on an already-open huge board shows grace immediately instead of waiting for LoadBoard.

Verification:
- cd gui/frontend && npm test -- --run src/lib/boardSwitch.test.ts src/lib/stores/board.test.ts (25 tests passed)
- cd gui/frontend && npm run check (0 errors, 0 warnings)
- cd gui/frontend && npm test -- --run (294 tests passed)
- cd gui/frontend && npm run lint (passed)
- git diff --check -- gui/frontend/src/lib/boardSwitch.ts gui/frontend/src/lib/boardSwitch.test.ts gui/frontend/src/lib/stores/board.ts gui/frontend/src/routes/+page.svelte (passed)
- code review: no CRITICAL or IMPORTANT findings after fixes

Backend verification note:
- cd gui && go test ./... was attempted and failed in existing internal/agent prompt coverage: PromptGroom missing {{TASK_TITLE}} and {{TASK_BODY}}. This is outside TB-318 frontend board-switch scope and is tracked by TB-338.

Manual note:
- Desktop GUI Writer Studio switch smoke not completed in this pass. Prior smoke memory shows opening Writer Studio can launch real autonomous work from recent-board startup; I did not risk that side effect. Automated coverage exercises switch-start loading, stale-card hiding, invalid pre-commit rollback, committed refresh failure, overlapping open reconciliation, stale event guards, and persisted-board startup grace ordering.

## Review Findings

- No blocking findings.

## Related Tasks

- **TB-90** — Prior repeated board-switch UI refresh regression.
- **TB-145** — Existing stale-card clearing and actionable load-error behavior for committed load failures.
- **TB-208** — Candidate-board validation must still prevent invalid switches from committing.
- **TB-312** — Recent board-switch fix and large-board render/load burst reduction.
- **TB-313** — Future large-board virtualization; this task only adds switch/loading feedback.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited agent=codex
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited priority=P2, type=bug, size=M, module=gui, tags=gui,board-switch,loading,ux, title=GUI: show loading state during board switch, goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success, implemented-by=none, implement-status=none
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Moved to ready
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
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited reviewref=20c82ea
- 2026-05-21: Edited review-target
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited reviewref=479891e
- 2026-05-21: Edited review-target
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited agentstatus=none, reviewed-by=codex, review-status=success
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited review-target
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewref=30daff0
- 2026-05-21: Submitted to code-review
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited review-findings
- 2026-05-21: Cleared review-failed marker on resubmit
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Passed code review
- 2026-05-21: Edited agentstatus=success, reviewed-by=codex, review-status=success

