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
  // Type-creation hooks used by the auto-generated AutoGroomStatus /
  // BoardSnapshot models. Stub them as identity helpers so imports
  // succeed without dragging the real runtime in.
  Create: {
    Any: (value: unknown) => value,
    Array: (createItem: (value: unknown) => unknown) => (values: unknown[] = []) =>
      values.map(createItem),
    Map: (_createKey: (value: unknown) => unknown, createValue: (value: unknown) => unknown) =>
      (value: Record<string, unknown> = {}) =>
        Object.fromEntries(Object.entries(value).map(([key, item]) => [key, createValue(item)])),
  },
  Call: { ByID: vi.fn() },
  CancellablePromise: Promise,
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
  suggestBoardDialogDirectory,
  addAttachments,
  removeAttachments,
  openAttachment,
  listAttachments,
  pickAttachmentFiles,
  renameTask,
  errorString,
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

  it('starts in the supplied directory instead of reusing the native picker location', async () => {
    mocks.runtimeOpenFile.mockResolvedValueOnce('/Users/ralist/projects/books/writer-studio');

    await expect(pickBoardDialog('/Users/ralist/projects/books')).resolves.toBe(
      '/Users/ralist/projects/books/writer-studio',
    );

    expect(mocks.runtimeOpenFile).toHaveBeenCalledWith(
      expect.objectContaining({
        Directory: '/Users/ralist/projects/books',
      }),
    );
  });

  it('suggests the parent of a different recent project for board switching', () => {
    expect(
      suggestBoardDialogDirectory('/Users/ralist/projects/task-board-tools', [
        { projectRoot: '/Users/ralist/projects/task-board-tools' },
        { projectRoot: '/Users/ralist/projects/books/writer-studio' },
      ] as any),
    ).toBe('/Users/ralist/projects/books');
  });

  it('falls back to the active project parent when no different recent exists', () => {
    expect(
      suggestBoardDialogDirectory('/Users/ralist/projects/task-board-tools/', [
        { projectRoot: '/Users/ralist/projects/task-board-tools' },
      ] as any),
    ).toBe('/Users/ralist/projects');
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

describe('errorString', () => {
  it('keeps simple Error and string rejections readable', () => {
    expect(errorString(new Error('plain failure'))).toBe('plain failure');
    expect(errorString('string failure')).toBe('string failure');
  });

  it('extracts actionable CLI stderr from Wails runtime mutation payloads', () => {
    const payload = {
      name: 'RuntimeError',
      message: 'RuntimeError: error calling BoardService.ReadyTask',
      cause: {
        message: 'tb ready: validation: structured envelope should not win',
        cause: {
          Kind: 3,
          Op: 'ready',
          Args: ['ready', 'TB-285'],
          Stderr: [
            'TB-285 is not ready - needs grooming.',
            'Fix with:',
            '  tb edit TB-285 --priority P2',
            '  tb triage TB-285',
          ].join('\n'),
          Cause: { message: 'exit status 1' },
        },
      },
    };

    const message = errorString(payload);

    expect(message).toContain('TB-285 is not ready - needs grooming.');
    expect(message).toContain('tb edit TB-285');
    expect(message).toContain('tb triage TB-285');
    expect(message).not.toContain('RuntimeError');
    expect(message).not.toContain('Kind');
    expect(message).not.toContain('Op');
    expect(message).not.toContain('Args');
    expect(message).not.toContain('Cause');
    expect(message).not.toContain('[object Object]');
  });

  it('preserves CLI validation messages when an Error cause only carries exit status', () => {
    const err = new Error(
      [
        'tb ready: validation: TB-285 is not ready - needs grooming.',
        'Fix with:',
        '  tb triage TB-285',
      ].join('\n'),
      { cause: new Error('exit status 1') },
    );

    const message = errorString(err);

    expect(message).toContain('TB-285 is not ready - needs grooming.');
    expect(message).toContain('tb triage TB-285');
    expect(message).not.toBe('exit status 1');
  });

  it('keeps primitive-looking string rejections as plain strings', () => {
    expect(errorString('404')).toBe('404');
    expect(errorString('true')).toBe('true');
  });
});
