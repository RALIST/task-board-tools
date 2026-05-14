import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// `@wailsio/runtime`'s `Events.On` is a no-op for these tests — the Card
// subscribes to triage events on mount via `registerTaskTriageEventHandler`
// but we don't drive any events here.
vi.mock('@wailsio/runtime', () => ({
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
