// Lightweight toast store. M3 will replace this with a Toast component +
// queue; for M2 we only need a primitive any error handler can call.

import { writable } from 'svelte/store';

export interface Toast {
  id: number;
  message: string;
  kind: 'error' | 'info';
  ts: number;
}

const _toasts = writable<Toast[]>([]);
let _nextId = 1;

export const toasts = { subscribe: _toasts.subscribe };

export function pushToast(message: string, kind: Toast['kind'] = 'error'): void {
  const id = _nextId++;
  _toasts.update((list) => [...list, { id, message, kind, ts: Date.now() }]);
  // Auto-dismiss after 5s. Long enough to read, short enough not to stack.
  setTimeout(() => dismissToast(id), 5000);
}

export function dismissToast(id: number): void {
  _toasts.update((list) => list.filter((t) => t.id !== id));
}
