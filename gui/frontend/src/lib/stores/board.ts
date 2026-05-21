// Reactive board store. The single source of truth on the frontend for the
// current snapshot. Patched in two ways:
//   1. loadBoard() — full refetch (called on mount and on `board:reloaded`).
//   2. patchTask(task) — surgical update from `task:updated:<id>` events.

import { writable, get } from 'svelte/store';
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
export const switchingTo = writable<string | null>(null);
let refreshInFlight: Promise<void> | null = null;
let refreshQueued = false;
let switchSeq = 0;
let boardLoadSeq = 0;
const patchSeqByID = new Map<string, number>();

// statusMode controls whether refresh() requests the archive bucket. The
// FilterBar's "Show archived" toggle writes this; callers shouldn't poke
// the underlying api directly.
export const statusMode = writable<StatusMode>('active');

export function beginBoardSwitch(projectRoot: string): void {
  switchSeq += 1;
  boardLoadSeq += 1;
  switchingTo.set(projectRoot);
  board.set(newEmptySnapshot());
  loadError.set(null);
  loaded.set(false);
  if (refreshInFlight) {
    refreshQueued = true;
  }
}

export async function refresh(): Promise<void> {
  if (refreshInFlight) {
    refreshQueued = true;
    return refreshInFlight;
  }

  refreshInFlight = drainRefreshQueue();
  try {
    await refreshInFlight;
  } finally {
    refreshInFlight = null;
  }
}

async function drainRefreshQueue(): Promise<void> {
  do {
    refreshQueued = false;
    await refreshOnce();
  } while (refreshQueued);
}

async function refreshOnce(): Promise<void> {
  const seq = switchSeq;
  try {
    const mode = get(statusMode);
    const snap = await loadBoard(mode);
    if (seq !== switchSeq) return;
    boardLoadSeq += 1;
    board.set(snap);
    loadError.set(null);
    loaded.set(true);
    switchingTo.set(null);
  } catch (err) {
    if (seq !== switchSeq || refreshQueued) return;
    // Clearing the snapshot prevents the previously-opened board's cards
    // from rendering under the newly selected project root when a board
    // switch fails (TB-145). Use a fresh object so mutators (e.g.
    // optimisticMove) can't poison the module-level reference.
    boardLoadSeq += 1;
    board.set(newEmptySnapshot());
    loadError.set(stringifyError(err));
    loaded.set(false);
    switchingTo.set(null);
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
  const seq = switchSeq;
  const loadSeq = boardLoadSeq;
  const patchSeq = (patchSeqByID.get(id) ?? 0) + 1;
  patchSeqByID.set(id, patchSeq);

  try {
    const detail = await getTask(id);
    if (seq !== switchSeq || loadSeq !== boardLoadSeq || patchSeqByID.get(id) !== patchSeq) return;
    const next = spliceTask(get(board), detail.metadata);
    board.set(next);
  } catch {
    // If the task was deleted in the meantime, a `board:reloaded` will
    // arrive shortly and a full refresh will reconcile.
  }
}

function spliceTask(snap: BoardSnapshot, t: Task): BoardSnapshot {
  const next = {
    ...snap,
    backlog: snap.backlog.filter((x) => x.id !== t.id),
    ready: (snap.ready ?? []).filter((x) => x.id !== t.id),
    inProgress: snap.inProgress.filter((x) => x.id !== t.id),
    codeReview: (snap.codeReview ?? []).filter((x) => x.id !== t.id),
    done: snap.done.filter((x) => x.id !== t.id),
    archive: (snap.archive ?? []).filter((x) => x.id !== t.id),
  } as BoardSnapshot;

  const target = columnForStatus(t.status);
  if (target === null) return next;
  if (target === 'archive' && get(statusMode) !== 'all') return next;

  const original = columnTasks(snap, target);
  const insertAt = original.findIndex((x) => x.id === t.id);
  const current = columnTasks(next, target);
  const index = insertAt === -1 ? current.length : Math.min(insertAt, current.length);
  setColumnTasks(next, target, [
    ...current.slice(0, index),
    t,
    ...current.slice(index),
  ]);
  return next;
}

type BoardColumn = 'backlog' | 'ready' | 'inProgress' | 'codeReview' | 'done' | 'archive';

function columnForStatus(status: string): BoardColumn | null {
  switch (status) {
    case 'backlog':
      return 'backlog';
    case 'ready':
      return 'ready';
    case 'in-progress':
      return 'inProgress';
    case 'code-review':
      return 'codeReview';
    case 'done':
      return 'done';
    case 'archive':
      return 'archive';
    default:
      return null;
  }
}

function columnTasks(snap: BoardSnapshot, column: BoardColumn): Task[] {
  switch (column) {
    case 'backlog':
      return snap.backlog;
    case 'ready':
      return snap.ready ?? [];
    case 'inProgress':
      return snap.inProgress;
    case 'codeReview':
      return snap.codeReview ?? [];
    case 'done':
      return snap.done;
    case 'archive':
      return snap.archive ?? [];
  }
}

function setColumnTasks(snap: BoardSnapshot, column: BoardColumn, tasks: Task[]): void {
  switch (column) {
    case 'backlog':
      snap.backlog = tasks;
      break;
    case 'ready':
      snap.ready = tasks;
      break;
    case 'inProgress':
      snap.inProgress = tasks;
      break;
    case 'codeReview':
      snap.codeReview = tasks;
      break;
    case 'done':
      snap.done = tasks;
      break;
    case 'archive':
      snap.archive = tasks;
      break;
  }
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

function stringifyError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}

function newEmptySnapshot(): BoardSnapshot {
  return {
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
}
