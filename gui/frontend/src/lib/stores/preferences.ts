import { derived, get, writable } from 'svelte/store';
import {
  getAgentTimeoutMinutes,
  getAutoGroomEnabled,
  getAutoGroomSettleMinutes,
  getAutoImplementEnabled,
  getAutoImplementQuery,
  getAutoReviewEnabled,
  getCLIPath,
  getDefaultAgent,
  getMaxWorkers,
  getMaxWorkersLimit,
  getPeriodicRecoveryEnabled,
  setAgentTimeoutMinutes as apiSetAgentTimeoutMinutes,
  setAutoGroomEnabled as apiSetAutoGroomEnabled,
  setAutoGroomSettleMinutes as apiSetAutoGroomSettleMinutes,
  setAutoImplementEnabled as apiSetAutoImplementEnabled,
  setAutoImplementQuery as apiSetAutoImplementQuery,
  setAutoReviewEnabled as apiSetAutoReviewEnabled,
  setCLIPath as apiSetCLIPath,
  setDefaultAgent as apiSetDefaultAgent,
  setMaxWorkers as apiSetMaxWorkers,
  setPeriodicRecoveryEnabled as apiSetPeriodicRecoveryEnabled,
} from '$lib/api';
import {
  emptyAutoImplementFilter,
  type AutoImplementFilter,
} from '$lib/autoImplementFilter';
import { pushToast } from './toast';

export type DefaultAgent = 'none' | 'claude' | 'codex';

export interface PreferencesState {
  maxWorkers: number;
  maxWorkersLimit: number;
  agentTimeoutMinutes: number;
  defaultAgent: DefaultAgent;
  cliPath: string;
  periodicRecoveryEnabled: boolean;
  autoGroomEnabled: boolean;
  autoGroomSettleMinutes: number;
  autoImplementEnabled: boolean;
  autoImplementQuery: AutoImplementFilter;
  autoReviewEnabled: boolean;
  loaded: boolean;
}

const DEFAULT_STATE: PreferencesState = {
  maxWorkers: 1,
  maxWorkersLimit: 1,
  agentTimeoutMinutes: 30,
  defaultAgent: 'none',
  cliPath: '',
  periodicRecoveryEnabled: true,
  autoGroomEnabled: false,
  autoGroomSettleMinutes: 5,
  autoImplementEnabled: false,
  autoImplementQuery: { ...emptyAutoImplementFilter },
  autoReviewEnabled: false,
  loaded: false,
};

const preferences = writable<PreferencesState>({ ...DEFAULT_STATE });
let loadPromise: Promise<void> | null = null;

export const defaultAgent = derived(preferences, ($preferences) => $preferences.defaultAgent);

export async function loadPreferences(): Promise<void> {
  if (get(preferences).loaded) return;
  if (loadPromise) return loadPromise;

  loadPromise = (async () => {
    try {
      const [
        maxWorkers,
        maxWorkersLimit,
        agentTimeoutMinutes,
        rawDefaultAgent,
        cliPath,
        periodicRecoveryEnabled,
        autoGroomEnabled,
        autoGroomSettleMinutes,
        autoImplementEnabled,
        autoImplementQuery,
        autoReviewEnabled,
      ] = await Promise.all([
        getMaxWorkers(),
        getMaxWorkersLimit(),
        getAgentTimeoutMinutes(),
        getDefaultAgent(),
        getCLIPath(),
        getPeriodicRecoveryEnabled(),
        getAutoGroomEnabled(),
        getAutoGroomSettleMinutes(),
        getAutoImplementEnabled(),
        getAutoImplementQuery(),
        getAutoReviewEnabled(),
      ]);

      const normalizedMaxWorkersLimit = normalizeStoredInt(
        maxWorkersLimit,
        1,
        Number.MAX_SAFE_INTEGER,
        DEFAULT_STATE.maxWorkersLimit,
      );
      preferences.set({
        maxWorkers: normalizeStoredInt(
          maxWorkers,
          1,
          normalizedMaxWorkersLimit,
          DEFAULT_STATE.maxWorkers,
        ),
        maxWorkersLimit: normalizedMaxWorkersLimit,
        agentTimeoutMinutes: normalizeStoredInt(
          agentTimeoutMinutes,
          1,
          240,
          DEFAULT_STATE.agentTimeoutMinutes,
        ),
        defaultAgent: normalizeDefaultAgent(rawDefaultAgent),
        cliPath: cliPath ?? '',
        periodicRecoveryEnabled: periodicRecoveryEnabled !== false,
        autoGroomEnabled: autoGroomEnabled === true,
        autoGroomSettleMinutes: normalizeStoredInt(
          autoGroomSettleMinutes,
          0,
          60,
          DEFAULT_STATE.autoGroomSettleMinutes,
        ),
        autoImplementEnabled: autoImplementEnabled === true,
        autoImplementQuery: normalizeAutoImplementQuery(autoImplementQuery),
        autoReviewEnabled: autoReviewEnabled === true,
        loaded: true,
      });
    } catch (err) {
      pushToast(`Could not load settings: ${stringifyError(err)}`);
      throw err;
    } finally {
      loadPromise = null;
    }
  })();

  return loadPromise;
}

export async function setMaxWorkers(value: number): Promise<void> {
  const limit = get(preferences).maxWorkersLimit;
  const next = clampSettingInt(value, 1, limit, DEFAULT_STATE.maxWorkers);
  await optimisticWrite('maxWorkers', next, 'max workers', () => apiSetMaxWorkers(next));
}

