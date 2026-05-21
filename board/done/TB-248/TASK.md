# TB-248: Manual macOS verification of Task Board Tools branding

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** branding,verification,macos,testing
**Agent:** codex
**AgentStatus:** success
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Manually verify the packaged macOS GUI bundle presents the `Task Board Tools` name and the project kanban icon across native macOS surfaces, then record evidence and file standalone follow-up tasks for any branding/icon gap found.

## Context

TB-248 is the manual verification follow-up for TB-202. TB-202 renamed the GUI's user-facing identity to `Task Board Tools`, refreshed generated Wails metadata, replaced the default Wails icon with the kanban-themed source icon, and left manual macOS verification unchecked because headless agents could not validate Dock, Cmd-Tab, Finder, and menu-bar behavior.

The production macOS package path is expected to come from `cd gui && task package`, which delegates to the Darwin packaging task and creates `gui/bin/tb-gui.app`. `gui/Taskfile.yml` intentionally keeps `APP_NAME=tb-gui` as the binary/bundle path identifier while `PRODUCT_NAME=Task Board Tools` is the user-facing name.

TB-246 separately tracks regenerating `gui/build/darwin/Assets.car` on a working Xcode environment. Do not fold that implementation into this verification task unless the manual check proves a user-facing icon/name gap that needs an additional follow-up.

## Constraints

- This is a manual macOS verification task only; do not change code, assets, docs, config, or generated files while completing it.
- Verify the packaged `.app` bundle, not only `task dev` or the raw `gui/bin/tb-gui` binary.
- Record macOS version, build/package command, resulting app path, and any Xcode/Wails/codesign blockers in the task log.
- Capture enough evidence for review: screenshots or precise notes for the window title bar, system menu-bar app name, Cmd-Tab switcher, Dock tile, and Finder Get Info.
- If any checked surface is wrong or unclear, create a standalone backlog follow-up and link it here; keep TB-248 focused on verification.
- Treat missing `Assets.car` as the known TB-246 issue if `icons.icns` fallback still displays the expected user-facing icon.

## Related Tasks

- **TB-202** — prerequisite: implemented the `Task Board Tools` name/icon change and left the manual macOS verification AC unchecked.
- **TB-246** — sibling follow-up: regenerate `gui/build/darwin/Assets.car` on a working Xcode environment if the asset-catalog gap still matters.

## Acceptance Criteria

- [ ] Build/package the macOS GUI from `gui/` with `task package` or an equivalent Wails package/build command, then record the exact command, macOS version, and resulting `.app` path.
- [ ] Launch the packaged `.app` bundle and confirm the main window title/titlebar shows `Task Board Tools`.
- [ ] Confirm the macOS system menu-bar application name, including the app menu immediately left of `File`, shows `Task Board Tools` while the packaged app is active.
- [ ] Confirm the Cmd-Tab app switcher shows `Task Board Tools` with the kanban icon.
- [ ] Confirm the Dock tile shows the kanban icon and the expected `Task Board Tools` display name/tooltip.
- [ ] Confirm Finder Get Info for the built `.app` shows the expected display name/product metadata and kanban icon.
- [ ] Manual test note: close and relaunch the packaged app once to catch cached Finder/Dock icon or name behavior; if clearing LaunchServices/icon cache is required, record the exact steps.
- [ ] For every incorrect or missing surface, create a standalone backlog follow-up task and link it under Related Tasks or in the log; if all surfaces pass, explicitly log that no follow-ups were needed.
- [ ] Reference this verification evidence when closing out TB-202's previously unchecked manual macOS test note.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited tags=branding,verification,macos,testing, goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-19: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Moved to done

