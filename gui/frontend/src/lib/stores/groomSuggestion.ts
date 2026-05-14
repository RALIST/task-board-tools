import { get, writable } from 'svelte/store';

export const groomSuggestedFor = writable<string | null>(null);

export function suggestGroom(id: string): void {
  groomSuggestedFor.set(id);
}

export function consumeGroomSuggestion(id: string): boolean {
  if (get(groomSuggestedFor) !== id) return false;
  groomSuggestedFor.set(null);
  return true;
}
