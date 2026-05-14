import { describe, expect, it, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';

const getAgentUsage = vi.fn();
const refreshAgentUsage = vi.fn();

vi.mock('../../../bindings/tools/tb-gui/app/usageservice.js', () => ({
  GetAgentUsage: () => getAgentUsage(),
  RefreshAgentUsage: () => refreshAgentUsage(),
}));

// Import after the mock so the store reads the stubbed module.
import {
  usageStore,
  hydrate,
  refresh,
  registerUsageEventHandler,
  setUsage,
  currentUsage,
  _resetUsageStoreForTesting,
  type AgentUsage,
} from './usage';

function codexUsage(pct: number): AgentUsage {
  return {
    agent: 'codex',
    available: true,
    primary: { usedPercent: pct, windowLabel: '5h', resetsAt: '' },
    secondary: { usedPercent: pct + 1, windowLabel: 'weekly', resetsAt: '' },
    plan: 'prolite',
    source: 'codex-session-jsonl',
    lastUpdated: '2026-05-14T17:00:00Z',
  };
}

function claudeUnknown(): AgentUsage {
  return {
    agent: 'claude',
    available: false,
    reason: 'claude /usage data is not available without OAuth keychain access',
    source: 'claude-stub',
    lastUpdated: '2026-05-14T17:00:00Z',
  };
}

describe('usageStore', () => {
  beforeEach(() => {
    _resetUsageStoreForTesting();
    getAgentUsage.mockReset();
    refreshAgentUsage.mockReset();
  });

  it('hydrates from GetAgentUsage', async () => {
    getAgentUsage.mockResolvedValue([codexUsage(12.5), claudeUnknown()]);
    await hydrate();
    const snap = get(usageStore);
    expect(snap).toHaveLength(2);
    expect(snap[0].agent).toBe('codex');
    expect(snap[0].available).toBe(true);
    expect(snap[0].primary?.usedPercent).toBe(12.5);
    expect(snap[1].available).toBe(false);
  });

  it('falls back to empty list when the binding throws', async () => {
    getAgentUsage.mockRejectedValue(new Error('boom'));
    await hydrate();
    expect(get(usageStore)).toEqual([]);
  });

  it('refresh() replaces the cached snapshot and returns it', async () => {
    setUsage([codexUsage(10)]);
    refreshAgentUsage.mockResolvedValue([codexUsage(50)]);
    const next = await refresh();
    expect(next[0].primary?.usedPercent).toBe(50);
    expect(currentUsage()[0].primary?.usedPercent).toBe(50);
  });

  it('refresh() keeps the previous value when the binding throws', async () => {
    setUsage([codexUsage(10)]);
    refreshAgentUsage.mockRejectedValue(new Error('offline'));
    const next = await refresh();
    expect(next[0].primary?.usedPercent).toBe(10);
  });

  it('event handler updates the store from agent-usage:updated', () => {
    let captured: ((e: { data: unknown[] }) => void) | null = null;
    const off = registerUsageEventHandler((name, h) => {
      expect(name).toBe('agent-usage:updated');
      captured = h;
      return () => {};
    });
    expect(captured).not.toBeNull();
    captured!({ data: [[codexUsage(33)]] });
    expect(get(usageStore)[0].primary?.usedPercent).toBe(33);
    off();
  });

  it('normalizes non-finite percent to null', () => {
    setUsage([
      {
        agent: 'codex',
        available: true,
        primary: { usedPercent: Number.NaN, windowLabel: '5h' },
      } as AgentUsage,
    ]);
    expect(get(usageStore)[0].primary?.usedPercent).toBeNull();
  });
});
