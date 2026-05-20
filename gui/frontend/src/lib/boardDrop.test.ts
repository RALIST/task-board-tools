import { describe, expect, it, vi } from 'vitest';
import { handleBoardDrop, type BoardDropDependencies } from './boardDrop';
import type { BoardSnapshot, Task } from './api';

describe('handleBoardDrop', () => {
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

function snapshot(backlog: Task[], ready: Task[] = []): BoardSnapshot {
  return {
    backlog,
    ready,
    inProgress: [],
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
