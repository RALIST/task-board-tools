import type { Readable } from 'svelte/store';
import type { BoardSnapshot, Task } from './api';

type EventRegistrar = (event: string, handler: () => void) => () => void;
type TaskPatcher = (id: string) => void | Promise<void>;

export function registerBoardTaskUpdateHandlers(
  boardStore: Readable<BoardSnapshot>,
  on: EventRegistrar,
  patchTask: TaskPatcher,
): () => void {
  const taskListeners = new Map<string, () => void>();

  const offBoard = boardStore.subscribe((snap) => {
    const nextIds = taskIDs(snap);

    for (const [id, off] of taskListeners) {
      if (!nextIds.has(id)) {
        safeOff(off);
        taskListeners.delete(id);
      }
    }

    for (const id of nextIds) {
      if (taskListeners.has(id)) continue;
      taskListeners.set(id, on(`task:updated:${id}`, () => {
        void patchTask(id);
      }));
    }
  });

  return () => {
    safeOff(offBoard);
    for (const off of taskListeners.values()) safeOff(off);
    taskListeners.clear();
  };
}

function taskIDs(snap: BoardSnapshot): Set<string> {
  const ids = new Set<string>();
  for (const task of allTasks(snap)) {
    if (task.id) ids.add(task.id);
  }
  return ids;
}

function allTasks(snap: BoardSnapshot): Task[] {
  return [
    ...snap.backlog,
    ...(snap.ready ?? []),
    ...snap.inProgress,
    ...(snap.codeReview ?? []),
    ...snap.done,
    ...(snap.archive ?? []),
  ];
}

function safeOff(off: () => void): void {
  try {
    off();
  } catch {
    /* ignore */
  }
}
