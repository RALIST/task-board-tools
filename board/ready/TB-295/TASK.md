# TB-295: GUI settings panel mirrors project .tb.yaml

**Type:** feature
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** settings,config,frontend,parent-tb292
**Branch:** —
**Parent:** TB-292

## Goal

Update the GUI Settings experience so project-scoped settings visibly mirror the active project's `.tb.yaml` values and save back to that file through SettingsService.

## Context

- `gui/frontend/src/lib/stores/preferences.ts` currently hydrates once from SettingsService and keeps a process-wide loaded flag.
- `gui/frontend/src/lib/components/SettingsPanel.svelte` renders and saves max workers, agent timeout, default agent, periodic recovery, auto-groom, and auto-implement controls from that store.
- The topbar automation toggles and drawer default-agent fallback also consume the same settings store, so they must stay in sync when board/project settings change.
- TB-288 may replace the auto-implement text query with a structured FilterBar-backed shape; this task should consume whichever shape the backend exposes when it lands.

## Constraints

- Settings must be project-aware: switching boards or reopening a project must reload the active `.tb.yaml` values rather than keeping stale values from the previous project.
- Keep user-local controls separate from project controls. Recent boards are not part of this panel, and `cli_path` should remain clearly user-local unless TB-293 reclassifies it.
- Preserve existing Settings save/rollback/toast behavior and inline validation feedback.
- Include a manual-test note because this is UI/UX work.

## Related Tasks

- **TB-292** — Parent epic.
- **TB-293** — `.tb.yaml` schema/template.
- **TB-294** — SettingsService backend persistence.
- **TB-288 / TB-289** — Auto-implement query/filter UI shape dependency.

## Acceptance Criteria

- [ ] Opening Settings after a board is opened shows the project settings from that board's `.tb.yaml`; switching to another board refreshes the visible values and automation toggles to that board's config.
- [ ] Saving project settings through Settings updates `.tb.yaml` through SettingsService, rolls back the UI on backend failure, and keeps existing toast/error behavior.
- [ ] Header quick toggles, default-agent fallback, auto-groom state, and auto-implement state stay synchronized with the same project settings store after load, save, and board switch.
- [ ] User-local controls are visually and behaviorally separated from project settings, and no stale `preferences.json` project values are shown as if they came from the active board.
- [ ] Frontend tests cover settings reload on board switch, save/rollback behavior, automation toggle sync, and the auto-implement query/filter shape exposed by the backend.
- [ ] Manual test note: run the GUI, open Project A and Project B with different `.tb.yaml` settings, confirm Settings and header toggles change on switch, hand-edit Project A's `.tb.yaml` then reopen/reload it and confirm the GUI reflects the file, save from Settings and verify the file contents change.
- [ ] Verification includes `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- --run`.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
