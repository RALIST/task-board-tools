import { derived, writable } from 'svelte/store';
import { type AutoGroomStatus, getAutoGroomStatus } from '$lib/api';

// AutoGroomState is the frontend-shaped projection of the coordinator's
// Status snapshot, plus event-driven flags that the coordinator
// doesn't include in Status (because they're edge-triggered).
export interface AutoGroomState {
  enabled: boolean;
  defaultAgent: string;
  needsDefaultAgent: boolean;
  settleMinutes: number;
  lastScanAt: string;
  lastSkipReasons: Record<string, string>;
  settleEligibleAtMs: Record<string, number>;
  loaded: boolean;
}

const DEFAULT_STATE: AutoGroomState = {
  enabled: false,
  defaultAgent: 'none',
  needsDefaultAgent: false,
  settleMinutes: 5,
  lastScanAt: '',
  lastSkipReasons: {},
  settleEligibleAtMs: {},
  loaded: false,
};

const state = writable<AutoGroomState>({ ...DEFAULT_STATE });
let refreshing = false;

export const autoGroomStore = {
  subscribe: state.subscribe,
  refresh,
};

// autoGroomNeedsDefaultAgent is the single derived flag both the
// SettingsPanel and the board-header toggle consume to render the
// actionable warning. Mirrors the coordinator's edge-triggered Wails
// events so the warning appears + clears reactively without polling.
export const autoGroomNeedsDefaultAgent = derived(
  state,
  ($state) => $state.enabled && ($state.needsDefaultAgent || $state.defaultAgent === 'none'),
);

// settleSkipReason returns the most-recent skip reason for a task, or
// undefined if no skip is recorded. Cards/drawers use this to render
// the "Auto-groom waiting" pill in the .groom-slot without polling.
export function settleSkipReasonFor(id: string) {
  return derived(state, ($state) => $state.lastSkipReasons[id]);
}

export function settleEligibleAtFor(id: string) {
  return derived(state, ($state) => $state.settleEligibleAtMs[id]);
}

export async function refresh(): Promise<void> {
  if (refreshing) return;
  refreshing = true;
  try {
    const snap = await getAutoGroomStatus();
    state.set(mapSnapshot(snap));
  } catch {
    // Coordinator may not be activated yet (no board open). Don't
    // toast — the UI just shows the default-disabled state.
  } finally {
    refreshing = false;
  }
}

export function registerAutoGroomEventHandlers(
  on: (event: string, handler: (e: { data: unknown[] }) => void) => () => void,
): () => void {
  // Every coordinator event triggers a Status() refetch so the
  // frontend stays consistent with the backend's source of truth.
  // Earlier versions flipped flags in-place for the two no-default
  // events to avoid a network round-trip, but that raced an in-flight
  // refresh() and produced a visible flicker. Refetching uniformly
  // is one extra Wails call per event and removes the race entirely.
  const onAny = () => {
    void refresh();
  };
  const offs = [
    on('auto-groom:needs-default-agent', onAny),
    on('auto-groom:default-agent-cleared', onAny),
    on('auto-groom:queued', onAny),
    on('auto-groom:guarded-skip', onAny),
    on('auto-groom:promote-failed', onAny),
    // Fires at the end of every scan, even one that produced only
    // settle-window skips (which emit no other events). Without this
    // the frontend Card pill + drawer countdown would never appear
    // for the dominant new-task-inside-settle-window flow.
    on('auto-groom:scan-complete', onAny),
  ];
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

export function _resetAutoGroomStoreForTesting(): void {
  state.set({ ...DEFAULT_STATE });
  refreshing = false;
}

function mapSnapshot(snap: AutoGroomStatus): AutoGroomState {
  return {
    enabled: snap.enabled === true,
    defaultAgent: snap.default_agent ?? 'none',
    needsDefaultAgent: snap.needs_default_agent === true,
    settleMinutes: typeof snap.settle_minutes === 'number' ? snap.settle_minutes : 5,
    lastScanAt: snap.last_scan_at ?? '',
    lastSkipReasons: copyStringMap(snap.last_skip_reasons),
    settleEligibleAtMs: copyNumberMap(snap.settle_eligible_at_ms),
    loaded: true,
  };
}

function copyStringMap(raw: { [k in string]?: string } | undefined): Record<string, string> {
  const out: Record<string, string> = {};
  if (!raw) return out;
  for (const [k, v] of Object.entries(raw)) {
    if (typeof v === 'string') out[k] = v;
  }
  return out;
}

function copyNumberMap(raw: { [k in string]?: number } | undefined): Record<string, number> {
  const out: Record<string, number> = {};
  if (!raw) return out;
  for (const [k, v] of Object.entries(raw)) {
    if (typeof v === 'number') out[k] = v;
  }
  return out;
}
