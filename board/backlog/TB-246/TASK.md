# TB-246: Regenerate darwin/Assets.car on working Xcode env

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** branding,build,macos,verification,quick-win
**Agent:** codex
**AgentStatus:** success
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Regenerate and commit the missing macOS asset catalog so packaged `tb-gui` builds include `gui/build/darwin/Assets.car` again while preserving the Task Board Tools icon sources from TB-202.

## Context

TB-202 refreshed the GUI display name and project kanban icon, but deleted `gui/build/darwin/Assets.car` because the available macOS/Xcode environment failed while loading `actool` plugins. The fallback `gui/build/darwin/icons.icns` remains present, so this is not a runtime blocker, but Wails icon generation is intended to produce both `icons.icns` and `Assets.car` for modern macOS packages.

Relevant paths:
- `gui/build/Taskfile.yml` task `common:generate:icons` runs `wails3 generate icons -input appicon.png -macfilename darwin/icons.icns -windowsfilename windows/icon.ico -iconcomposerinput appicon.icon -macassetdir darwin`.
- `gui/build/appicon.icon/Assets/tb_kanban_vector.svg` and `gui/build/appicon.png` are the current project-specific source icon assets.
- `gui/build/darwin/icons.icns` is the current macOS fallback icon output.
- `gui/build/config.yml` documents the optional `cfBundleIconName` behavior when `Assets.car` exists.

## Constraints and Non-goals

- Use a working macOS Xcode/Command Line Tools environment where `actool`/Icon Composer plugin loading succeeds.
- Prefer `cd gui && wails3 task common:generate:icons`; only use a direct `actool`/Icon Composer invocation against `gui/build/appicon.icon` if the Wails wrapper is the blocker, and record the exact command.
- Keep the scope to regenerating/committing the macOS asset catalog and any icon outputs that the same deterministic generation command legitimately updates.
- Do not redesign the icon, rename the app, change Wails build configuration, or redo TB-248's full manual branding verification.
- If the generator unexpectedly rewrites unrelated platform assets or source icon files, inspect the diff and either explain why the churn is required or create a separate follow-up instead of folding unrelated changes into this task.

## Related Tasks

- **TB-202** — prerequisite: implemented the Task Board Tools name/icon rebrand and left this `Assets.car` follow-up because the active Xcode environment could not generate it.
- **TB-248** — sibling verification: manually checks the packaged macOS app's name/icon surfaces; this task should stay focused on restoring the asset catalog.

## Acceptance Criteria

- [ ] On a macOS machine with working Xcode/Command Line Tools, regenerate icons from `gui/` with `wails3 task common:generate:icons`, or record the exact direct `actool`/Icon Composer command used against `gui/build/appicon.icon` if the Wails wrapper cannot be used.
- [ ] `gui/build/darwin/Assets.car` exists after regeneration and is committed alongside any deterministic icon-output changes from the same generation command.
- [ ] The regenerated diff does not reintroduce the default Wails icon identity and still uses the project kanban icon sources in `gui/build/appicon.icon/Assets/tb_kanban_vector.svg` and `gui/build/appicon.png`.
- [ ] Build or package the macOS GUI from `gui/` with `task package`, `wails3 build -config ./build/config.yml`, or an equivalent packaging command, then confirm the resulting `.app` includes the regenerated asset catalog or document the exact packaged resource path Wails uses.
- [ ] Manual test note: launch the packaged macOS `.app` once and confirm the Dock/Finder/app-switcher icon still shows the Task Board Tools kanban icon; defer any broader name/icon surface audit to TB-248.
- [ ] Record the macOS version, Xcode or Command Line Tools version, icon-generation command, packaging command, and verification result in the task log before moving the task to done.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited tags=branding,build,macos,verification,quick-win, goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-19: Edited agentstatus=success, groomed-by=codex, groom-status=success

