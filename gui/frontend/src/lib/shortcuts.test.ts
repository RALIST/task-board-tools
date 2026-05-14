import { describe, expect, it } from 'vitest';
import { shortcutAction } from './shortcuts';

function keydown(key: string, target: EventTarget = document.body): KeyboardEvent {
  const event = new KeyboardEvent('keydown', { key, bubbles: true });
  Object.defineProperty(event, 'target', { value: target });
  return event;
}

describe('shortcutAction', () => {
  it('suppresses bare-letter shortcuts inside typing targets', () => {
    const input = document.createElement('input');
    const cm = document.createElement('div');
    cm.className = 'cm-editor';
    const nested = document.createElement('span');
    cm.append(nested);

    expect(shortcutAction(keydown('n', input), emptyState())).toBe('none');
    expect(shortcutAction(keydown('/', nested), emptyState())).toBe('none');
  });

  it('closes the topmost surface with Escape', () => {
    expect(
      shortcutAction(keydown('Escape'), {
        createOpen: true,
        settingsOpen: true,
        drawerOpen: true,
        focusedCardId: 'TB-1',
      }),
    ).toBe('close-settings');

    expect(
      shortcutAction(keydown('Escape'), {
        createOpen: true,
        settingsOpen: false,
        drawerOpen: true,
        focusedCardId: 'TB-1',
      }),
    ).toBe('close-create');
  });

  it('opens the focused card on Enter when no modal is open', () => {
    expect(
      shortcutAction(keydown('Enter'), {
        ...emptyState(),
        focusedCardId: 'TB-1',
      }),
    ).toBe('open-focused-card');
  });
});

function emptyState() {
  return {
    createOpen: false,
    settingsOpen: false,
    drawerOpen: false,
    focusedCardId: null,
  };
}
