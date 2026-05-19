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
  mode: 'implement' | 'groom' | 'review' | 'resume';
  queuedAt: string;
  startedAt: string;
  finishedAt: string;
  status: 'queued' | 'running' | 'success' | 'failed' | 'cancelled' | 'interrupted' | '';
  exitCode: number;
  logPath: string;
  // TB-130 resume linkage. sessionId is the agent-side conversation id
  // (claude/codex). resumedFrom carries the parent session's id for runs
  // launched via ResumeAgent. resumedFromRun is the parent run's id —
  // that's what the UI shows as a chip; session ids stay internal.
  sessionId?: string;
  resumedFrom?: string;
  resumedFromRun?: string;
}

type RunMap = Map<string, Run>;

const runs = writable<RunMap>(new Map());

/** selectedRunID is null until the drawer picks a row. Consumed by
 * AgentRunLog (which renders that run's log) and by TaskDrawer's
 * "Past runs" list highlighting. */
export const selectedRunID = writable<string | null>(null);

/** Subscribe to the raw runs map. Most consumers should use runsByTask. */
export const runsStore = { subscribe: runs.subscribe };

/** Merge a freshly-fetched run list for a task into the store.
 *
 * The list comes from `AgentService.ListRuns` (i.e. the JSONL snapshot on
 * disk). Because the backend writes JSONL FIRST and then emits Wails
 * `agent:run-*` events, the event-driven handlers in
 * `registerAgentEventHandlers` can advance a run's `status` (queued →
 * running → terminal) BEFORE a slightly older `listRuns` promise resolves
 * with a snapshot that still shows the earlier status. A naive bulk-replace
 * would regress the visible status (TB-219: drawer kept showing QUEUED for
 * a run whose `agent:run-started` event had already fired).
 *
 * Strategy: pure merge — for each incoming run, prefer whichever side
 * carries the more-advanced status (terminal > running > queued > '') and
 * let incoming fill in any blank timing fields the store didn't yet know
 * about. NEVER delete entries the snapshot omits: a live event could have
 * just inserted a run whose queued JSONL line the snapshot reader missed
 * because of disk-flush ordering. The next snapshot (or another live
 * event) will reconcile, and a leaked entry is far less harmful than a
 * dropped live run.
 */
export function setRunsForTask(taskId: string, list: Run[]): void {
  runs.update((m) => {
    for (const r of list) {
      const prev = m.get(r.runId);
      m.set(r.runId, mergePreferAdvanced(prev, normalize(r)));
    }
    return new Map(m);
  });
}

/** Status precedence used to decide which side of a merge wins:
 *  terminal (success/failed/cancelled) > running > queued > unset. */
function statusRank(status: Run['status']): number {
  switch (status) {
    case '':
      return -1;
    case 'queued':
      return 0;
    case 'running':
      return 1;
    default:
      return 2;
  }
}

function mergePreferAdvanced(prev: Run | undefined, incoming: Run): Run {
  if (!prev) return incoming;
  if (statusRank(prev.status) > statusRank(incoming.status)) {
    // The in-store entry has seen a later event than the snapshot. Keep
    // its status/exitCode/timestamps but accept any fields the snapshot
    // can fill in (agent name, mode, runId — all stable across events).
    return {
      ...incoming,
      status: prev.status,
      exitCode: prev.exitCode,
      queuedAt: prev.queuedAt || incoming.queuedAt,
      startedAt: prev.startedAt || incoming.startedAt,
      finishedAt: prev.finishedAt || incoming.finishedAt,
    };
  }
  return incoming;
}

/** Patch (or insert) a single Run by runId.
 *
 * Routes through `mergePreferAdvanced` so a late `queued` or `started`
 * event can't regress a run that has already advanced to a terminal
 * status — e.g. a `agent:run-started` event delayed in the Wails channel
 * arriving after the synchronous `agent:run-finished` is processed.
 * The optimistic `queued` insert at `TaskDrawer.svelte` (when the user
 * clicks Run) is also protected: if Wails delivers `agent:run-started`
 * before our optimistic insert lands, the insert won't wipe `running`.
 */
export function upsertRun(patch: Partial<Run> & { runId: string }): void {
  runs.update((m) => {
    const prev = m.get(patch.runId);
    const candidate: Run = normalize({ ...emptyRun(patch.runId), ...prev, ...patch });
    m.set(patch.runId, mergePreferAdvanced(prev, candidate));
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
      const patch: Partial<Run> & { runId: string } = {
        runId: String(p.run_id),
        taskId: String(p.task_id ?? ''),
        agent: String(p.agent ?? ''),
        mode: normalizeMode(p.mode),
        status: 'queued',
        queuedAt: nowISO(),
      };
      // TB-130 resume linkage — only the resume path emits these so
      // they stay undefined for regular RunAgent / GroomTask events.
      if (typeof p.resumed_from === 'string' && p.resumed_from) {
        patch.resumedFrom = p.resumed_from;
      }
      if (typeof p.resumed_from_run === 'string' && p.resumed_from_run) {
        patch.resumedFromRun = p.resumed_from_run;
      }
      upsertRun(patch);
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
  if (mode === 'review') return 'review';
  if (mode === 'resume') return 'resume';
  return 'implement';
}
