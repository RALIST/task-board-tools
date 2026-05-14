# FilterBar redesign — dropdown bar + active chips

**Status:** Draft (brainstorming output, pre-implementation)
**Date:** 2026-05-14
**Supersedes:** TB-92 (the "limit tags in header to 10" approach)
**Owner:** GUI / `gui/frontend/src/lib/components/FilterBar.svelte`

## Problem

In busy projects the GUI header is dominated by filter chips. `FilterBar.svelte` renders every distinct value for **types, priorities, modules, tags, agents, epics** inline as chips, wrap-flowing into multiple rows. In `writer-studio` (≈150 modules, ≈250 tags) the header consumes the upper half of the viewport before any task is visible.

TB-92 capped *tags only* at 10 inline + `+N more` overflow popover. The cap worked for tags, but four other chip categories have the same scaling problem — modules in particular flood the screen. The row-and-overflow pattern does not generalize because labeled rows for every category still occupies many vertical bands and labels eat horizontal space.

## Goal

Header consumes at most **two rows** regardless of project size:

- **Row 1 (always visible):** search input + one compact dropdown trigger per filter category + archived checkbox + clear.
- **Row 2 (only when filters are active):** removable chips for each currently-selected value.

Every filter remains discoverable; values are searchable inside dropdowns when the option list is long; visual scannability of the existing colored chip styles is preserved in the active-chips row.

## Non-goals

- Changing the underlying filter model (`BoardFilter` shape, store API, `filtering.ts` selectors).
- Changing card/drawer rendering or any non-header tag display.
- Adding new filter categories or new filter semantics.
- Persisting filter UI state (open/closed popovers) across reloads.

