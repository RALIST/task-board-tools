import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// `@wailsio/runtime`'s `Events.On` is a no-op for these tests — the Card
// subscribes to triage events on mount via `registerTaskTriageEventHandler`
// but we don't drive any events here.
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

// Triage and groomSuggestion stores hit the Wails bindings transitively;
// stub their public surface so the Card can mount under jsdom.
vi.mock('$lib/stores/triage', () => ({
  triageForTask: () => ({ subscribe: (cb: (v: string[]) => void) => { cb([]); return () => {}; } }),
  registerTaskTriageEventHandler: () => () => {},
}));

vi.mock('$lib/stores/groomSuggestion', () => ({
  suggestGroom: vi.fn(),
}));

// Mock $lib/api so the rename flow under test never reaches the real Wails
// bridge. Importing the real module would pull in the generated bindings
// (which try to talk to the runtime); spying on `renameTask` keeps the
// component isolated.
const apiMocks = vi.hoisted(() => ({ renameTask: vi.fn() }));
vi.mock('$lib/api', () => apiMocks);

const toastMock = vi.hoisted(() => ({ pushToast: vi.fn() }));
vi.mock('$lib/stores/toast', () => toastMock);

import Card from './Card.svelte';
import type { Task } from '$lib/api';

function makeTask(overrides: Partial<Task> = {}): Task {
  return {
    id: 'TB-126',
    title: 'GUI: dropping a file on a task card attaches it',
    type: 'improvement',
    priority: 'P1',
    size: 'S',
    module: 'gui',
    tags: ['epic-tb93', 'gui', 'dnd', 'attachments', 'follow-up'],
    branch: '',
    parent: 'TB-93',
    status: 'backlog',
    filePath: 'board/backlog/TB-126/TASK.md',
    agent: '',
    agentStatus: '',
    ...overrides,
  } as Task;
}

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
  apiMocks.renameTask.mockReset();
  toastMock.pushToast.mockReset();
});

afterEach(async () => {
  if (component) await unmount(component);
  component = null;
  document.body.innerHTML = '';
});

function cardEl(): HTMLElement {
  const el = document.querySelector<HTMLElement>('.card');
  if (!el) throw new Error('card not found');
  return el;
}

describe('Card.svelte file-drop wiring (TB-126)', () => {
  it('exposes data-file-drop-target so Wails can resolve drops on the card', async () => {
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask() },
    });
    await tick();

    const el = cardEl();
    expect(el.hasAttribute('data-file-drop-target')).toBe(true);
    expect(el.getAttribute('data-task-id')).toBe('TB-126');
  });

  it('keeps the drop attributes on the closest ancestor of nested children', async () => {
    // Wails routes drops via `element.closest('[data-file-drop-target]')`
    // from `document.elementFromPoint(x, y)`. Dropping on any child of the
    // card (title, glyph, priority pill, …) must still resolve to the card.
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-7' }) },
    });
    await tick();

    const inner = document.querySelector<HTMLElement>('.card .ttl');
    if (!inner) throw new Error('inner title element not found');
    const resolved = inner.closest('[data-file-drop-target]') as HTMLElement | null;
    expect(resolved).not.toBeNull();
    expect(resolved!.getAttribute('data-task-id')).toBe('TB-7');
  });

  it('reflects the task id even for in-progress and done cards', async () => {
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-42', status: 'in-progress' }) },
    });
    await tick();
    expect(cardEl().getAttribute('data-task-id')).toBe('TB-42');
  });
});

