# TB-288: FilterBar-driven auto-implement query (replaces text DSL)

**Type:** feature
**Priority:** P2
**Size:** L
**Module:** gui
**Tags:** auto-implement,ux,filter,frontend,parent-tb177
**Branch:** —

## Goal

Replace the freeform text-based auto-implement query with a structured filter that reuses the existing FilterBar UI. The user already has a multi-select filter component on the board (search, type, priority, module, tags, agent, epic, archived); auto-implement should consume the same data shape via a "Save as auto-implement query" button rather than asking the user to learn a text DSL.

The text-format parser introduced by TB-178 ships in M11 and has not had time to grow real-user data, so this task replaces it cleanly rather than maintaining both representations.

## Acceptance Criteria

- [ ] **Storage shape**: `Preferences.AutoImplementQuery` changes from `string` to a structured `AutoImplementFilter` JSON object mirroring the frontend `BoardFilter` shape: `{search: string, types: []string, priorities: []string, modules: []string, tags: []string, agents: []string, parents: []string, archived: false}`. Old `string` values in existing `preferences.json` are migrated to an empty filter on first load (or rejected with a warning, choose one and document it).
- [ ] **Backend matcher**: `gui/internal/automation/query` is deleted or rewritten as a structured matcher consuming the new `AutoImplementFilter`. Each field is treated as OR within (any value matches) and AND across fields. The existing `query.Task` projection is preserved (or migrated to a struct named to fit the new package layout).
- [ ] **Wails surface**: `SettingsService.GetAutoImplementQuery` returns the structured object; `SetAutoImplementQuery` takes the structured object. `ValidateAutoImplementQuery` is removed or replaced with a cheap "is this filter non-empty" check (the structured form can't have parser errors).
- [ ] **Frontend store**: `preferencesStore.autoImplementQuery` is the same shape as `BoardFilter`. The validateAutoImplementQuery proxy is removed.
- [ ] **FilterBar button**: a "Save as auto-implement query" button appears in the FilterBar (near the existing Clear button or directly under the chip row), enabled only when at least one filter is active. Click serializes the current `$filter` store to `preferencesStore.setAutoImplementQuery` and surfaces a success toast.
- [ ] **Saved-query indicator**: when the FilterBar's current state matches the persisted auto-implement query, the button shows a subtle "saved" state (border accent, checkmark, or similar) so the user knows nothing's drifted. Optional but recommended.
- [ ] **SettingsPanel cleanup**: the freeform text input and `Auto-implement filter` label are removed. In their place, render a read-only human-readable summary of the persisted filter (e.g., "Type: bug, improvement · Module: gui · Tags: macos") plus a "Edit in board filter" link/button that navigates to the board and focuses the FilterBar. The toggle + validation warnings (needs-default-agent / needs-query) remain.
- [ ] **Daemon coordinator**: `gui/app/auto_implement.go` swaps the `query.Match` call for the structured matcher. Existing tests update their setup to construct the structured filter; the AC fixture `bug, S size, gui` becomes `{types:[bug], modules:[gui]}` plus a separate `sizes:[S]` field (add to the BoardFilter shape if absent; current FilterBar may not expose size — confirm and add a Size dropdown in the same task if it's missing).
- [ ] **Size filter UX**: if the current FilterBar lacks a Size selector, add one alongside Type/Priority/Module (multi-select, options S/M/L/XL) so the AC example `bug, S size, gui` remains expressible. Required because the original TB-178 AC fixture covers size.
- [ ] **Migration story**: a one-line CHANGELOG note + an `internal/agent/state.go`-style guard that logs a warning when the old string-shaped query is detected on disk, then continues with an empty filter. Document the migration in `docs/ARCHITECTURE.md` if it touches an invariant.
- [ ] **Test surface**: structured matcher unit tests covering OR-within-field (e.g., `types:[bug,improvement]` matches both); AND-across-fields; empty-field-means-no-constraint; tag case-sensitivity preserved. SettingsPanel test asserts the freeform text input is gone and the summary renders. FilterBar test asserts the new button calls `setAutoImplementQuery` with the right shape.
- [ ] **Verification**: `cd gui && go test ./...`; `cd gui/frontend && npm run check`; `cd gui/frontend && npm test -- --run`; `make lint-go`.
- [ ] **Manual test note**: open Settings, observe the summary and Edit-in-board-filter button (no text input); apply a filter on the board; click Save as auto-implement query; verify the persisted filter reflects the selection; toggle auto-implement on; verify a matching ready task auto-starts; clear the filter and confirm the saved query is unchanged until explicitly re-saved.

## Non-goals
- Re-introducing the text DSL as an advanced "expert mode" view. If a power user wants raw access they can hand-edit `preferences.json`.
- Multi-saved-query slots (one saved query per board today; multi-query is a separate future task).
- Cross-board query sharing.

## Related Tasks

- **TB-177** — auto-implement epic (just closed to done). This task is a follow-up that replaces the freeform query introduced under TB-178; the rest of the epic (coordinator, epic-order helper, review-failed sort, UI toggle, header pill) stays as shipped.
- **TB-178** — the freeform query parser this task replaces. `gui/internal/automation/query` is the package to delete/rewrite.
- **TB-179** — coordinator that consumes the matcher (`gui/app/auto_implement.go`); the `query.Match` call swaps for the structured matcher.
- **TB-180** — Settings UI that needs the text input + validator-error UI replaced with a read-only summary + "Edit in board filter" link.
- **No task ID — FilterBar** — `gui/frontend/src/lib/components/FilterBar.svelte` is the UI being repurposed; `gui/frontend/src/lib/stores/filter.ts` is the `BoardFilter` shape the persisted query mirrors; `gui/frontend/src/lib/filtering.ts` is the existing matcher whose semantics the backend must preserve.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited priority=P2, type=feature, size=L, tags=auto-implement,ux,filter,frontend,parent-tb177, acceptance

