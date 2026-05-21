import { describe, expect, it, vi } from 'vitest';
import { handleBoardDrop, type BoardDropDependencies } from './boardDrop';
import type { BoardSnapshot, Task } from './api';

describe('handleBoardDrop', () => {
  it('routes backlog-to-ready drops through readyTask', async () => {
    const deps = depsFor(snapshot([task('TB-1', 'Backlog task', 'backlog')]));

    await handleBoardDrop('TB-1', 'ready', deps);

    expect(deps.readyTask).toHaveBeenCalledWith('TB-1');
    expect(deps.pullTask).not.toHaveBeenCalled();
    expect(deps.moveTask).not.toHaveBeenCalled();
  });

  it('routes ready-to-in-progress drops through pullTask', async () => {
    const deps = depsFor(snapshot([], [task('TB-2', 'Ready task', 'ready')]));

    await handleBoardDrop('TB-2', 'in-progress', deps);

    expect(deps.readyTask).not.toHaveBeenCalled();
    expect(deps.pullTask).toHaveBeenCalledWith('TB-2');
    expect(deps.moveTask).not.toHaveBeenCalled();
  });

  it('routes generic active-column drops through moveTask', async () => {
    const deps = depsFor(snapshot([], [], [task('TB-3', 'Active task', 'in-progress')]));

    await handleBoardDrop('TB-3', 'done', deps);

    expect(deps.readyTask).not.toHaveBeenCalled();
    expect(deps.pullTask).not.toHaveBeenCalled();
    expect(deps.moveTask).toHaveBeenCalledWith('TB-3', 'done');
  });

  it('reverts and pushes a readable toast when backlog-to-ready validation fails', async () => {
    const original = snapshot([task('TB-285', 'Ungroomed example', 'backlog')]);
    const moved = snapshot([], [task('TB-285', 'Ungroomed example', 'ready')]);
    const err = new Error('structured transport envelope');
    const deps: BoardDropDependencies = {
      snapshot: () => original,
      optimisticMove: vi.fn(() => moved),
      revert: vi.fn(),
      readyTask: vi.fn().mockRejectedValue(err),
      pullTask: vi.fn(),
      moveTask: vi.fn(),
      pushToast: vi.fn(),
      formatError: vi.fn(() => 'TB-285 is not ready - needs grooming. Fix with: tb triage TB-285'),
    };

    await handleBoardDrop('TB-285', 'ready', deps);

    expect(deps.readyTask).toHaveBeenCalledWith('TB-285');
    expect(deps.pullTask).not.toHaveBeenCalled();
    expect(deps.moveTask).not.toHaveBeenCalled();
    expect(deps.optimisticMove).toHaveBeenCalledWith('TB-285', 'ready');
    expect(deps.revert).toHaveBeenCalledWith(moved);
    expect(deps.pushToast).toHaveBeenCalledWith(
      'Move failed: TB-285 is not ready - needs grooming. Fix with: tb triage TB-285',
    );
  });
});

function depsFor(original: BoardSnapshot): BoardDropDependencies {
  return {
    snapshot: () => original,
    optimisticMove: vi.fn(() => original),
    revert: vi.fn(),
    readyTask: vi.fn(),
    pullTask: vi.fn(),
    moveTask: vi.fn(),
    pushToast: vi.fn(),
    formatError: vi.fn((err: unknown) => (err instanceof Error ? err.message : String(err))),
  };
}

function snapshot(
  backlog: Task[],
  ready: Task[] = [],
  inProgress: Task[] = [],
): BoardSnapshot {
  return {
    backlog,
    ready,
    inProgress,
    codeReview: [],
    done: [],
    archive: [],
    wipLimits: {},
    wipCounts: {},
    wipEnforcement: 'warn',
  } as BoardSnapshot;
}

function task(id: string, title: string, status: Task['status']): Task {
  return {
    id,
    title,
    type: 'bug',
    priority: 'P2',
    size: 'S',
    module: 'gui-frontend',
    tags: [],
    branch: '',
    reviewRef: '',
    parent: '',
    status,
    filePath: `board/${status}/${id}/TASK.md`,
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
