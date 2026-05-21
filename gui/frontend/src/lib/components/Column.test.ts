import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Task } from '$lib/api';
import Column from './Column.svelte';
import ColumnHarness from './Column.harness.test.svelte';
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

const dndMocks = vi.hoisted(() => {
  function dndzone(node: HTMLElement) {
    let dragTarget: HTMLElement | null = null;
    const childListeners = new Map<Element, EventListener>();

    function removeWindowListeners() {
      window.removeEventListener('mousemove', handleMouseMoveMaybeDragStart);
      window.removeEventListener('mouseup', handleMouseUp);
    }

    function handleMouseMoveMaybeDragStart() {
      removeWindowListeners();
      const originDropZone = dragTarget?.parentElement ?? null;
      originDropZone!.closest('dialog');
      dragTarget = null;
    }

    function handleMouseUp() {
      removeWindowListeners();
      dragTarget = null;
    }

    function bindChildren() {
      for (const [child, listener] of childListeners) {
        child.removeEventListener('mousedown', listener);
      }
      childListeners.clear();
      for (const child of Array.from(node.children)) {
        const listener: EventListener = (event) => {
          if (!(event instanceof MouseEvent)) return;
          if (event.button !== 0) return;
          dragTarget = event.currentTarget as HTMLElement;
          window.addEventListener('mousemove', handleMouseMoveMaybeDragStart);
          window.addEventListener('mouseup', handleMouseUp);
        };
        child.addEventListener('mousedown', listener);
        childListeners.set(child, listener);
      }
    }

    bindChildren();

    return {
      update: bindChildren,
      destroy: () => {
        for (const [child, listener] of childListeners) {
          child.removeEventListener('mousedown', listener);
        }
        childListeners.clear();
        removeWindowListeners();
      },
    };
  }

  return { dndzone: vi.fn(dndzone) };
});
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
  it('does not crash when a board refresh removes the source card before drag startup', async () => {
    component = mount(ColumnHarness, {
      target: document.body,
      props: { initialTasks: [task('TB-323', 'ready')], status: 'ready', title: 'Ready' },
    });
    await tick();

    const card = document.querySelector<HTMLElement>('[data-task-id="TB-323"]');
    const item = card?.closest('li');
    expect(item).not.toBeNull();
    item!.dispatchEvent(
      new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0, clientX: 0, clientY: 0 }),
    );

    (component as { setTasks: (tasks: Task[]) => void }).setTasks([]);
    await tick();

    expect(() => {
      window.dispatchEvent(new MouseEvent('mousemove', { cancelable: true, clientX: 8, clientY: 0 }));
    }).not.toThrow();

    window.dispatchEvent(new MouseEvent('mouseup', { cancelable: true }));
    await tick();
    expect(document.querySelector('[data-task-id="TB-323"]')).toBeNull();
  });

  it('keeps draggable task cards marked as file-drop targets', async () => {
    component = mount(Column, {
      target: document.body,
      props: { title: 'Ready', status: 'ready', tasks: [task('TB-126', 'ready')] },
    });
    await tick();

    const card = document.querySelector<HTMLElement>('[data-task-id="TB-126"]');
    expect(card).not.toBeNull();
    expect(card?.hasAttribute('data-file-drop-target')).toBe(true);
    expect(card?.querySelector('.ttl')?.closest('[data-file-drop-target]')).toBe(card);
  });

  it('refreshes draggable card content when a patched task keeps the same ID', async () => {
    const stale = task('TB-325', 'ready');
    stale.title = 'Stale card title';
    component = mount(ColumnHarness, {
      target: document.body,
      props: { initialTasks: [stale], status: 'ready', title: 'Ready' },
    });
    await tick();

    expect(document.querySelector('[data-task-id="TB-325"] .ttl')?.textContent).toBe('Stale card title');

    const fresh = {
      ...stale,
      title: 'Fresh card title',
      tags: ['gui', 'live-updates'],
      agent: 'codex',
      agentStatus: 'needs-user',
      implementStatus: 'running',
    } as Task;
    (component as { setTasks: (tasks: Task[]) => void }).setTasks([fresh]);
    await tick();

    expect(document.querySelector('[data-task-id="TB-325"] .ttl')?.textContent).toBe('Fresh card title');
  });

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

  it('preserves supplied task order and card badges after virtualized scrolling', async () => {
    const ordered = Array.from({ length: 3000 }, (_, index) => task(`TB-${3000 - index}`));
    const scrollTop = VIRTUAL_COLUMN_ITEM_HEIGHT * 1000;
    const range = virtualTaskRange(ordered.length, scrollTop, VIRTUAL_COLUMN_ITEM_HEIGHT * 4);
    const badgeTask = ordered[range.start + 2];
    badgeTask.tags = ['gui', 'virtual-badge'];
    badgeTask.agent = 'codex';
    badgeTask.agentStatus = 'running';
    component = mount(Column, {
      target: document.body,
      props: {
        title: 'Done',
        status: 'done',
        tasks: ordered,
      },
    });
    await tick();

    const list = scrollList();
    setViewport(list, VIRTUAL_COLUMN_ITEM_HEIGHT * 4);
    list.scrollTop = scrollTop;
    list.dispatchEvent(new Event('scroll'));
    await tick();

    const renderedIDs = Array.from(document.querySelectorAll<HTMLElement>('.card')).map((card) =>
      card.dataset.taskId,
    );
    expect(renderedIDs).toEqual(ordered.slice(range.start, range.end).map((t) => t.id));
    const badgeCard = document.querySelector<HTMLElement>(`[data-task-id="${badgeTask.id}"]`);
    expect(badgeCard?.querySelector('.tag')?.textContent).toBe('gui');
    expect(badgeCard?.querySelector('.agent')?.textContent).toContain('codex');
  });

  it('keeps large backlog columns fully mounted and draggable', async () => {
    component = mount(Column, {
      target: document.body,
      props: { title: 'Backlog', status: 'backlog', tasks: tasks(260, 'backlog') },
    });
    await tick();
    await tick();

    expect(dndMocks.dndzone).toHaveBeenCalledTimes(1);
    expect(document.querySelectorAll('.card')).toHaveLength(260);
    expect(document.querySelector('.show-more')).toBeNull();
    expect(document.querySelector('.virtual-dnd-note')).toBeNull();
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

  it('keeps archive columns non-draggable without showing a virtualized DnD guard', async () => {
    component = mount(Column, {
      target: document.body,
      props: { title: 'Archive', status: 'archive', tasks: tasks(3000, 'archive') },
    });
    await tick();

    expect(dndMocks.dndzone).not.toHaveBeenCalled();
    expect(document.querySelector('.virtual-dnd-note')).toBeNull();
  });
});
