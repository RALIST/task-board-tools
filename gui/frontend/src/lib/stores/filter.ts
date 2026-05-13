// Filter store — placeholder for M2. M3 will wire FilterBar into this and
// add tag/module/priority predicates. The shape is intentionally minimal so
// stores/board.ts doesn't have to import it yet.

import { writable } from 'svelte/store';

export interface BoardFilter {
  search: string;
  showArchive: boolean;
}

export const filter = writable<BoardFilter>({ search: '', showArchive: false });