export async function setAgentTimeoutMinutes(value: number): Promise<void> {
  const next = clampSettingInt(value, 1, 240, DEFAULT_STATE.agentTimeoutMinutes);
  await optimisticWrite('agentTimeoutMinutes', next, 'agent timeout', () =>
    apiSetAgentTimeoutMinutes(next),
  );
}

export async function setDefaultAgent(value: string): Promise<void> {
  const next = normalizeDefaultAgent(value);
  await optimisticWrite('defaultAgent', next, 'default agent', () => apiSetDefaultAgent(next));
}

export async function setCLIPath(value: string): Promise<void> {
  const next = value.trim();
  await optimisticWrite('cliPath', next, 'CLI path', () => apiSetCLIPath(next));
}

export async function setPeriodicRecoveryEnabled(value: boolean): Promise<void> {
  await optimisticWrite('periodicRecoveryEnabled', value, 'periodic recovery', () =>
    apiSetPeriodicRecoveryEnabled(value),
  );
}

export async function setAutoGroomEnabled(value: boolean): Promise<void> {
  await optimisticWrite('autoGroomEnabled', value, 'auto-groom', () =>
    apiSetAutoGroomEnabled(value),
  );
}

export async function setAutoGroomSettleMinutes(value: number): Promise<void> {
  const next = clampSettingInt(value, 0, 60, DEFAULT_STATE.autoGroomSettleMinutes);
  await optimisticWrite('autoGroomSettleMinutes', next, 'auto-groom settle window', () =>
    apiSetAutoGroomSettleMinutes(next),
  );
}

export async function setAutoImplementEnabled(value: boolean): Promise<void> {
  await optimisticWrite('autoImplementEnabled', value, 'auto-implement', () =>
    apiSetAutoImplementEnabled(value),
  );
}

export async function setAutoImplementQuery(value: AutoImplementFilter): Promise<void> {
  const next = normalizeAutoImplementQuery(value);
  await optimisticWrite('autoImplementQuery', next, 'auto-implement query', () =>
    apiSetAutoImplementQuery(next),
  );
}

export async function setAutoReviewEnabled(value: boolean): Promise<void> {
  await optimisticWrite('autoReviewEnabled', value, 'auto-review', () =>
    apiSetAutoReviewEnabled(value),
  );
}

export const preferencesStore = {
  subscribe: preferences.subscribe,
  load: loadPreferences,
  setMaxWorkers,
  setAgentTimeoutMinutes,
  setDefaultAgent,
  setCLIPath,
  setPeriodicRecoveryEnabled,
  setAutoGroomEnabled,
  setAutoGroomSettleMinutes,
  setAutoImplementEnabled,
  setAutoImplementQuery,
  setAutoReviewEnabled,
};

export function _resetPreferencesStoreForTesting(): void {
  preferences.set({ ...DEFAULT_STATE });
  loadPromise = null;
}

async function optimisticWrite<K extends keyof PreferencesState>(
  key: K,
  value: PreferencesState[K],
  label: string,
  write: () => Promise<void>,
): Promise<void> {
  const previous = get(preferences);
  preferences.update((state) => ({ ...state, [key]: value }));
  try {
    await write();
  } catch (err) {
    preferences.set(previous);
    pushToast(`Could not save ${label}: ${stringifyError(err)}`);
    throw err;
  }
}

function normalizeDefaultAgent(value: unknown): DefaultAgent {
  if (value === 'claude' || value === 'codex') return value;
  return 'none';
}

function normalizeStoredInt(value: unknown, min: number, max: number, fallback: number): number {
  const n = Number(value);
  if (!Number.isFinite(n)) return fallback;
  if (n < min) return fallback;
  if (n > max) return max;
  return Math.trunc(n);
}

function clampSettingInt(value: unknown, min: number, max: number, fallback: number): number {
  const n = Number(value);
  if (!Number.isFinite(n)) return fallback;
  if (n < min) return min;
  if (n > max) return max;
  return Math.trunc(n);
}

function stringifyError(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (err == null) return 'unknown error';
  return String(err);
}

// normalizeAutoImplementQuery shields the store from anything that
// isn't a proper structured filter. The Go side migrates legacy text
// shapes silently (preferences.go logs the warning), but a partially
// constructed object reaching the bindings would still surface here —
// this helper just coerces to the empty filter and continues. Trims
// each value the same way the Go side does so round-trip semantics
// hold.
function normalizeAutoImplementQuery(value: unknown): AutoImplementFilter {
  if (!value || typeof value !== 'object') return { ...emptyAutoImplementFilter };
  const obj = value as Partial<Record<keyof AutoImplementFilter, unknown>>;
  return {
    search: typeof obj.search === 'string' ? obj.search.trim() : '',
    types: cleanStringArray(obj.types),
    priorities: cleanStringArray(obj.priorities),
    modules: cleanStringArray(obj.modules),
    sizes: cleanStringArray(obj.sizes),
    tags: cleanStringArray(obj.tags),
    agents: cleanStringArray(obj.agents),
    parents: cleanStringArray(obj.parents),
  };
}

function cleanStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  const out: string[] = [];
  for (const v of value) {
    if (typeof v !== 'string') continue;
    const t = v.trim();
    if (t !== '') out.push(t);
  }
  return out;
}
