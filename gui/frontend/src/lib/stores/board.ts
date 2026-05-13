// Reactive board store. The single source of truth on the frontend for the
// current snapshot. Patched in two ways:
//   1. loadBoard() — full refetch (called on mount and on `board:reloaded`).
//   2. patchTask(task) — surgical update from `task:updated:<id>` events.

import { writable, derived, get } from 'svelte/store';
import type { BoardSnapshot, Task } from '../api';
import { loadBoard, getTask } from '../api';

const emptySnapshot: BoardSnapshot = {
  backlog: [],
  inProgress: [],
  done: [],
} as BoardSnapshot;

// loaded toggles to true after the first successful loadBoard so empty
// columns and "still loading" can be distinguished by callers.
export const loaded = writable<boolean>(false);
export const board = writable<BoardSnapshot>(emptySnapshot);
export const loadError = writable<string | null>(null);

export async function refresh(): Promise<void> {
  try {
    const snap = await loadBoard();
    board.set(snap);
    loadError.set(null);
    loaded.set(true);
  } catch (err) {
    loadError.set(stringifyError(err));
  }
}

// patchTask re-fetches a single task and splices it into whichever column
// matches its current status. Used by the `task:updated:<id>` Wails event
// stream so a metadata edit never triggers a full LoadBoard.
export async function patchTask(id: string): Promise<void> {
  try {
    const detail = await getTask(id);
    const next = spliceTask(get(board), detail.metadata);
    board.set(next);
  } catch {
    // If the task was deleted in the meantime, a `board:reloaded` will
    // arrive shortly and a full refresh will reconcile.
  }
}

function spliceTask(snap: BoardSnapshot, t: Task): BoardSnapshot {
  // Remove the task from every column first, then insert into the correct
  // one. Cheaper than diffing.
  const withoutTask = {
    backlog: snap.backlog.filter((x) => x.id !== t.id),
    inProgress: snap.inProgress.filter((x) => x.id !== t.id),
    done: snap.done.filter((x) => x.id !== t.id),
  } as BoardSnapshot;

  switch (t.status) {
    case 'backlog':
      withoutTask.backlog = [...withoutTask.backlog, t];
      break;
    case 'in-progress':
      withoutTask.inProgress = [...withoutTask.inProgress, t];
      break;
    case 'done':
      withoutTask.done = [...withoutTask.done, t];
      break;
    // archive / unknown → drop
  }
  return withoutTask;
}

export const totalTaskCount = derived(board, ($b) =>
  $b.backlog.length + $b.inProgress.length + $b.done.length,
);

function stringifyError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}
