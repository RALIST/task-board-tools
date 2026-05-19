import { derived, get, writable } from 'svelte/store';
import {
  getAgentTimeoutMinutes,
  getAutoGroomEnabled,
  getAutoGroomSettleMinutes,
  getAutoImplementEnabled,
  getAutoImplementQuery,
  getCLIPath,
  getDefaultAgent,
  getMaxWorkers,
  getPeriodicRecoveryEnabled,
  setAgentTimeoutMinutes as apiSetAgentTimeoutMinutes,
  setAutoGroomEnabled as apiSetAutoGroomEnabled,
  setAutoGroomSettleMinutes as apiSetAutoGroomSettleMinutes,
  setAutoImplementEnabled as apiSetAutoImplementEnabled,
  setAutoImplementQuery as apiSetAutoImplementQuery,
  setCLIPath as apiSetCLIPath,
  setDefaultAgent as apiSetDefaultAgent,
  setMaxWorkers as apiSetMaxWorkers,
  setPeriodicRecoveryEnabled as apiSetPeriodicRecoveryEnabled,
  validateAutoImplementQuery as apiValidateAutoImplementQuery,
} from '$lib/api';
import { pushToast } from './toast';

export type DefaultAgent = 'none' | 'claude' | 'codex';

export interface PreferencesState {
  maxWorkers: number;
  agentTimeoutMinutes: number;
  defaultAgent: DefaultAgent;
  cliPath: string;
  periodicRecoveryEnabled: boolean;
  autoGroomEnabled: boolean;
  autoGroomSettleMinutes: number;
  autoImplementEnabled: boolean;
  autoImplementQuery: string;
  loaded: boolean;
}

const DEFAULT_STATE: PreferencesState = {
  maxWorkers: 1,
  agentTimeoutMinutes: 30,
  defaultAgent: 'none',
  cliPath: '',
  periodicRecoveryEnabled: true,
  autoGroomEnabled: false,
  autoGroomSettleMinutes: 5,
  autoImplementEnabled: false,
  autoImplementQuery: '',
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
        agentTimeoutMinutes,
        rawDefaultAgent,
        cliPath,
        periodicRecoveryEnabled,
        autoGroomEnabled,
        autoGroomSettleMinutes,
        autoImplementEnabled,
        autoImplementQuery,
      ] = await Promise.all([
        getMaxWorkers(),
        getAgentTimeoutMinutes(),
        getDefaultAgent(),
        getCLIPath(),
        getPeriodicRecoveryEnabled(),
        getAutoGroomEnabled(),
        getAutoGroomSettleMinutes(),
        getAutoImplementEnabled(),
        getAutoImplementQuery(),
      ]);

      preferences.set({
        maxWorkers: normalizeStoredInt(maxWorkers, 1, 4, DEFAULT_STATE.maxWorkers),
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
        autoImplementQuery: typeof autoImplementQuery === 'string' ? autoImplementQuery : '',
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
  const next = clampSettingInt(value, 1, 4, DEFAULT_STATE.maxWorkers);
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

export async function setAutoImplementQuery(value: string): Promise<void> {
  const next = value.trim();
  await optimisticWrite('autoImplementQuery', next, 'auto-implement query', () =>
    apiSetAutoImplementQuery(next),
  );
}

// validateAutoImplementQuery proxies the backend non-mutating validator.
// Components use it to render inline parse errors without round-tripping
// a save. Returns a string error message on failure or null on success
// so callers can wire it into existing reactive validation patterns.
export async function validateAutoImplementQuery(expr: string): Promise<string | null> {
  try {
    await apiValidateAutoImplementQuery(expr);
    return null;
  } catch (err) {
    return stringifyError(err);
  }
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
  validateAutoImplementQuery,
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
