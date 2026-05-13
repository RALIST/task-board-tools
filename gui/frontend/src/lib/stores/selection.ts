// Selection store — tracks which task is open in the drawer. Null = drawer
// closed. Persisting selection across reloads is intentionally not done
// here — it can be added later by hashing on the route.

import { writable } from 'svelte/store';

export const selectedTaskId = writable<string | null>(null);

export function openTask(id: string): void {
  selectedTaskId.set(id);
}

export function closeTask(): void {
  selectedTaskId.set(null);
}
