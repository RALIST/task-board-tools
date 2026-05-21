import { describe, expect, it, vi } from 'vitest';
import { createBoardSwitchCoordinator } from './boardSwitch';

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
    const coordinator = createBoardSwitchCoordinator({ openBoard, onCommitted });

    const directOpen = coordinator.open('/tmp/target-board');
    expect(coordinator.isOpening()).toBe(true);
    await coordinator.handleOpenedEvent('/tmp/target-board', '');
    pending.resolve();
    await directOpen;

    expect(onCommitted).toHaveBeenCalledTimes(1);
    expect(onCommitted).toHaveBeenCalledWith('/tmp/target-board');
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
