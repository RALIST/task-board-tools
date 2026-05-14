import { get } from 'svelte/store';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getTriage = vi.fn<() => Promise<Record<string, string[]>>>();

vi.mock('$lib/api', () => ({
  getTriage: () => getTriage(),
}));

const {
  _resetTriageStoreForTesting,
  needsGrooming,
  reasonsFor,
  refreshTriageTask,
  registerTaskTriageEventHandler,
  registerTriageEventHandlers,
  triageForTask,
  triageStore,
} = await import('./triage');

describe('triageStore', () => {
  beforeEach(() => {
    _resetTriageStoreForTesting();
    getTriage.mockReset();
  });

  it('starts empty and seeds on first subscribe', async () => {
    getTriage.mockResolvedValue({ 'TB-1': ['no goal'] });
    const seen: Array<Map<string, string[]>> = [];

    const off = triageStore.subscribe((value) => seen.push(value));
    await vi.waitFor(() => expect(getTriage).toHaveBeenCalledTimes(1));

    expect(seen[0].size).toBe(0);
    expect(needsGrooming('TB-1')).toBe(true);
    expect(reasonsFor('TB-1')).toEqual(['no goal']);
    off();
  });

  it('retries seeding after an initial triage load failure', async () => {
    getTriage
      .mockRejectedValueOnce(new Error('backend unavailable'))
      .mockResolvedValueOnce({ 'TB-1': ['no goal'] });

    const offStore = triageStore.subscribe(() => {});
    await vi.waitFor(() => expect(getTriage).toHaveBeenCalledTimes(1));
    expect(needsGrooming('TB-1')).toBe(false);

    const offTask = triageForTask('TB-1').subscribe(() => {});
    await vi.waitFor(() => expect(getTriage).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(needsGrooming('TB-1')).toBe(true));

    offTask();
    offStore();
  });

  it('refreshes the full map on board:reloaded', async () => {
    getTriage
      .mockResolvedValueOnce({ 'TB-1': ['no goal'] })
      .mockResolvedValueOnce({ 'TB-2': ['no module'] });
    const handlers: Record<string, (e: { data: unknown[] }) => void> = {};

    const offStore = triageStore.subscribe(() => {});
    await vi.waitFor(() => expect(needsGrooming('TB-1')).toBe(true));
    const offEvents = registerTriageEventHandlers((name, handler) => {
      handlers[name] = handler;
      return () => delete handlers[name];
    });

    handlers['board:reloaded']({ data: [] });
    await vi.waitFor(() => expect(needsGrooming('TB-2')).toBe(true));

    expect(needsGrooming('TB-1')).toBe(false);
    offEvents();
    offStore();
  });

  it('refreshes a single task on task:updated:<id>', async () => {
    getTriage
      .mockResolvedValueOnce({ 'TB-1': ['no goal'], 'TB-2': ['no module'] })
      .mockResolvedValueOnce({ 'TB-1': ['no goal', 'no acceptance criteria'] });
    const handlers: Record<string, (e: { data: unknown[] }) => void> = {};

    const offStore = triageStore.subscribe(() => {});
    await vi.waitFor(() => expect(needsGrooming('TB-2')).toBe(true));
    const offTask = registerTaskTriageEventHandler('TB-1', (name, handler) => {
      handlers[name] = handler;
      return () => delete handlers[name];
    });

    handlers['task:updated:TB-1']({ data: ['TB-1'] });
    await vi.waitFor(() => expect(reasonsFor('TB-1')).toEqual(['no goal', 'no acceptance criteria']));

    expect(needsGrooming('TB-2')).toBe(true);
    offTask();
    offStore();
  });

  it('removes a single task when it is no longer flagged', async () => {
    getTriage
      .mockResolvedValueOnce({ 'TB-1': ['no goal'] })
      .mockResolvedValueOnce({});

    const offStore = triageStore.subscribe(() => {});
    await vi.waitFor(() => expect(needsGrooming('TB-1')).toBe(true));

    await refreshTriageTask('TB-1');
    expect(needsGrooming('TB-1')).toBe(false);
    expect(get(triageForTask('TB-1'))).toEqual([]);
    offStore();
  });

  it('keeps existing state when a per-task refresh fails', async () => {
    getTriage
      .mockResolvedValueOnce({ 'TB-1': ['no goal'] })
      .mockRejectedValueOnce(new Error('backend unavailable'));

    const offStore = triageStore.subscribe(() => {});
    await vi.waitFor(() => expect(needsGrooming('TB-1')).toBe(true));

    await expect(refreshTriageTask('TB-1')).resolves.toBeUndefined();
    expect(needsGrooming('TB-1')).toBe(true);
    expect(reasonsFor('TB-1')).toEqual(['no goal']);

    offStore();
  });
});
