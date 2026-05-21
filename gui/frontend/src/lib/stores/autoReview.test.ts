import { get } from 'svelte/store';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getAutoReviewStatus = vi.fn<() => Promise<unknown>>();

vi.mock('$lib/api', () => ({
  getAutoReviewStatus: () => getAutoReviewStatus(),
}));

const {
  _resetAutoReviewStoreForTesting,
  autoReviewNeedsDefaultAgent,
  autoReviewSkipReasonFor,
  autoReviewStore,
  refresh,
  registerAutoReviewEventHandlers,
} = await import('./autoReview');

function snapshot(overrides: Record<string, unknown> = {}) {
  return {
    enabled: true,
    default_agent: 'claude',
    needs_default_agent: false,
    last_scan_at: '2026-05-21T12:00:00Z',
    last_skip_reasons: {} as Record<string, string>,
    ...overrides,
  };
}

describe('autoReviewStore', () => {
  beforeEach(() => {
    _resetAutoReviewStoreForTesting();
    getAutoReviewStatus.mockReset();
  });

  it('starts disabled / unloaded until refresh', () => {
    const state = get(autoReviewStore);
    expect(state.enabled).toBe(false);
    expect(state.loaded).toBe(false);
  });

  it('refresh() pulls coordinator status', async () => {
    getAutoReviewStatus.mockResolvedValue(
      snapshot({ last_skip_reasons: { 'TB-1': 'missing ReviewRef' } }),
    );

    await refresh();

    expect(get(autoReviewStore)).toMatchObject({
      enabled: true,
      defaultAgent: 'claude',
      lastSkipReasons: { 'TB-1': 'missing ReviewRef' },
      loaded: true,
    });
  });

  it('keeps default state quietly when backend rejects', async () => {
    getAutoReviewStatus.mockRejectedValue(new Error('no board'));

    await refresh();

    expect(get(autoReviewStore).loaded).toBe(false);
  });

  it('exposes derived needs-default and per-task skip selectors', async () => {
    getAutoReviewStatus.mockResolvedValue(
      snapshot({
        default_agent: 'none',
        needs_default_agent: true,
        last_skip_reasons: { 'TB-1': 'worker capacity full' },
      }),
    );
    await refresh();

    expect(get(autoReviewNeedsDefaultAgent)).toBe(true);
    expect(get(autoReviewSkipReasonFor('TB-1'))).toBe('worker capacity full');
    expect(get(autoReviewSkipReasonFor('TB-X'))).toBeUndefined();
  });

  it('refetches Status on coordinator events', async () => {
    getAutoReviewStatus
      .mockResolvedValueOnce(snapshot({ last_skip_reasons: {} }))
      .mockResolvedValueOnce(snapshot({ needs_default_agent: true, default_agent: 'none' }))
      .mockResolvedValueOnce(snapshot({ last_skip_reasons: { 'TB-1': 'duplicate review epoch' } }))
      .mockResolvedValueOnce(snapshot({ last_skip_reasons: { 'TB-1': 'missing ReviewRef' } }))
      .mockResolvedValueOnce(snapshot({ last_skip_reasons: {} }));
    await refresh();
    expect(getAutoReviewStatus).toHaveBeenCalledTimes(1);

    const handlers: Record<string, (e: { data: unknown[] }) => void> = {};
    const off = registerAutoReviewEventHandlers((name, handler) => {
      handlers[name] = handler;
      return () => delete handlers[name];
    });

    handlers['auto-review:needs-default-agent']({ data: [] });
    await vi.waitFor(() => expect(getAutoReviewStatus).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(get(autoReviewStore).needsDefaultAgent).toBe(true));

    handlers['auto-review:queued']({ data: [] });
    await vi.waitFor(() => expect(getAutoReviewStatus).toHaveBeenCalledTimes(3));

    handlers['auto-review:needs-user']({ data: [] });
    await vi.waitFor(() => expect(getAutoReviewStatus).toHaveBeenCalledTimes(4));

    handlers['auto-review:scan-complete']({ data: [] });
    await vi.waitFor(() => expect(getAutoReviewStatus).toHaveBeenCalledTimes(5));

    off();
  });
});
