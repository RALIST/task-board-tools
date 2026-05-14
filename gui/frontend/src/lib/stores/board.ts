// Reactive board store. The single source of truth on the frontend for the
// current snapshot. Patched in two ways:
//   1. loadBoard() — full refetch (called on mount and on `board:reloaded`).
//   2. patchTask(task) — surgical update from `task:updated:<id>` events.

import { writable, derived, get } from 'svelte/store';
import type { BoardSnapshot, StatusMode, Task } from '../api';
import { loadBoard, getTask } from '../api';

const emptySnapshot: BoardSnapshot = {
  backlog: [],
  inProgress: [],
  done: [],
  archive: [],
} as BoardSnapshot;

// loaded toggles to true after the first successful loadBoard so empty
// columns and "still loading" can be distinguished by callers.
export const loaded = writable<boolean>(false);
export const board = writable<BoardSnapshot>(emptySnapshot);
export const loadError = writable<string | null>(null);
let refreshSeq = 0;

// statusMode controls whether refresh() requests the archive bucket. The
// FilterBar's "Show archived" toggle writes this; callers shouldn't poke
// the underlying api directly.
export const statusMode = writable<StatusMode>('active');

export async function refresh(): Promise<void> {
  const seq = ++refreshSeq;
  try {
    const mode = get(statusMode);
    const snap = await loadBoard(mode);
    if (seq !== refreshSeq) return;
    board.set(snap);
    loadError.set(null);
    loaded.set(true);
  } catch (err) {
    if (seq !== refreshSeq) return;
    loadError.set(stringifyError(err));
  }
}

export function setStatusMode(mode: StatusMode): void {
  statusMode.set(mode);
  // Fire-and-forget: callers can await refresh() themselves if they need
  // the new snapshot to be in store before continuing.
  void refresh();
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
    archive: (snap.archive ?? []).filter((x) => x.id !== t.id),
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
    case 'archive':
      // Only surface archive entries when the store is in 'all' mode;
      // otherwise drop so the active board doesn't show archived cards.
      if (get(statusMode) === 'all') {
        withoutTask.archive = [...withoutTask.archive, t];
      }
      break;
  }
  return withoutTask;
}

// optimisticMove updates the local snapshot synchronously so a drag-drop
// feels instant. Callers must keep the original snapshot if MoveTask fails
// so they can revert with board.set(original).
export function optimisticMove(id: string, target: 'backlog' | 'in-progress' | 'done'): BoardSnapshot {
  const snap = get(board);
  // Locate the task in any column.
  let task: Task | undefined;
  for (const col of [snap.backlog, snap.inProgress, snap.done, snap.archive ?? []]) {
    const hit = col.find((x) => x.id === id);
    if (hit) { task = { ...hit, status: target }; break; }
  }
  if (!task) return snap;
  const next = spliceTask(snap, task);
  board.set(next);
  return snap;
}

export function revert(snap: BoardSnapshot): void {
  board.set(snap);
}

export const totalTaskCount = derived(board, ($b) =>
  $b.backlog.length + $b.inProgress.length + $b.done.length,
);

function stringifyError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}
