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
const getPeriodicRecoveryEnabled = vi.fn<() => Promise<boolean>>();
const setPeriodicRecoveryEnabled = vi.fn<(enabled: boolean) => Promise<void>>();
const getAutoGroomEnabled = vi.fn<() => Promise<boolean>>();
const setAutoGroomEnabled = vi.fn<(enabled: boolean) => Promise<void>>();
const getAutoGroomSettleMinutes = vi.fn<() => Promise<number>>();
const setAutoGroomSettleMinutes = vi.fn<(n: number) => Promise<void>>();
const getAutoImplementEnabled = vi.fn<() => Promise<boolean>>();
const setAutoImplementEnabled = vi.fn<(enabled: boolean) => Promise<void>>();
const getAutoImplementQuery = vi.fn<() => Promise<string>>();
const setAutoImplementQuery = vi.fn<(expr: string) => Promise<void>>();
const validateAutoImplementQuery = vi.fn<(expr: string) => Promise<void>>();
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
  getPeriodicRecoveryEnabled: () => getPeriodicRecoveryEnabled(),
  setPeriodicRecoveryEnabled: (enabled: boolean) => setPeriodicRecoveryEnabled(enabled),
  getAutoGroomEnabled: () => getAutoGroomEnabled(),
  setAutoGroomEnabled: (enabled: boolean) => setAutoGroomEnabled(enabled),
  getAutoGroomSettleMinutes: () => getAutoGroomSettleMinutes(),
  setAutoGroomSettleMinutes: (n: number) => setAutoGroomSettleMinutes(n),
  getAutoImplementEnabled: () => getAutoImplementEnabled(),
  setAutoImplementEnabled: (enabled: boolean) => setAutoImplementEnabled(enabled),
  getAutoImplementQuery: () => getAutoImplementQuery(),
  setAutoImplementQuery: (expr: string) => setAutoImplementQuery(expr),
  validateAutoImplementQuery: (expr: string) => validateAutoImplementQuery(expr),
}));

vi.mock('./toast', () => ({
  pushToast: (message: string, kind?: string) => pushToast(message, kind),
}));

const {
  _resetPreferencesStoreForTesting,
  loadPreferences,
  preferencesStore,
  setAgentTimeoutMinutes: storeSetAgentTimeoutMinutes,
  setAutoGroomEnabled: storeSetAutoGroomEnabled,
  setAutoGroomSettleMinutes: storeSetAutoGroomSettleMinutes,
  setAutoImplementEnabled: storeSetAutoImplementEnabled,
  setAutoImplementQuery: storeSetAutoImplementQuery,
  setCLIPath: storeSetCLIPath,
  setDefaultAgent: storeSetDefaultAgent,
  setMaxWorkers: storeSetMaxWorkers,
  setPeriodicRecoveryEnabled: storeSetPeriodicRecoveryEnabled,
  validateAutoImplementQuery: storeValidateAutoImplementQuery,
} = await import('./preferences');

