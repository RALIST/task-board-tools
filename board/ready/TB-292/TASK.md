# TB-292: Epic: mirror project settings between GUI and .tb.yaml

**Type:** feature
**Priority:** P2
**Size:** XL
**Agent:** codex
**Tags:** settings,config,epic
**Module:** gui
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Make `.tb.yaml` the editable source of truth for project-scoped tb GUI settings, with the GUI Settings panel reading from and saving to the same project config file instead of a separate project-preference store.

## Context

- Users can already hand-edit `.tb.yaml` for board-level config (`board`, `prefix`, WIP limits, `wip_enforcement`, `scan_extensions`), and `tb init` reconciles that file through the annotated CLI template.
- The GUI currently stores project-like settings in `$XDG_CONFIG_HOME/tb-gui/preferences.json` through `gui/app/preferences.go` and `SettingsService`: `max_workers`, `agent_timeout_minutes`, `default_agent`, `disable_periodic_recovery`, `auto_groom_enabled`, `auto_groom_settle_minutes`, `auto_implement_enabled`, and auto-implement query/filter state.
- The Settings panel (`gui/frontend/src/lib/components/SettingsPanel.svelte`) and preferences store (`gui/frontend/src/lib/stores/preferences.ts`) mirror those backend getters/setters today.
- Keep user-local app state out of project config: recent boards stay in `recent.json`; `cli_path` is machine-specific and should remain user-local unless a child task explicitly reclassifies it.

## Constraints

- Do not break existing boards or hand-edited `.tb.yaml` files. Unknown config fields must remain preserved, invalid values must fall back safely, and current `tb init` no-op/backup behavior must remain intact.
- The GUI must not leak settings between boards. Opening or switching projects should make Settings reflect that project's `.tb.yaml` values.
- Existing automation behavior remains staged and opt-in: auto-groom, auto-implement, and future auto-review stay independently controlled.
- Coordinate with TB-288/TB-289 for the auto-implement filter shape; do not reintroduce the old text DSL if the structured filter work lands first.

## Child Tasks

- **TB-293** — Config: add GUI project settings to `.tb.yaml` schema.
- **TB-294** — GUI backend: persist project settings in `.tb.yaml`.
- **TB-295** — GUI settings panel mirrors project `.tb.yaml`.

## Related Tasks

- **TB-56 / TB-76 / TB-81** — Original `preferences.json` and Settings panel path for max workers, timeout, default agent, and CLI path.
- **TB-172 / TB-177** — Auto-groom and auto-implement settings consumers.
- **TB-288 / TB-289** — Structured auto-implement filter follow-up that may change the stored query shape.

## Subtasks

- **TB-293** (M) — Config: add GUI project settings to .tb.yaml schema
- **TB-294** (M) — GUI backend: persist project settings in .tb.yaml
- **TB-295** (M) — GUI settings panel mirrors project .tb.yaml
## Acceptance Criteria

- [ ] **TB-293** is done: `.tb.yaml` has a documented, backwards-compatible project-settings schema and `tb init` renders/reconciles it without losing unknown fields or current config behavior.
- [ ] **TB-294** is done: SettingsService reads/writes project-scoped settings from the active project's `.tb.yaml`, handles legacy `preferences.json` values safely, and preserves existing automation notifications.
- [ ] **TB-295** is done: the Settings UI reloads project settings on board switch, saves through SettingsService to `.tb.yaml`, and keeps header/automation controls synchronized.
- [ ] User-local state remains user-local: recent boards are not moved into `.tb.yaml`, and `cli_path` is either left in user preferences or explicitly reclassified with rationale in TB-293.
- [ ] Docs in `docs/ARCHITECTURE.md`, `docs/FEATURES.md`, `docs/IMPLEMENTATION.md`, and README mention the new project-settings source of truth and the remaining user-local settings.
- [ ] Manual test note: open the GUI against two projects with different `.tb.yaml` project settings, verify Settings/header values change per project, hand-edit one `.tb.yaml` and reload/reopen it, then save from Settings and confirm the file changes.
- [ ] Verification covers the affected layers: `cd cli && go test ./...`, `cd gui && go test ./app/... ./internal/...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited type=feature, size=XL, module=gui, tags=settings,config,epic, title=Epic: mirror project settings between GUI and .tb.yaml
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Committed — moved to ready
