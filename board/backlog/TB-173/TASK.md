# TB-173: GUI: persist auto-groom setting and toggle

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** auto-groom,settings,frontend,backend
**Branch:** —
**Parent:** TB-172

## Goal

Add a persisted `auto_groom_enabled` preference and Settings panel toggle that controls whether the GUI may start automatic grooming; default is disabled.

## Context

- `gui/app/preferences.go` persists settings in `$XDG_CONFIG_HOME/tb-gui/preferences.json`; TB-76 added `max_workers`, `agent_timeout_minutes`, `default_agent`, and `cli_path`.
- `gui/frontend/src/lib/stores/preferences.ts` and `gui/frontend/src/lib/components/SettingsPanel.svelte` are the frontend settings path from TB-80/TB-81.
- `default_agent` is already the runner fallback for manual Run/Groom in `TaskDrawer.svelte`; auto-groom must require a real `claude|codex` default before claiming it can run.

**Constraints / non-goals**

- Default `auto_groom_enabled` is false for existing users.
- This task only stores and displays the setting. Do not enqueue tasks or change daemon behavior here; that is TB-174.
- Keep `preferences.json` backwards compatible and tolerate missing/unknown fields like existing preferences do.

## Acceptance Criteria

- [ ] `Preferences` includes `AutoGroomEnabled bool` with JSON key `auto_groom_enabled`; missing field reads as `false`; `SettingsService` exposes `GetAutoGroomEnabled() bool` and `SetAutoGroomEnabled(bool) error`.
- [ ] Go tests cover missing-file default, true/false round trip, and partial/corrupt preference files without regressing existing `max_workers`, timeout, default-agent, or CLI-path normalization.
- [ ] Wails/API wrappers and `preferencesStore` expose `autoGroomEnabled` plus a setter; store tests cover load, optimistic save, and rollback on setter failure.
- [ ] `SettingsPanel.svelte` renders an `Enable auto groom` toggle with normal dirty-state/save/toast behavior and inline guidance when it is enabled while `default_agent` is `none`.
- [ ] Verification passes: `cd gui && go test ./...`; `cd gui/frontend && npm run check`; `cd gui/frontend && npm test -- --run`.
- [ ] Manual test: open Settings, toggle `Enable auto groom`, save, close/reopen Settings, confirm the value persists; set default agent to `none` and confirm the panel tells the user to set a default agent before automation can run.

## Related Tasks

- **TB-76** — Existing backend preferences foundation.
- **TB-80** — Existing frontend preferences store/API pattern.
- **TB-81** — Existing Settings panel this extends.
- **TB-174** — Consumes the setting to enqueue auto-groom runs.
- **TB-175** — Surfaces user-facing auto-groom state and fallback UX.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

