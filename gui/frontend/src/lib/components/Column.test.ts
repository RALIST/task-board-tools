import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Task } from '$lib/api';
import Column from './Column.svelte';
import { VIRTUAL_COLUMN_ITEM_HEIGHT, virtualTaskRange } from '$lib/columnVisibility';

vi.mock('@wailsio/runtime', () => ({
  Create: {
    Any: (value: unknown) => value,
    Array: (createItem: (value: unknown) => unknown) => (values: unknown[] = []) =>
      values.map(createItem),
    Map: (_createKey: (value: unknown) => unknown, createValue: (value: unknown) => unknown) =>
      (value: Record<string, unknown> = {}) =>
        Object.fromEntries(Object.entries(value).map(([key, item]) => [key, createValue(item)])),
  },
  Events: { On: () => () => {} },
}));

vi.mock('$lib/stores/triage', () => ({
  triageForTask: () => ({
    subscribe: (cb: (v: string[]) => void) => {
      cb([]);
      return () => {};
    },
  }),
  registerTaskTriageEventHandler: () => () => {},
}));

vi.mock('$lib/stores/groomSuggestion', () => ({ suggestGroom: vi.fn() }));

const preferenceMocks = vi.hoisted(() => {
  let current: any = {
    autoGroomEnabled: false,
    autoReviewEnabled: false,
    loaded: true,
  };
  const subs = new Set<(v: any) => void>();
  return {
    preferencesStore: {
      subscribe(cb: (v: any) => void) {
        cb(current);
        subs.add(cb);
        return () => subs.delete(cb);
      },
      reset() {
        current = { autoGroomEnabled: false, autoReviewEnabled: false, loaded: true };
        for (const cb of subs) cb(current);
      },
    },
  };
});
vi.mock('$lib/stores/preferences', () => ({
  preferencesStore: preferenceMocks.preferencesStore,
  defaultAgent: {
    subscribe: (cb: (v: string) => void) => {
      cb('none');
      return () => {};
    },
  },
}));

const autoGroomMocks = vi.hoisted(() => {
  let current: any = { lastSkipReasons: {} };
  const subs = new Set<(v: any) => void>();
  return {
    autoGroomStore: {
      subscribe(cb: (v: any) => void) {
        cb(current);
        subs.add(cb);
        return () => subs.delete(cb);
      },
      reset() {
        current = { lastSkipReasons: {} };
        for (const cb of subs) cb(current);
      },
    },
  };
});
vi.mock('$lib/stores/autoGroom', () => ({ autoGroomStore: autoGroomMocks.autoGroomStore }));

const autoReviewMocks = vi.hoisted(() => {
  let current: any = { lastSkipReasons: {} };
  const subs = new Set<(v: any) => void>();
  return {
    autoReviewStore: {
      subscribe(cb: (v: any) => void) {
        cb(current);
        subs.add(cb);
        return () => subs.delete(cb);
      },
      reset() {
        current = { lastSkipReasons: {} };
        for (const cb of subs) cb(current);
      },
    },
  };
});
vi.mock('$lib/stores/autoReview', () => ({ autoReviewStore: autoReviewMocks.autoReviewStore }));

const runsMocks = vi.hoisted(() => ({
  runsForTask: () => ({
    subscribe: (cb: (v: any[]) => void) => {
      cb([]);
      return () => {};
    },
  }),
  upsertRun: vi.fn(),
}));
vi.mock('$lib/stores/runs', () => ({
  runsForTask: runsMocks.runsForTask,
  upsertRun: runsMocks.upsertRun,
}));

vi.mock('$lib/api', () => ({ renameTask: vi.fn(), resumeAgent: vi.fn() }));
vi.mock('$lib/stores/toast', () => ({ pushToast: vi.fn() }));

const boardMocks = vi.hoisted(() => {
  const current: any = {
    backlog: [],
    ready: [],
    inProgress: [],
    codeReview: [],
    done: [],
    archive: [],
    wipLimits: {},
    wipCounts: {},
    wipEnforcement: 'warn',
  };
  return {
    board: {
      subscribe(cb: (v: any) => void) {
        cb(current);
        return () => {};
      },
    },
  };
});
vi.mock('$lib/stores/board', () => ({ board: boardMocks.board }));

