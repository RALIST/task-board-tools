import { describe, expect, it } from 'vitest';
import type { BoardSnapshot, Task } from './api';
import { applyFilter, observedTags, selectInlineTags } from './filtering';
import type { BoardFilter } from './stores/filter';

const baseFilter: BoardFilter = {
  search: '',
  types: [],
  priorities: [],
  modules: [],
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
    parent: '',
    status: 'backlog',
    filePath: '',
    agent: '',
    agentStatus: '',
    ...overrides,
  };
}

function snapshot(columns: Partial<BoardSnapshot>): BoardSnapshot {
  return {
    backlog: columns.backlog ?? [],
    inProgress: columns.inProgress ?? [],
    done: columns.done ?? [],
    archive: columns.archive ?? [],
  };
}

function ids(snap: BoardSnapshot): string[] {
  return [...snap.backlog, ...snap.inProgress, ...snap.done, ...(snap.archive ?? [])].map((t) => t.id);
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

  it('keeps active tags inline even when they rank below the visible limit', () => {
    expect(selectInlineTags(['popular', 'common', 'rare', 'tiny'], ['tiny'], 2)).toEqual({
      inline: ['popular', 'common', 'tiny'],
      overflow: ['rare'],
    });
  });
});
