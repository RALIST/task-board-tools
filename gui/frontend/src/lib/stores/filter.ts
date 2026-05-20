// Filter store: holds the FilterBar's selection plus the "Show archived"
// toggle. Multi-select filters are arrays — empty = no constraint, populated
// = OR within the category. Categories AND together.

import { writable } from 'svelte/store';

export interface BoardFilter {
  search: string;
  types: string[];
  priorities: string[];
  modules: string[];
  sizes: string[];
  tags: string[];
  agents: string[];
  parentEpic: string; // single parent ID, '' = no constraint
  showArchive: boolean;
}

export const initialFilter: BoardFilter = {
  search: '',
  types: [],
  priorities: [],
  modules: [],
  sizes: [],
  tags: [],
  agents: [],
  parentEpic: '',
  showArchive: false,
};

export const filter = writable<BoardFilter>({ ...initialFilter });

// clearFilter resets every constraint but keeps the showArchive toggle —
// users expect the archive column to stay visible while they re-filter.
export function clearFilter(): void {
  filter.update((f) => ({ ...initialFilter, showArchive: f.showArchive }));
}
