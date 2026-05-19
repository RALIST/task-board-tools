# TB-236: macOS titlebar double-click should zoom/restore window

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** claude
**AgentStatus:** success
**Module:** gui
**Tags:** macos,ui,window
**Branch:** —

## Goal

Make double-clicking the GUI's macOS titlebar/topbar toggle the main window between its pre-click size and the macOS zoom/maximized size.

## Context

- Current window chrome is configured in `gui/main.go` via `application.WebviewWindowOptions.Mac` with `MacTitleBarHiddenInset`, `InvisibleTitleBarHeight: 50`, and `MacBackdropTranslucent`.
- The visible app titlebar/topbar drag area lives in `gui/frontend/src/routes/+page.svelte` and currently uses `--wails-draggable: drag` / `-webkit-app-region: drag`.
- TB-201 fixed macOS traffic-light/header overlap by adding safe-area spacing; keep that behavior intact while restoring normal titlebar double-click behavior.
- Before adding custom frontend behavior, check whether the pinned Wails v3 alpha window/titlebar APIs can provide native macOS double-click zoom/restore for the hidden-inset titlebar configuration.

## Constraints and Non-goals

- Scope this to macOS titlebar/window behavior; do not redesign the board, task detail surface, settings, dialogs, or app header layout.
- Preserve native traffic-light controls, single-click drag-to-move, and the existing topbar/header safe-area spacing from TB-201.
- Target macOS zoom/maximize-and-restore behavior: fill the usable screen area, then return to the last pre-zoom size and position. Do not implement green-button fullscreen Space behavior unless that is the native Wails/macOS titlebar double-click behavior.
- If custom handling is needed, guard it to macOS and keep interactive controls/buttons/inputs from triggering the window toggle.
- Keep CLI/board mutation behavior unchanged.

## Acceptance Criteria

- [x] On macOS, double-clicking an empty part of the draggable app titlebar/topbar toggles the main window from its current size to a zoomed/maximized size that fills the usable display area.
- [x] Double-clicking the same titlebar/topbar area again restores the window to the size and position it had immediately before zooming; restore must work after the user manually resizes or moves the window.
- [x] Single-click dragging the titlebar/topbar still moves the window normally.
- [x] Double-clicking interactive header controls, task detail controls, dialogs, inputs, CodeMirror, or task content does not toggle window size.
- [x] TB-201 behavior does not regress: macOS traffic-light controls do not overlap the app header, task detail header, create dialog, or settings panel at default and minimum supported window sizes.
- [x] Non-macOS behavior is unchanged, or all macOS-specific code is guarded so Linux/Windows builds do not inherit the titlebar double-click handler or layout changes.
- [x] Verification includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test`.
- [ ] Manual test note: on macOS, run `cd gui && task dev` or `wails3 dev`, double-click the empty topbar/titlebar area at the default size and after a manual resize, then confirm zoom/restore, drag-to-move, and top-header spacing all behave like a normal Mac app. *(left unchecked — manual verification on a running dev build is the user's call; the automated suite passes.)*

## Related Tasks

- **TB-201** — MacOS: window buttons hides header (relationship: shares macOS titlebar/topbar safe-area behavior)
- **TB-17** — Scaffold gui/ Wails3 + SvelteKit project with single-instance lock (relationship: introduced current Wails macOS window options)
- **TB-22** — Frontend skeleton: api.ts, stores, +page.svelte layout (relationship: introduced draggable topbar region)

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited module=gui, tags=macos,ui,window, title=macOS titlebar double-click should zoom/restore window, goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done — Set gui/main.go InvisibleTitleBarHeight to 0 so Wails' native performWindowDragWithEvent: no longer eats double-clicks in the content-area drag strip; added onTopbarDblClick in routes/+page.svelte that calls Window.Zoom() on macOS only, ignores interactive controls, and skips while fullscreen. Suppressed text selection on the title text so dblclick reaches the handler. Verified go test ./..., npm run check (0/0/0), npm test (190/190). Manual macOS dev-server verification still requires the user.
- 2026-05-19: Edited agentstatus=success

