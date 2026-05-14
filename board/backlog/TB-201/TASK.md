# TB-201: MacOS: window buttons hides header

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** macos,ui
**Branch:** —

## Goal

Prevent macOS traffic-light window controls from overlapping or obscuring the GUI app header and task-detail header while preserving native drag/titlebar behavior.

## Context

- Repro is shown in `attachments/Снимок экрана 2026-05-15 в 02.12.41.png`: the task drawer/header area sits underneath the macOS red/yellow/green window controls.
- Window options are configured in `gui/main.go` in `application.WebviewWindowOptions.Mac`; the current setup uses `MacTitleBarHiddenInset`, `InvisibleTitleBarHeight: 50`, and `MacBackdropTranslucent`.
- Header surfaces to check include the app topbar in `gui/frontend/src/routes/+page.svelte` and the task drawer header (`.surface-head`) in `gui/frontend/src/lib/components/TaskDrawer.svelte`; top-aligned modal headers such as create task and settings should not regress.
- Wails v3 docs to consult: `https://v3.wails.io/features/windows/options/` and `https://v3.wails.io/features/windows/frameless/` for macOS titlebar, full-size content, and invisible titlebar drag-area options.

Constraints:
- Scope this to macOS window chrome/header safe-area behavior; do not redesign the board, drawer, task body editor, or attachment workflow.
- Preserve native macOS traffic-light controls and a draggable top region.
- Isolate any macOS-only CSS or Wails option changes so Linux/Windows behavior does not change unexpectedly.
- Keep all board-format and structured mutation behavior unchanged.

Related tasks:
- **TB-17** — Scaffold gui/ Wails3 + SvelteKit project with single-instance lock (introduced the current macOS window options).
- **TB-22** — Frontend skeleton: api.ts, stores, +page.svelte layout (introduced the draggable topbar region).

## Acceptance Criteria

- [ ] On macOS, launching `cd gui && task dev` or `wails3 dev` shows the app topbar/header content fully clear of the red/yellow/green traffic-light controls.
- [ ] Opening a task drawer, including TB-201, shows the task ID/title and close button fully visible and not covered by the native window controls at the default 1280x800 window size.
- [ ] Resizing the macOS window down to the supported minimum keeps topbar actions, drawer/dialog headers, and close controls reachable with no overlap by native window controls.
- [ ] The chosen Wails/macOS titlebar configuration uses the Wails v3 option names available in this repo's `github.com/wailsapp/wails/v3 v3.0.0-alpha.91` API and preserves draggable titlebar/topbar behavior.
- [ ] Non-macOS behavior is unchanged, or macOS-only layout/window changes are guarded so Linux/Windows builds do not inherit the titlebar padding/configuration.
- [ ] Verification includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and the manual macOS smoke test below.

Manual test note: on macOS, run the GUI in dev mode, open the main board, open TB-201 or another task drawer, open `+ New`, and open Settings; confirm each top header is visually clear of the traffic-light controls before and after resizing the window.

## Attachments

- attachments/Снимок экрана 2026-05-15 в 02.12.41.png

## Log

- 2026-05-15: Created
- 2026-05-15: Attached Снимок экрана 2026-05-15 в 02.12.41.png
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited module=gui, tags=macos,ui, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success

