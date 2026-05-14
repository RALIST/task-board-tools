// Reactive store of agent runs, keyed by run_id.
//
// Hydration: when the drawer opens for a task, call hydrate(taskId) — that
// bulk-inserts the rolled-up Run records from `AgentService.ListRuns`.
//
// Mutation: incoming Wails events (`agent:run-queued`, `agent:run-started`,
// `agent:run-finished`) patch the matching entry. `agent:run-log` flows
// through AgentRunLog directly and does NOT pass through this store — log
// lines would blow up the store's update fan-out.
//
// Selectors: runsByTask returns the runs for a task, sorted by StartedAt
// desc (with QueuedAt as the tiebreaker for queued-only rows).

import { writable, derived, get } from 'svelte/store';

export interface Run {
  runId: string;
  taskId: string;
  agent: string;
  mode: 'implement' | 'groom';
  queuedAt: string;
  startedAt: string;
  finishedAt: string;
  status: 'queued' | 'running' | 'success' | 'failed' | 'cancelled' | '';
  exitCode: number;
  logPath: string;
}

type RunMap = Map<string, Run>;

const runs = writable<RunMap>(new Map());

/** selectedRunID is null until the drawer picks a row. Consumed by
 * AgentRunLog (which renders that run's log) and by TaskDrawer's
 * "Past runs" list highlighting. */
export const selectedRunID = writable<string | null>(null);

/** Subscribe to the raw runs map. Most consumers should use runsByTask. */
export const runsStore = { subscribe: runs.subscribe };

/** Replace every Run entry for a task with the freshly-fetched list.
 * Other tasks' entries are kept intact. */
export function setRunsForTask(taskId: string, list: Run[]): void {
  runs.update((m) => {
    // Drop any prior entries for this task.
    for (const [rid, r] of m) {
      if (r.taskId === taskId) m.delete(rid);
    }
    for (const r of list) {
      m.set(r.runId, normalize(r));
    }
    return new Map(m);
  });
}

/** Patch (or insert) a single Run by runId. */
export function upsertRun(patch: Partial<Run> & { runId: string }): void {
  runs.update((m) => {
    const prev = m.get(patch.runId);
    const next: Run = normalize({ ...emptyRun(patch.runId), ...prev, ...patch });
    m.set(patch.runId, next);
    return new Map(m);
  });
}

/** Selector: runs for a given task, sorted by StartedAt desc (QueuedAt
 * tiebreaker). */
export function runsByTask(taskId: string): Run[] {
  const map = get(runs);
  const list: Run[] = [];
  for (const r of map.values()) {
    if (r.taskId === taskId) list.push(r);
  }
  return list.sort(compareDesc);
}

/** Reactive variant of runsByTask. Use when the caller wants the list to
 * update with the store. */
export function runsForTask(taskId: string) {
  return derived(runs, ($runs) => {
    const list: Run[] = [];
    for (const r of $runs.values()) {
      if (r.taskId === taskId) list.push(r);
    }
    return list.sort(compareDesc);
  });
}

/** Reactive selector for a single Run by runId. */
export function runById(runId: string | null) {
  return derived(runs, ($runs) => (runId ? $runs.get(runId) ?? null : null));
}

/** Wire up the three Wails event handlers. Call once at app start. The
 * returned function unsubscribes — useful for tests; production hooks
 * stay live for the process lifetime. */
export function registerAgentEventHandlers(
  on: (event: string, handler: (e: { data: unknown[] }) => void) => () => void,
): () => void {
  const offs: Array<() => void> = [];

  offs.push(
    on('agent:run-queued', (e) => {
      const p = pickPayload(e);
      if (!p?.run_id) return;
      upsertRun({
        runId: String(p.run_id),
        taskId: String(p.task_id ?? ''),
        agent: String(p.agent ?? ''),
        mode: normalizeMode(p.mode),
        status: 'queued',
        queuedAt: nowISO(),
      });
    }),
  );

  offs.push(
    on('agent:run-started', (e) => {
      const p = pickPayload(e);
      if (!p?.run_id) return;
      upsertRun({
        runId: String(p.run_id),
        taskId: String(p.task_id ?? ''),
        agent: String(p.agent ?? ''),
        mode: normalizeMode(p.mode),
        status: 'running',
        startedAt: nowISO(),
      });
    }),
  );

  offs.push(
    on('agent:run-finished', (e) => {
      const p = pickPayload(e);
      if (!p?.run_id) return;
      upsertRun({
        runId: String(p.run_id),
        taskId: String(p.task_id ?? ''),
        status: (p.status as Run['status']) ?? 'failed',
        exitCode: Number(p.exit_code ?? -1),
        finishedAt: nowISO(),
      });
    }),
  );

  return () => {
    for (const off of offs) {
      try {
        off();
      } catch {
        /* ignore */
      }
    }
  };
}

/** Reset for tests. NOT for production use. */
export function _resetRunsStoreForTesting(): void {
  runs.set(new Map());
  selectedRunID.set(null);
}

// --- internals ---

function emptyRun(runId: string): Run {
  return {
    runId,
    taskId: '',
    agent: '',
    mode: 'implement',
    queuedAt: '',
    startedAt: '',
    finishedAt: '',
    status: '',
    exitCode: 0,
    logPath: '',
  };
}

function normalize(r: Run): Run {
  return { ...r, taskId: r.taskId ?? '', mode: normalizeMode(r.mode) };
}

function compareDesc(a: Run, b: Run): number {
  const keyA = Date.parse(a.startedAt) || Date.parse(a.queuedAt) || 0;
  const keyB = Date.parse(b.startedAt) || Date.parse(b.queuedAt) || 0;
  return keyB - keyA;
}

function pickPayload(e: { data: unknown[] }): Record<string, unknown> | null {
  if (!e?.data?.length) return null;
  const first = e.data[0];
  if (typeof first !== 'object' || first === null) return null;
  return first as Record<string, unknown>;
}

function nowISO(): string {
  return new Date().toISOString();
}

function normalizeMode(mode: unknown): Run['mode'] {
  if (mode === 'groom') return 'groom';
  return 'implement';
}