## Layout

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ [🔍 Search…] [Type ▾] [Priority (2) ▾] [Module ▾] [Tags ▾] [Agent ▾]        │
│              [Epic ▾] [☐ Archived]                                  [Clear] │ ← row 1 (always)
├─────────────────────────────────────────────────────────────────────────────┤
│ Active:  bug ×   P1 ×   P0 ×   cli ×   agent ×                              │ ← row 2 (only when ≥1 filter)
└─────────────────────────────────────────────────────────────────────────────┘
```

- Row 1 uses `flex-wrap` — on narrow windows dropdowns wrap to a second visual line; this is still strictly bounded vs today's unbounded chip flood.
- Row 2 is rendered by `ActiveFilters.svelte`; the component returns nothing when zero filters are set, so the header collapses to exactly one row at rest.
- Each `[Category ▾]` trigger displays `Label (N)` when N ≥ 1 selected, plain `Label` otherwise.
- Categories with `options.length <= 1` are hidden entirely (same guard as today's `if (types.length > 1)` checks).
- "Tags" follows the existing convention of showing as soon as `tags.length > 0`.

## Components

### `FilterDropdown.svelte` (new)

Generalizes the existing tag-overflow popover.

**Props:**

| Name | Type | Notes |
|---|---|---|
| `label` | `string` | Trigger text, e.g. "Type". |
| `options` | `string[]` | All distinct values for this category, in source-defined order (same as today). |
| `selected` | `string[]` | Currently-selected values. |
| `onToggle` | `(value: string) => void` | Called when an option is clicked. |
| `onClear?` | `() => void` | Optional; shows a "Clear" link in the popover footer when provided. |
| `single?` | `boolean` | Single-select mode for the epic dropdown. |

**Behavior:**

- Trigger button shows `label` or `label (selected.length)`; click toggles popover.
- Popover lists every option as a checkbox row (`role="menuitemcheckbox"`, `aria-checked`).
- **Multi-select (default):** clicking an option toggles its membership in `selected` via `onToggle`; popover stays open so the user can pick multiple values in one interaction.
- **Single-select (`single` prop true, used for epic):** clicking an option calls `onToggle(value)` and closes the popover. Clicking the already-selected option calls `onToggle(value)` (which clears it in the parent's handler) and closes the popover. The popover also contains a top-of-list "(any)" option whose label is configurable via an optional `nullLabel` prop (default `"(any)"`); selecting it clears the selection. Trigger shows `label` when nothing selected, `label: {value}` when one is selected (e.g. `Epic: TB-5`).
- When `options.length > 10`, a `Filter…` text input appears at the top of the popover, filtering visible options via case-insensitive substring match. The input autofocuses when the popover opens; arrow keys move focus from the input down into the option list.
- Click outside / Escape closes the popover and returns focus to the trigger.
- Keyboard: Enter/Space/ArrowDown open; ArrowUp opens with last item focused; ArrowDown/ArrowUp navigate; Home/End jump; Escape closes.
- All these behaviors (minus the new search input + single-select + autofocus rules) already exist in `FilterBar.svelte:30-148` for the tag menu; the rewrite extracts, parameterizes, and extends them.

**Boundaries:** receives raw arrays + callbacks; owns no global state. Lives next to `FilterBar.svelte`. Tested in isolation.

### `ActiveFilters.svelte` (new)

Renders row 2.

**Props:**

| Name | Type | Notes |
|---|---|---|
| `filter` | `BoardFilter` | Same shape used everywhere else. |
| `onRemove` | `(category: keyof BoardFilter, value: string) => void` | Called when a chip's × is clicked. |

**Behavior:**

- Iterates each multi-value category (types, priorities, modules, tags, agents) and renders one chip per selected value with the same class as today's category chips (`chip.pri`, `chip.tag`, `chip.mod`) so colors are preserved.
- Renders the epic chip when `filter.parentEpic` is set (single-value).
- Each chip is a `<button aria-label="Remove {label} filter {value}">{value} ×</button>`.
- A non-interactive `Active:` text label appears at the start of the row (visually distinct from the chips; not present in the chip list itself).
- The component renders nothing (no `<section>`, no wrapper, no "Active:" label) when zero filters are active, so the header has no row 2 chrome at rest.
- Does not display the free-text `search` value — the search input already shows it.

**Boundaries:** pure read of `filter`, emits one event shape. No store access.

### `FilterBar.svelte` (rewrite)

Thin composition shell:

1. Subscribes to the `filter` store as today and mirrors into local state for two-way binding (existing pattern at line 22).
2. Reads `observedValues`/`observedTags`/`observedEpics` for the snapshot.
3. Renders row 1: search input → one `<FilterDropdown>` per non-trivial category → archived checkbox → Clear button.
4. Renders `<ActiveFilters>` below row 1.
5. Owns the `onToggle` / `onRemove` handlers; both translate to the same `toggle(category, value)` mutation we already have (line 60).

Drops:

- Inline `{#each tagSelection.inline as tg}` block (lines 195-229).
- Inline chip loops for types/priorities/modules/agents (lines 174-194, 239-245).
- Local tag-menu state (`tagMenuOpen`, `tagMoreRoot`, all six keyboard/focus helpers at lines 30-148).
- Import of `selectInlineTags`.

The epic `<select>` (lines 231-238) becomes a `<FilterDropdown single>` for visual consistency. Behavior is unchanged: at most one epic selected.

## Filtering model

**Unchanged.** `BoardFilter`, `filter.set()`, `clearFilter()`, and the selectors in `filtering.ts` are not touched.

**Removed from `filtering.ts`:** `FILTER_BAR_INLINE_TAG_LIMIT`, `selectInlineTags`. Their tests in `filtering.test.ts` go with them.

## Accessibility

- Trigger button: `aria-haspopup="menu"`, `aria-expanded`, `aria-label="Filter by {label}"`.
- Popover: `role="menu"`, items `role="menuitemcheckbox"` + `aria-checked`. Same pattern the current tag menu uses (verified accessible).
- Active chips row: `aria-label="Active filters"`; each chip is `<button aria-label="Remove {category} filter {value}">`.
- Focus management: opening a popover focuses the search input if present, else the first option; closing returns focus to the trigger.
- Keyboard: arrow navigation inside popover; Esc closes; Home/End jump to first/last option.

## Visual treatment

- Trigger styled as a chip-sized button matching today's `.chip` baseline (rounded pill, same height); count badge styled inline (e.g. `Type (2)` rather than a separate badge) to avoid layout complexity.
- Popover reuses the existing `.tag-menu` styles in `FilterBar.svelte:311-326` (now lifted into `FilterDropdown.svelte`).
- Active-row chips reuse `.chip`, `.chip.pri`, `.chip.tag`, `.chip.mod` so semantics-by-color stays intact. The `×` is a small adjacent span, not a separate button — the whole chip is clickable and acts as "remove".
- Row 2, when present, sits between row 1 and the kanban columns with the same 1px border-bottom row 1 currently has.

## Tests

**New:**

- `FilterDropdown.test.ts`: opens & closes on click/keyboard; toggles selection; search input filters options; click outside closes; single-select closes on pick; trigger shows count when selected.
- `ActiveFilters.test.ts`: renders nothing when filter is empty; renders one chip per selected value across all categories; remove fires `onRemove` with correct category/value.

**Rewritten:**

- `FilterBar.test.ts`: replace the three TB-92 tag-cap tests (caps at 10, +N more, active-overflow handling) with: row-1 shows one dropdown per non-trivial category; count badge accurate; `ActiveFilters` appears only when filters set; clear-all clears every category.

**Removed:**

- `filtering.test.ts` cases for `selectInlineTags` (the function is deleted).

## Migration / TB-92 disposition

- TB-92 is currently `AgentStatus: failed` in `board/backlog/TB-92.md` because the user observed the wider header-flood problem after the tag-only cap shipped. The cap itself works for tags but does not address the broader regression.
- After this redesign lands, TB-92's row-cap behavior becomes irrelevant (no inline overflow for any category). A Log entry on TB-92 should mark it superseded by the new task created from this spec; the task file is not deleted (the failure log is useful institutional memory).

## Files changed

| File | Change |
|---|---|
| `gui/frontend/src/lib/components/FilterBar.svelte` | Rewrite to compose `FilterDropdown` + `ActiveFilters`. |
| `gui/frontend/src/lib/components/FilterDropdown.svelte` | New. |
| `gui/frontend/src/lib/components/ActiveFilters.svelte` | New. |
| `gui/frontend/src/lib/components/FilterBar.test.ts` | Drop tag-cap tests, add layout/count/clear tests. |
| `gui/frontend/src/lib/components/FilterDropdown.test.ts` | New. |
| `gui/frontend/src/lib/components/ActiveFilters.test.ts` | New. |
| `gui/frontend/src/lib/filtering.ts` | Remove `FILTER_BAR_INLINE_TAG_LIMIT`, `selectInlineTags`. |
| `gui/frontend/src/lib/filtering.test.ts` | Remove `selectInlineTags` tests. |
| `board/backlog/TB-92.md` | Add Log entry marking superseded; leave the file in backlog. |

No backend (`cli/`) changes; no `BoardFilter` shape changes; no IPC changes.

## Risks

- **Discoverability regression for power users**: today, all type/priority chips are one click away. After change, they're one click + one click. Mitigated by: count badges make active state visible without opening; search-in-popover makes finding values faster than scrolling chip rows.
- **Popover positioning**: `position: absolute; top: calc(100% + 6px)` works for the tag menu today but with 6 popovers across the row, edge cases near the viewport right edge may overflow. Plan: clamp `left` via `max(0, min(triggerLeft, viewportWidth - menuWidth))` at open time.
- **Single-select epic**: existing UX is a `<select>` element with platform-native keyboard behavior. Replacing it with a custom popover is a slight downgrade in keyboard ergonomics. Acceptable for consistency, but if this proves disruptive in dogfooding, reverting just the epic control to `<select>` is a one-line fallback.

## Open questions

None at spec time. The single point under active discussion (where to display selected values) is resolved: separate "Active" chips row.

## Acceptance criteria

- [ ] Header consumes exactly 1 row when no filters are set, exactly 2 rows when ≥1 filter is set, in `task-board-tools` and `writer-studio` projects at default window width.
- [ ] All six filter categories accessible via dropdown popovers; long option lists (>10) searchable in-popover.
- [ ] Count badge on each trigger matches `selected.length` for that category.
- [ ] Active-chips row renders one removable chip per selected value across all categories; clicking a chip removes only that value.
- [ ] Existing color/styling (priority chip color, monospace for modules/tags) preserved in active-chips row.
- [ ] Keyboard accessibility for popover open/close/navigate/select retained.
- [ ] `filtering.ts` no longer exports `selectInlineTags` or `FILTER_BAR_INLINE_TAG_LIMIT`.
- [ ] `vitest` and `svelte-check` pass; new component tests cover the cases listed in the Tests section.
- [ ] `TB-92.md` carries a Log line noting it's superseded by the new task.
