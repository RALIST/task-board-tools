import { get } from 'svelte/store';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { BoardSnapshot, StatusMode, Task } from '../api';

const loadBoard = vi.fn<(mode?: StatusMode) => Promise<BoardSnapshot>>();
const getTask = vi.fn();

vi.mock('../api', () => ({
  loadBoard: (mode?: StatusMode) => loadBoard(mode),
  getTask: (id: string) => getTask(id),
}));

const { board, loaded, loadError, refresh, statusMode } = await import('./board');

describe('board store refresh ordering', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    board.set(snapshot('EMPTY', 'empty'));
    loaded.set(false);
    loadError.set(null);
    statusMode.set('active');
  });

  it('keeps the switch-back board when an older refresh finishes late', async () => {
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
    await switchBackRefresh;

    expect(get(board).backlog[0]?.id).toBe('TB-92');

    writerStudio.resolve(writerSnapshot);
    await writerRefresh;

    expect(get(board).backlog[0]?.id).toBe('TB-92');
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
    await newRefresh;
    expect(get(board).backlog[0]?.id).toBe('TB-92');

    staleErr.reject(new Error('stale failure'));
    await staleRefresh;
    expect(get(board).backlog[0]?.id).toBe('TB-92');
    expect(get(loadError)).toBeNull();
  });
});

function snapshot(id: string, title: string): BoardSnapshot {
  return {
    backlog: [task(id, title, 'backlog')],
    inProgress: [],
    codeReview: [],
    done: [],
    archive: [],
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
    parent: '',
    status,
    filePath: `board/backlog/${id}.md`,
    agent: '',
    agentStatus: '',
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
