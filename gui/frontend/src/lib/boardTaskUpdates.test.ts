import { writable } from 'svelte/store';
import { describe, expect, it, vi } from 'vitest';
import type { BoardSnapshot, Task } from './api';
import { registerBoardTaskUpdateHandlers } from './boardTaskUpdates';

describe('board task update event registration', () => {
  it('patches a visible task from its exact task:updated:<id> event', () => {
    const snapshotStore = writable(snapshot([task('TB-1', 'ready'), task('TB-2', 'in-progress')]));
    const handlers: Record<string, () => void> = {};
    const patchTask = vi.fn();

    const off = registerBoardTaskUpdateHandlers(snapshotStore, (name, handler) => {
      handlers[name] = handler;
      return () => delete handlers[name];
    }, patchTask);

    expect(Object.keys(handlers).sort()).toEqual(['task:updated:TB-1', 'task:updated:TB-2']);
    expect(handlers['task:updated']).toBeUndefined();

    handlers['task:updated:TB-1']();

    expect(patchTask).toHaveBeenCalledOnce();
    expect(patchTask).toHaveBeenCalledWith('TB-1');

    off();
    expect(handlers).toEqual({});
  });

  it('keeps subscriptions aligned with the current board snapshot', () => {
    const snapshotStore = writable(snapshot([task('TB-1', 'ready')]));
    const handlers: Record<string, () => void> = {};
    const patchTask = vi.fn();

    const off = registerBoardTaskUpdateHandlers(snapshotStore, (name, handler) => {
      handlers[name] = handler;
      return () => delete handlers[name];
    }, patchTask);

    expect(Object.keys(handlers)).toEqual(['task:updated:TB-1']);

    snapshotStore.set(snapshot([task('TB-2', 'ready'), task('TB-3', 'archive')]));

    expect(Object.keys(handlers).sort()).toEqual(['task:updated:TB-2', 'task:updated:TB-3']);

    handlers['task:updated:TB-3']();
    expect(patchTask).toHaveBeenCalledWith('TB-3');

    off();
    expect(handlers).toEqual({});
  });
});

function snapshot(tasks: Task[]): BoardSnapshot {
  return {
    backlog: tasks.filter((t) => t.status === 'backlog'),
    ready: tasks.filter((t) => t.status === 'ready'),
    inProgress: tasks.filter((t) => t.status === 'in-progress'),
    codeReview: tasks.filter((t) => t.status === 'code-review'),
    done: tasks.filter((t) => t.status === 'done'),
    archive: tasks.filter((t) => t.status === 'archive'),
    wipLimits: {},
    wipCounts: {},
    wipEnforcement: 'warn',
  } as BoardSnapshot;
}

function task(id: string, status: Task['status']): Task {
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
    status,
    filePath: `board/${status}/${id}.md`,
    agent: '',
    agentStatus: '',
    agentResumable: false,
    groomedBy: '',
    groomStatus: '',
    implementedBy: '',
    implementStatus: '',
    reviewedBy: '',
    reviewStatus: '',
  };
}
