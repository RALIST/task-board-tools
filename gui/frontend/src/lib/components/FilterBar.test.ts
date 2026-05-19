import { get } from 'svelte/store';
import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { BoardSnapshot, Task } from '$lib/api';
import { filter, initialFilter } from '$lib/stores/filter';
import FilterBar from './FilterBar.svelte';

let component: ReturnType<typeof mount> | null = null;

function task(id: string, overrides: Partial<Task> = {}): Task {
  return {
    id,
    title: id,
    type: 'task',
    priority: 'P2',
    size: 'M',
    module: 'core',
    tags: [],
    branch: '',
    reviewRef: '',
    parent: '',
    status: 'backlog',
    filePath: '',
    agent: '',
    agentStatus: '', agentResumable: false, groomedBy: '', groomStatus: '', implementedBy: '', implementStatus: '', reviewedBy: '', reviewStatus: '',
    ...overrides,
  };
}

function snapshot(tasks: Task[]): BoardSnapshot {
  return { backlog: tasks, inProgress: [], ready: [], codeReview: [], done: [], archive: [], wipLimits: {}, wipCounts: {}, wipEnforcement: 'warn' };
}

function triggers(): HTMLButtonElement[] {
  return [...document.querySelectorAll<HTMLButtonElement>('.fd-trigger')];
}

function triggerByLabel(label: string): HTMLButtonElement {
  for (const t of triggers()) {
    const text = t.textContent?.replace(/\s+/g, ' ').trim() ?? '';
    if (text === label || text.startsWith(`${label} `) || text.startsWith(`${label}:`)) return t;
  }
  throw new Error(`trigger for label "${label}" not found; saw: ${triggers().map((t) => t.textContent?.trim()).join(', ')}`);
}

beforeEach(() => {
  filter.set({ ...initialFilter });
});

afterEach(async () => {
  if (component) await unmount(component);
  component = null;
  document.body.innerHTML = '';
  filter.set({ ...initialFilter });
});

describe('FilterBar layout', () => {
  it('renders one dropdown per category that has more than one observed value', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug', priority: 'P1', module: 'cli', agent: 'claude', tags: ['ui'] }),
      task('TB-2', { type: 'feature', priority: 'P0', module: 'gui', agent: 'codex', tags: ['docs'] }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();

    const labels = triggers().map((t) => t.textContent?.replace(/\s+/g, ' ').trim() ?? '');
    expect(labels.some((l) => l === 'Type' || l.startsWith('Type '))).toBe(true);
    expect(labels.some((l) => l === 'Priority' || l.startsWith('Priority '))).toBe(true);
    expect(labels.some((l) => l === 'Module' || l.startsWith('Module '))).toBe(true);
    expect(labels.some((l) => l === 'Tags' || l.startsWith('Tags '))).toBe(true);
    expect(labels.some((l) => l === 'Agent' || l.startsWith('Agent '))).toBe(true);
    expect(labels.find((l) => l.startsWith('Epic'))).toBeUndefined();
  });

  it('hides categories with one or zero distinct values', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug', priority: 'P1', module: 'cli' }),
      task('TB-2', { type: 'bug', priority: 'P1', module: 'cli' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();

    const labels = triggers().map((t) => t.textContent?.replace(/\s+/g, ' ').trim() ?? '');
    expect(labels.find((l) => l.startsWith('Type'))).toBeUndefined();
    expect(labels.find((l) => l.startsWith('Priority'))).toBeUndefined();
    expect(labels.find((l) => l.startsWith('Module'))).toBeUndefined();
  });

  it('shows a count badge on the trigger label when filters are selected', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
    ]);
    filter.set({ ...initialFilter, types: ['bug', 'feature'] });
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();

    const typeTriggerText = triggerByLabel('Type').textContent?.replace(/\s+/g, ' ').trim();
    expect(typeTriggerText).toContain('Type (2)');
  });

  it('renders ActiveFilters only when at least one filter is set', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    expect(document.querySelector('.af')).toBeNull();

    filter.set({ ...initialFilter, types: ['bug'] });
    await tick();
    expect(document.querySelector('.af')).not.toBeNull();
    expect(document.querySelector('.af-chip')?.textContent?.replace(/\s+/g, ' ').trim()).toBe('bug ×');
  });

  it('removes a single filter when its active chip is clicked', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug', priority: 'P1' }),
      task('TB-2', { type: 'feature', priority: 'P0' }),
    ]);
    filter.set({ ...initialFilter, types: ['bug', 'feature'], priorities: ['P1'] });
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    const bugChip = [...document.querySelectorAll<HTMLButtonElement>('.af-chip')].find(
      (b) => b.textContent?.includes('bug'),
    );
    bugChip?.click();
    await tick();
    expect(get(filter).types).toEqual(['feature']);
    expect(get(filter).priorities).toEqual(['P1']);
  });

  it('clear button resets every filter category but preserves showArchive', async () => {
    const snap = snapshot([task('TB-1', { type: 'bug' }), task('TB-2', { type: 'feature' })]);
    filter.set({ ...initialFilter, types: ['bug'], priorities: ['P1'], showArchive: true });
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();

    document.querySelector<HTMLButtonElement>('button.clear')?.click();
    await tick();
    expect(get(filter).types).toEqual([]);
    expect(get(filter).priorities).toEqual([]);
    expect(get(filter).showArchive).toBe(true);
  });

  it('multi-toggles values inside a Type dropdown popover', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
      task('TB-3', { type: 'spike' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    triggerByLabel('Type').click();
    await tick();
    const opts = [...document.querySelectorAll<HTMLButtonElement>('.fd-option')];
    opts.find((b) => b.textContent?.trim() === 'bug')?.click();
    await tick();
    opts.find((b) => b.textContent?.trim() === 'spike')?.click();
    await tick();
    expect(get(filter).types).toEqual(['bug', 'spike']);
  });
});
