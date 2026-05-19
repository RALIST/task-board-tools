# TB-202: Create proper name and icon for app

**Type:** improvement
**Priority:** P2
**Size:** S
**Agent:** claude
**AgentStatus:** success
**Module:** gui
**Tags:** branding,ui,quick-win
**Branch:** —

## Goal

Rename the Wails GUI's user-visible desktop identity to `Task Board Tools` and give it a simple project-specific icon so native desktop surfaces no longer show the generic `tb-gui` or default Wails identity.

## Context

User-visible naming currently appears in `gui/main.go` (`application.Options.Name`, window `Title`), `gui/Taskfile.yml` (`APP_NAME` for builds/packages), and generated/build metadata from `gui/build/config.yml`. Existing generated platform metadata includes `gui/build/darwin/Info.plist`, `gui/build/darwin/Info.dev.plist`, `gui/build/windows/info.json`, `gui/build/linux/desktop`, and `gui/build/linux/nfpm/nfpm.yaml`.

Icon generation is already wired through `gui/build/Taskfile.yml` via `common:generate:icons`, using `gui/build/appicon.png` and `gui/build/appicon.icon` as source inputs and producing platform assets such as `gui/build/darwin/icons.icns`, `gui/build/darwin/Assets.car`, and `gui/build/windows/icon.ico`. The current `.icon` source still references `wails_icon_vector.svg`.

Constraints and non-goals:
- Keep the scope to the GUI application's display name, title, packaged metadata, and icon assets.
- Refresh generated Wails build assets with Wails tasks instead of hand-editing generated platform outputs.
- Do not rename the CLI command, board prefix, Go modules, config directories, or task markdown format as part of this task.

## Acceptance Criteria

- [x] The GUI's user-facing display/product name is `Task Board Tools` in `gui/main.go`, `gui/Taskfile.yml`, and Wails build metadata, so the window title, native app name, packaged app metadata, and installer/package labels no longer display the generic `tb-gui` name.
- [x] A simple project-specific source icon replaces the current default/generic source assets in `gui/build/appicon.png` and `gui/build/appicon.icon`, including removing the default `wails_icon_vector.svg` identity from the active icon source.
- [x] Generated platform icons are refreshed through the existing Wails icon generation path, producing synced macOS and Windows assets such as `gui/build/darwin/icons.icns`, `gui/build/darwin/Assets.car`, and `gui/build/windows/icon.ico`.
- [x] Build metadata is refreshed through `wails3 task common:update:build-assets` from `gui/` after changing `gui/build/config.yml`, and generated platform metadata remains consistent with `Task Board Tools`.
- [x] Icon generation is verified with `wails3 task common:generate:icons` from `gui/` or an equivalent Wails build task that depends on it.
- [ ] Manual test note: run the GUI on macOS and confirm `Task Board Tools` and the new icon appear in the window title, application menu/app switcher, Dock, and built app metadata; record any platform-specific gaps as follow-up tasks.
- [x] The implementation does not rename the CLI binary, board prefix, Go modules, config directories, or markdown task format.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited module=gui, tags=branding,ui,quick-win, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Groomed — clarified target name, icon/build surfaces, constraints, and manual verification.
- 2026-05-15: Edited agentstatus=success
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Done — Renamed user-facing app to "Task Board Tools" in gui/main.go (Options.Name + window Title), gui/build/config.yml productName, and regenerated Wails build metadata via `wails3 task common:update:build-assets` (Info.plist CFBundleName, Info.dev.plist, ios plists, windows/info.json ProductName, linux/desktop Name, NSIS INFO_PRODUCTNAME). Manually updated MSIX app_manifest.xml + template.xml DisplayName fields (wails3 update doesn't template those). Binary identity stays as `tb-gui` per task constraints. Replaced the default Wails 'W' icon with a project-specific kanban-themed SVG (three columns + green check card) at gui/build/appicon.icon/Assets/tb_kanban_vector.svg, removed wails_icon_vector.svg, rerendered appicon.png via rsvg-convert, and regenerated darwin/icons.icns + windows/icon.ico via `wails3 task common:generate:icons`. Deleted stale darwin/Assets.car so macOS falls back to icons.icns (actool's plugin loading is broken in this dev environment; Assets.car can be re-rendered on a working Xcode install). Added explicit PRODUCT_NAME var in gui/Taskfile.yml to document the display-name/binary-name split. Verified: `go test ./internal/... ./app/...` pass, frontend svelte-check 0/0/0, npm test 190/190, cli tests pass, `go build .` produces a launching binary. Manual macOS verification (Dock label, app switcher, window title, built .app icon) still requires the user.
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited agentstatus=success

