import { describe, expect, it, vi } from 'vitest';
import {
  createBoardSwitchCoordinator,
  refreshPersistedBoardOnStartup,
  shouldAcceptBoardReload,
} from './boardSwitch';

describe('board switch coordinator', () => {
  it('does not commit UI state when OpenBoard rejects before backend commit', async () => {
    const openBoard = vi.fn().mockRejectedValueOnce(new Error('path has no .tb.yaml'));
    const onCommitted = vi.fn();
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    await expect(coordinator.open('/tmp/not-a-board')).rejects.toThrow(/no \.tb\.yaml/);

    expect(onCommitted).not.toHaveBeenCalled();
    expect(coordinator.isOpening()).toBe(false);
  });

  it('ignores an older OpenBoard completion after a newer switch starts', async () => {
    const first = deferred<void>();
    const second = deferred<void>();
    const openBoard = vi
      .fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    const firstOpen = coordinator.open('/tmp/old-board');
    const secondOpen = coordinator.open('/tmp/new-board');

    second.resolve();
    await secondOpen;
    first.resolve();
    await firstOpen;

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/new-board');
  });

  it('lets direct OpenBoard calls handle their own board:opened event without duplicate commit', async () => {
    const pending = deferred<void>();
    const openBoard = vi.fn().mockReturnValueOnce(pending.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const onActivated = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted, onActivated });

    const directOpen = coordinator.open('/tmp/target-board');
    expect(coordinator.isOpening()).toBe(true);
    await coordinator.handleOpenedEvent('/tmp/target-board', '');
    pending.resolve();
    await directOpen;

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/target-board');
    expect(onActivated).toHaveBeenCalledTimes(1);
    expect(onActivated).toHaveBeenCalledWith('/tmp/target-board');
  });

  it('reports activation from board:opened before committed refresh finishes', async () => {
    const pending = deferred<void>();
    const committed = deferred<void>();
    const calls: string[] = [];
    const openBoard = vi.fn().mockReturnValueOnce(pending.promise);
    const onCommitted = vi.fn(async () => {
      calls.push('commit:start');
      await committed.promise;
      calls.push('commit:done');
    });
    const onActivated = vi.fn(() => {
      calls.push('activate');
    });
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted, onActivated });

    const directOpen = coordinator.open('/tmp/target-board');
    const opened = coordinator.handleOpenedEvent('/tmp/target-board', '/tmp/target-board');
    await Promise.resolve();

    expect(onActivated).toHaveBeenCalledTimes(1);
    expect(calls).toContain('activate');
    expect(calls).not.toContain('commit:done');
    await Promise.resolve();
    expect(onCommitted).toHaveBeenCalledTimes(1);

    committed.resolve();
    await opened;
    pending.resolve();
    await directOpen;
  });

  it('does not report activation when OpenBoard resolves without board:opened', async () => {
    const openBoard = vi.fn().mockResolvedValueOnce(undefined);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const onActivated = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted, onActivated });

    await coordinator.open('/tmp/current-board');

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/current-board');
    expect(onActivated).not.toHaveBeenCalled();
  });

  it('reports activation when board:opened arrives after direct OpenBoard fallback commit', async () => {
    const openBoard = vi.fn().mockResolvedValueOnce(undefined);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const onActivated = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted, onActivated });

    await coordinator.open('/tmp/target-board');
    await expect(coordinator.handleOpenedEvent('/tmp/target-board', '/tmp/target-board')).resolves.toBe(true);

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onActivated).toHaveBeenCalledTimes(1);
    expect(onActivated).toHaveBeenCalledWith('/tmp/target-board');
  });

  it('ignores a stale board:opened event for a different target while a newer open is pending', async () => {
    const pending = deferred<void>();
    const openBoard = vi.fn().mockReturnValueOnce(pending.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    const directOpen = coordinator.open('/tmp/new-board');
    await expect(coordinator.handleOpenedEvent('/tmp/old-board', '/tmp/new-board')).resolves.toBe(false);
    pending.resolve();
    await directOpen;

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/new-board');
  });

  it('ignores a stale board:opened event for an older direct open after a newer open commits', async () => {
    const first = deferred<void>();
    const second = deferred<void>();
    const openBoard = vi
      .fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    const firstOpen = coordinator.open('/tmp/old-board');
    const secondOpen = coordinator.open('/tmp/new-board');

    second.resolve();
    await secondOpen;
    first.resolve();
    await firstOpen;
    await expect(coordinator.handleOpenedEvent('/tmp/old-board', '/tmp/new-board')).resolves.toBe(false);

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/new-board');
  });

  it('commits an older backend-committed open after a newer open rejects before commit', async () => {
    const first = deferred<void>();
    const second = deferred<void>();
    const openBoard = vi
      .fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const onActivated = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted, onActivated });

    const firstOpen = coordinator.open('/tmp/committed-board');
    const secondOpen = coordinator.open('/tmp/not-a-board');
    await expect(coordinator.handleOpenedEvent('/tmp/committed-board', '/tmp/committed-board')).resolves.toBe(false);
    expect(onCommitted).not.toHaveBeenCalled();

    second.reject(new Error('path has no .tb.yaml'));
    await expect(secondOpen).rejects.toThrow(/no \.tb\.yaml/);

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/committed-board');
    expect(onActivated).toHaveBeenCalledTimes(1);
    expect(onActivated).toHaveBeenCalledWith('/tmp/committed-board');

    first.resolve();
    await firstOpen;
    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onActivated).toHaveBeenCalledTimes(1);
  });

  it('accepts a delayed older direct-open event after a newer open rejects before commit', async () => {
    const first = deferred<void>();
    const second = deferred<void>();
    const openBoard = vi
      .fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    const firstOpen = coordinator.open('/tmp/committed-board');
    const secondOpen = coordinator.open('/tmp/not-a-board');
    second.reject(new Error('path has no .tb.yaml'));
    await expect(secondOpen).rejects.toThrow(/no \.tb\.yaml/);

    await expect(coordinator.handleOpenedEvent('/tmp/committed-board', '/tmp/committed-board')).resolves.toBe(true);
    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/committed-board');

    first.resolve();
    await firstOpen;
    expect(onCommitted).toHaveBeenCalledTimes(1);
  });

  it('commits an older direct-open completion after a newer open rejects before commit', async () => {
    const first = deferred<void>();
    const second = deferred<void>();
    const openBoard = vi
      .fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const onActivated = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted, onActivated });

    const firstOpen = coordinator.open('/tmp/committed-board');
    const secondOpen = coordinator.open('/tmp/not-a-board');
    second.reject(new Error('path has no .tb.yaml'));
    await expect(secondOpen).rejects.toThrow(/no \.tb\.yaml/);
    expect(onCommitted).not.toHaveBeenCalled();

    first.resolve();
    await firstOpen;

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/committed-board');
    expect(onActivated).not.toHaveBeenCalled();
  });

  it('commits an older direct-open completion when it resolves before a newer open rejects', async () => {
    const first = deferred<void>();
    const second = deferred<void>();
    const openBoard = vi
      .fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    const firstOpen = coordinator.open('/tmp/committed-board');
    const secondOpen = coordinator.open('/tmp/not-a-board');
    first.resolve();
    await firstOpen;
    expect(onCommitted).not.toHaveBeenCalled();

    second.reject(new Error('path has no .tb.yaml'));
    await expect(secondOpen).rejects.toThrow(/no \.tb\.yaml/);

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/committed-board');
  });

  it('does not report activation for stale board:opened events', async () => {
    const first = deferred<void>();
    const second = deferred<void>();
    const openBoard = vi
      .fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const onActivated = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted, onActivated });

    const firstOpen = coordinator.open('/tmp/old-board');
    const secondOpen = coordinator.open('/tmp/new-board');

    second.resolve();
    await secondOpen;
    first.resolve();
    await firstOpen;
    await expect(coordinator.handleOpenedEvent('/tmp/old-board', '/tmp/new-board')).resolves.toBe(false);

    expect(onActivated).not.toHaveBeenCalled();
  });

  it('ignores an older direct-open event even if the backend active root matches it later', async () => {
    const first = deferred<void>();
    const second = deferred<void>();
    const openBoard = vi
      .fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    const firstOpen = coordinator.open('/tmp/old-board');
    const secondOpen = coordinator.open('/tmp/new-board');

    second.resolve();
    await secondOpen;
    first.resolve();
    await firstOpen;
    await expect(coordinator.handleOpenedEvent('/tmp/old-board', '/tmp/old-board')).resolves.toBe(false);

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/new-board');
  });

  it('ignores backend-originated board:opened events when the active root cannot be verified', async () => {
    const openBoard = vi.fn();
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    await expect(coordinator.handleOpenedEvent('/tmp/menu-board', null)).resolves.toBe(false);

    expect(onCommitted).not.toHaveBeenCalled();
  });

  it('commits backend-originated board:opened events when no direct open is in flight', async () => {
    const openBoard = vi.fn();
    const onCommitted = vi.fn().mockResolvedValue(undefined);
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    await expect(coordinator.handleOpenedEvent('/tmp/menu-board', '')).resolves.toBe(true);

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/menu-board');
    expect(openBoard).not.toHaveBeenCalled();
  });
});

