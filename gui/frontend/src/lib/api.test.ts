import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  runtimeOpenFile: vi.fn<() => Promise<string | string[]>>(),
  servicePickBoardDialog: vi.fn<() => Promise<string>>(),
}));

vi.mock('@wailsio/runtime', () => ({
  Dialogs: {
    OpenFile: mocks.runtimeOpenFile,
  },
}));

vi.mock('../../bindings/tools/tb-gui/app/boardservice', () => ({
  CloseTask: vi.fn(),
  CreateTask: vi.fn(),
  EditTask: vi.fn(),
  EditTaskBody: vi.fn(),
  GetTask: vi.fn(),
  LoadBoard: vi.fn(),
  LoadBoardWithMode: vi.fn(),
  MoveTask: vi.fn(),
  Regenerate: vi.fn(),
  Triage: vi.fn(),
}));

vi.mock('../../bindings/tools/tb-gui/app/agentservice', () => ({
  AssignAgent: vi.fn(),
  CancelRun: vi.fn(),
  GetRunLog: vi.fn(),
  GroomTask: vi.fn(),
  ListRuns: vi.fn(),
  RunAgent: vi.fn(),
}));

vi.mock('../../bindings/tools/tb-gui/app/settingsservice', () => ({
  GetBoardInfo: vi.fn(),
  GetProjectRoot: vi.fn(),
  ListRecentBoards: vi.fn(),
  OpenBoard: vi.fn(),
  PickBoardDialog: mocks.servicePickBoardDialog,
}));

const { pickBoardDialog } = await import('./api');

describe('pickBoardDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('uses a fresh runtime directory picker for each header selection', async () => {
    mocks.runtimeOpenFile
      .mockResolvedValueOnce('/Users/ralist/projects/books/writer-studio')
      .mockResolvedValueOnce('/Users/ralist/projects/task-board-tools');

    await expect(pickBoardDialog()).resolves.toBe('/Users/ralist/projects/books/writer-studio');
    await expect(pickBoardDialog()).resolves.toBe('/Users/ralist/projects/task-board-tools');

    expect(mocks.runtimeOpenFile).toHaveBeenCalledTimes(2);
    expect(mocks.runtimeOpenFile).toHaveBeenCalledWith({
      CanChooseDirectories: true,
      CanChooseFiles: false,
      CanCreateDirectories: false,
      AllowsMultipleSelection: false,
      Title: 'Open tb board',
      Message: 'Pick a directory that contains .tb.yaml',
      ButtonText: 'Open',
    });
    expect(mocks.servicePickBoardDialog).not.toHaveBeenCalled();
  });

  it('normalizes array results to the selected path', async () => {
    mocks.runtimeOpenFile.mockResolvedValueOnce(['/Users/ralist/projects/task-board-tools']);

    await expect(pickBoardDialog()).resolves.toBe('/Users/ralist/projects/task-board-tools');
  });
});
