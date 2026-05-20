import type { BoardSnapshot, Task } from './api';

export type BoardDropTarget = 'backlog' | 'ready' | 'in-progress' | 'code-review' | 'done';
type BoardSourceStatus = BoardDropTarget | 'archive';

export interface BoardDropDependencies {
  snapshot: () => BoardSnapshot;
  optimisticMove: (id: string, target: BoardDropTarget) => BoardSnapshot;
  revert: (snap: BoardSnapshot) => void;
  readyTask: (id: string) => Promise<void>;
  pullTask: (id: string) => Promise<void>;
  moveTask: (id: string, target: BoardDropTarget) => Promise<void>;
  pushToast: (message: string) => void;
  formatError: (err: unknown) => string;
}

export async function handleBoardDrop(
  taskId: string,
  target: BoardDropTarget,
  deps: BoardDropDependencies,
): Promise<void> {
  const source = sourceStatusOfSnapshot(deps.snapshot(), taskId);
  const before = deps.optimisticMove(taskId, target);
  try {
    if (source === 'backlog' && target === 'ready') {
      await deps.readyTask(taskId);
    } else if (source === 'ready' && target === 'in-progress') {
      await deps.pullTask(taskId);
    } else {
      await deps.moveTask(taskId, target);
    }
  } catch (err) {
    deps.revert(before);
    deps.pushToast(`Move failed: ${deps.formatError(err)}`);
  }
}

export function sourceStatusOfSnapshot(
  snap: BoardSnapshot,
  id: string,
): BoardSourceStatus | undefined {
  const columns: Array<[BoardSourceStatus, Task[] | undefined]> = [
    ['backlog', snap.backlog],
    ['ready', snap.ready],
    ['in-progress', snap.inProgress],
    ['code-review', snap.codeReview],
    ['done', snap.done],
    ['archive', snap.archive],
  ];
  for (const [status, tasks] of columns) {
    if ((tasks ?? []).some((task) => task.id === id)) return status;
  }
  return undefined;
}
