// TB-180: smoke + validation render tests for the auto-implement
// controls added to SettingsPanel. The data path (round-trip / TOCTOU
// guard / validator proxy) is covered by stores/preferences.test.ts;
// this file confirms the UI surfaces the right inline warnings and
// wires the store API correctly when the user changes inputs.
import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { emptyAutoImplementFilter } from '$lib/autoImplementFilter';

const mocks = vi.hoisted(() => ({
  setAutoImplementEnabled: vi.fn<(value: boolean) => Promise<void>>(),
  setAutoImplementQuery: vi.fn<(value: unknown) => Promise<void>>(),
  setAutoReviewEnabled: vi.fn<(value: boolean) => Promise<void>>(),
  load: vi.fn<() => Promise<void>>(),
  pushToast: vi.fn(),
  enableClaudeUsageTap: vi.fn<() => Promise<unknown>>(),
  disableClaudeUsageTap: vi.fn<() => Promise<unknown>>(),
  getClaudeUsageTap: vi.fn<() => Promise<unknown>>(),
  openFile: vi.fn<() => Promise<string | string[]>>(),
}));

vi.mock('@wailsio/runtime', () => ({
  Events: { On: () => () => {} },
  Dialogs: { OpenFile: () => mocks.openFile() },
  Create: {
    Any: (v: unknown) => v,
    Array: (createItem: (value: unknown) => unknown) => (values: unknown[] = []) =>
      values.map(createItem),
    Map: (_k: (v: unknown) => unknown, createValue: (value: unknown) => unknown) =>
      (value: Record<string, unknown> = {}) =>
        Object.fromEntries(Object.entries(value).map(([key, item]) => [key, createValue(item)])),
    Nullable: (createValue: (value: unknown) => unknown) => (value: unknown) =>
      value == null ? null : createValue(value),
    Struct: (_fields: unknown) => (value: unknown) => value,
  },
  Call: { ByID: vi.fn() },
  CancellablePromise: Promise,
}));

vi.mock('$lib/api', () => ({
  EnableClaudeUsageTap: () => mocks.enableClaudeUsageTap(),
  DisableClaudeUsageTap: () => mocks.disableClaudeUsageTap(),
  GetClaudeUsageTap: () => mocks.getClaudeUsageTap(),
}));

vi.mock('$lib/stores/toast', () => ({
  pushToast: (m: string, k?: string) => mocks.pushToast(m, k),
}));

vi.mock('$lib/stores/runs', () => ({
  refreshUsage: vi.fn(),
}));

// Build a writable store mock that mirrors $preferencesStore. The
// component reads it via the $-prefix store auto-subscription; we
// control its value to drive every test scenario.
const fakeStore = vi.hoisted(() => {
  type Listener = (state: unknown) => void;
  let state: Record<string, unknown> = {
    maxWorkers: 1,
    maxWorkersLimit: 8,
    agentTimeoutMinutes: 30,
    defaultAgent: 'claude',
    cliPath: '',
    periodicRecoveryEnabled: true,
    autoGroomEnabled: false,
    autoGroomSettleMinutes: 5,
    automationStartupGraceSeconds: 30,
    autoImplementEnabled: false,
    autoReviewEnabled: false,
    autoImplementQuery: {
      search: '',
      types: [] as string[],
      priorities: [] as string[],
      modules: [] as string[],
      sizes: [] as string[],
      tags: [] as string[],
      agents: [] as string[],
      parents: [] as string[],
    },
    loaded: true,
  };
  const listeners = new Set<Listener>();
  return {
    subscribe(fn: Listener) {
      listeners.add(fn);
      fn(state);
      return () => listeners.delete(fn);
    },
    set(next: Record<string, unknown>) {
      state = { ...state, ...next };
      for (const fn of listeners) fn(state);
    },
    getValue() {
      return state;
    },
  };
});

vi.mock('$lib/stores/preferences', () => ({
  preferencesStore: {
    subscribe: fakeStore.subscribe,
    load: () => mocks.load(),
    setMaxWorkers: vi.fn().mockResolvedValue(undefined),
    setAgentTimeoutMinutes: vi.fn().mockResolvedValue(undefined),
    setDefaultAgent: vi.fn().mockResolvedValue(undefined),
    setCLIPath: vi.fn().mockResolvedValue(undefined),
    setPeriodicRecoveryEnabled: vi.fn().mockResolvedValue(undefined),
    setAutoGroomEnabled: vi.fn().mockResolvedValue(undefined),
    setAutoGroomSettleMinutes: vi.fn().mockResolvedValue(undefined),
    setAutomationStartupGraceSeconds: vi.fn().mockResolvedValue(undefined),
    setAutoImplementEnabled: (v: boolean) => mocks.setAutoImplementEnabled(v),
    setAutoImplementQuery: (v: unknown) => mocks.setAutoImplementQuery(v),
    setAutoReviewEnabled: (v: boolean) => mocks.setAutoReviewEnabled(v),
  },
}));

