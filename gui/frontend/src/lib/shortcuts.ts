export type ShortcutAction =
  | 'none'
  | 'open-create'
  | 'focus-search'
  | 'close-settings'
  | 'close-create'
  | 'close-drawer'
  | 'blur-card'
  | 'open-focused-card';

export interface ShortcutState {
  createOpen: boolean;
  settingsOpen: boolean;
  drawerOpen: boolean;
  focusedCardId: string | null;
}

export function shortcutAction(event: KeyboardEvent, state: ShortcutState): ShortcutAction {
  const key = event.key.toLowerCase();

  if (key === 'escape') {
    if (state.settingsOpen) return 'close-settings';
    if (state.createOpen) return 'close-create';
    if (state.drawerOpen) return 'close-drawer';
    if (state.focusedCardId) return 'blur-card';
    return 'none';
  }

  if (event.metaKey || event.ctrlKey || event.altKey) return 'none';

  if (isTypingTarget(event.target)) return 'none';

  if (key === 'n') {
    return state.createOpen || state.settingsOpen || state.drawerOpen ? 'none' : 'open-create';
  }
  if (key === '/') {
    return state.createOpen || state.settingsOpen || state.drawerOpen ? 'none' : 'focus-search';
  }
  if (key === 'enter') {
    return state.focusedCardId && !state.createOpen && !state.settingsOpen && !state.drawerOpen
      ? 'open-focused-card'
      : 'none';
  }

  return 'none';
}

export function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false;
  const tag = target.tagName.toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return true;
  if (target.closest('[contenteditable="true"], [contenteditable=""], .cm-editor')) return true;
  return false;
}
