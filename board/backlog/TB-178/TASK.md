# TB-178: GUI: persist auto-implement settings and query

**Type:** feature
**Priority:** P0
**Size:** M
**Module:** gui
**Tags:** auto-implement,settings,frontend
**Branch:** —
**Parent:** TB-177

## Goal

Persist the opt-in auto-implement settings needed by the GUI before any daemon scheduling exists.

## Context

- Backend preferences live in `gui/app/preferences.go` and are exposed by `gui/app/settings_service.go`.
- Frontend settings flow lives in `gui/frontend/src/lib/stores/preferences.ts`, `gui/frontend/src/lib/api.ts`, and `gui/frontend/src/lib/components/SettingsPanel.svelte`.
- Board filter semantics live in `gui/frontend/src/lib/stores/filter.ts` and `gui/frontend/src/lib/filtering.ts`; auto-implement query matching should use the same fields users already understand, plus explicit size support from task metadata.

**Constraints / non-goals**

- This task does not start agent runs or change daemon scheduling; it only persists, validates, and exposes configuration.
- Auto-implement is off by default for new and existing preference files.
- The GUI must not allow `auto_implement_enabled=true` unless `default_agent` is `claude` or `codex` and the query is non-empty and valid.
- Query parsing should be deterministic and testable; avoid ad-hoc substring checks in daemon code.

## Related Tasks

- **TB-177** — parent epic.
- **TB-179** — consumes these settings for daemon candidate selection.
- **TB-180** — renders the user-facing controls and feedback around these settings.

## Acceptance Criteria

- [ ] `Preferences` persists `auto_implement_enabled` and `auto_implement_query` in `preferences.json`, with missing/corrupt values normalized to disabled + empty query.
- [ ] `SettingsService` exposes typed get/set methods for both values, and enabling fails without mutating preferences when `default_agent=none` or the query is blank/invalid.
- [ ] A shared query parser/matcher supports at least type, priority, size, module, tag, agent, parent epic, and free-text title/id terms; `bug, S size, gui` matches S-sized GUI bugs and rejects non-S, non-GUI, or non-bug tasks.
- [ ] `gui/frontend/src/lib/stores/preferences.ts` and `gui/frontend/src/lib/api.ts` include the new fields and optimistic writes with rollback/toast behavior matching existing settings.
- [ ] Tests cover default preferences, round-trip persistence, validation failures, parser examples, and frontend store load/save behavior.
- [ ] Verification includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-19: Moved to code-review
- 2026-05-19: Moved to done
- 2026-05-19: Moved to backlog

