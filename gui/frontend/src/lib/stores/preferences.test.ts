import { get } from 'svelte/store';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getMaxWorkers = vi.fn<() => Promise<number>>();
const setMaxWorkers = vi.fn<(n: number) => Promise<void>>();
const getAgentTimeoutMinutes = vi.fn<() => Promise<number>>();
const setAgentTimeoutMinutes = vi.fn<(n: number) => Promise<void>>();
const getDefaultAgent = vi.fn<() => Promise<string>>();
const setDefaultAgent = vi.fn<(agent: string) => Promise<void>>();
const getCLIPath = vi.fn<() => Promise<string>>();
const setCLIPath = vi.fn<(path: string) => Promise<void>>();
const pushToast = vi.fn();

vi.mock('$lib/api', () => ({
  getMaxWorkers: () => getMaxWorkers(),
  setMaxWorkers: (n: number) => setMaxWorkers(n),
  getAgentTimeoutMinutes: () => getAgentTimeoutMinutes(),
  setAgentTimeoutMinutes: (n: number) => setAgentTimeoutMinutes(n),
  getDefaultAgent: () => getDefaultAgent(),
  setDefaultAgent: (agent: string) => setDefaultAgent(agent),
  getCLIPath: () => getCLIPath(),
  setCLIPath: (path: string) => setCLIPath(path),
}));

vi.mock('./toast', () => ({
  pushToast: (message: string, kind?: string) => pushToast(message, kind),
}));

const {
  _resetPreferencesStoreForTesting,
  loadPreferences,
  preferencesStore,
  setAgentTimeoutMinutes: storeSetAgentTimeoutMinutes,
  setCLIPath: storeSetCLIPath,
  setDefaultAgent: storeSetDefaultAgent,
  setMaxWorkers: storeSetMaxWorkers,
} = await import('./preferences');

describe('preferencesStore', () => {
  beforeEach(() => {
    _resetPreferencesStoreForTesting();
    vi.clearAllMocks();
    getMaxWorkers.mockResolvedValue(1);
    getAgentTimeoutMinutes.mockResolvedValue(30);
    getDefaultAgent.mockResolvedValue('none');
    getCLIPath.mockResolvedValue('');
    setMaxWorkers.mockResolvedValue();
    setAgentTimeoutMinutes.mockResolvedValue();
    setDefaultAgent.mockResolvedValue();
    setCLIPath.mockResolvedValue();
  });

  it('hydrates preferences and marks the store loaded', async () => {
    getMaxWorkers.mockResolvedValue(3);
    getAgentTimeoutMinutes.mockResolvedValue(45);
    getDefaultAgent.mockResolvedValue('codex');
    getCLIPath.mockResolvedValue('/usr/local/bin/tb');

    await loadPreferences();

    expect(get(preferencesStore)).toEqual({
      maxWorkers: 3,
      agentTimeoutMinutes: 45,
      defaultAgent: 'codex',
      cliPath: '/usr/local/bin/tb',
      loaded: true,
    });
  });

  it('loads only once after successful hydration', async () => {
    await loadPreferences();
    await loadPreferences();

    expect(getMaxWorkers).toHaveBeenCalledTimes(1);
    expect(getAgentTimeoutMinutes).toHaveBeenCalledTimes(1);
    expect(getDefaultAgent).toHaveBeenCalledTimes(1);
    expect(getCLIPath).toHaveBeenCalledTimes(1);
  });

  it('round-trips all set methods through the api', async () => {
    await loadPreferences();

    await storeSetMaxWorkers(4);
    await storeSetAgentTimeoutMinutes(60);
    await storeSetDefaultAgent('claude');
    await storeSetCLIPath('/opt/bin/tb');

    expect(setMaxWorkers).toHaveBeenCalledWith(4);
    expect(setAgentTimeoutMinutes).toHaveBeenCalledWith(60);
    expect(setDefaultAgent).toHaveBeenCalledWith('claude');
    expect(setCLIPath).toHaveBeenCalledWith('/opt/bin/tb');
    expect(get(preferencesStore)).toMatchObject({
      maxWorkers: 4,
      agentTimeoutMinutes: 60,
      defaultAgent: 'claude',
      cliPath: '/opt/bin/tb',
    });
  });

  it('rolls back optimistic writes on rejected promises', async () => {
    getMaxWorkers.mockResolvedValue(2);
    getAgentTimeoutMinutes.mockResolvedValue(20);
    getDefaultAgent.mockResolvedValue('codex');
    getCLIPath.mockResolvedValue('/old/tb');
    await loadPreferences();
    setCLIPath.mockRejectedValueOnce(new Error('permission denied'));

    await expect(storeSetCLIPath('/new/tb')).rejects.toThrow('permission denied');

    expect(get(preferencesStore)).toMatchObject({
      maxWorkers: 2,
      agentTimeoutMinutes: 20,
      defaultAgent: 'codex',
      cliPath: '/old/tb',
    });
    expect(pushToast).toHaveBeenCalledWith(
      'Could not save CLI path: permission denied',
      undefined,
    );
  });
});
