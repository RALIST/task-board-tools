import { derived, writable } from 'svelte/store';
import { type AutoReviewStatus, getAutoReviewStatus } from '$lib/api';

export interface AutoReviewState {
  enabled: boolean;
  defaultAgent: string;
  needsDefaultAgent: boolean;
  lastScanAt: string;
  lastSkipReasons: Record<string, string>;
  loaded: boolean;
}

const DEFAULT_STATE: AutoReviewState = {
  enabled: false,
  defaultAgent: 'none',
  needsDefaultAgent: false,
  lastScanAt: '',
  lastSkipReasons: {},
  loaded: false,
};

const state = writable<AutoReviewState>({ ...DEFAULT_STATE });
let refreshing = false;

export const autoReviewStore = {
  subscribe: state.subscribe,
  refresh,
};

export const autoReviewNeedsDefaultAgent = derived(
  state,
  ($state) => $state.enabled && ($state.needsDefaultAgent || $state.defaultAgent === 'none'),
);

export function autoReviewSkipReasonFor(id: string) {
  return derived(state, ($state) => $state.lastSkipReasons[id]);
}

export async function refresh(): Promise<void> {
  if (refreshing) return;
  refreshing = true;
  try {
    const snap = await getAutoReviewStatus();
    state.set(mapSnapshot(snap));
  } catch {
    // No board open / coordinator not ready. Keep quiet default state.
  } finally {
    refreshing = false;
  }
}

export function registerAutoReviewEventHandlers(
  on: (event: string, handler: (e: { data: unknown[] }) => void) => () => void,
): () => void {
  const onAny = () => {
    void refresh();
  };
  const offs = [
    on('auto-review:needs-default-agent', onAny),
    on('auto-review:default-agent-cleared', onAny),
    on('auto-review:queued', onAny),
    on('auto-review:skipped', onAny),
    on('auto-review:needs-user', onAny),
    on('auto-review:missing-review-target-prose', onAny),
    on('auto-review:worker-capacity-full', onAny),
    on('auto-review:run-failed', onAny),
    on('auto-review:resumed', onAny),
    on('auto-review:resume-failed', onAny),
    on('auto-review:scan-complete', onAny),
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

export function _resetAutoReviewStoreForTesting(): void {
  state.set({ ...DEFAULT_STATE });
  refreshing = false;
}

function mapSnapshot(snap: AutoReviewStatus): AutoReviewState {
  return {
    enabled: snap.enabled === true,
    defaultAgent: snap.default_agent ?? 'none',
    needsDefaultAgent: snap.needs_default_agent === true,
    lastScanAt: snap.last_scan_at ?? '',
    lastSkipReasons: copyStringMap(snap.last_skip_reasons),
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
