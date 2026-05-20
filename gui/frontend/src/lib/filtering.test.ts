import { describe, expect, it } from 'vitest';
import type { BoardSnapshot, Task } from './api';
import { applyFilter, epicProgress, observedTags } from './filtering';
import type { BoardFilter } from './stores/filter';

const baseFilter: BoardFilter = {
  search: '',
  types: [],
  priorities: [],
  modules: [],
  sizes: [],
  tags: [],
  agents: [],
  parentEpic: '',
  showArchive: false,
};

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

function snapshot(columns: Partial<BoardSnapshot>): BoardSnapshot {
  return {
    backlog: columns.backlog ?? [],
    ready: columns.ready ?? [],
    inProgress: columns.inProgress ?? [],
    codeReview: columns.codeReview ?? [],
    done: columns.done ?? [],
    archive: columns.archive ?? [],
    wipLimits: columns.wipLimits ?? {},
    wipCounts: columns.wipCounts ?? {},
    wipEnforcement: columns.wipEnforcement ?? 'warn',
  };
}

function ids(snap: BoardSnapshot): string[] {
  return [
    ...snap.backlog,
    ...snap.inProgress,
    ...(snap.codeReview ?? []),
    ...snap.done,
    ...(snap.archive ?? []),
  ].map((t) => t.id);
}

describe('applyFilter', () => {
  it('matches the selected parent epic and its child tasks', () => {
    const snap = snapshot({
      backlog: [
        task('TB-1', { title: 'Parent epic', tags: ['epic'], type: 'epic' }),
        task('TB-3', { title: 'Unrelated task' }),
      ],
      inProgress: [
        task('TB-2', { title: 'Child task', parent: 'TB-1' }),
      ],
    });

    const filtered = applyFilter(snap, { ...baseFilter, parentEpic: 'TB-1' });

    expect(ids(filtered)).toEqual(['TB-1', 'TB-2']);
  });

  it('composes parent epic matching with the other filter predicates', () => {
    const snap = snapshot({
      backlog: [
        task('TB-1', { title: 'Checkout epic', tags: ['epic', 'ui'], type: 'epic' }),
        task('TB-2', { title: 'Checkout child UI', parent: 'TB-1', tags: ['ui'], type: 'task' }),
        task('TB-3', { title: 'Checkout child backend', parent: 'TB-1', tags: ['backend'], type: 'task' }),
        task('TB-4', { title: 'Other child UI', parent: 'TB-9', tags: ['ui'], type: 'task' }),
      ],
    });

    const filtered = applyFilter(snap, {
      ...baseFilter,
      parentEpic: 'TB-1',
      tags: ['ui'],
      types: ['task'],
      search: 'child',
    });

    expect(ids(filtered)).toEqual(['TB-2']);
  });

  it('preserves code-review tasks through the filter', () => {
    const snap = snapshot({
      backlog: [task('TB-1', { tags: ['epic'] })],
      codeReview: [
        task('TB-2', { status: 'code-review', tags: ['ui'] }),
        task('TB-3', { status: 'code-review', tags: ['backend'] }),
      ],
    });

    const all = applyFilter(snap, baseFilter);
    expect(all.codeReview.map((t) => t.id)).toEqual(['TB-2', 'TB-3']);

    const uiOnly = applyFilter(snap, { ...baseFilter, tags: ['ui'] });
    expect(uiOnly.codeReview.map((t) => t.id)).toEqual(['TB-2']);
  });
});

describe('epicProgress', () => {
  it('reports zero counts for an epic with no children', () => {
    const snap = snapshot({
      backlog: [task('TB-1', { tags: ['epic'] })],
    });
    expect(epicProgress(snap, 'TB-1')).toEqual({ done: 0, total: 0, percent: 0 });
  });

  it('returns partial progress for a mix of done and not-done children', () => {
    const snap = snapshot({
      backlog: [
        task('TB-1', { tags: ['epic'] }),
        task('TB-2', { parent: 'TB-1', status: 'backlog' }),
      ],
      inProgress: [
        task('TB-3', { parent: 'TB-1', status: 'in-progress' }),
      ],
      done: [
        task('TB-4', { parent: 'TB-1', status: 'done' }),
      ],
    });
    expect(epicProgress(snap, 'TB-1')).toEqual({ done: 1, total: 3, percent: 33 });
  });

  it('reports 100% when every child is done', () => {
    const snap = snapshot({
      backlog: [task('TB-1', { tags: ['epic'] })],
      done: [
        task('TB-2', { parent: 'TB-1', status: 'done' }),
        task('TB-3', { parent: 'TB-1', status: 'done' }),
      ],
    });
    expect(epicProgress(snap, 'TB-1')).toEqual({ done: 2, total: 2, percent: 100 });
  });

  it('ignores tasks whose parent points elsewhere', () => {
    const snap = snapshot({
      backlog: [
        task('TB-1', { tags: ['epic'] }),
        task('TB-9', { tags: ['epic'] }),
        task('TB-2', { parent: 'TB-9', status: 'done' }),
      ],
    });
    expect(epicProgress(snap, 'TB-1')).toEqual({ done: 0, total: 0, percent: 0 });
  });

  it('includes archive children when the snapshot already loaded them', () => {
    const snap = snapshot({
      backlog: [task('TB-1', { tags: ['epic'] })],
      done: [task('TB-2', { parent: 'TB-1', status: 'done' })],
      archive: [task('TB-3', { parent: 'TB-1', status: 'archive' })],
    });
    expect(epicProgress(snap, 'TB-1')).toEqual({ done: 1, total: 2, percent: 50 });
  });

  it('counts children sitting in code-review', () => {
    const snap = snapshot({
      backlog: [task('TB-1', { tags: ['epic'] })],
      codeReview: [task('TB-2', { parent: 'TB-1', status: 'code-review' })],
      done: [task('TB-3', { parent: 'TB-1', status: 'done' })],
    });
    expect(epicProgress(snap, 'TB-1')).toEqual({ done: 1, total: 2, percent: 50 });
  });
});

describe('tag helpers', () => {
  it('ranks observed tags by frequency with deterministic tie sorting', () => {
    const snap = snapshot({
      backlog: [
        task('TB-1', { tags: ['ui', 'backend', 'cli'] }),
        task('TB-2', { tags: ['ui', 'backend'] }),
        task('TB-3', { tags: ['ui', 'docs'] }),
        task('TB-4', { tags: ['docs'] }),
      ],
    });

    expect(observedTags(snap)).toEqual(['ui', 'backend', 'docs', 'cli']);
  });
});