const dndMocks = vi.hoisted(() => ({
  dndzone: vi.fn(() => ({ update: vi.fn(), destroy: vi.fn() })),
}));
vi.mock('svelte-dnd-action', () => ({
  dndzone: dndMocks.dndzone,
  TRIGGERS: {
    DRAGGED_ENTERED: 'DRAGGED_ENTERED',
    DRAGGED_OVER_INDEX: 'DRAGGED_OVER_INDEX',
    DRAGGED_LEFT: 'DRAGGED_LEFT',
    DRAGGED_LEFT_ALL: 'DRAGGED_LEFT_ALL',
  },
}));

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
  dndMocks.dndzone.mockClear();
  preferenceMocks.preferencesStore.reset();
  autoGroomMocks.autoGroomStore.reset();
  autoReviewMocks.autoReviewStore.reset();
});

afterEach(async () => {
  if (component) await unmount(component);
  component = null;
  document.body.innerHTML = '';
});

function task(id: string, status: Task['status'] = 'done'): Task {
  return {
    id,
    title: `Task ${id}`,
    type: 'improvement',
    priority: 'P2',
    size: 'M',
    module: 'gui',
    tags: ['gui', 'performance'],
    branch: '',
    reviewRef: '',
    parent: '',
    status,
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
  } as Task;
}

function tasks(count: number, status: Task['status'] = 'done'): Task[] {
  return Array.from({ length: count }, (_, index) => task(`TB-${index + 1}`, status));
}

function scrollList(): HTMLUListElement {
  const list = document.querySelector<HTMLUListElement>('ul.static');
  if (!list) throw new Error('static task list not found');
  return list;
}

function setViewport(list: HTMLElement, height: number) {
  Object.defineProperty(list, 'clientHeight', { value: height, configurable: true });
}

describe('Column virtualization', () => {
  it.each([
    ['done', 'Done'],
    ['archive', 'Archive'],
  ] as const)('keeps large %s-column card DOM bounded without manual paging', async (status, title) => {
    component = mount(Column, {
      target: document.body,
      props: { title, status, tasks: tasks(3000, status) },
    });
    await tick();

    const cards = document.querySelectorAll('.card');
    expect(cards.length).toBeGreaterThan(0);
    expect(cards.length).toBeLessThan(40);
    expect(document.querySelector('[data-task-id="TB-1"]')).not.toBeNull();
    expect(document.querySelector('[data-task-id="TB-500"]')).toBeNull();
    expect(document.querySelector('.show-more')).toBeNull();
    expect(document.querySelector('.count')?.textContent).toBe('3000');
  });

  it('opens the intended virtualized card after scrolling it into view', async () => {
    const selected: string[] = [];
    component = mount(Column, {
      target: document.body,
      props: {
        title: 'Done',
        status: 'done',
        tasks: tasks(3000),
        onSelect: (id: string) => selected.push(id),
      },
    });
    await tick();

    const list = scrollList();
    setViewport(list, VIRTUAL_COLUMN_ITEM_HEIGHT * 4);
    const scrollTop = VIRTUAL_COLUMN_ITEM_HEIGHT * 1000;
    list.scrollTop = scrollTop;
    list.dispatchEvent(new Event('scroll'));
    await tick();

    const range = virtualTaskRange(3000, scrollTop, VIRTUAL_COLUMN_ITEM_HEIGHT * 4);
    const targetID = `TB-${range.start + 3}`;
    const card = document.querySelector<HTMLElement>(`[data-task-id="${targetID}"]`);
    expect(card).not.toBeNull();
    card!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();

    expect(selected).toEqual([targetID]);
  });

  it('guards drag and drop for virtualized done columns', async () => {
    component = mount(Column, {
      target: document.body,
      props: { title: 'Done', status: 'done', tasks: tasks(3000) },
    });
    await tick();

    expect(dndMocks.dndzone).not.toHaveBeenCalled();
    expect(document.querySelector('.virtual-dnd-note')?.textContent).toContain('Drag disabled');
  });
});
