import { describe, expect, it, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  runsByTask,
  runsForTask,
  selectedRunID,
  setRunsForTask,
  upsertRun,
  registerAgentEventHandlers,
  runById,
  _resetRunsStoreForTesting,
  type Run,
} from './runs';

function makeRun(over: Partial<Run> & { runId: string; taskId: string }): Run {
  return {
    agent: 'claude',
    mode: 'implement',
    queuedAt: '',
    startedAt: '',
    finishedAt: '',
    status: '',
    exitCode: 0,
    logPath: `/tmp/board/.agent-logs/${over.taskId}/${over.runId}.log`,
    ...over,
  };
}

describe('runsStore', () => {
  beforeEach(() => {
    _resetRunsStoreForTesting();
  });

  it('hydrates a task with 3 runs and returns them sorted desc', () => {
    setRunsForTask('TB-1', [
      makeRun({ runId: 'r_2', taskId: 'TB-1', startedAt: '2026-05-13T12:00:00Z', status: 'success' }),
      makeRun({ runId: 'r_1', taskId: 'TB-1', startedAt: '2026-05-13T10:00:00Z', status: 'success' }),
      makeRun({ runId: 'r_3', taskId: 'TB-1', startedAt: '2026-05-13T14:00:00Z', status: 'failed' }),
    ]);
    const list = runsByTask('TB-1');
    expect(list.map((r) => r.runId)).toEqual(['r_3', 'r_2', 'r_1']);
  });

  it('runsByTask filters by taskId', () => {
    setRunsForTask('TB-1', [makeRun({ runId: 'r_a', taskId: 'TB-1', startedAt: '2026-05-13T10:00:00Z' })]);
    setRunsForTask('TB-2', [makeRun({ runId: 'r_b', taskId: 'TB-2', startedAt: '2026-05-13T11:00:00Z' })]);
    expect(runsByTask('TB-1').map((r) => r.runId)).toEqual(['r_a']);
    expect(runsByTask('TB-2').map((r) => r.runId)).toEqual(['r_b']);
  });

  it('queued-without-started sorts by QueuedAt', () => {
    setRunsForTask('TB-1', [
      makeRun({ runId: 'r_old', taskId: 'TB-1', startedAt: '2026-05-13T10:00:00Z' }),
      // newest is queued but never started; expect it on top.
      makeRun({ runId: 'r_new', taskId: 'TB-1', queuedAt: '2026-05-13T15:00:00Z', status: 'queued' }),
    ]);
    const list = runsByTask('TB-1');
    expect(list[0].runId).toBe('r_new');
  });

  it('upsertRun patches existing entries', () => {
    upsertRun({ runId: 'r_x', taskId: 'TB-1', status: 'queued', queuedAt: '2026-05-13T10:00:00Z' });
    expect(runsByTask('TB-1')[0].status).toBe('queued');
    upsertRun({ runId: 'r_x', status: 'running', startedAt: '2026-05-13T10:00:01Z' });
    const after = runsByTask('TB-1')[0];
    expect(after.status).toBe('running');
    expect(after.taskId).toBe('TB-1'); // preserved
    expect(after.queuedAt).toBe('2026-05-13T10:00:00Z'); // preserved
  });

  it('runById is reactive', () => {
    const handle = runById('r_only');
    expect(get(handle)).toBeNull();
    upsertRun({ runId: 'r_only', taskId: 'TB-1', status: 'queued' });
    expect(get(handle)?.status).toBe('queued');
  });

  it('selectedRunID is a writable store', () => {
    selectedRunID.set('r_pick');
    expect(get(selectedRunID)).toBe('r_pick');
  });

  it('registerAgentEventHandlers handles queued/started/finished', () => {
    type Handler = (e: { data: unknown[] }) => void;
    const handlers: Record<string, Handler> = {};
    const fakeOn = (name: string, h: Handler) => {
      handlers[name] = h;
      return () => delete handlers[name];
    };
    registerAgentEventHandlers(fakeOn);

    handlers['agent:run-queued']({
      data: [{ run_id: 'r_e', task_id: 'TB-3', agent: 'claude', mode: 'implement' }],
    });
    expect(runsByTask('TB-3')[0].status).toBe('queued');

    handlers['agent:run-started']({
      data: [{ run_id: 'r_e', task_id: 'TB-3', agent: 'claude', mode: 'implement', pid: 1234 }],
    });
    expect(runsByTask('TB-3')[0].status).toBe('running');

    handlers['agent:run-finished']({
      data: [{ run_id: 'r_e', task_id: 'TB-3', status: 'success', exit_code: 0 }],
    });
    const final = runsByTask('TB-3')[0];
    expect(final.status).toBe('success');
    expect(final.exitCode).toBe(0);
  });

  it('runsForTask is reactive across upserts', () => {
    const r = runsForTask('TB-1');
    expect(get(r)).toEqual([]);
    upsertRun({ runId: 'r_a', taskId: 'TB-1', status: 'queued', queuedAt: '2026-05-13T10:00:00Z' });
    expect(get(r)).toHaveLength(1);
    upsertRun({ runId: 'r_b', taskId: 'TB-1', status: 'queued', queuedAt: '2026-05-13T11:00:00Z' });
    expect(get(r).map((x) => x.runId)).toEqual(['r_b', 'r_a']);
  });
});
