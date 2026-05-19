# TB-180: GUI: show auto-implement controls and feedback

**Type:** feature
**Priority:** P0
**Size:** S
**Module:** gui
**Tags:** auto-implement,ux,frontend
**ImplementedBy:** claude
**ImplementStatus:** success
**ReviewRef:** TB-180 ships in next commit
**Branch:** —
**Parent:** TB-177

## Goal

Surface auto-implement state, validation feedback, and a compact quick toggle while keeping Settings as the source of truth.

## Context

- Settings UI lives in `gui/frontend/src/lib/components/SettingsPanel.svelte` and preferences flow in `gui/frontend/src/lib/stores/preferences.ts`.
- The header/topbar is composed in `gui/frontend/src/routes/+page.svelte`, near Settings and `AgentUsageHeader`.
- Run state and task drawer controls already come from `runsStore`, `TaskDrawer.svelte`, and the existing Agent run events.
- Backend settings and daemon behavior come from **TB-178** and **TB-179**.

**Constraints / non-goals**

- Depends on **TB-178** and **TB-179**.
- Settings remains the canonical edit surface for the query, default-agent prerequisite, and persisted enabled state.
- The header quick toggle must only mirror and update the persisted setting; it must not own separate local state or bypass validation.
- Manual Run and Groom actions remain available and understandable when auto-implement is disabled, invalid, or currently running another task.

## Review Target

- gui/frontend/src/lib/components/SettingsPanel.svelte: auto-implement toggle + query input + three inline warnings (needs-default-agent / needs-query / parser-error). The parser-error path goes through a 250ms-debounced `preferencesStore.validateAutoImplementQuery` proxy with a token guard against out-of-order responses. Query write is ordered before the enable write in `save()` so the backend validator sees the freshly typed query.
- gui/frontend/src/routes/+page.svelte: header "Auto-impl" pill mirroring the existing "Auto-groom" pill. Shares the .auto-groom-toggle CSS so the compact layout doesn't overflow. When prereqs are missing AND the toggle is currently off, clicking opens Settings instead of trying to enable (the backend would reject anyway).
- gui/frontend/src/lib/components/SettingsPanel.test.ts (new, 4 tests): asserts the three inline warnings render under the right conditions and clear once prereqs are met.
- Manual Run / Groom controls remain unchanged.

## Related Tasks

- **TB-177** — parent epic.
- **TB-178** — settings/query persistence.
- **TB-179** — daemon queueing behavior.

## Acceptance Criteria

- [ ] Settings shows an auto-implement toggle, query input/summary, and clear validation feedback when default agent is `none` or the query is blank/invalid.
- [ ] Saving Settings persists the auto-implement fields through the existing preferences store and displays success/error feedback consistent with the rest of the panel.
- [ ] The header includes a compact quick toggle that reflects the persisted enabled state, disables or surfaces an actionable message when prerequisites are missing, and opens Settings for query changes.
- [ ] Auto-started implementation runs are visibly indistinguishable from normal implement runs except for copy/state that makes clear they were started by auto-implement; run log, status pill, and cancel behavior still work.
- [ ] Manual Run and Groom controls remain available and do not become hidden by auto-implement state.
- [ ] Frontend tests cover Settings validation rendering, preferences store updates, header toggle disabled/enabled behavior, and no text overflow at the compact header control.
- [ ] Manual test note: toggle auto-implement off/on in Settings, try enabling with `default_agent=none`, set query `bug, S size, gui`, use the header quick toggle, watch an eligible task auto-start, cancel it, and confirm manual Run/Groom still work.
- [ ] Verification includes `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- --run`.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-19: Moved to code-review
- 2026-05-19: Moved to backlog
- 2026-05-19: Moved to code-review
- 2026-05-19: Moved to in-progress
- 2026-05-19: Moved to backlog
- 2026-05-19: Moved to code-review
- 2026-05-19: Moved to backlog
- 2026-05-19: Moved to code-review
- 2026-05-19: Moved to in-progress
- 2026-05-19: Moved to backlog
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited implemented-by=claude, implement-status=success, reviewref=TB-180 ships in next commit
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited review-target
- 2026-05-20: Done