describe('Card.svelte inline title rename (TB-207)', () => {
  function dispatchDoubleClick(el: HTMLElement) {
    el.dispatchEvent(new MouseEvent('dblclick', { bubbles: true, cancelable: true }));
  }

  it('double-clicking the title swaps in a prefilled, focused input', async () => {
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-7', title: 'Old title' }) },
    });
    await tick();

    const title = document.querySelector<HTMLElement>('.card .ttl');
    if (!title) throw new Error('title not found');
    dispatchDoubleClick(title);
    await tick();
    // queueMicrotask defers focus/select until after Svelte's update.
    await Promise.resolve();
    await tick();

    const input = document.querySelector<HTMLInputElement>('.card .ttl-input');
    expect(input).not.toBeNull();
    expect(input!.value).toBe('Old title');
    // The plain title should be removed while editing.
    expect(document.querySelector('.card .ttl')).toBeNull();
  });

  it('real click+click+dblclick sequence on the title does NOT open the drawer', async () => {
    // Browsers deliver dblclick as click→click→dblclick. Each `click` bubbles
    // to the card whose handler would otherwise open the drawer via onSelect.
    // This test asserts the title swallows the clicks so the drawer never
    // opens — only rename mode does.
    const selected = vi.fn();
    vi.useFakeTimers();
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-7', title: 'Old title' }), onSelect: selected },
    });
    await tick();

    const title = document.querySelector<HTMLElement>('.card .ttl')!;
    title.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    title.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    title.dispatchEvent(new MouseEvent('dblclick', { bubbles: true, cancelable: true }));
    // Advance past the 250ms deferred-click threshold to be sure no delayed
    // onSelect fires after the dblclick handler enters rename mode.
    vi.advanceTimersByTime(500);
    await tick();
    vi.useRealTimers();
    await Promise.resolve();
    await tick();

    expect(selected).not.toHaveBeenCalled();
    expect(document.querySelector('.card .ttl-input')).not.toBeNull();
  });

  it('a lone single click on the title eventually opens the drawer', async () => {
    const selected = vi.fn();
    vi.useFakeTimers();
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-7' }), onSelect: selected },
    });
    await tick();
    const title = document.querySelector<HTMLElement>('.card .ttl')!;
    title.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    // Before the threshold, nothing has fired.
    expect(selected).not.toHaveBeenCalled();
    vi.advanceTimersByTime(300);
    vi.useRealTimers();
    expect(selected).toHaveBeenCalledWith('TB-7');
  });

  it('Enter inside the input invokes renameTask with the trimmed draft', async () => {
    apiMocks.renameTask.mockResolvedValue(undefined);
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-7', title: 'Old title' }) },
    });
    await tick();
    dispatchDoubleClick(document.querySelector<HTMLElement>('.card .ttl')!);
    await tick();

    const input = document.querySelector<HTMLInputElement>('.card .ttl-input')!;
    input.value = '  New title  ';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    await Promise.resolve();
    await tick();

    expect(apiMocks.renameTask).toHaveBeenCalledTimes(1);
    expect(apiMocks.renameTask).toHaveBeenCalledWith('TB-7', 'New title');
  });

  it('Escape cancels the draft without invoking renameTask', async () => {
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-7', title: 'Original' }) },
    });
    await tick();
    dispatchDoubleClick(document.querySelector<HTMLElement>('.card .ttl')!);
    await tick();
    const input = document.querySelector<HTMLInputElement>('.card .ttl-input')!;
    input.value = 'Drafted change';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
    await tick();

    expect(apiMocks.renameTask).not.toHaveBeenCalled();
    // After cancel the editor unmounts and the plain title is shown again
    // with the unchanged value.
    expect(document.querySelector('.card .ttl-input')).toBeNull();
    expect(document.querySelector<HTMLElement>('.card .ttl')?.textContent).toBe('Original');
  });

  it('empty title is rejected with a toast and does not call renameTask', async () => {
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-7', title: 'Original' }) },
    });
    await tick();
    dispatchDoubleClick(document.querySelector<HTMLElement>('.card .ttl')!);
    await tick();
    const input = document.querySelector<HTMLInputElement>('.card .ttl-input')!;
    input.value = '   ';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    await Promise.resolve();
    await tick();

    expect(apiMocks.renameTask).not.toHaveBeenCalled();
    expect(toastMock.pushToast).toHaveBeenCalled();
    // Draft remains open so the user can fix it.
    expect(document.querySelector('.card .ttl-input')).not.toBeNull();
  });

  it('unchanged title closes the editor without calling renameTask', async () => {
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-7', title: 'Same' }) },
    });
    await tick();
    dispatchDoubleClick(document.querySelector<HTMLElement>('.card .ttl')!);
    await tick();
    const input = document.querySelector<HTMLInputElement>('.card .ttl-input')!;
    // Draft equals current title (after trim).
    input.value = 'Same';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    await Promise.resolve();
    await tick();

    expect(apiMocks.renameTask).not.toHaveBeenCalled();
    // Editor closed without a network round trip.
    expect(document.querySelector('.card .ttl-input')).toBeNull();
  });

  it('renameTask failure keeps the draft open and toasts the error', async () => {
    apiMocks.renameTask.mockRejectedValue(new Error('CLI boom'));
    component = mount(Card, {
      target: document.body,
      props: { task: makeTask({ id: 'TB-7', title: 'Old' }) },
    });
    await tick();
    dispatchDoubleClick(document.querySelector<HTMLElement>('.card .ttl')!);
    await tick();
    const input = document.querySelector<HTMLInputElement>('.card .ttl-input')!;
    input.value = 'Attempted';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    await Promise.resolve();
    await tick();
    await Promise.resolve();
    await tick();

    expect(apiMocks.renameTask).toHaveBeenCalledTimes(1);
    // The draft input is still mounted so the user can retry.
    const still = document.querySelector<HTMLInputElement>('.card .ttl-input');
    expect(still).not.toBeNull();
    expect(still!.value).toBe('Attempted');
    expect(toastMock.pushToast).toHaveBeenCalledWith(
      expect.stringContaining('Rename failed'),
    );
  });
});
