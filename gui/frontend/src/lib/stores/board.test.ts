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
});

function snapshot(id: string, title: string): BoardSnapshot {
  return {
    backlog: [task(id, title, 'backlog')],
    inProgress: [],
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

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => { resolve = r; });
  return { promise, resolve };
}
