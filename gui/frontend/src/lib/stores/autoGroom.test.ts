import { get } from 'svelte/store';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getAutoGroomStatus = vi.fn<() => Promise<unknown>>();

vi.mock('$lib/api', () => ({
  getAutoGroomStatus: () => getAutoGroomStatus(),
}));

const {
  _resetAutoGroomStoreForTesting,
  autoGroomStore,
  autoGroomNeedsDefaultAgent,
  refresh,
  registerAutoGroomEventHandlers,
  settleSkipReasonFor,
  settleEligibleAtFor,
} = await import('./autoGroom');

function snapshot(overrides: Record<string, unknown> = {}) {
  return {
    enabled: true,
    default_agent: 'claude',
    needs_default_agent: false,
    settle_minutes: 5,
    last_scan_at: '2026-05-20T12:00:00Z',
    last_skip_reasons: {} as Record<string, string>,
    settle_eligible_at_ms: {} as Record<string, number>,
    ...overrides,
  };
}

describe('autoGroomStore', () => {
  beforeEach(() => {
    _resetAutoGroomStoreForTesting();
    getAutoGroomStatus.mockReset();
  });

  it('starts disabled / unloaded until refresh', () => {
    const state = get(autoGroomStore);
    expect(state.enabled).toBe(false);
    expect(state.loaded).toBe(false);
  });

  it('refresh() pulls the coordinator snapshot and marks loaded', async () => {
    getAutoGroomStatus.mockResolvedValue(
      snapshot({
        enabled: true,
        default_agent: 'claude',
        settle_minutes: 7,
        last_skip_reasons: { 'TB-1': 'settle' },
        settle_eligible_at_ms: { 'TB-1': 1_700_000_000_000 },
      }),
    );

    await refresh();

    const state = get(autoGroomStore);
    expect(state).toMatchObject({
      enabled: true,
      defaultAgent: 'claude',
      settleMinutes: 7,
      lastSkipReasons: { 'TB-1': 'settle' },
      settleEligibleAtMs: { 'TB-1': 1_700_000_000_000 },
      loaded: true,
    });
  });

  it('keeps default state quietly when the backend rejects (no toast)', async () => {
    getAutoGroomStatus.mockRejectedValue(new Error('no board'));

    await refresh();

    const state = get(autoGroomStore);
    expect(state.loaded).toBe(false);
    expect(state.enabled).toBe(false);
  });

  it('exposes a derived autoGroomNeedsDefaultAgent flag', async () => {
    getAutoGroomStatus.mockResolvedValue(
      snapshot({ enabled: true, default_agent: 'none', needs_default_agent: true }),
    );
    await refresh();

    expect(get(autoGroomNeedsDefaultAgent)).toBe(true);
  });

  it('refetches Status on every coordinator event (uniform, race-free)', async () => {
    getAutoGroomStatus
      .mockResolvedValueOnce(snapshot({ last_skip_reasons: {} }))
      .mockResolvedValueOnce(snapshot({ default_agent: 'none', needs_default_agent: true }))
      .mockResolvedValueOnce(snapshot({ default_agent: 'claude', needs_default_agent: false }))
      .mockResolvedValueOnce(snapshot({ last_skip_reasons: { 'TB-1': 'settle' } }))
      .mockResolvedValueOnce(snapshot({ last_skip_reasons: { 'TB-1': 'dedupe' } }))
      .mockResolvedValueOnce(snapshot({ last_skip_reasons: {} }));
    await refresh();
    expect(getAutoGroomStatus).toHaveBeenCalledTimes(1);

    const handlers: Record<string, (e: { data: unknown[] }) => void> = {};
    const off = registerAutoGroomEventHandlers((name, handler) => {
      handlers[name] = handler;
      return () => delete handlers[name];
    });

    handlers['auto-groom:needs-default-agent']({ data: [] });
    await vi.waitFor(() => expect(getAutoGroomStatus).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(get(autoGroomStore).needsDefaultAgent).toBe(true));

    handlers['auto-groom:default-agent-cleared']({ data: [] });
    await vi.waitFor(() => expect(getAutoGroomStatus).toHaveBeenCalledTimes(3));
    await vi.waitFor(() => expect(get(autoGroomStore).needsDefaultAgent).toBe(false));

    handlers['auto-groom:queued']({ data: [] });
    await vi.waitFor(() => expect(getAutoGroomStatus).toHaveBeenCalledTimes(4));

    handlers['auto-groom:guarded-skip']({ data: [] });
    await vi.waitFor(() => expect(getAutoGroomStatus).toHaveBeenCalledTimes(5));

    handlers['auto-groom:promote-failed']({ data: [] });
    await vi.waitFor(() => expect(getAutoGroomStatus).toHaveBeenCalledTimes(6));

    off();
  });

  it('refetches Status on auto-groom:scan-complete (settle-only scans)', async () => {
    getAutoGroomStatus
      .mockResolvedValueOnce(snapshot({ last_skip_reasons: {} }))
      .mockResolvedValueOnce(
        snapshot({ last_skip_reasons: { 'TB-1': 'settle' }, settle_eligible_at_ms: { 'TB-1': 1 } }),
      );
    await refresh();

    const handlers: Record<string, (e: { data: unknown[] }) => void> = {};
    const off = registerAutoGroomEventHandlers((name, handler) => {
      handlers[name] = handler;
      return () => delete handlers[name];
    });

    handlers['auto-groom:scan-complete']({ data: [] });
    await vi.waitFor(() => expect(getAutoGroomStatus).toHaveBeenCalledTimes(2));
    await vi.waitFor(() =>
      expect(get(autoGroomStore).lastSkipReasons['TB-1']).toBe('settle'),
    );

    off();
  });

  it('settleSkipReasonFor + settleEligibleAtFor are reactive per-task selectors', async () => {
    getAutoGroomStatus.mockResolvedValue(
      snapshot({
        last_skip_reasons: { 'TB-1': 'settle', 'TB-2': 'dedupe' },
        settle_eligible_at_ms: { 'TB-1': 1_700_000_000_000 },
      }),
    );
    await refresh();

    expect(get(settleSkipReasonFor('TB-1'))).toBe('settle');
    expect(get(settleSkipReasonFor('TB-2'))).toBe('dedupe');
    expect(get(settleSkipReasonFor('TB-NOPE'))).toBeUndefined();

    expect(get(settleEligibleAtFor('TB-1'))).toBe(1_700_000_000_000);
    expect(get(settleEligibleAtFor('TB-2'))).toBeUndefined();
  });
});
