# TB-145: Board switch error

**Type:** bug
**Priority:** P0
**Size:** M
**Agent:** codex
**AgentStatus:** failed
**Module:** gui
**Tags:** gui,board-switch,error-handling,robustness
**Branch:** —

## Goal

Make board switching resilient when the selected board is structurally invalid: if `tb ls --json --status active` fails because one task ID exists in multiple active-scope status directories, the GUI must show a clear recoverable board-load error instead of leaving the user with a raw Wails binding failure or stale board contents.

## Context

Reported while switching the GUI to `/Users/ralist/projects/books/writer-studio`; the first board load failed with:

```text
tb [ls --json --status active]: exit 1: error: task WS-1486 resolves to multiple canonical markdown paths in requested status scope: /Users/ralist/projects/books/writer-studio/board/backlog/WS-1486.md and /Users/ralist/projects/books/writer-studio/board/done/WS-1486.md
```

Relevant current seams:

- `gui/app/board_service.go`: `BoardService.LoadBoardWithMode` shells out through `c.RunJSON(ctx, &tasks, "ls", "--json", "--status", statusArg)`.
- `gui/app/settings_service.go`: `SettingsService.OpenBoard` validates `.tb.yaml`, switches watcher/client state, emits `board:opened` and `board:reloaded`, and the frontend then calls `refresh()`.
- `gui/frontend/src/lib/stores/board.ts`: `refresh()` records `loadError` but must not let a failed switch continue rendering the previous board snapshot as if it belonged to the newly selected root.
- `cli/board.go`: duplicate canonical task paths inside the requested status scope fail loudly; this is an intentional board invariant, not a parser fallback case.

## Constraints

- Do not loosen the CLI duplicate-ID invariant, skip duplicate task IDs silently, or mutate/repair the selected board automatically.
- Keep the fix in the GUI board-open/load path and preserve an actionable diagnostic for the user.
- Existing `ErrNoBoard`, missing `.tb.yaml`, watcher-switch failure, and normal CLI failure behavior should keep their current semantics unless the test proves they are part of this bug.
- Manual test note: use a temp board or the Writer Studio repro shape with the same task ID in `backlog` and `done`, open it from the GUI, and confirm the app remains usable and the error is visible.

## Acceptance Criteria

- [x] Add a regression test for the duplicate canonical path failure from `tb ls --json --status active` during board load after selecting a board.
- [x] When the selected board cannot load because of duplicate active-scope task IDs, the GUI surfaces a concise actionable message that includes the task ID and conflicting paths/statuses, without using `Binding call failed` as the user-facing text.
- [x] A failed load after board switch must not render stale cards from the previously opened board under the newly selected project root.
- [x] After this failure, the user can still open another valid board from the picker or recent-board menu without restarting the app.
- [x] Existing no-board, missing `.tb.yaml`, watcher-switch failure, and non-duplicate CLI failure behavior remains covered and unchanged.
- [ ] Manual smoke: open a board containing the same task ID in `backlog` and `done`; observe the visible load error, confirm stale cards are not shown for that board, then open a valid board successfully. *(human-in-the-loop; pending operator verification)*
- [x] Verify with `cd gui && go test ./...` and `cd gui/frontend && npm run check && npm test`.

## Related Tasks

- **TB-21** - SettingsService project root, recent boards, and board-open state handoff.
- **TB-40** - BoardService archive-aware `LoadBoard` status mode.
- **TB-88** - Similar GUI robustness fix for raw Wails binding failures from advisory triage refreshes.
- **TB-96** - Folder-form task discovery and duplicate canonical path invariant.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited priority=P0, type=bug, size=M, module=gui, tags=gui,board-switch,error-handling,robustness, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Edited agentstatus=failed
- 2026-05-15: Implemented — backend `boardLoadError` already landed in TB-93 epic (concise diagnostic + Go regression tests `TestLoadBoard_DuplicateCanonicalPathsAreActionable` and `TestLoadBoard_NonDuplicateCLIFailurePreservesExitError`); finished frontend by clearing the board snapshot in `gui/frontend/src/lib/stores/board.ts:refresh()` on rejection so stale cards from the previous root no longer render, and added two regression tests in `gui/frontend/src/lib/stores/board.test.ts` covering the failure-clears-stale path and the seq-guard against stale rejections. `cd gui && go test ./...` and `cd gui/frontend && npm run check && npm test` (77/77) pass. Manual smoke remains for human operator.
- 2026-05-15: Done

