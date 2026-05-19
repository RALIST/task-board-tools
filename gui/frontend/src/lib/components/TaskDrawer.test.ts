import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Attachment, TaskDetail } from '$lib/api';

const runtimeEvents = vi.hoisted(() => ({
  handlers: new Map<string, Set<(ev?: unknown) => void>>(),
}));

vi.mock('@wailsio/runtime', () => ({
  Events: {
    On: (name: string, cb: (ev?: unknown) => void) => {
      let set = runtimeEvents.handlers.get(name);
      if (!set) {
        set = new Set();
        runtimeEvents.handlers.set(name, set);
      }
      set.add(cb);
      return () => set?.delete(cb);
    },
  },
}));

// $lib/api spies — all functions stubbed; tests selectively reset mock behavior.
const apiMocks = vi.hoisted(() => ({
  getTask: vi.fn(),
  listAttachments: vi.fn(),
  listRuns: vi.fn(),
  addAttachments: vi.fn(),
  removeAttachments: vi.fn(),
  openAttachment: vi.fn(),
  pickAttachmentFiles: vi.fn(),
  editTask: vi.fn(),
  editTaskBody: vi.fn(),
  assignAgent: vi.fn(),
  runAgent: vi.fn(),
  cancelRun: vi.fn(),
  groomTask: vi.fn(),
  closeTask: vi.fn(),
  renameTask: vi.fn(),
  errorString: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock('$lib/api', () => apiMocks);

vi.mock('$lib/stores/toast', () => ({
  pushToast: vi.fn(),
}));

vi.mock('$lib/stores/runs', () => ({
  runsForTask: () => ({ subscribe: (cb: (v: unknown[]) => void) => { cb([]); return () => {}; } }),
  selectedRunID: { subscribe: (cb: (v: unknown) => void) => { cb(null); return () => {}; }, set: () => {} },
  setRunsForTask: vi.fn(),
  upsertRun: vi.fn(),
}));

vi.mock('$lib/stores/groomSuggestion', () => ({
  consumeGroomSuggestion: () => false,
  groomSuggestedFor: { subscribe: () => () => {} },
}));

vi.mock('$lib/stores/preferences', () => ({
  defaultAgent: { subscribe: (cb: (v: string) => void) => { cb('none'); return () => {}; } },
}));

vi.mock('$lib/stores/triage', () => ({
  triageForTask: () => ({ subscribe: (cb: (v: string[]) => void) => { cb([]); return () => {}; } }),
}));

// Board store powers the Details rail's epic progress row; mocked here so
// tests can inject child task fixtures without touching the real loader.
const boardMocks = vi.hoisted(() => {
  let current: any = { backlog: [], inProgress: [], codeReview: [], done: [], archive: [] };
  const subs = new Set<(v: any) => void>();
  return {
    boardStore: {
      subscribe(cb: (v: any) => void) {
        cb(current);
        subs.add(cb);
        return () => subs.delete(cb);
      },
      set(next: any) {
        current = next;
        for (const cb of subs) cb(current);
      },
      reset() {
        current = { backlog: [], inProgress: [], codeReview: [], done: [], archive: [] };
        for (const cb of subs) cb(current);
      },
    },
  };
});
vi.mock('$lib/stores/board', () => ({ board: boardMocks.boardStore }));

// Heavy child components are not exercised here; stubbing keeps the test focused on
// drawer-level attachment behavior.
vi.mock('./BodyEditor.svelte', () => ({ default: () => ({}) }));
vi.mock('./AgentRunLog.svelte', () => ({ default: () => ({}) }));

import TaskDrawer from './TaskDrawer.svelte';
import type { BoardSnapshot } from '$lib/api';

const boardStore = boardMocks.boardStore as { set: (s: BoardSnapshot) => void; reset: () => void };

function makeTaskFixture(overrides: Record<string, unknown> = {}) {
  return {
    id: 'TB-X',
    title: 'X',
    type: 'task',
    priority: 'P2',
    size: 'M',
    module: 'core',
    tags: [],
    branch: '',
    parent: '',
    status: 'backlog',
    filePath: '',
    agent: '',
    agentStatus: '',
    ...overrides,
  };
}

function makeDetail(overrides: Partial<TaskDetail['metadata']> = {}): TaskDetail {
  return {
    metadata: {
      id: 'TB-99',
      title: 'Attachments demo',
      type: 'feature',
      priority: 'P1',
      size: 'M',
      module: 'gui',
      tags: [],
      branch: '',
      parent: '',
      status: 'backlog',
      filePath: 'board/backlog/TB-99/TASK.md',
      agent: '',
      agentStatus: '',
      ...overrides,
    },
    body: '# TB-99: Attachments demo\n\nbody',
  } as TaskDetail;
}

function flushMicrotasks() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

function emitRuntimeEvent(name: string, payload?: unknown) {
  for (const cb of runtimeEvents.handlers.get(name) ?? []) cb(payload);
}

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
  runtimeEvents.handlers.clear();
  vi.useRealTimers();
  apiMocks.getTask.mockReset();
  apiMocks.listAttachments.mockReset();
  apiMocks.listRuns.mockReset();
  apiMocks.addAttachments.mockReset();
  apiMocks.removeAttachments.mockReset();
  apiMocks.openAttachment.mockReset();
  apiMocks.pickAttachmentFiles.mockReset();
  apiMocks.renameTask.mockReset();
  apiMocks.editTask.mockReset();
  apiMocks.editTaskBody.mockReset();
  apiMocks.assignAgent.mockReset();
  apiMocks.runAgent.mockReset();
  apiMocks.cancelRun.mockReset();
  apiMocks.groomTask.mockReset();
  apiMocks.closeTask.mockReset();

  apiMocks.getTask.mockResolvedValue(makeDetail());
  apiMocks.listRuns.mockResolvedValue([]);
  boardStore.reset();
});

afterEach(async () => {
  if (component) await unmount(component);
  component = null;
  document.body.innerHTML = '';
});

describe('TaskDrawer attachments UI (TB-152)', () => {
  it('renders attachment rows with name and human-readable size', async () => {
    const attachments: Attachment[] = [
      { name: 'design.pdf', size: 2048 },
      { name: 'photo.png', size: 5 * 1024 * 1024 },
    ];
    apiMocks.listAttachments.mockResolvedValue(attachments);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const names = Array.from(document.querySelectorAll<HTMLButtonElement>('.att-name')).map((b) => b.textContent?.trim());
    expect(names).toEqual(['design.pdf', 'photo.png']);

    const sizes = Array.from(document.querySelectorAll<HTMLElement>('.att-size')).map((s) => s.textContent?.trim());
    expect(sizes[0]).toBe('2.0 KiB');
    expect(sizes[1]).toBe('5.0 MiB');
  });

  it('renders mixed task-root and legacy attachment refs without rewriting names', async () => {
    apiMocks.listAttachments.mockResolvedValue([
      { name: 'attachments/legacy.txt', size: 7 },
      { name: 'root.txt', size: 4 },
    ]);
    apiMocks.openAttachment.mockResolvedValue(undefined);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const names = Array.from(document.querySelectorAll<HTMLButtonElement>('.att-name'));
    expect(names.map((b) => b.textContent?.trim())).toEqual(['attachments/legacy.txt', 'root.txt']);

    names[0].click();
    names[1].click();
    await tick();

    expect(apiMocks.openAttachment).toHaveBeenNthCalledWith(1, 'TB-99', 'attachments/legacy.txt');
    expect(apiMocks.openAttachment).toHaveBeenNthCalledWith(2, 'TB-99', 'root.txt');
  });

  it('exposes data-file-drop-target and data-task-id on the surface', async () => {
    apiMocks.listAttachments.mockResolvedValue([]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();

    const surface = document.querySelector<HTMLElement>('.surface');
    expect(surface).not.toBeNull();
    expect(surface!.hasAttribute('data-file-drop-target')).toBe(true);
    expect(surface!.getAttribute('data-task-id')).toBe('TB-99');
  });

  it('row click invokes openAttachment with the task id and file name', async () => {
    apiMocks.listAttachments.mockResolvedValue([{ name: 'attachments/spec.txt', size: 12 }]);
    apiMocks.openAttachment.mockResolvedValue(undefined);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const btn = document.querySelector<HTMLButtonElement>('.att-name');
    expect(btn).not.toBeNull();
    btn!.click();
    await tick();

    expect(apiMocks.openAttachment).toHaveBeenCalledWith('TB-99', 'attachments/spec.txt');
  });

  it('Add files button invokes pickAttachmentFiles then addAttachments', async () => {
    apiMocks.listAttachments.mockResolvedValue([]);
    apiMocks.pickAttachmentFiles.mockResolvedValue(['/tmp/a.txt']);
    apiMocks.addAttachments.mockResolvedValue(undefined);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const addBtn = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find(
      (b) => b.textContent?.trim().startsWith('Add files'),
    );
    expect(addBtn).toBeDefined();
    addBtn!.click();
    await tick();
    await flushMicrotasks();

    expect(apiMocks.pickAttachmentFiles).toHaveBeenCalled();
    expect(apiMocks.addAttachments).toHaveBeenCalledWith('TB-99', ['/tmp/a.txt']);
  });

  it('waits for watcher event before refreshing after picker add', async () => {
    apiMocks.listAttachments
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([{ name: 'root.txt', size: 4 }]);
    apiMocks.pickAttachmentFiles.mockResolvedValue(['/tmp/root.txt']);
    apiMocks.addAttachments.mockResolvedValue(undefined);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();
    expect(apiMocks.listAttachments).toHaveBeenCalledTimes(1);

    const addBtn = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find(
      (b) => b.textContent?.trim().startsWith('Add files'),
    );
    addBtn!.click();
    await tick();
    await flushMicrotasks();
    await tick();

    expect(apiMocks.addAttachments).toHaveBeenCalledWith('TB-99', ['/tmp/root.txt']);
    expect(apiMocks.listAttachments).toHaveBeenCalledTimes(1);

    emitRuntimeEvent('board:reloaded');
    await flushMicrotasks();
    await tick();

    expect(apiMocks.listAttachments).toHaveBeenCalledTimes(2);
    expect(document.querySelector<HTMLButtonElement>('.att-name')?.textContent?.trim()).toBe('root.txt');
  });

  it('disables attachment controls while a file-drop attach is running for this task', async () => {
    apiMocks.listAttachments.mockResolvedValue([{ name: 'root.txt', size: 4 }]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const addBtn = Array.from(document.querySelectorAll<HTMLButtonElement>('button')).find(
      (b) => b.textContent?.trim().startsWith('Add files'),
    );
    const removeBtn = document.querySelector<HTMLButtonElement>('.att-remove');
    expect(addBtn?.disabled).toBe(false);
    expect(removeBtn?.disabled).toBe(false);

    emitRuntimeEvent('attach:dropping', { data: { taskId: 'TB-99' } });
    await tick();
    expect(addBtn?.textContent?.trim()).toBe('Working…');
    expect(addBtn?.disabled).toBe(true);
    expect(removeBtn?.disabled).toBe(true);

    emitRuntimeEvent('attach:dropped', { data: { taskId: 'TB-99', ok: true } });
    await tick();
    expect(addBtn?.textContent?.trim()).toBe('Add files…');
    expect(addBtn?.disabled).toBe(false);
    expect(removeBtn?.disabled).toBe(false);
  });
});

describe('TaskDrawer attachment remove confirmation (TB-153)', () => {
  it('first click arms the remove button without invoking removeAttachments', async () => {
    apiMocks.listAttachments.mockResolvedValue([{ name: 'note.txt', size: 10 }]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const remove = document.querySelector<HTMLButtonElement>('.att-remove');
    expect(remove).not.toBeNull();
    remove!.click();
    await tick();

    expect(apiMocks.removeAttachments).not.toHaveBeenCalled();
    expect(remove!.classList.contains('armed')).toBe(true);
    expect(remove!.getAttribute('aria-label')).toMatch(/Click again to remove/);
  });

  it('second click within the window commits the removal', async () => {
    apiMocks.listAttachments.mockResolvedValue([{ name: 'attachments/note.txt', size: 10 }]);
    apiMocks.removeAttachments.mockResolvedValue(undefined);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const remove = document.querySelector<HTMLButtonElement>('.att-remove');
    remove!.click();
    await tick();
    remove!.click();
    await tick();
    await flushMicrotasks();

    expect(apiMocks.removeAttachments).toHaveBeenCalledWith('TB-99', ['attachments/note.txt']);
  });

  it('waits for watcher event before refreshing after remove', async () => {
    apiMocks.listAttachments
      .mockResolvedValueOnce([{ name: 'root.txt', size: 10 }])
      .mockResolvedValueOnce([]);
    apiMocks.removeAttachments.mockResolvedValue(undefined);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();
    expect(apiMocks.listAttachments).toHaveBeenCalledTimes(1);

    const remove = document.querySelector<HTMLButtonElement>('.att-remove');
    remove!.click();
    await tick();
    remove!.click();
    await tick();
    await flushMicrotasks();
    await tick();

    expect(apiMocks.removeAttachments).toHaveBeenCalledWith('TB-99', ['root.txt']);
    expect(apiMocks.listAttachments).toHaveBeenCalledTimes(1);

    emitRuntimeEvent('board:reloaded');
    await flushMicrotasks();
    await tick();

    expect(apiMocks.listAttachments).toHaveBeenCalledTimes(2);
    expect(document.querySelector<HTMLButtonElement>('.att-name')).toBeNull();
  });
});

describe('TaskDrawer attachment remove confirm survives task switch (TB-153 regression)', () => {
  it('disarms remove on task switch so a same-named attachment on the new task is not single-click deletable', async () => {
    // Arm removal on TB-99/foo.txt
    apiMocks.getTask.mockResolvedValueOnce(makeDetail({ id: 'TB-99' }));
    apiMocks.listAttachments.mockResolvedValueOnce([{ name: 'foo.txt', size: 10 }]);
    component = mount(TaskDrawer, { target: document.body, props: { taskId: 'TB-99' } });
    await tick();
    await flushMicrotasks();
    await tick();

    let remove = document.querySelector<HTMLButtonElement>('.att-remove');
    remove!.click();
    await tick();
    expect(remove!.classList.contains('armed')).toBe(true);

    // Switch to TB-100; the next task also has an attachment named foo.txt.
    apiMocks.getTask.mockResolvedValueOnce(makeDetail({ id: 'TB-100' }));
    apiMocks.listAttachments.mockResolvedValueOnce([{ name: 'foo.txt', size: 20 }]);
    apiMocks.removeAttachments.mockResolvedValue(undefined);
    await unmount(component!);
    component = mount(TaskDrawer, { target: document.body, props: { taskId: 'TB-100' } });
    await tick();
    await flushMicrotasks();
    await tick();

    remove = document.querySelector<HTMLButtonElement>('.att-remove');
    expect(remove).not.toBeNull();
    expect(remove!.classList.contains('armed')).toBe(false);

    // A single click on TB-100's foo.txt must arm, not commit.
    remove!.click();
    await tick();
    expect(apiMocks.removeAttachments).not.toHaveBeenCalled();
  });
});

describe('TaskDrawer attachment accessibility (TB-154)', () => {
  it('attachment name button carries an open-in-default-app aria-label', async () => {
    apiMocks.listAttachments.mockResolvedValue([{ name: 'design.pdf', size: 1 }]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const name = document.querySelector<HTMLButtonElement>('.att-name');
    expect(name!.getAttribute('aria-label')).toBe('Open design.pdf in default application');
  });

  it('attachment list has an aria-label for screen readers', async () => {
    apiMocks.listAttachments.mockResolvedValue([{ name: 'design.pdf', size: 1 }]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const list = document.querySelector<HTMLElement>('.attachment-list');
    expect(list!.getAttribute('aria-label')).toBe('Attachments');
  });

  it('empty-state hint mentions both file picker and drag-and-drop', async () => {
    apiMocks.listAttachments.mockResolvedValue([]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const hint = Array.from(document.querySelectorAll<HTMLElement>('.attachments-section .hint')).find(
      (h) => h.textContent?.includes('No attachments'),
    );
    expect(hint).toBeDefined();
    expect(hint!.textContent).toMatch(/Add files/);
    expect(hint!.textContent).toMatch(/drag-and-drop files onto this drawer/);
  });
});

describe('TaskDrawer inline title rename (TB-207)', () => {
  beforeEach(() => {
    apiMocks.listAttachments.mockResolvedValue([]);
  });

  async function openDrawer(id = 'TB-99', title = 'Original Title') {
    apiMocks.getTask.mockResolvedValue(makeDetail({ id, title }));
    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: id },
    });
    await tick();
    await flushMicrotasks();
    await tick();
  }

  it('exposes a Rename button alongside the title for keyboard users', async () => {
    await openDrawer();
    const renameBtn = document.querySelector<HTMLButtonElement>('.rename-btn');
    expect(renameBtn).not.toBeNull();
    expect(renameBtn!.getAttribute('aria-label')).toBe('Rename task');
  });

  it('Rename button swaps in an input prefilled with the current title', async () => {
    await openDrawer('TB-99', 'Original Title');
    document.querySelector<HTMLButtonElement>('.rename-btn')!.click();
    await tick();
    await Promise.resolve();
    await tick();

    const input = document.querySelector<HTMLInputElement>('.title-input');
    expect(input).not.toBeNull();
    expect(input!.value).toBe('Original Title');
    // h2 is gone while editing.
    expect(document.querySelector('.surface-head h2')).toBeNull();
  });

  it('double-click on the title also enters rename mode', async () => {
    await openDrawer('TB-99', 'Original');
    const h2 = document.querySelector<HTMLElement>('.surface-head h2')!;
    h2.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
    await tick();
    expect(document.querySelector('.title-input')).not.toBeNull();
  });

  it('Save button invokes renameTask with trimmed value and closes editor on success', async () => {
    apiMocks.renameTask.mockResolvedValue(undefined);
    await openDrawer('TB-99', 'Original');
    document.querySelector<HTMLButtonElement>('.rename-btn')!.click();
    await tick();

    const input = document.querySelector<HTMLInputElement>('.title-input')!;
    input.value = '   New Title   ';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    await tick();

    const saveBtn = Array.from(document.querySelectorAll<HTMLButtonElement>('.title-edit-actions button'))
      .find((b) => b.textContent?.trim().startsWith('Save'));
    expect(saveBtn).toBeDefined();
    saveBtn!.click();
    await tick();
    await flushMicrotasks();
    await tick();

    expect(apiMocks.renameTask).toHaveBeenCalledWith('TB-99', 'New Title');
    // Editor closed.
    expect(document.querySelector('.title-input')).toBeNull();
  });

  it('Enter inside the input submits the rename', async () => {
    apiMocks.renameTask.mockResolvedValue(undefined);
    await openDrawer('TB-99', 'Original');
    document.querySelector<HTMLButtonElement>('.rename-btn')!.click();
    await tick();

    const input = document.querySelector<HTMLInputElement>('.title-input')!;
    input.value = 'Renamed via Enter';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    await flushMicrotasks();
    await tick();

    expect(apiMocks.renameTask).toHaveBeenCalledWith('TB-99', 'Renamed via Enter');
  });

  it('Cancel button discards the draft without calling renameTask', async () => {
    await openDrawer('TB-99', 'Stable');
    document.querySelector<HTMLButtonElement>('.rename-btn')!.click();
    await tick();
    const input = document.querySelector<HTMLInputElement>('.title-input')!;
    input.value = 'Drafted';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    const cancelBtn = Array.from(document.querySelectorAll<HTMLButtonElement>('.title-edit-actions button'))
      .find((b) => b.textContent?.trim() === 'Cancel');
    cancelBtn!.click();
    await tick();

    expect(apiMocks.renameTask).not.toHaveBeenCalled();
    expect(document.querySelector('.title-input')).toBeNull();
    expect(document.querySelector<HTMLElement>('.surface-head h2')?.textContent).toBe('Stable');
  });

  it('Escape inside the input cancels the draft without closing the drawer', async () => {
    await openDrawer('TB-99', 'Stable');
    document.querySelector<HTMLButtonElement>('.rename-btn')!.click();
    await tick();
    const input = document.querySelector<HTMLInputElement>('.title-input')!;
    input.value = 'Drafted';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
    await tick();

    expect(apiMocks.renameTask).not.toHaveBeenCalled();
    expect(document.querySelector('.title-input')).toBeNull();
    // Drawer surface is still mounted.
    expect(document.querySelector('.surface')).not.toBeNull();
  });

  it('empty title rejects with a toast and leaves the editor open', async () => {
    await openDrawer('TB-99', 'Stable');
    document.querySelector<HTMLButtonElement>('.rename-btn')!.click();
    await tick();
    const input = document.querySelector<HTMLInputElement>('.title-input')!;
    input.value = '   ';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();

    expect(apiMocks.renameTask).not.toHaveBeenCalled();
    expect(document.querySelector('.title-input')).not.toBeNull();
  });

  it('unchanged title is treated as a no-op without calling renameTask', async () => {
    await openDrawer('TB-99', 'Same');
    document.querySelector<HTMLButtonElement>('.rename-btn')!.click();
    await tick();
    const input = document.querySelector<HTMLInputElement>('.title-input')!;
    input.value = 'Same';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    await flushMicrotasks();
    await tick();

    expect(apiMocks.renameTask).not.toHaveBeenCalled();
    expect(document.querySelector('.title-input')).toBeNull();
  });

  it('rename failure keeps the draft open and surfaces the error', async () => {
    apiMocks.renameTask.mockRejectedValue(new Error('boom'));
    await openDrawer('TB-99', 'Old');
    document.querySelector<HTMLButtonElement>('.rename-btn')!.click();
    await tick();
    const input = document.querySelector<HTMLInputElement>('.title-input')!;
    input.value = 'Attempt';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await tick();
    await flushMicrotasks();
    await tick();
    await flushMicrotasks();
    await tick();

    expect(apiMocks.renameTask).toHaveBeenCalledTimes(1);
    const still = document.querySelector<HTMLInputElement>('.title-input');
    expect(still).not.toBeNull();
    expect(still!.value).toBe('Attempt');
  });
});

describe('TaskDrawer epic progress (TB-204)', () => {
  it('shows a Progress row for epic tasks with done/total derived from the board', async () => {
    apiMocks.getTask.mockResolvedValue(
      makeDetail({ id: 'TB-1', tags: ['epic'] }),
    );
    apiMocks.listAttachments.mockResolvedValue([]);
    boardStore.set({
      backlog: [makeTaskFixture({ id: 'TB-2', parent: 'TB-1', status: 'backlog' })],
      inProgress: [makeTaskFixture({ id: 'TB-3', parent: 'TB-1', status: 'in-progress' })],
      codeReview: [],
      done: [makeTaskFixture({ id: 'TB-4', parent: 'TB-1', status: 'done' })],
      archive: [],
    } as BoardSnapshot);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-1' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const labels = Array.from(document.querySelectorAll('.readonly-meta dt')).map((el) => el.textContent?.trim());
    expect(labels).toContain('Progress');

    const cell = document.querySelector('.epic-progress-cell .epic-progress-label');
    expect(cell?.textContent?.replace(/\s+/g, ' ').trim()).toBe('1/3 (33%)');
  });

  it('renders an empty-state Progress row for an epic with no children', async () => {
    apiMocks.getTask.mockResolvedValue(
      makeDetail({ id: 'TB-1', tags: ['epic'] }),
    );
    apiMocks.listAttachments.mockResolvedValue([]);
    boardStore.set({
      backlog: [], inProgress: [], codeReview: [], done: [], archive: [],
    } as BoardSnapshot);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-1' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const cell = document.querySelector<HTMLElement>('.epic-progress-cell');
    expect(cell).not.toBeNull();
    const label = cell!.querySelector('.epic-progress-label');
    expect(label?.textContent?.replace(/\s+/g, ' ').trim()).toBe('0/0 no children yet');
    expect(cell!.querySelector('.epic-progress-bar')?.classList.contains('empty')).toBe(true);
  });

  it('omits the Progress row for non-epic tasks', async () => {
    apiMocks.getTask.mockResolvedValue(makeDetail({ id: 'TB-99', tags: ['feature'] }));
    apiMocks.listAttachments.mockResolvedValue([]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const labels = Array.from(document.querySelectorAll('.readonly-meta dt')).map((el) => el.textContent?.trim());
    expect(labels).not.toContain('Progress');
    expect(document.querySelector('.epic-progress-cell')).toBeNull();
  });
});

describe('TaskDrawer user-attention UI (TB-182)', () => {
  it('renders the needs-user pill, attention panel, and disables Run/Groom', async () => {
    const body = [
      '# TB-99: Needs help',
      '',
      '## Goal',
      '',
      'Do the thing.',
      '',
      '## User Attention',
      '',
      'Reason: clarification needed.',
      '',
      'Question: should we keep legacy support?',
      '',
      '## Log',
      '',
      '- 2026-05-19: Created',
    ].join('\n');
    apiMocks.getTask.mockResolvedValue({
      ...makeDetail({ agent: 'claude', agentStatus: 'needs-user' }),
      body,
    });
    apiMocks.listAttachments.mockResolvedValue([]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    // Pill shows needs-user (the source of truth for this status is the
    // task's agentStatus, not the run history).
    const pill = document.querySelector('.pill.pill-needs-user');
    expect(pill).not.toBeNull();
    expect(pill?.textContent?.trim()).toBe('needs-user');

    // Panel is present with the rendered ## User Attention body.
    const panel = document.querySelector('.user-attention-panel');
    expect(panel).not.toBeNull();
    expect(panel?.textContent ?? '').toMatch(/clarification needed/);
    expect(panel?.textContent ?? '').toMatch(/legacy support/);

    // Run and Groom buttons are disabled with the needs-user tooltip.
    const buttons = Array.from(document.querySelectorAll<HTMLButtonElement>('.agent-buttons button'));
    const runBtn = buttons.find((b) => b.textContent?.trim().startsWith('Run'));
    const groomBtn = buttons.find((b) => b.textContent?.trim().startsWith('Groom'));
    expect(runBtn).toBeDefined();
    expect(groomBtn).toBeDefined();
    expect(runBtn?.disabled).toBe(true);
    expect(groomBtn?.disabled).toBe(true);
    expect(runBtn?.title ?? '').toMatch(/needs user input/i);
    expect(groomBtn?.title ?? '').toMatch(/needs user input/i);
  });

  it('shows fallback copy when needs-user has no ## User Attention section', async () => {
    apiMocks.getTask.mockResolvedValue({
      ...makeDetail({ agent: 'claude', agentStatus: 'needs-user' }),
      body: '# TB-99: T\n\n## Goal\n\nGoal text only.\n',
    });
    apiMocks.listAttachments.mockResolvedValue([]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const panel = document.querySelector('.user-attention-panel');
    expect(panel).not.toBeNull();
    expect(panel?.textContent ?? '').toMatch(/no.*User Attention.*section/i);
  });

  it('does not render the needs-user panel for other statuses', async () => {
    apiMocks.getTask.mockResolvedValue(makeDetail({ agentStatus: 'success' }));
    apiMocks.listAttachments.mockResolvedValue([]);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    expect(document.querySelector('.user-attention-panel')).toBeNull();
    expect(document.querySelector('.pill.pill-needs-user')).toBeNull();
  });

  it('Clear status button calls editTask with agentStatus=none', async () => {
    apiMocks.getTask.mockResolvedValue(makeDetail({ agent: 'claude', agentStatus: 'needs-user' }));
    apiMocks.listAttachments.mockResolvedValue([]);
    apiMocks.editTask.mockResolvedValue(undefined);

    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-99' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    const clearBtn = Array.from(document.querySelectorAll<HTMLButtonElement>('.user-attention-resolve button'))
      .find((b) => b.textContent?.trim().startsWith('Clear status'));
    expect(clearBtn).toBeDefined();
    clearBtn!.click();
    await tick();
    await flushMicrotasks();

    expect(apiMocks.editTask).toHaveBeenCalledWith('TB-99', { agentStatus: 'none' });
  });
});

describe('TaskDrawer metadata autosave (TB-190)', () => {
  // Matches AUTOSAVE_DEBOUNCE_MS in TaskDrawer.svelte. We can't import a
  // const from the .svelte file without churn, so this is kept in sync by
  // convention: any change there must be mirrored here.
  const DEBOUNCE_MS = 600;

  beforeEach(() => {
    apiMocks.listAttachments.mockResolvedValue([]);
  });

  // Open the drawer under real timers (mount has internal async work that
  // relies on setTimeout-based microtask flushing), then flip to fake
  // timers so the autosave debounce becomes deterministic. Without this
  // split, flushMicrotasks (which schedules via setTimeout) deadlocks.
  async function openDrawer(detail = makeDetail({ id: 'TB-77', module: 'core', tags: ['t1'] })) {
    apiMocks.getTask.mockResolvedValue(detail);
    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: detail.metadata.id },
    });
    await tick();
    await flushMicrotasks();
    await tick();
    return detail;
  }

  async function drainMicrotasks(n = 4) {
    for (let i = 0; i < n; i++) {
      await Promise.resolve();
    }
    await tick();
  }

  function changeFormSelect(label: string, next: string) {
    const fields = Array.from(document.querySelectorAll<HTMLElement>('.rail .field'));
    const node = fields.find((el) => el.querySelector('.field-label')?.textContent?.trim() === label);
    if (!node) throw new Error(`field "${label}" not found`);
    const select = node.querySelector<HTMLSelectElement>('select');
    if (!select) throw new Error(`select "${label}" not found`);
    select.value = next;
    select.dispatchEvent(new Event('change', { bubbles: true }));
    return select;
  }

  function changeFormInput(label: string, next: string) {
    const fields = Array.from(document.querySelectorAll<HTMLElement>('.rail .field'));
    const node = fields.find((el) => el.querySelector('.field-label')?.textContent?.trim() === label);
    if (!node) throw new Error(`field "${label}" not found`);
    const input = node.querySelector<HTMLInputElement>('input');
    if (!input) throw new Error(`input "${label}" not found`);
    input.value = next;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    return input;
  }

  function metaStatus(): string {
    // The Details section is the first rail-section with .details-section.
    const sec = document.querySelector<HTMLElement>('.details-section');
    return sec?.querySelector<HTMLElement>('.autosave-status')?.getAttribute('data-state') ?? '';
  }

  it('removes the Details Save button — no <button> labeled Save exists in the Details section', async () => {
    await openDrawer();
    const sec = document.querySelector<HTMLElement>('.details-section');
    expect(sec).not.toBeNull();
    const saveBtn = Array.from(sec!.querySelectorAll<HTMLButtonElement>('button')).find(
      (b) => b.textContent?.trim() === 'Save' || b.textContent?.trim() === 'Saved',
    );
    expect(saveBtn).toBeUndefined();
    expect(sec!.querySelector('.autosave-status')).not.toBeNull();
  });

  it('debounces metadata edits and saves a single coalesced payload', async () => {
    apiMocks.editTask.mockResolvedValue(undefined);
    await openDrawer(makeDetail({ id: 'TB-77', priority: 'P2', module: 'core' }));

    vi.useFakeTimers();
    try {
      changeFormSelect('Priority', 'P0');
      await tick();
      changeFormSelect('Priority', 'P1');
      await tick();
      // Multiple edits in a single debounce window must result in one CLI call.
      expect(apiMocks.editTask).not.toHaveBeenCalled();

      vi.advanceTimersByTime(DEBOUNCE_MS - 1);
      expect(apiMocks.editTask).not.toHaveBeenCalled();
      vi.advanceTimersByTime(2);
      await drainMicrotasks();
      expect(apiMocks.editTask).toHaveBeenCalledTimes(1);
      expect(apiMocks.editTask).toHaveBeenCalledWith('TB-77', { priority: 'P1' });
    } finally {
      vi.useRealTimers();
    }
  });

  it('preserves the user draft when board:reloaded fires mid-edit (form-reset guard)', async () => {
    apiMocks.editTask.mockResolvedValue(undefined);
    await openDrawer(makeDetail({ id: 'TB-77', module: 'core' }));

    changeFormInput('Module', 'edited-but-not-yet-saved');
    await tick();
    expect(metaStatus()).toBe('pending');

    // Watcher fires before debounce — fetchOnce must NOT clobber the draft.
    // The next getTask resolves with the same disk content (no remote
    // change), but the drawer should still preserve the in-progress draft.
    apiMocks.getTask.mockResolvedValue(makeDetail({ id: 'TB-77', module: 'core' }));
    emitRuntimeEvent('board:reloaded');
    await drainMicrotasks();

    const moduleInput = Array.from(document.querySelectorAll<HTMLElement>('.rail .field'))
      .find((el) => el.querySelector('.field-label')?.textContent?.trim() === 'Module')
      ?.querySelector<HTMLInputElement>('input');
    expect(moduleInput?.value).toBe('edited-but-not-yet-saved');
    // CLI call hasn't fired yet (still debouncing).
    expect(apiMocks.editTask).not.toHaveBeenCalled();
  });

  it('promotes the Saved indicator only after the watcher refresh catches up', async () => {
    apiMocks.editTask.mockResolvedValue(undefined);
    await openDrawer(makeDetail({ id: 'TB-77', priority: 'P2' }));

    vi.useFakeTimers();
    try {
      changeFormSelect('Priority', 'P0');
      await tick();
      vi.advanceTimersByTime(DEBOUNCE_MS + 5);
      await drainMicrotasks();
      // Between save success and watcher refresh: detail.metadata still
      // reads P2, form is P0 → metadataDirty true → status NOT 'saved' yet.
      expect(apiMocks.editTask).toHaveBeenCalledTimes(1);
      expect(metaStatus()).not.toBe('saved');

      // Simulate watcher catching up: next getTask returns the new value.
      apiMocks.getTask.mockResolvedValue(makeDetail({ id: 'TB-77', priority: 'P0' }));
      emitRuntimeEvent('board:reloaded');
      await drainMicrotasks();
      expect(metaStatus()).toBe('saved');
    } finally {
      vi.useRealTimers();
    }
  });

  it('shows error state and keeps draft intact when editTask rejects', async () => {
    apiMocks.editTask.mockRejectedValue(new Error('boom'));
    await openDrawer(makeDetail({ id: 'TB-77', priority: 'P2' }));

    vi.useFakeTimers();
    try {
      changeFormSelect('Priority', 'P0');
      await tick();
      vi.advanceTimersByTime(DEBOUNCE_MS + 5);
      await drainMicrotasks();

      expect(apiMocks.editTask).toHaveBeenCalled();
      expect(metaStatus()).toBe('error');
      const sec = document.querySelector<HTMLElement>('.details-section');
      const prioritySelect = Array.from(sec!.querySelectorAll<HTMLSelectElement>('select'))[0];
      expect(prioritySelect.value).toBe('P0');
      const retry = Array.from(sec!.querySelectorAll<HTMLButtonElement>('button')).find(
        (b) => b.textContent?.trim() === 'Retry',
      );
      expect(retry).toBeDefined();
    } finally {
      vi.useRealTimers();
    }
  });

  it('flushes a pending save when the drawer unmounts (close/task switch)', async () => {
    apiMocks.editTask.mockResolvedValue(undefined);
    await openDrawer(makeDetail({ id: 'TB-77', priority: 'P2' }));

    changeFormSelect('Priority', 'P1');
    await tick();
    // No timer advance — debounce is still pending; the teardown flush
    // should fire it synchronously.
    expect(apiMocks.editTask).not.toHaveBeenCalled();

    await unmount(component!);
    component = null;
    await Promise.resolve();
    await Promise.resolve();

    expect(apiMocks.editTask).toHaveBeenCalledTimes(1);
    expect(apiMocks.editTask).toHaveBeenCalledWith('TB-77', { priority: 'P1' });
  });

  it('flushes a pending save when the user clicks the × close button', async () => {
    apiMocks.editTask.mockResolvedValue(undefined);
    const onClose = vi.fn();
    apiMocks.getTask.mockResolvedValue(makeDetail({ id: 'TB-77', priority: 'P2' }));
    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-77', onClose },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    changeFormSelect('Priority', 'P1');
    await tick();
    expect(apiMocks.editTask).not.toHaveBeenCalled();

    // tryClose() runs the flush before invoking onClose.
    const closeBtn = document.querySelector<HTMLButtonElement>('.surface-head .close');
    expect(closeBtn).not.toBeNull();
    closeBtn!.click();
    await tick();
    await Promise.resolve();
    await Promise.resolve();

    expect(apiMocks.editTask).toHaveBeenCalledTimes(1);
    expect(apiMocks.editTask).toHaveBeenCalledWith('TB-77', { priority: 'P1' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('coalesces edits arriving during an in-flight save and resaves once with the latest payload', async () => {
    // Hold the first save in flight by deferring resolution.
    let resolveFirst!: () => void;
    apiMocks.editTask.mockImplementationOnce(
      () => new Promise<void>((res) => { resolveFirst = res; }),
    );
    apiMocks.editTask.mockResolvedValueOnce(undefined);
    await openDrawer(makeDetail({ id: 'TB-77', priority: 'P2', size: 'M' }));

    vi.useFakeTimers();
    try {
      changeFormSelect('Priority', 'P0');
      await tick();
      vi.advanceTimersByTime(DEBOUNCE_MS + 5);
      await drainMicrotasks();
      // First save kicked off, still in flight.
      expect(apiMocks.editTask).toHaveBeenCalledTimes(1);
      expect(apiMocks.editTask).toHaveBeenLastCalledWith('TB-77', { priority: 'P0' });

      // User keeps editing while the save is in flight.
      changeFormSelect('Size', 'L');
      await tick();
      vi.advanceTimersByTime(DEBOUNCE_MS + 5);
      await drainMicrotasks();
      // Debounce-triggered save sees metaSaving=true and queues a resave.
      expect(apiMocks.editTask).toHaveBeenCalledTimes(1);

      // First save resolves — finally hook re-fires with the latest
      // form state. The resave's payload is diffed against the still-
      // stale `detail.metadata` (no watcher refresh in this test), so
      // it re-includes priority on top of the new size edit. That's
      // correct: the CLI is idempotent under `.board.lock`, so resending
      // an already-saved field has no functional cost. The contract here
      // is "exactly one resave after coalesce", not minimum payload size.
      resolveFirst();
      await drainMicrotasks();
      await drainMicrotasks();
      expect(apiMocks.editTask).toHaveBeenCalledTimes(2);
      expect(apiMocks.editTask).toHaveBeenLastCalledWith(
        'TB-77',
        expect.objectContaining({ size: 'L' }),
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it('does not call editTask for an unsupported clear-field and snaps the draft back to disk', async () => {
    await openDrawer(makeDetail({ id: 'TB-77', module: 'core' }));

    vi.useFakeTimers();
    try {
      changeFormInput('Module', '');
      await tick();
      vi.advanceTimersByTime(DEBOUNCE_MS + 5);
      await drainMicrotasks();

      expect(apiMocks.editTask).not.toHaveBeenCalled();
      const moduleInput = Array.from(document.querySelectorAll<HTMLElement>('.rail .field'))
        .find((el) => el.querySelector('.field-label')?.textContent?.trim() === 'Module')
        ?.querySelector<HTMLInputElement>('input');
      // Per AC #6: drawer must NOT show a value that wasn't persisted.
      expect(moduleInput?.value).toBe('core');
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('TaskDrawer body autosave (TB-190)', () => {
  beforeEach(() => {
    apiMocks.listAttachments.mockResolvedValue([]);
  });

  // The BodyEditor.svelte default export is mocked above; the autosave
  // logic lives entirely in TaskDrawer so we don't need the editor for
  // these assertions — we just need to confirm the "Save body" button is
  // gone in edit mode (autosave replaces it).
  it('Edit mode renders no Save body button, only Discard and an autosave status', async () => {
    apiMocks.getTask.mockResolvedValue({
      ...makeDetail({ id: 'TB-77' }),
      body: '# TB-77: Title\n\n## Goal\n\nold goal\n',
    });
    component = mount(TaskDrawer, {
      target: document.body,
      props: { taskId: 'TB-77' },
    });
    await tick();
    await flushMicrotasks();
    await tick();

    // Click the Description "Edit" toggle to enter edit mode. enterEdit()
    // awaits getTask, so flush microtasks before checking the toolbar.
    const bodySec = document.querySelector<HTMLElement>('.body-section');
    expect(bodySec).not.toBeNull();
    const editBtn = Array.from(bodySec!.querySelectorAll<HTMLButtonElement>('button')).find(
      (b) => b.textContent?.trim() === 'Edit',
    );
    expect(editBtn).toBeDefined();
    editBtn!.click();
    await tick();
    await flushMicrotasks();
    await tick();

    const buttons = Array.from(bodySec!.querySelectorAll<HTMLButtonElement>('button')).map(
      (b) => b.textContent?.trim(),
    );
    expect(buttons).not.toContain('Save body');
    expect(buttons).toContain('Discard');
    expect(bodySec!.querySelector('.autosave-status')).not.toBeNull();
  });
});
