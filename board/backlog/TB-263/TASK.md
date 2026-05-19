# TB-263: GUI: persist auto-review setting and controls

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** auto-review,settings,frontend,backend
**Branch:** —
**Parent:** TB-262

## Goal

Persist an auto-review enablement preference and expose it in Settings plus a compact board-header control, sharing the same validation and default-agent guidance as the other automation stages.

## Context

- TB-172 and TB-177 already frame auto-groom and auto-implement as opt-in settings-backed stages.
- Existing preference plumbing lives in `gui/app/preferences.go`, `gui/app/settings_service.go`, `gui/frontend/src/lib/stores/preferences.ts`, and `gui/frontend/src/lib/components/SettingsPanel.svelte`.
- Existing run controls and board-header automation affordances should be reused so users can toggle stages without opening a different configuration model.

## Constraints / Non-goals

- Default value is disabled.
- The setting is independent from auto-groom and auto-implement. Enabling one stage must not implicitly enable another.
- Enabling requires a valid `default_agent`; when missing, keep the preference disabled or surface a typed validation result without mutating queued task state.
- No daemon candidate-selection logic in this task; TB-264 owns queueing.

## Acceptance Criteria

- [ ] Preferences gain an `auto_review_enabled` field that persists to the existing preferences file and survives app restart.
- [ ] SettingsService/Wails bindings/frontend preferences store expose the field with the same update/error conventions as the other automation toggles.
- [ ] Settings panel renders an auto-review toggle near the auto-groom/auto-implement controls and explains missing-default-agent state through existing UI feedback patterns, not in-task prose.
- [ ] Board header exposes a compact auto-review toggle wired to the same persisted preference; Settings and header stay in sync without restart.
- [ ] Enabling auto-review with `default_agent=none` or an invalid configured agent surfaces a typed, actionable error and does not mutate any task metadata or JSONL.
- [ ] Backend tests cover default value, persistence, validation failure, successful enable/disable, and restart load.
- [ ] Frontend tests cover Settings toggle, header toggle, shared-store sync, and missing-default-agent feedback.
- [ ] Verification includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Related Tasks

- **TB-262** — Parent epic.
- **TB-264** — Consumes the setting to enqueue review-mode daemon runs.
- **TB-265** — Surfaces runtime feedback and manual fallback.
- **TB-172** — Existing auto-groom setting pattern.
- **TB-177** — Existing auto-implement setting/query pattern.

## Attachments

## Log

- 2026-05-19: Created
