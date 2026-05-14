// Reactive store of per-agent global quota usage (TB-107).
//
// Lifecycle:
//   1. hydrate() — called once on app boot; pulls the current snapshot from
//      UsageService.GetAgentUsage().
//   2. registerUsageEventHandler() — wires the `agent-usage:updated` Wails
//      event so backend ticker / manual refresh updates re-render the header
//      without polling.
//   3. refresh() — invoked by the manual "refresh" button in the header.
//      Calls UsageService.RefreshAgentUsage() and replaces the cache.
//
// Shape is whatever the Go side ships in agent.Usage; we keep it loose here
// because parsing happens once at hydration / event time.

import { writable, get } from 'svelte/store';

import {
  GetAgentUsage,
  RefreshAgentUsage,
} from '../../../bindings/tools/tb-gui/app/usageservice.js';

export interface UsageWindow {
  usedPercent?: number | null;
  windowLabel?: string;
  resetsAt?: string;
}

export interface AgentUsage {
  agent: string;
  available: boolean;
  reason?: string;
  primary?: UsageWindow | null;
  secondary?: UsageWindow | null;
  plan?: string;
  source?: string;
  lastUpdated?: string;
}

const usage = writable<AgentUsage[]>([]);

/** Subscribe to the current per-agent snapshots, sorted by agent name. */
export const usageStore = { subscribe: usage.subscribe };

/** Returns the latest snapshots synchronously. Useful for unit tests. */
export function currentUsage(): AgentUsage[] {
  return get(usage);
}

/** Replace the store with the given snapshots. Exported for tests; production
 * code should call hydrate() / refresh() / the event handler instead. */
export function setUsage(list: AgentUsage[]): void {
  usage.set(list.map(normalize));
}

/** Pull the current per-agent quota snapshot from the backend cache. Called
 * once at app boot; subsequent updates flow through the event handler. */
export async function hydrate(): Promise<void> {
  try {
    const raw = await GetAgentUsage();
    setUsage(toArray(raw));
  } catch {
    /* leave the store empty; header renders nothing until the first real
       update arrives via the event handler. */
  }
}

/** Force a refresh on the backend. Resolves to the new snapshot. */
export async function refresh(): Promise<AgentUsage[]> {
  try {
    const raw = await RefreshAgentUsage();
    const next = toArray(raw);
    setUsage(next);
    return next;
  } catch {
    return currentUsage();
  }
}

/** Wire the `agent-usage:updated` Wails event. Call once at app start. */
export function registerUsageEventHandler(
  on: (event: string, handler: (e: { data: unknown[] }) => void) => () => void,
): () => void {
  return on('agent-usage:updated', (e) => {
    const payload = e?.data?.[0];
    setUsage(toArray(payload));
  });
}

/** Reset for tests. NOT for production use. */
export function _resetUsageStoreForTesting(): void {
  usage.set([]);
}

// --- internals ---

function toArray(raw: unknown): AgentUsage[] {
  if (Array.isArray(raw)) return raw as AgentUsage[];
  return [];
}

function normalize(u: AgentUsage): AgentUsage {
  return {
    agent: u.agent ?? '',
    available: u.available === true,
    reason: u.reason,
    plan: u.plan,
    source: u.source,
    lastUpdated: u.lastUpdated,
    primary: normalizeWindow(u.primary),
    secondary: normalizeWindow(u.secondary),
  };
}

function normalizeWindow(w: UsageWindow | null | undefined): UsageWindow | null {
  if (!w) return null;
  const pct = typeof w.usedPercent === 'number' && Number.isFinite(w.usedPercent) ? w.usedPercent : null;
  return {
    usedPercent: pct,
    windowLabel: w.windowLabel ?? '',
    resetsAt: w.resetsAt ?? '',
  };
}
