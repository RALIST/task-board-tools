# TB-225: Ask user to init board from GUI when he tries to open folder without initialized tb status 
**Agent:** claude
**Type:** feature
**Priority:** P2
**Size:** M
**AgentStatus:** failed
**Module:** gui
**Tags:** gui,board-open,init,ux
**Branch:** —

## Goal

Let users initialize a new tb board from the GUI when they pick a project folder that does not yet contain `.tb.yaml`.

## Context

Current GUI board switching requires an existing project root with `.tb.yaml`: `SettingsService.OpenBoard` returns `ErrNoTbYaml` from `readBoardInfo`, and `gui/frontend/src/routes/+page.svelte` currently turns that into a toast saying the previous board is still active. The CLI already supports `tb init [path] --board-path=board --prefix=PR`; the GUI should expose that path after the user selects an uninitialized folder.

Relevant seams:

- `gui/app/settings_service.go`: add or expose an init flow, then open the initialized board through the existing `OpenBoard` path so watcher, BoardService, daemon, recents, and `board:opened`/`board:reloaded` behavior stay consistent.
- `gui/frontend/src/routes/+page.svelte` and `gui/frontend/src/lib/api.ts`: replace the missing-`.tb.yaml` dead-end toast with a confirm/modal flow that collects project config and opens the board after init.
- `cli/init.go`: keep behavior and defaults aligned with `tb init` (`board`, `PR`) instead of inventing a second board layout.

## Constraints

- Do not weaken existing board-switch safeguards from TB-145/TB-208: failed init/open attempts must not corrupt or silently replace the previously active board.
- Do not auto-initialize on folder pick without explicit user confirmation.
- Validate prefix and board path before writing, keep defaults aligned with CLI `tb init`, and show recoverable errors in the GUI.
- Non-goal: redesign the broader settings panel, recent-board storage, or existing board-switch validation behavior.

## Acceptance Criteria

- [x] Picking a folder without `.tb.yaml` from the initial empty state or the `Open board...` action does not stop at a toast; the user is offered an explicit initialize-board flow and can cancel back to the picker or previously active board.
- [x] The init flow collects at least project root (pre-filled from the selected folder), board path (default `board`), and prefix (default `PR`), validates invalid/empty values, and does not write files until the user confirms.
- [x] Confirming init creates the same on-disk artifacts as `tb init <path> --board-path=<board> --prefix=<prefix>`: `.tb.yaml`, board status directories, `.next-id`, `BOARD.md`, `CONVENTIONS.md`, and `SKILL.md`.
- [x] After successful init, the GUI opens the newly initialized board through the normal `OpenBoard` path, updates header and recents, emits/reloads consistently, and leaves the app ready to create tasks.
- [x] If init fails or the subsequent open/validation fails, the GUI shows an actionable error and preserves the previously active board, BoardService client/board dir, watcher target, daemon activation, and recents.
- [x] Regression coverage covers backend init/open behavior and frontend missing-`.tb.yaml` prompt/cancel/confirm behavior.
- [ ] Manual smoke: start the GUI, choose a temporary folder with no `.tb.yaml`, confirm defaults in the init modal, verify the board opens and the folder contains `.tb.yaml` plus `board/`; repeat with Cancel and confirm no files are created. (Not run in this headless session; backend integration test `TestInitBoard_HappyPath` exercises the equivalent on-disk path end-to-end.)
- [x] Verify with `cd gui && go test ./...` and `cd gui/frontend && npm run check && npm test`. (TB-225-scoped tests pass; pre-existing `attachments_test.go` failures are tracked under in-progress TB-224 and unrelated to this work.)

## Related Tasks

- **TB-21** — Owns `SettingsService.OpenBoard`, project-root parsing, recents, and the original `ErrNoTbYaml` behavior this flow extends.
- **TB-64** — Regression coverage for repeated Open board clicks and missing-`.tb.yaml` not blocking the picker.
- **TB-145** — Keeps board-switch load errors actionable instead of stale/raw binding failures.
- **TB-208** — Candidate-board validation must remain pre-commit when the newly initialized board is opened.

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Edited agent=codex
- 2026-05-17: Edited agent=codex
- 2026-05-17: Edited body via GUI
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Edited priority=P2, type=feature, size=M, module=gui, tags=gui,board-open,init,ux
- 2026-05-17: Edited goal
- 2026-05-17: Edited acceptance
- 2026-05-17: Edited agentstatus=success
- 2026-05-17: Edited agent=claude
- 2026-05-17: Edited agentstatus=success
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Started — moved to in-progress
- 2026-05-17: Edited agentstatus=failed
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Done — InitBoard backend (SettingsService.InitBoard) validates project root, board path, prefix, then invokes `tb init` and routes through OpenBoard so watcher/BoardService/daemon/recents stay consistent. Frontend InitBoardDialog replaces the missing-`.tb.yaml` toast with a confirm modal (defaults `board`/`PR`, pre-validated). Failure paths preserve previously active board. Backend+frontend regression coverage added (settings_init_test.go, settings_open_validate_test.go, InitBoardDialog.test.ts).
- 2026-05-17: Edited agentstatus=failed

