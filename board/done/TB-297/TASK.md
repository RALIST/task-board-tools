# TB-297: Remove auto-implement filter from settings UI

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** auto-implement,settings,ux,frontend
**GroomedBy:** codex
**GroomStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewedBy:** codex
**ReviewStatus:** success
**ReviewRef:** ce50c51
**Branch:** —

## Goal

Remove the saved auto-implement filter controls from the Settings panel so Settings only owns the auto-implement enable/disable preference, while the board FilterBar remains the only place users save or change the auto-implement query.

## Context

- Current UI: `gui/frontend/src/lib/components/SettingsPanel.svelte` renders an `Auto-implement filter` summary, `No filter saved` placeholder, `Edit in board filter` button, and helper copy that points back to the FilterBar.
- Current source of truth for editing/saving the filter: `gui/frontend/src/lib/components/FilterBar.svelte` has the `data-testid="save-as-auto-implement"` control and saved-state affordance.
- TB-288 replaced the old freeform text query with a structured FilterBar-backed query. This task is the follow-up that removes the remaining filter display/shortcut from Settings rather than changing storage or daemon behavior.
- Product direction from `docs/PROJECT.md`: keep the GUI simple and avoid settings rabbit holes; Settings should not duplicate normal board-filter UI.

## Constraints

- Do not remove `AutoImplementFilter` storage, preferences APIs, backend validation, daemon candidate filtering, or the FilterBar `Save as auto-implement` flow.
- Keep auto-implement opt-in and gated by the existing prerequisites: supported default agent plus a non-empty saved filter.
- Keep the Settings auto-implement toggle and inline prerequisite warnings, but remove the filter summary/edit UI from Settings.
- Scope should stay frontend-only unless an existing test fixture needs a small adjustment to match the UI removal.

## Acceptance Criteria

- [x] `SettingsPanel.svelte` no longer renders the auto-implement filter summary, `No filter saved` placeholder, `Edit in board filter` button, or helper copy telling users to build the filter from Settings.
- [x] Settings still renders the auto-implement enable toggle and prerequisite warnings for missing default agent and missing saved filter; enabling auto-implement with no saved filter still fails safely.
- [x] `FilterBar.svelte` remains the only UI for changing the saved auto-implement query; its `Save as auto-implement` button and saved-state affordance still work.
- [x] Frontend tests were updated: deleted SettingsPanel summary/edit-shortcut assertions, preserved toggle/prerequisite-warning coverage, and preserved FilterBar save-as-auto-implement coverage.
- [x] Verification passed: `cd gui/frontend && npm run check`, `cd gui/frontend && npm run lint`, and `cd gui/frontend && npm test -- --run`.
- [x] Manual test note: desktop Settings smoke was not rerun in this API/headless session after the full-suite fix; `SettingsPanel.test.ts` and `FilterBar.save.test.ts` cover the removed controls, toggle/prerequisite warnings, and FilterBar saved-filter path.

## User Attention

**Resolved:** The previous full-frontend blocker was cleared by the TB-252 resume-gating fixes. `cd gui/frontend && npm run check`, `npm run lint`, and `npm test -- --run` now pass in this worktree; the remaining desktop-only smoke note is recorded in Acceptance Criteria.

## Related Tasks

- **TB-288** — Replaced the auto-implement text DSL with a structured FilterBar-backed query and introduced the current SettingsPanel summary/edit shortcut this task removes.
- **TB-177** — Parent auto-implement epic; this task must not change persisted query semantics or daemon candidate selection.
- **TB-180** — Original auto-implement settings/feedback UI; this task narrows that UI now that FilterBar owns query editing.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited type=improvement, size=S, module=gui, tags=auto-implement,settings,ux,frontend, goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Edited type=tech-debt
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited user-attention
- 2026-05-20: Edited agentstatus=needs-user
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=codex, review-status=success, reviewref=ce50c51, acceptance
- 2026-05-20: Edited user-attention
- 2026-05-20: Done
