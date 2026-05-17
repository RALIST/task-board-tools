import { mount, tick, unmount } from 'svelte';
import { writable, type Writable } from 'svelte/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Capture every Events.On handler so tests can drive `agent:run-log` events
// at known points (during snapshot fetch, after snapshot resolves, etc).
const onMock = vi.hoisted(() => {
  const handlers: Array<{ event: string; cb: (e: { data: unknown[] }) => void }> = [];
  const off = vi.fn();
  const On = vi.fn((event: string, cb: (e: { data: unknown[] }) => void) => {
    handlers.push({ event, cb });
    return off;
  });
  return { handlers, off, On };
});

vi.mock('@wailsio/runtime', () => ({
  Events: { On: onMock.On },
}));

const apiMocks = vi.hoisted(() => ({
  getRunLog: vi.fn(),
  isRunLogNotFoundError: (e: unknown) => {
    const s = e instanceof Error ? e.message : String(e);
    return s.includes('run log not found');
  },
}));

vi.mock('$lib/api', () => apiMocks);

// runById returns a writable store per runId; tests set its value to drive
// the component's `isLive` derivation.
type RunRecord = { runId: string; taskId: string; status: string; exitCode: number } | null;
const runStores = new Map<string | null, Writable<RunRecord>>();

vi.mock('$lib/stores/runs', () => ({
  runById: (runId: string | null) => {
    let store = runStores.get(runId);
    if (!store) {
      store = writable<RunRecord>(null);
      runStores.set(runId, store);
    }
    return store;
  },
}));

import AgentRunLog from './AgentRunLog.svelte';
import Harness from './AgentRunLog.harness.test.svelte';

function setRun(runId: string, status: string, taskId = 'TB-1'): void {
  const store = runStores.get(runId) ?? writable<RunRecord>(null);
  runStores.set(runId, store);
  store.set({ runId, taskId, status, exitCode: 0 });
}

function emitLogLine(runId: string, line: string): void {
  for (const h of onMock.handlers) {
    if (h.event !== 'agent:run-log') continue;
    h.cb({ data: [{ run_id: runId, line, stream: 'stdout' }] });
  }
}

function preText(): string {
  return document.querySelector('pre')?.textContent ?? '';
}

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
  onMock.handlers.length = 0;
  onMock.off.mockReset();
  onMock.On.mockClear();
  apiMocks.getRunLog.mockReset();
  runStores.clear();
});

afterEach(() => {
  if (component) {
    try { unmount(component); } catch { /* ignore */ }
    component = null;
  }
});

async function flushAll() {
  // Run pending microtasks (promise continuations) then let Svelte reactivity settle.
  await Promise.resolve();
  await Promise.resolve();
  await tick();
}

