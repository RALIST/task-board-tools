# TB-294: GUI backend: persist project settings in .tb.yaml

**Type:** feature
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** settings,config,parent-tb292
**Branch:** —
**Parent:** TB-292

## Goal

Move the GUI backend's project-scoped settings persistence from `$XDG_CONFIG_HOME/tb-gui/preferences.json` into the active project's `.tb.yaml`, using the schema from TB-293.

## Context

- `gui/app/preferences.go` currently backs SettingsService getters/setters for `max_workers`, `agent_timeout_minutes`, `default_agent`, `disable_periodic_recovery`, `auto_groom_enabled`, `auto_groom_settle_minutes`, `auto_implement_enabled`, and auto-implement query/filter state.
- `gui/app/settings_service.go` already reads `<projectRoot>/.tb.yaml` in `readBoardInfo`, but only for board path, prefix, and legacy WIP info.
- Settings changes notify the daemon/automation coordinators today; those notifications must keep working after storage moves.
- Existing `preferences.json` values need a safe compatibility path so users do not silently lose their current GUI settings.

## Constraints

- Do not move user-local state into `.tb.yaml`: recent boards stay in `recent.json`; `cli_path` should remain per-user/per-machine unless TB-293 explicitly reclassifies it.
- Save `.tb.yaml` atomically and preserve unrelated keys and generated config comments as far as the current config renderer/parser contract allows.
- Missing or invalid project settings should fall back to the same defaults/clamps the GUI uses today, with warnings rather than startup-breaking errors.
- Board switching must isolate settings by project root; values from one board must not leak into another board.

## Related Tasks

- **TB-292** — Parent epic.
- **TB-293** — Defines the `.tb.yaml` schema and template contract.
- **TB-295** — Frontend settings panel/store work that consumes this backend surface.

## Acceptance Criteria

- [ ] `SettingsService` project-scoped getters/setters read from and write to the active project's `.tb.yaml`; no project setting is newly persisted to `preferences.json`.
- [ ] Legacy `preferences.json` values are handled without data loss: missing `.tb.yaml` keys use compatible defaults or a documented one-time fallback/migration path, and user-local fields remain untouched.
- [ ] Saving any project setting writes `.tb.yaml` atomically, preserves unrelated config keys, and keeps board-open validation, watcher switching, daemon activation, and recents behavior unchanged.
- [ ] Automation notifications still fire after setting changes: default-agent, auto-groom, auto-implement, and periodic recovery changes are reflected by the existing coordinators without requiring an app restart except where current behavior already requires one (`max_workers`).
- [ ] Backend tests cover per-board isolation, legacy fallback/migration, invalid value normalization, atomic write failure behavior, and no regression to `OpenBoard` missing-`.tb.yaml` / watcher-failure rollback semantics.
- [ ] Verification includes `cd gui && go test ./app/... ./internal/...`.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