import SettingsPanel from './SettingsPanel.svelte';

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
  vi.clearAllMocks();
  mocks.load.mockResolvedValue(undefined);
  mocks.getClaudeUsageTap.mockResolvedValue({ enabled: false, projectRoot: '' });
  fakeStore.set({
    autoImplementEnabled: false,
    autoReviewEnabled: false,
    autoImplementQuery: { ...emptyAutoImplementFilter },
    defaultAgent: 'claude',
    maxWorkersLimit: 8,
  });
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

function mountPanel() {
  component = mount(SettingsPanel, {
    target: document.body,
    props: { open: true, onClose: vi.fn() as unknown as () => void },
  });
}

function findCheckbox(testid: string): HTMLInputElement {
  const el = document.querySelector<HTMLInputElement>(`input[data-testid="${testid}"]`);
  if (!el) throw new Error(`no checkbox testid=${testid}`);
  return el;
}

function visibleText(): string {
  return document.body.textContent || '';
}

describe('SettingsPanel auto-implement', () => {
  it('renders the backend-provided max worker range', async () => {
    mountPanel();
    await tick();
    const input = document.querySelector<HTMLInputElement>('input[type="number"]');
    expect(input?.getAttribute('max')).toBe('8');
    expect(visibleText()).toContain('1-8');
  });

  it('shows needs-filter warning when enabled with empty saved filter', async () => {
    mountPanel();
    await tick();
    const toggle = findCheckbox('auto-implement-toggle');
    toggle.checked = true;
    toggle.dispatchEvent(new Event('change', { bubbles: true }));
    toggle.dispatchEvent(new Event('input', { bubbles: true }));
    await tick();
    expect(visibleText()).toContain('Auto-implement needs a saved filter');
  });

  it('shows needs-default-agent warning when enabled without an agent', async () => {
    fakeStore.set({ defaultAgent: 'none' });
    mountPanel();
    await tick();
    const toggle = findCheckbox('auto-implement-toggle');
    toggle.checked = true;
    toggle.dispatchEvent(new Event('change', { bubbles: true }));
    toggle.dispatchEvent(new Event('input', { bubbles: true }));
    await tick();
    expect(visibleText()).toContain('Set a default agent before auto-implement can run');
  });

  it('clears warnings once prereqs are met (saved filter + default agent)', async () => {
    fakeStore.set({
      defaultAgent: 'claude',
      autoImplementQuery: {
        ...emptyAutoImplementFilter,
        types: ['bug'],
        sizes: ['S'],
        modules: ['gui'],
      },
    });
    mountPanel();
    await tick();
    const toggle = findCheckbox('auto-implement-toggle');
    toggle.checked = true;
    toggle.dispatchEvent(new Event('change', { bubbles: true }));
    toggle.dispatchEvent(new Event('input', { bubbles: true }));
    await tick();
    expect(visibleText()).not.toContain('Auto-implement needs a saved filter');
    expect(visibleText()).not.toContain('Set a default agent before auto-implement');
  });

  it('does not render saved-filter controls in Settings', async () => {
    fakeStore.set({
      autoImplementQuery: {
        ...emptyAutoImplementFilter,
        types: ['bug', 'improvement'],
        modules: ['gui'],
        tags: ['macos'],
      },
    });
    mountPanel();
    await tick();
    expect(document.querySelector('[data-testid="auto-implement-filter-summary"]')).toBeNull();
    expect(document.querySelector('[data-testid="auto-implement-edit-filter"]')).toBeNull();
    expect(document.querySelector('[data-testid="auto-implement-query"]')).toBeNull();
    expect(visibleText()).not.toContain('Auto-implement filter');
    expect(visibleText()).not.toContain('No filter saved');
    expect(visibleText()).not.toContain('Edit in board filter');
    expect(visibleText()).not.toContain('Build the filter on the board');
  });

  it('shows needs-default-agent warning when auto-review is enabled without an agent', async () => {
    fakeStore.set({ defaultAgent: 'none' });
    mountPanel();
    await tick();
    const toggle = findCheckbox('auto-review-toggle');
    toggle.checked = true;
    toggle.dispatchEvent(new Event('change', { bubbles: true }));
    toggle.dispatchEvent(new Event('input', { bubbles: true }));
    await tick();
    expect(visibleText()).toContain('Set a default agent before auto-review can run');
  });

  it('saves auto-review enablement through the preferences store', async () => {
    mocks.setAutoReviewEnabled.mockResolvedValue(undefined);
    mountPanel();
    await tick();
    const toggle = findCheckbox('auto-review-toggle');
    toggle.checked = true;
    toggle.dispatchEvent(new Event('change', { bubbles: true }));
    toggle.dispatchEvent(new Event('input', { bubbles: true }));
    await tick();
    const save = document.querySelector<HTMLButtonElement>('button.primary');
    expect(save?.disabled).toBe(false);
    save?.click();
    await tick();
    await tick();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(mocks.setAutoReviewEnabled).toHaveBeenCalledWith(true);
  });
});