describe('board reload guard', () => {
  it('accepts reloads only for the visible active board', () => {
    expect(shouldAcceptBoardReload('/tmp/current-board', '/tmp/current-board')).toBe(true);
    expect(shouldAcceptBoardReload('/tmp/new-board', '/tmp/old-board')).toBe(false);
    expect(shouldAcceptBoardReload('/tmp/new-board', null)).toBe(false);
    expect(shouldAcceptBoardReload('', '/tmp/current-board')).toBe(false);
  });
});

describe('persisted board startup', () => {
  it('starts startup grace before waiting for the initial board refresh', async () => {
    const refresh = deferred<void>();
    const calls: string[] = [];

    const startup = refreshPersistedBoardOnStartup(
      '/tmp/current-board',
      (root) => calls.push(`grace:${root}`),
      async () => {
        calls.push('refresh:start');
        await refresh.promise;
        calls.push('refresh:done');
      },
    );
    await Promise.resolve();

    expect(calls).toEqual(['grace:/tmp/current-board', 'refresh:start']);

    refresh.resolve();
    await startup;

    expect(calls).toEqual(['grace:/tmp/current-board', 'refresh:start', 'refresh:done']);
  });
});

function deferred<T>(): {
  promise: Promise<T>;
  resolve: (value: T) => void;
  reject: (reason?: unknown) => void;
} {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  promise.catch(() => {});
  return { promise, resolve, reject };
}
