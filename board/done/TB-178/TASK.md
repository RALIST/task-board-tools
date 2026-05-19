# TB-178: GUI: persist auto-implement settings and query

**Type:** feature
**Priority:** P0
**Size:** M
**Module:** gui
**Tags:** auto-implement,settings,frontend
**ImplementedBy:** claude
**ImplementStatus:** success
**ReviewRef:** TB-178 ships in next commit
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

## Review Target

- gui/internal/automation/query/query.go (new): shared parser/matcher.
- gui/internal/automation/query/query_test.go: AC fixture "bug, S size, gui" + negatives, explicit fields, invalid enums, free-text, unknown-field-fallback contract.
- gui/app/preferences.go: AutoImplementEnabled/Query fields, errors, Get/Set/Validate, transactional updatePreferencesWithValidator (TOCTOU fix), AutoImplementController interface, SetDefaultAgent now notifies both controllers.
- gui/app/preferences_test.go: 9 tests covering round-trip, validation gates, blank-while-enabled, invalid query, validator, missing-file defaults, and the TOCTOU revalidation test.
- gui/frontend/src/lib/api.ts: 5 new bindings (Get/Set Enabled, Get/Set Query, Validate).
- gui/frontend/src/lib/stores/preferences.ts: AutoImplementEnabled/Query in state, hydration + setters + validate proxy.
- gui/frontend/src/lib/stores/preferences.test.ts: hydration / round-trip / trim / rollback / validator-proxy tests.
- gui/frontend/src/lib/components/SettingsPanel.svelte: extend baseline + toEditable so types compile (auto-implement UI lands in TB-180).
- gui/frontend/bindings/tools/tb-gui/app/settingsservice.ts: regenerated Wails bindings include the 5 new methods.

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
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited implemented-by=claude, implement-status=success, reviewref=TB-178 ships in next commit
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited review-target
- 2026-05-20: Done

