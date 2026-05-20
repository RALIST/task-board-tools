// AutoImplementFilter is the persisted/CLI-shaped representation of the
// auto-implement query. It uses arrays for every multi-select field so
// it maps cleanly onto `tb ls`'s comma-separated flags (TB-289). The
// frontend BoardFilter store keeps `parentEpic: string` because the
// board UI is single-epic-focus today; saving converts it into a
// one-element parents array via boardFilterToAutoImplement.
//
// This file is one-way: BoardFilter → AutoImplementFilter (save path).
// There's no inverse helper today — restoring a saved filter into the
// FilterBar isn't part of TB-288's scope and the next reader shouldn't
// add the inverse without a deliberate UX call.

import type { BoardFilter } from '$lib/stores/filter';

export interface AutoImplementFilter {
  search: string;
  types: string[];
  priorities: string[];
  modules: string[];
  sizes: string[];
  tags: string[];
  agents: string[];
  parents: string[];
}

export const emptyAutoImplementFilter: AutoImplementFilter = {
  search: '',
  types: [],
  priorities: [],
  modules: [],
  sizes: [],
  tags: [],
  agents: [],
  parents: [],
};

// boardFilterToAutoImplement serializes the FilterBar state into the
// persisted shape. `parentEpic: ''` becomes `parents: []`; anything
// else becomes a one-element parents array.
export function boardFilterToAutoImplement(f: BoardFilter): AutoImplementFilter {
  return {
    search: f.search.trim(),
    types: [...f.types],
    priorities: [...f.priorities],
    modules: [...f.modules],
    sizes: [...f.sizes],
    tags: [...f.tags],
    agents: [...f.agents],
    parents: f.parentEpic ? [f.parentEpic] : [],
  };
}

// isAutoImplementFilterEmpty mirrors the Go AutoImplementFilter.IsEmpty
// check so the FilterBar Save button, the SettingsPanel needs-filter
// warning, and the +page.svelte missing-prereqs derivation all agree
// on what "no filter saved" means.
export function isAutoImplementFilterEmpty(f: AutoImplementFilter): boolean {
  if (f.search.trim() !== '') return false;
  return (
    f.types.length === 0 &&
    f.priorities.length === 0 &&
    f.modules.length === 0 &&
    f.sizes.length === 0 &&
    f.tags.length === 0 &&
    f.agents.length === 0 &&
    f.parents.length === 0
  );
}

// isBoardFilterActive reports whether the FilterBar's current state
// holds at least one constraint — drives the Save-button enable state.
export function isBoardFilterActive(f: BoardFilter): boolean {
  return !isAutoImplementFilterEmpty(boardFilterToAutoImplement(f));
}

// autoImplementFilterEquals is the equality check that powers the
// "saved" affordance on the FilterBar Save button. Compares structural
// equality of every field including ordered array contents. Ordering
// must match the Go AutoImplementFilter.normalize() output (which only
// trims and drops empty segments, never sorts) — do not sort the
// arrays here, or the "Saved" affordance will flicker after every
// save when the in-memory FilterBar order diverges from disk.
export function autoImplementFilterEquals(a: AutoImplementFilter, b: AutoImplementFilter): boolean {
  if (a.search.trim() !== b.search.trim()) return false;
  return (
    arraysEqual(a.types, b.types) &&
    arraysEqual(a.priorities, b.priorities) &&
    arraysEqual(a.modules, b.modules) &&
    arraysEqual(a.sizes, b.sizes) &&
    arraysEqual(a.tags, b.tags) &&
    arraysEqual(a.agents, b.agents) &&
    arraysEqual(a.parents, b.parents)
  );
}

function arraysEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}

// summarize renders the persisted filter as a one-line human-readable
// summary for the SettingsPanel — "Type: bug, improvement · Module:
// gui · Tags: macos". Returns '' for an empty filter so callers can
// switch to a "no filter saved" placeholder.
export function summarizeAutoImplementFilter(f: AutoImplementFilter): string {
  const parts: string[] = [];
  if (f.search.trim() !== '') parts.push(`Search: ${f.search.trim()}`);
  if (f.types.length) parts.push(`Type: ${f.types.join(', ')}`);
  if (f.priorities.length) parts.push(`Priority: ${f.priorities.join(', ')}`);
  if (f.modules.length) parts.push(`Module: ${f.modules.join(', ')}`);
  if (f.sizes.length) parts.push(`Size: ${f.sizes.join(', ')}`);
  if (f.tags.length) parts.push(`Tags: ${f.tags.join(', ')}`);
  if (f.agents.length) parts.push(`Agent: ${f.agents.join(', ')}`);
  if (f.parents.length) parts.push(`Epic: ${f.parents.join(', ')}`);
  return parts.join(' · ');
}
