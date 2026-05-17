import { beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  runtimeOpenFile: vi.fn<() => Promise<string | string[]>>(),
  servicePickBoardDialog: vi.fn<() => Promise<string>>(),
  bindAddAttachments: vi.fn(),
  bindRemoveAttachments: vi.fn(),
  bindOpenAttachment: vi.fn(),
  bindListAttachments: vi.fn(),
  bindEditTask: vi.fn(),
}));

vi.mock('@wailsio/runtime', () => ({
  Dialogs: {
    OpenFile: mocks.runtimeOpenFile,
  },
}));

vi.mock('../../bindings/tools/tb-gui/app/boardservice', () => ({
  AddAttachments: mocks.bindAddAttachments,
  CloseTask: vi.fn(),
  CreateTask: vi.fn(),
  EditTask: mocks.bindEditTask,
  EditTaskBody: vi.fn(),
  GetTask: vi.fn(),
  ListAttachments: mocks.bindListAttachments,
  LoadBoard: vi.fn(),
  LoadBoardWithMode: vi.fn(),
  MoveTask: vi.fn(),
  OpenAttachment: mocks.bindOpenAttachment,
  Regenerate: vi.fn(),
  RemoveAttachments: mocks.bindRemoveAttachments,
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

const {
  pickBoardDialog,
  addAttachments,
  removeAttachments,
  openAttachment,
  listAttachments,
  pickAttachmentFiles,
  renameTask,
} = await import('./api');

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

describe('attachment wrappers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('addAttachments forwards id and paths verbatim to the binding', async () => {
    mocks.bindAddAttachments.mockResolvedValueOnce(undefined);

    await addAttachments('TB-1', ['/abs/a.txt', '/abs/b.png']);

    expect(mocks.bindAddAttachments).toHaveBeenCalledTimes(1);
    expect(mocks.bindAddAttachments).toHaveBeenCalledWith('TB-1', ['/abs/a.txt', '/abs/b.png']);
  });

  it('removeAttachments forwards id and names verbatim to the binding', async () => {
    mocks.bindRemoveAttachments.mockResolvedValueOnce(undefined);

    await removeAttachments('TB-1', ['x.txt', 'attachments/legacy.txt']);

    expect(mocks.bindRemoveAttachments).toHaveBeenCalledTimes(1);
    expect(mocks.bindRemoveAttachments).toHaveBeenCalledWith('TB-1', ['x.txt', 'attachments/legacy.txt']);
  });

  it('openAttachment forwards id and name verbatim to the binding', async () => {
    mocks.bindOpenAttachment.mockResolvedValueOnce(undefined);

    await openAttachment('TB-1', 'attachments/evidence.txt');

    expect(mocks.bindOpenAttachment).toHaveBeenCalledTimes(1);
    expect(mocks.bindOpenAttachment).toHaveBeenCalledWith('TB-1', 'attachments/evidence.txt');
  });

  it('listAttachments returns the raw rows from the binding', async () => {
    mocks.bindListAttachments.mockResolvedValueOnce([
      { name: 'a.txt', size: 12 },
      { name: 'b.png', size: 2048 },
    ]);

    const rows = await listAttachments('TB-1');
    expect(rows).toEqual([
      { name: 'a.txt', size: 12 },
      { name: 'b.png', size: 2048 },
    ]);
  });

  it('listAttachments normalises null binding output to an empty array', async () => {
    mocks.bindListAttachments.mockResolvedValueOnce(null);

    const rows = await listAttachments('TB-1');
    expect(rows).toEqual([]);
  });

  it('propagates binding errors so the caller can surface a toast', async () => {
    const err = new Error('tb attach: validation: at least one attachment path is required');
    mocks.bindAddAttachments.mockRejectedValueOnce(err);

    await expect(addAttachments('TB-1', [])).rejects.toThrow(/at least one attachment path/);
  });

  it('removeAttachments propagates binding errors', async () => {
    const err = new Error('attachment "missing.txt" not found on TB-1');
    mocks.bindRemoveAttachments.mockRejectedValueOnce(err);

    await expect(removeAttachments('TB-1', ['missing.txt'])).rejects.toThrow(/not found on TB-1/);
  });

  it('openAttachment propagates binding errors (missing file, OS failure)', async () => {
    const err = new Error('attachment "spec.txt" not found on TB-1');
    mocks.bindOpenAttachment.mockRejectedValueOnce(err);

    await expect(openAttachment('TB-1', 'spec.txt')).rejects.toThrow(/not found on TB-1/);
  });

  it('pickAttachmentFiles requests a multi-select file picker and filters empty entries', async () => {
    mocks.runtimeOpenFile.mockResolvedValueOnce([
      '/Users/me/a.txt',
      '',
      '/Users/me/b.png',
    ]);

    const paths = await pickAttachmentFiles();
    expect(paths).toEqual(['/Users/me/a.txt', '/Users/me/b.png']);
    // Match only the behavior-bearing options; assert structurally so UX copy
    // tweaks (Title/Message/ButtonText) don't break the test.
    expect(mocks.runtimeOpenFile).toHaveBeenCalledWith(
      expect.objectContaining({
        CanChooseFiles: true,
        CanChooseDirectories: false,
        AllowsMultipleSelection: true,
      }),
    );
  });

  it('pickAttachmentFiles returns an empty array when the picker is cancelled', async () => {
    mocks.runtimeOpenFile.mockResolvedValueOnce('');

    const paths = await pickAttachmentFiles();
    expect(paths).toEqual([]);
  });

  it('pickAttachmentFiles wraps a single-string result into a one-element array', async () => {
    mocks.runtimeOpenFile.mockResolvedValueOnce('/single/file.txt');

    const paths = await pickAttachmentFiles();
    expect(paths).toEqual(['/single/file.txt']);
  });
});

describe('renameTask', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('forwards a trimmed title to EditTask', async () => {
    mocks.bindEditTask.mockResolvedValueOnce(undefined);
    await renameTask('TB-1', '   New Title   ');
    expect(mocks.bindEditTask).toHaveBeenCalledTimes(1);
    expect(mocks.bindEditTask).toHaveBeenCalledWith('TB-1', { title: 'New Title' });
  });

  it('rejects empty/whitespace-only titles without hitting the binding', async () => {
    await expect(renameTask('TB-1', '')).rejects.toThrow(/empty/i);
    await expect(renameTask('TB-1', '   ')).rejects.toThrow(/empty/i);
    expect(mocks.bindEditTask).not.toHaveBeenCalled();
  });

  it('propagates binding errors so callers can show a toast', async () => {
    mocks.bindEditTask.mockRejectedValueOnce(new Error('tb edit: validation: bad input'));
    await expect(renameTask('TB-1', 'Anything')).rejects.toThrow(/bad input/);
  });
});
