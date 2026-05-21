import { get } from 'svelte/store';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { BoardSnapshot, StatusMode, Task } from '../api';

const loadBoard = vi.fn<(mode?: StatusMode) => Promise<BoardSnapshot>>();
const getTask = vi.fn();

vi.mock('../api', () => ({
  loadBoard: (mode?: StatusMode) => loadBoard(mode),
  getTask: (id: string) => getTask(id),
}));

const {
  beginBoardSwitch,
  board,
  loaded,
  loadError,
  refresh,
  statusMode,
  switchingTo,
} = await import('./board');

describe('board store refresh ordering', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    board.set(snapshot('EMPTY', 'empty'));
    loaded.set(false);
    loadError.set(null);
    statusMode.set('active');
    switchingTo.set(null);
  });

  it('clears stale cards and exposes the target board while a switch refresh loads', async () => {
    const taskBoardInitial = snapshot('TB-90', 'Board switching is not working');
    const writerStudio = deferred<BoardSnapshot>();

    loadBoard.mockResolvedValueOnce(taskBoardInitial);
    await refresh();
    expect(get(board).backlog[0]?.id).toBe('TB-90');
    expect(get(loaded)).toBe(true);

    loadBoard.mockImplementationOnce(() => writerStudio.promise);
    beginBoardSwitch('/Users/ralist/projects/books/writer-studio');
    const switchRefresh = refresh();

    expect(get(board).backlog).toHaveLength(0);
    expect(get(board).done).toHaveLength(0);
    expect(get(loaded)).toBe(false);
    expect(get(loadError)).toBeNull();
    expect(get(switchingTo)).toBe('/Users/ralist/projects/books/writer-studio');

    writerStudio.resolve(snapshot('WS-001', 'Proofreading chunker epic'));
    await switchRefresh;

    expect(get(board).backlog[0]?.id).toBe('WS-001');
    expect(get(loaded)).toBe(true);
    expect(get(loadError)).toBeNull();
    expect(get(switchingTo)).toBeNull();
  });

  it('keeps stale cards hidden and shows the error when a committed switch refresh fails', async () => {
    const taskBoardInitial = snapshot('TB-90', 'Board switching is not working');
    const failure = new Error('cannot load active board: duplicate task TB-1');

    loadBoard.mockResolvedValueOnce(taskBoardInitial);
    await refresh();
    expect(get(board).backlog[0]?.id).toBe('TB-90');

    loadBoard.mockRejectedValueOnce(failure);
    beginBoardSwitch('/tmp/committed-broken-board');
    await refresh();

    expect(get(board).backlog).toHaveLength(0);
    expect(get(board).inProgress).toHaveLength(0);
    expect(get(board).done).toHaveLength(0);
    expect(get(loaded)).toBe(false);
    expect(get(loadError)).toBe(failure.message);
    expect(get(switchingTo)).toBeNull();
  });

  it('ignores a stale refresh result after a newer board switch starts', async () => {
    const initial = snapshot('TB-90', 'Initial board');
    const stale = deferred<BoardSnapshot>();
    const newer = snapshot('WS-001', 'Newer board');

    loadBoard.mockResolvedValueOnce(initial);
    await refresh();
    expect(get(board).backlog[0]?.id).toBe('TB-90');

    loadBoard
      .mockImplementationOnce(() => stale.promise)
      .mockResolvedValueOnce(newer);

    const staleRefresh = refresh();
    beginBoardSwitch('/tmp/newer-board');
    const newRefresh = refresh();
    stale.resolve(snapshot('OLD-1', 'Older board'));
    await Promise.all([staleRefresh, newRefresh]);

    expect(get(board).backlog[0]?.id).toBe('WS-001');
    expect(get(loadError)).toBeNull();
    expect(get(switchingTo)).toBeNull();
  });

  it('runs one follow-up load for a burst of refresh requests', async () => {
    const writerStudio = deferred<BoardSnapshot>();
    const taskBoardInitial = snapshot('TB-90', 'Board switching is not working');
    const taskBoardSwitchBack = snapshot('TB-92', 'Limit showed tags in header to 10');
    const writerSnapshot = snapshot('WS-001', 'Proofreading chunker epic');

    loadBoard.mockResolvedValueOnce(taskBoardInitial);
    await refresh();
    expect(get(board).backlog[0]?.id).toBe('TB-90');

    loadBoard
      .mockImplementationOnce(() => writerStudio.promise)
      .mockResolvedValueOnce(taskBoardSwitchBack);

    const writerRefresh = refresh();
    const switchBackRefresh = refresh();
    expect(loadBoard).toHaveBeenCalledTimes(2);

    writerStudio.resolve(writerSnapshot);
    await Promise.all([writerRefresh, switchBackRefresh]);

    expect(get(board).backlog[0]?.id).toBe('TB-92');
    expect(loadBoard).toHaveBeenCalledTimes(3);
  });

  it('clears the previous board when a switch refresh fails (TB-145)', async () => {
    const taskBoardInitial = snapshot('TB-90', 'Board switching is not working');
    const writerSnapshot = snapshot('WS-001', 'Proofreading chunker epic');

    loadBoard.mockResolvedValueOnce(taskBoardInitial);
    await refresh();
    expect(get(board).backlog[0]?.id).toBe('TB-90');
    expect(get(loadError)).toBeNull();

    const duplicateErr = new Error(
      'cannot load active board: task WS-1486 appears in multiple status directories (backlog: /tmp/board/backlog/WS-1486.md; done: /tmp/board/done/WS-1486.md). Move or remove one duplicate task file, then reload.',
    );
    loadBoard.mockRejectedValueOnce(duplicateErr);
    await refresh();

    expect(get(board).backlog).toHaveLength(0);
    expect(get(board).inProgress).toHaveLength(0);
    expect(get(board).done).toHaveLength(0);
    expect(get(loadError)).toBe(duplicateErr.message);
    expect(get(loadError)).not.toContain('Binding call failed');

    loadBoard.mockResolvedValueOnce(writerSnapshot);
    await refresh();
    expect(get(board).backlog[0]?.id).toBe('WS-001');
    expect(get(loadError)).toBeNull();
  });

  it('does not clobber a newer success with a stale error', async () => {
    const initial = snapshot('TB-90', 'Initial board');
    const newer = snapshot('TB-92', 'Newer board');
    const staleErr = deferred<BoardSnapshot>();

    loadBoard.mockResolvedValueOnce(initial);
    await refresh();
    expect(get(board).backlog[0]?.id).toBe('TB-90');

    loadBoard
      .mockImplementationOnce(() => staleErr.promise)
      .mockResolvedValueOnce(newer);

    const staleRefresh = refresh();
    const newRefresh = refresh();
    expect(loadBoard).toHaveBeenCalledTimes(2);

    staleErr.reject(new Error('stale failure'));
    await Promise.all([staleRefresh, newRefresh]);
    expect(get(board).backlog[0]?.id).toBe('TB-92');
    expect(get(loadError)).toBeNull();
    expect(loadBoard).toHaveBeenCalledTimes(3);
  });
});

function snapshot(id: string, title: string): BoardSnapshot {
  return {
    backlog: [task(id, title, 'backlog')],
    inProgress: [],
    ready: [], codeReview: [],
    done: [],
    archive: [], wipLimits: {}, wipCounts: {}, wipEnforcement: 'warn',
  } as BoardSnapshot;
}

function task(id: string, title: string, status: Task['status']): Task {
  return {
    id,
    title,
    type: 'bug',
    priority: 'P0',
    size: 'S',
    module: '',
    tags: [],
    branch: '',
    reviewRef: '',
    parent: '',
    status,
    filePath: `board/backlog/${id}.md`,
    agent: '',
    agentStatus: '', agentResumable: false, groomedBy: '', groomStatus: '', implementedBy: '', implementStatus: '', reviewedBy: '', reviewStatus: '',
  };
}

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
