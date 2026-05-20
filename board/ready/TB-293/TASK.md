# TB-293: Config: add GUI project settings to .tb.yaml schema

**Type:** feature
**Priority:** P2
**Size:** M
**Module:** cli
**Tags:** settings,config,parent-tb292
**Branch:** —
**Parent:** TB-292

## Goal

Define the `.tb.yaml` project-settings schema that the GUI can use as the single source of truth for project-scoped settings.

## Context

- `.tb.yaml` currently owns board config such as `board`, `prefix`, WIP limits, `wip_enforcement`, and `scan_extensions`.
- `cli/config_template.go` renders the annotated config that `tb init` writes or reconciles.
- `cli/board.go` parses `.tb.yaml` with a minimal `key: value` reader and preserves the CLI defaults for board discovery and WIP behavior.
- GUI project settings currently live in `$XDG_CONFIG_HOME/tb-gui/preferences.json` via `gui/app/preferences.go`: `max_workers`, `agent_timeout_minutes`, `default_agent`, `disable_periodic_recovery`, `auto_groom_enabled`, `auto_groom_settle_minutes`, `auto_implement_enabled`, and the auto-implement query/filter shape.
- `cli_path` and recent boards are machine/user-local state, not project settings, unless a later product decision explicitly reclassifies them.

## Constraints

- Keep existing `.tb.yaml` behavior backwards compatible: current boards still load, unknown fields are preserved by `tb init`, and byte-identical config refreshes stay no-op.
- Match the validation/default semantics already used by the GUI preferences path: worker count 1-4, agent timeout 1-240 minutes, default agent `none|claude|codex`, auto-groom settle window 0-60 minutes, automation toggles off by default, periodic recovery enabled by default.
- Coordinate with TB-288/TB-289 if they land first: use the current structured auto-implement filter shape rather than restoring the old text DSL.

## Related Tasks

- **TB-292** — Parent epic for moving project-scoped GUI settings into `.tb.yaml`.
- **TB-294** — GUI backend persistence consumes this schema.
- **TB-295** — Settings panel mirrors the resulting project config.
- **TB-288 / TB-289** — May change the auto-implement query storage shape before this lands.

## Acceptance Criteria

- [ ] `.tb.yaml` has documented project-setting keys for the GUI-owned project settings listed in the task context, with explicit defaults and validation ranges.
- [ ] `tb init` renders those keys in the annotated config template and still preserves unknown existing fields, active values, `.bak` behavior, and byte-identical no-op refresh behavior.
- [ ] CLI/config tests cover minimal existing config expansion, active project-setting values, invalid value fallback/normalization, and unknown-key preservation.
- [ ] Docs that describe `.tb.yaml` and settings classify project-scoped vs user-local settings so future GUI settings do not drift back into a separate project preference file.
- [ ] Verification includes `cd cli && go test ./...`.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
