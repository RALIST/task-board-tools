// TB-288: covers the "Save as auto-implement" button added to the
// FilterBar. The full layout/dropdown coverage lives in
// FilterBar.test.ts; this file isolates the Save button so the mock
// surface stays small.
import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  filter,
  focusFilterBarToken,
  initialFilter,
  requestFocusFilterBar,
} from '$lib/stores/filter';
import type { BoardSnapshot, Task } from '$lib/api';

const mocks = vi.hoisted(() => ({
  setAutoImplementQuery: vi.fn(),
  pushToast: vi.fn(),
}));

// preferencesStore is shaped to satisfy the FilterBar's reads + the
// saveAsAutoImplement click handler. autoImplementQuery is the
// "empty" filter so the "Saved" affordance starts off.
vi.mock('$lib/stores/preferences', () => {
  const empty = {
    search: '',
    types: [] as string[],
    priorities: [] as string[],
    modules: [] as string[],
    sizes: [] as string[],
    tags: [] as string[],
    agents: [] as string[],
    parents: [] as string[],
  };
  return {
    preferencesStore: {
      subscribe(fn: (v: unknown) => void) {
        fn({ autoImplementQuery: empty });
        return () => {};
      },
      setAutoImplementQuery: (v: unknown) => mocks.setAutoImplementQuery(v),
    },
  };
});

vi.mock('$lib/stores/toast', () => ({
  pushToast: (m: string) => mocks.pushToast(m),
}));

import FilterBar from './FilterBar.svelte';

let component: ReturnType<typeof mount> | null = null;

function task(id: string, overrides: Partial<Task> = {}): Task {
  return {
    id,
    title: id,
    type: 'bug',
    priority: 'P1',
    size: 'S',
    module: 'gui',
    tags: [],
    branch: '',
    reviewRef: '',
    parent: '',
    status: 'backlog',
    filePath: '',
    agent: '',
    agentStatus: '',
    agentResumable: false,
    groomedBy: '',
    groomStatus: '',
    implementedBy: '',
    implementStatus: '',
    reviewedBy: '',
    reviewStatus: '',
    ...overrides,
  };
}

function snapshot(tasks: Task[]): BoardSnapshot {
  return {
    backlog: tasks,
    ready: [],
    inProgress: [],
    codeReview: [],
    done: [],
    archive: [],
    wipLimits: {},
    wipCounts: {},
    wipEnforcement: 'warn',
  };
}

function saveButton(): HTMLButtonElement {
  const el = document.querySelector<HTMLButtonElement>(
    '[data-testid="save-as-auto-implement"]',
  );
  if (!el) throw new Error('save-as-auto-implement button not found');
  return el;
}

beforeEach(() => {
  filter.set({ ...initialFilter });
  focusFilterBarToken.set(0);
  vi.clearAllMocks();
  mocks.setAutoImplementQuery.mockResolvedValue(undefined);
});

afterEach(async () => {
  if (component) await unmount(component);
  component = null;
  document.body.innerHTML = '';
  filter.set({ ...initialFilter });
});

describe('FilterBar Save as auto-implement', () => {
  it('is disabled when no filter is active', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    expect(saveButton().disabled).toBe(true);
  });

  it('is enabled when at least one filter constraint is set', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    filter.update((f) => ({ ...f, types: ['bug'] }));
    await tick();
    expect(saveButton().disabled).toBe(false);
  });

  it('focuses the search input when focusFilterBarToken is bumped', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    const search = document.querySelector<HTMLInputElement>('input.search');
    expect(search).not.toBeNull();
    expect(document.activeElement).not.toBe(search);
    requestFocusFilterBar();
    await tick();
    expect(document.activeElement).toBe(search);
  });

  it('persists the current filter as an AutoImplementFilter on click', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    filter.update((f) => ({
      ...f,
      types: ['bug'],
      sizes: ['S'],
      modules: ['gui'],
      parentEpic: 'TB-1',
      search: 'router',
    }));
    await tick();
    saveButton().click();
    await tick();
    expect(mocks.setAutoImplementQuery).toHaveBeenCalledTimes(1);
    expect(mocks.setAutoImplementQuery).toHaveBeenCalledWith({
      search: 'router',
      types: ['bug'],
      priorities: [],
      modules: ['gui'],
      sizes: ['S'],
      tags: [],
      agents: [],
      parents: ['TB-1'],
    });
    expect(mocks.pushToast).toHaveBeenCalledWith('Saved as auto-implement query');
  });
});
