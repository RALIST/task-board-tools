import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  setAutoReviewEnabled: vi.fn<(enabled: boolean) => Promise<void>>(),
}));

const fakeStore = vi.hoisted(() => {
  type State = {
    autoReviewEnabled: boolean;
    defaultAgent: string;
  };
  type Listener = (state: State) => void;
  let state: State = { autoReviewEnabled: false, defaultAgent: 'claude' };
  const listeners = new Set<Listener>();
  return {
    subscribe(fn: Listener) {
      listeners.add(fn);
      fn(state);
      return () => listeners.delete(fn);
    },
    set(next: Partial<State>) {
      state = { ...state, ...next };
      for (const fn of listeners) fn(state);
    },
  };
});

const autoReviewFakeStore = vi.hoisted(() => {
  type State = { needsDefaultAgent: boolean };
  type Listener = (state: State) => void;
  let state: State = { needsDefaultAgent: false };
  const listeners = new Set<Listener>();
  return {
    subscribe(fn: Listener) {
      listeners.add(fn);
      fn(state);
      return () => listeners.delete(fn);
    },
    set(next: Partial<State>) {
      state = { ...state, ...next };
      for (const fn of listeners) fn(state);
    },
  };
});

vi.mock('$lib/stores/preferences', () => ({
  preferencesStore: {
    subscribe: fakeStore.subscribe,
    setAutoReviewEnabled: (enabled: boolean) => mocks.setAutoReviewEnabled(enabled),
  },
}));

vi.mock('$lib/stores/autoReview', () => ({
  autoReviewStore: { subscribe: autoReviewFakeStore.subscribe },
}));

import AutoReviewHeaderToggle from './AutoReviewHeaderToggle.svelte';

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
  vi.clearAllMocks();
  mocks.setAutoReviewEnabled.mockResolvedValue(undefined);
  fakeStore.set({ autoReviewEnabled: false, defaultAgent: 'claude' });
  autoReviewFakeStore.set({ needsDefaultAgent: false });
});

afterEach(() => {
  if (component) {
    try {
      unmount(component);
    } catch {
      /* ignore */
    }
    component = null;
  }
});

function mountToggle(onOpenSettings = vi.fn()) {
  component = mount(AutoReviewHeaderToggle, {
    target: document.body,
    props: { onOpenSettings },
  });
  return onOpenSettings;
}

function pill(): HTMLButtonElement {
  const el = document.querySelector<HTMLButtonElement>('[data-testid="auto-review-pill"]');
  if (!el) throw new Error('auto-review pill not found');
  return el;
}

describe('AutoReviewHeaderToggle', () => {
  it('enables auto-review through the shared preferences store', async () => {
    mountToggle();
    pill().click();
    await tick();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(mocks.setAutoReviewEnabled).toHaveBeenCalledWith(true);
  });

  it('opens Settings instead of enabling when default agent is missing', async () => {
    fakeStore.set({ defaultAgent: 'none' });
    const onOpenSettings = mountToggle(vi.fn());
    pill().click();
    await tick();

    expect(onOpenSettings).toHaveBeenCalledTimes(1);
    expect(mocks.setAutoReviewEnabled).not.toHaveBeenCalled();
  });

  it('opens Settings when coordinator reports missing default agent', async () => {
    autoReviewFakeStore.set({ needsDefaultAgent: true });
    const onOpenSettings = mountToggle(vi.fn());
    pill().click();
    await tick();

    expect(onOpenSettings).toHaveBeenCalledTimes(1);
    expect(mocks.setAutoReviewEnabled).not.toHaveBeenCalled();
  });

  it('allows disabling even when default agent is missing', async () => {
    fakeStore.set({ autoReviewEnabled: true, defaultAgent: 'none' });
    const onOpenSettings = mountToggle(vi.fn());
    pill().click();
    await tick();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(onOpenSettings).not.toHaveBeenCalled();
    expect(mocks.setAutoReviewEnabled).toHaveBeenCalledWith(false);
  });

  it('reflects shared-store state changes in the header pill', async () => {
    mountToggle();
    expect(pill().getAttribute('aria-pressed')).toBe('false');

    fakeStore.set({ autoReviewEnabled: true });
    await tick();

    expect(pill().getAttribute('aria-pressed')).toBe('true');
    expect(pill().classList.contains('on')).toBe(true);
  });
});