describe('AgentRunLog live-run hydration', () => {
  it('hydrates from getRunLog when the selected run is running', async () => {
    setRun('r_live', 'running');
    apiMocks.getRunLog.mockResolvedValue('snap-line-1\nsnap-line-2\n');

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_live', taskId: 'TB-1' },
    });

    await flushAll();
    expect(apiMocks.getRunLog).toHaveBeenCalledWith('TB-1', 'r_live');
    expect(preText()).toBe('snap-line-1\nsnap-line-2');
  });

  it('hydrates from getRunLog when the selected run is queued', async () => {
    setRun('r_q', 'queued');
    apiMocks.getRunLog.mockResolvedValue('');

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_q', taskId: 'TB-1' },
    });

    await flushAll();
    expect(apiMocks.getRunLog).toHaveBeenCalledWith('TB-1', 'r_q');
    expect(preText()).toBe('');
  });

  it('appends live agent:run-log events after the snapshot resolves', async () => {
    setRun('r_live', 'running');
    apiMocks.getRunLog.mockResolvedValue('snap\n');

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_live', taskId: 'TB-1' },
    });

    await flushAll();
    emitLogLine('r_live', 'live-1');
    emitLogLine('r_live', 'live-2');
    await tick();
    expect(preText()).toBe('snap\nlive-1\nlive-2');
  });

  it('ignores events whose run_id does not match the selected run', async () => {
    setRun('r_live', 'running');
    apiMocks.getRunLog.mockResolvedValue('snap\n');

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_live', taskId: 'TB-1' },
    });

    await flushAll();
    emitLogLine('r_OTHER', 'other-1');
    emitLogLine('r_live', 'live-1');
    await tick();
    expect(preText()).toBe('snap\nlive-1');
  });

  it('does not duplicate a line that arrived during the in-flight snapshot fetch', async () => {
    setRun('r_live', 'running');
    let resolveSnapshot: ((text: string) => void) | undefined;
    apiMocks.getRunLog.mockReturnValue(new Promise<string>((resolve) => { resolveSnapshot = resolve; }));

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_live', taskId: 'TB-1' },
    });

    // Event fires BEFORE snapshot resolves; the line will also appear in
    // the snapshot the backend serves below. The merge must dedupe it.
    await tick();
    emitLogLine('r_live', 'overlap-line');
    expect(resolveSnapshot).toBeDefined();
    resolveSnapshot!('prior\noverlap-line\n');
    await flushAll();
    expect(preText()).toBe('prior\noverlap-line');
  });

  it('keeps genuinely-new pending events even at the cost of duplicating an overlap line (TB-144 review)', async () => {
    // Conservative dedupe (all-or-nothing): when pending starts with a
    // line that appears at the snapshot tail but the rest of pending
    // diverges, we cannot tell whether the first match is a true echo of
    // the snapshot or a real new emission. Preferring duplicates over
    // drops keeps `c` visible at the cost of one duplicate `b`.
    setRun('r_live', 'running');
    let resolveSnapshot: ((text: string) => void) | undefined;
    apiMocks.getRunLog.mockReturnValue(new Promise<string>((resolve) => { resolveSnapshot = resolve; }));

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_live', taskId: 'TB-1' },
    });

    await tick();
    emitLogLine('r_live', 'b');
    emitLogLine('r_live', 'c');
    resolveSnapshot!('a\nb\n');
    await flushAll();
    const text = preText();
    // Both `c` (the genuine new line) and at least one `b` must remain.
    expect(text).toContain('c');
    expect(text.startsWith('a\nb')).toBe(true);
  });

  it('strips ANSI from snapshot lines so dedupe matches stripped live lines (TB-144 review)', async () => {
    // The live branch already stripAnsi's incoming events; the snapshot
    // must do the same or colored on-disk content would never match a
    // stripped live event and we'd show the line twice.
    setRun('r_live', 'running');
    let resolveSnapshot: ((text: string) => void) | undefined;
    apiMocks.getRunLog.mockReturnValue(new Promise<string>((resolve) => { resolveSnapshot = resolve; }));

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_live', taskId: 'TB-1' },
    });

    await tick();
    emitLogLine('r_live', 'colored');
    // Snapshot has the same line with ANSI color sequences around it.
    resolveSnapshot!('[31mcolored[0m\n');
    await flushAll();
    expect(preText()).toBe('colored');
  });

  it('still renders live events when the snapshot returns not-found', async () => {
    setRun('r_live', 'queued');
    apiMocks.getRunLog.mockRejectedValue(new Error('run log not found'));

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_live', taskId: 'TB-1' },
    });

    await flushAll();
    emitLogLine('r_live', 'first-live');
    await tick();
    expect(preText()).toBe('first-live');
    expect(document.querySelector('.err')).toBeNull();
  });

  it('surfaces a non-not-found snapshot error and still flushes pending live lines', async () => {
    setRun('r_live', 'running');
    apiMocks.getRunLog.mockRejectedValue(new Error('disk read failed'));

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_live', taskId: 'TB-1' },
    });

    await flushAll();
    expect(document.querySelector('.err')?.textContent).toBe('disk read failed');
  });

  it('clears the old buffer when the selected run changes', async () => {
    setRun('r_old', 'running');
    setRun('r_new', 'running');
    apiMocks.getRunLog.mockImplementation(async (_t: string, runId: string) =>
      runId === 'r_old' ? 'old-snap\n' : 'new-snap\n',
    );

    component = mount(Harness, { target: document.body });
    (component as { setProps: (r: string | null, t: string | null) => void })
      .setProps('r_old', 'TB-1');
    await flushAll();
    emitLogLine('r_old', 'old-live');
    await tick();
    expect(preText()).toBe('old-snap\nold-live');

    (component as { setProps: (r: string | null, t: string | null) => void })
      .setProps('r_new', 'TB-1');
    await flushAll();
    // Events targeted at the old run must not leak into the new view.
    emitLogLine('r_old', 'leaked');
    emitLogLine('r_new', 'new-live');
    await tick();
    expect(preText()).toBe('new-snap\nnew-live');
  });
});

describe('AgentRunLog terminal-run behavior', () => {
  it('does not subscribe to agent:run-log when the run is terminal', async () => {
    setRun('r_done', 'success');
    apiMocks.getRunLog.mockResolvedValue('final-line\n');

    component = mount(AgentRunLog, {
      target: document.body,
      props: { runId: 'r_done', taskId: 'TB-1' },
    });

    await flushAll();
    expect(preText()).toBe('final-line\n');
    expect(onMock.On).not.toHaveBeenCalled();
  });
});
