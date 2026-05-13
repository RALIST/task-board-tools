// Toast store: a small queue of transient notifications with auto-dismiss
// after 5s. The Toast.svelte component subscribes and renders; pushToast is
// the only producer the rest of the app calls.

import { writable } from 'svelte/store';

export type ToastKind = 'error' | 'info' | 'success';

export interface Toast {
  id: number;
  message: string;
  kind: ToastKind;
  ts: number;
}

const _toasts = writable<Toast[]>([]);
let _nextId = 1;

export const toasts = { subscribe: _toasts.subscribe };

export function pushToast(message: string, kind: ToastKind = 'error'): void {
  const id = _nextId++;
  _toasts.update((list) => [...list, { id, message, kind, ts: Date.now() }]);
  // Auto-dismiss after 5s. Long enough to read, short enough not to stack.
  setTimeout(() => dismissToast(id), 5000);
}

export function dismissToast(id: number): void {
  _toasts.update((list) => list.filter((t) => t.id !== id));
}
