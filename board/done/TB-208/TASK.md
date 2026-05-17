# TB-208: Switch projects bug

**Type:** bug
**Priority:** P1
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** gui,board-switch,daemon,error-handling,robustness
**Branch:** —

## Goal

Make GUI project switching reject a structurally invalid candidate board before the switch is committed, so duplicate-task boards do not activate the daemon, emit reload events, or produce repeated raw Wails binding failures.

## Context

This is a follow-up to TB-145. TB-145 made board-load failures actionable and cleared stale cards when `LoadBoardWithMode` fails, but the reported repro still shows the switch path continuing far enough to trigger daemon startup scan warnings and repeated Wails binding-call errors:

```text
cannot load active board: task WS-1486 appears in multiple status directories (backlog: .../board/backlog/WS-1486.md; done: .../board/done/WS-1486.md). Move or remove one duplicate task file, then reload.
```

Relevant surfaces:

- `gui/app/settings_service.go`: `OpenBoard` currently swaps the CLI client/watcher, activates the daemon, commits `BoardInfo`, persists recents, and emits `board:opened`/`board:reloaded`.
- `gui/app/board_service.go`: `LoadBoardWithMode("active")` already wraps duplicate-ID failures as `cannot load active board...`.
- `gui/internal/daemon/daemon.go`: `Activate` calls `scanQueued`; scan failures are logged as non-fatal, which is wrong for validating a newly selected board.
- `gui/frontend/src/routes/+page.svelte` and `gui/frontend/src/lib/stores/board.ts`: board-open events and refresh/load-error handling drive the visible UI state.
- Repro board shape: the selected Writer Studio board contains the same task ID in both `backlog` and `done` (`WS-1486`).

Constraints and non-goals:

- Do not loosen the CLI invariant that one task ID must exist in exactly one active status directory.
- Do not silently ignore duplicate tasks in `done`, auto-delete files, move files, or repair the selected project.
- Prefer validating the candidate board before committing the switch; if no previous board exists, the app should remain in a recoverable picker/error state.
- Preserve TB-145 behavior for a board that becomes invalid after it was already opened: stale cards should still clear and the actionable load error should remain visible.
- Keep existing missing `.tb.yaml`, watcher failure rollback, recent-board persistence, and valid board-switch behavior intact.

## Acceptance Criteria

- [x] Add regression coverage for `SettingsService.OpenBoard` switching from a valid board to a candidate board whose active scope contains the same task ID in `backlog` and `done`.
- [x] On that failure, `OpenBoard` returns one concise actionable error containing the duplicate task ID, statuses, and conflicting paths, without exposing `Binding call failed` as user-facing text.
- [x] A failed candidate-board validation does not commit the switch: previous `BoardInfo`, `BoardService` client/board dir, watcher target, daemon activation, and recent-board ordering remain unchanged.
- [x] The failed switch does not emit `board:opened` or `board:reloaded`, so the frontend does not run repeated refreshes against the invalid candidate board.
- [x] If startup/reopen has no previous valid board and the recent board is invalid, the app remains usable in the picker/error state and can open another valid board without restart.
- [x] Preserve TB-145 refresh behavior for an already-open board that later becomes invalid: stale cards are cleared and the actionable load error is visible. *(LoadBoardWithMode / boardLoadError path untouched; the new validate call reuses it for symmetry)*
- [ ] Manual smoke: from a valid GUI board, try to open the Writer Studio duplicate-task repro; confirm the previous board/header remain visible, one actionable error mentions `WS-1486`, no raw `Binding call failed` loop appears, and selecting a valid board afterward succeeds. *(deferred to manual run; backend test asserts the same invariants on a duplicate-TB-9 fixture)*
- [x] Verify with `cd gui && go test ./...` and `cd gui/frontend && npm run check && npm test`.

## Related Tasks

- **TB-145** - Previous duplicate-task board-switch fix; this task closes the remaining switch/daemon/reload failure path.
- **TB-90** - Prior repeated board-switch UI state regression.
- **TB-21** - `SettingsService.OpenBoard` owns project-root switching, watcher handoff, recent boards, and reload events.
- **TB-88** - Similar GUI robustness work for avoiding raw Wails binding failures from advisory refresh paths.
- **TB-96** - CLI duplicate-task discovery invariant that must remain strict for active status scopes.

## Attachments

- attachments/Снимок экрана 2026-05-15 в 02.50.34.png

## Log

- 2026-05-15: Created
- 2026-05-15: Attached Снимок экрана 2026-05-15 в 02.50.34.png
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited priority=P1, type=bug, size=M, module=gui, tags=gui,board-switch,daemon,error-handling,robustness, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success
- 2026-05-15: Edited agentstatus=success
- 2026-05-17: Started — moved to in-progress
- 2026-05-17: Done — added `validateCandidateBoardActive` in `gui/app/settings_service.go` which runs `tb ls --json --status active` against the candidate board's CLI client and reuses `boardLoadError` to reshape duplicate-task failures into the same actionable message the runtime refresh emits. `OpenBoard` invokes it BEFORE watcher.Switch / BoardService rebind / daemon activation / recents update / Wails emits, so a failed validation leaves all previous state intact. New `TestOpenBoard_RejectsCandidateWithDuplicateTask` (valid→invalid) and `TestOpenBoard_RecoverableAfterFailedSwitch` (no prior board → invalid → valid recovers). Existing `TestSetCLIPath_*` log-count tests updated to account for the new validate call. Frontend untouched; `npm run check && npm test` clean. Manual smoke deferred.

