// Reactive board store. The single source of truth on the frontend for the
// current snapshot. Patched in two ways:
//   1. loadBoard() — full refetch (called on mount and on `board:reloaded`).
//   2. patchTask(task) — surgical update from `task:updated:<id>` events.

import { writable, derived, get } from 'svelte/store';
import type { BoardSnapshot, StatusMode, Task } from '../api';
import { loadBoard, getTask } from '../api';

const emptySnapshot: BoardSnapshot = {
  backlog: [],
  ready: [],
  inProgress: [],
  codeReview: [],
  done: [],
  archive: [],
  wipLimits: {},
  wipCounts: {},
  wipEnforcement: 'warn',
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
    // Clearing the snapshot prevents the previously-opened board's cards
    // from rendering under the newly selected project root when a board
    // switch fails (TB-145). Use a fresh object so mutators (e.g.
    // optimisticMove) can't poison the module-level reference.
    board.set({
      backlog: [],
      ready: [],
      inProgress: [],
      codeReview: [],
      done: [],
      archive: [],
      wipLimits: {},
      wipCounts: {},
      wipEnforcement: 'warn',
    });
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
    ready: (snap.ready ?? []).filter((x) => x.id !== t.id),
    inProgress: snap.inProgress.filter((x) => x.id !== t.id),
    codeReview: (snap.codeReview ?? []).filter((x) => x.id !== t.id),
    done: snap.done.filter((x) => x.id !== t.id),
    archive: (snap.archive ?? []).filter((x) => x.id !== t.id),
  } as BoardSnapshot;

  switch (t.status) {
    case 'backlog':
      withoutTask.backlog = [...withoutTask.backlog, t];
      break;
    case 'ready':
      withoutTask.ready = [...(withoutTask.ready ?? []), t];
      break;
    case 'in-progress':
      withoutTask.inProgress = [...withoutTask.inProgress, t];
      break;
    case 'code-review':
      withoutTask.codeReview = [...(withoutTask.codeReview ?? []), t];
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
export function optimisticMove(
  id: string,
  target: 'backlog' | 'ready' | 'in-progress' | 'code-review' | 'done',
): BoardSnapshot {
  const snap = get(board);
  // Locate the task in any column.
  let task: Task | undefined;
  for (const col of [snap.backlog, snap.ready ?? [], snap.inProgress, snap.codeReview ?? [], snap.done, snap.archive ?? []]) {
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
  $b.backlog.length + ($b.ready?.length ?? 0) + $b.inProgress.length + ($b.codeReview?.length ?? 0) + $b.done.length,
);

function stringifyError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}