describe('preferencesStore', () => {
  beforeEach(() => {
    _resetPreferencesStoreForTesting();
    vi.clearAllMocks();
    getMaxWorkers.mockResolvedValue(1);
    getAgentTimeoutMinutes.mockResolvedValue(30);
    getDefaultAgent.mockResolvedValue('none');
    getCLIPath.mockResolvedValue('');
    getPeriodicRecoveryEnabled.mockResolvedValue(true);
    getAutoGroomEnabled.mockResolvedValue(false);
    getAutoGroomSettleMinutes.mockResolvedValue(5);
    getAutoImplementEnabled.mockResolvedValue(false);
    getAutoImplementQuery.mockResolvedValue('');
    setMaxWorkers.mockResolvedValue();
    setAgentTimeoutMinutes.mockResolvedValue();
    setDefaultAgent.mockResolvedValue();
    setCLIPath.mockResolvedValue();
    setPeriodicRecoveryEnabled.mockResolvedValue();
    setAutoGroomEnabled.mockResolvedValue();
    setAutoGroomSettleMinutes.mockResolvedValue();
    setAutoImplementEnabled.mockResolvedValue();
    setAutoImplementQuery.mockResolvedValue();
    validateAutoImplementQuery.mockResolvedValue();
  });

  it('hydrates preferences and marks the store loaded', async () => {
    getMaxWorkers.mockResolvedValue(3);
    getAgentTimeoutMinutes.mockResolvedValue(45);
    getDefaultAgent.mockResolvedValue('codex');
    getCLIPath.mockResolvedValue('/usr/local/bin/tb');
    getPeriodicRecoveryEnabled.mockResolvedValue(false);
    getAutoGroomEnabled.mockResolvedValue(true);
    getAutoGroomSettleMinutes.mockResolvedValue(10);
    getAutoImplementEnabled.mockResolvedValue(true);
    getAutoImplementQuery.mockResolvedValue('bug, S size, gui');

    await loadPreferences();

    expect(get(preferencesStore)).toEqual({
      maxWorkers: 3,
      agentTimeoutMinutes: 45,
      defaultAgent: 'codex',
      cliPath: '/usr/local/bin/tb',
      periodicRecoveryEnabled: false,
      autoGroomEnabled: true,
      autoGroomSettleMinutes: 10,
      autoImplementEnabled: true,
      autoImplementQuery: 'bug, S size, gui',
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
    expect(getPeriodicRecoveryEnabled).toHaveBeenCalledTimes(1);
    expect(getAutoGroomEnabled).toHaveBeenCalledTimes(1);
    expect(getAutoGroomSettleMinutes).toHaveBeenCalledTimes(1);
    expect(getAutoImplementEnabled).toHaveBeenCalledTimes(1);
    expect(getAutoImplementQuery).toHaveBeenCalledTimes(1);
  });

  it('round-trips all set methods through the api', async () => {
    await loadPreferences();

    await storeSetMaxWorkers(4);
    await storeSetAgentTimeoutMinutes(60);
    await storeSetDefaultAgent('claude');
    await storeSetCLIPath('/opt/bin/tb');
    await storeSetPeriodicRecoveryEnabled(false);
    await storeSetAutoGroomEnabled(true);
    await storeSetAutoGroomSettleMinutes(15);

    expect(setMaxWorkers).toHaveBeenCalledWith(4);
    expect(setAgentTimeoutMinutes).toHaveBeenCalledWith(60);
    expect(setDefaultAgent).toHaveBeenCalledWith('claude');
    expect(setCLIPath).toHaveBeenCalledWith('/opt/bin/tb');
    expect(setPeriodicRecoveryEnabled).toHaveBeenCalledWith(false);
    expect(setAutoGroomEnabled).toHaveBeenCalledWith(true);
    expect(setAutoGroomSettleMinutes).toHaveBeenCalledWith(15);
    expect(get(preferencesStore)).toMatchObject({
      maxWorkers: 4,
      agentTimeoutMinutes: 60,
      defaultAgent: 'claude',
      cliPath: '/opt/bin/tb',
      periodicRecoveryEnabled: false,
      autoGroomEnabled: true,
      autoGroomSettleMinutes: 15,
    });
  });

  it('honors explicit zero for the auto-groom settle window (opt-out)', async () => {
    await loadPreferences();

    await storeSetAutoGroomSettleMinutes(0);

    expect(setAutoGroomSettleMinutes).toHaveBeenCalledWith(0);
    expect(get(preferencesStore)).toMatchObject({ autoGroomSettleMinutes: 0 });
  });

  it('clamps out-of-range settle minutes on write', async () => {
    await loadPreferences();

    await storeSetAutoGroomSettleMinutes(999);
    expect(setAutoGroomSettleMinutes).toHaveBeenCalledWith(60);

    await storeSetAutoGroomSettleMinutes(-3);
    expect(setAutoGroomSettleMinutes).toHaveBeenLastCalledWith(0);
  });

  it('rolls back an auto-groom toggle on rejected promise', async () => {
    getAutoGroomEnabled.mockResolvedValue(false);
    await loadPreferences();
    setAutoGroomEnabled.mockRejectedValueOnce(new Error('disk full'));

    await expect(storeSetAutoGroomEnabled(true)).rejects.toThrow('disk full');

    expect(get(preferencesStore)).toMatchObject({ autoGroomEnabled: false });
    expect(pushToast).toHaveBeenCalledWith(
      'Could not save auto-groom: disk full',
      undefined,
    );
  });

  it('round-trips auto-implement settings through the api', async () => {
    await loadPreferences();

    await storeSetAutoImplementQuery('bug, S size, gui');
    await storeSetAutoImplementEnabled(true);

    expect(setAutoImplementQuery).toHaveBeenCalledWith('bug, S size, gui');
    expect(setAutoImplementEnabled).toHaveBeenCalledWith(true);
    expect(get(preferencesStore)).toMatchObject({
      autoImplementEnabled: true,
      autoImplementQuery: 'bug, S size, gui',
    });
  });

  it('trims whitespace before persisting the auto-implement query', async () => {
    await loadPreferences();

    await storeSetAutoImplementQuery('   bug   ');

    expect(setAutoImplementQuery).toHaveBeenCalledWith('bug');
    expect(get(preferencesStore)).toMatchObject({ autoImplementQuery: 'bug' });
  });

  it('rolls back an auto-implement enable when the backend rejects', async () => {
    await loadPreferences();
    setAutoImplementEnabled.mockRejectedValueOnce(new Error('default-agent required'));

    await expect(storeSetAutoImplementEnabled(true)).rejects.toThrow('default-agent required');

    expect(get(preferencesStore)).toMatchObject({ autoImplementEnabled: false });
    expect(pushToast).toHaveBeenCalledWith(
      'Could not save auto-implement: default-agent required',
      undefined,
    );
  });

  it('validateAutoImplementQuery proxies the api and surfaces error strings', async () => {
    validateAutoImplementQuery.mockRejectedValueOnce(new Error('invalid size'));
    const errMsg = await storeValidateAutoImplementQuery('size:huge');
    expect(errMsg).toBe('invalid size');

    validateAutoImplementQuery.mockResolvedValueOnce(undefined);
    const ok = await storeValidateAutoImplementQuery('bug');
    expect(ok).toBeNull();
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
