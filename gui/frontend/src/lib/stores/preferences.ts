import { derived, get, writable } from 'svelte/store';
import {
  getAgentTimeoutMinutes,
  getCLIPath,
  getDefaultAgent,
  getMaxWorkers,
  setAgentTimeoutMinutes as apiSetAgentTimeoutMinutes,
  setCLIPath as apiSetCLIPath,
  setDefaultAgent as apiSetDefaultAgent,
  setMaxWorkers as apiSetMaxWorkers,
} from '$lib/api';
import { pushToast } from './toast';

export type DefaultAgent = 'none' | 'claude' | 'codex';

export interface PreferencesState {
  maxWorkers: number;
  agentTimeoutMinutes: number;
  defaultAgent: DefaultAgent;
  cliPath: string;
  loaded: boolean;
}

const DEFAULT_STATE: PreferencesState = {
  maxWorkers: 1,
  agentTimeoutMinutes: 30,
  defaultAgent: 'none',
  cliPath: '',
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
      const [maxWorkers, agentTimeoutMinutes, rawDefaultAgent, cliPath] = await Promise.all([
        getMaxWorkers(),
        getAgentTimeoutMinutes(),
        getDefaultAgent(),
        getCLIPath(),
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

export const preferencesStore = {
  subscribe: preferences.subscribe,
  load: loadPreferences,
  setMaxWorkers,
  setAgentTimeoutMinutes,
  setDefaultAgent,
  setCLIPath,
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
