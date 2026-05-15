import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Attachment, TaskDetail } from '$lib/api';

vi.mock('@wailsio/runtime', () => ({
  Events: { On: () => () => {} },
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

// Heavy child components are not exercised here; stubbing keeps the test focused on
// drawer-level attachment behavior.
vi.mock('./BodyEditor.svelte', () => ({ default: () => ({}) }));
vi.mock('./AgentRunLog.svelte', () => ({ default: () => ({}) }));

import TaskDrawer from './TaskDrawer.svelte';

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

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
  vi.useRealTimers();
  apiMocks.getTask.mockReset();
  apiMocks.listAttachments.mockReset();
  apiMocks.listRuns.mockReset();
  apiMocks.addAttachments.mockReset();
  apiMocks.removeAttachments.mockReset();
  apiMocks.openAttachment.mockReset();
  apiMocks.pickAttachmentFiles.mockReset();

  apiMocks.getTask.mockResolvedValue(makeDetail());
  apiMocks.listRuns.mockResolvedValue([]);
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
    apiMocks.listAttachments.mockResolvedValue([{ name: 'spec.txt', size: 12 }]);
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

    expect(apiMocks.openAttachment).toHaveBeenCalledWith('TB-99', 'spec.txt');
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
    apiMocks.listAttachments.mockResolvedValue([{ name: 'note.txt', size: 10 }]);
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

    expect(apiMocks.removeAttachments).toHaveBeenCalledWith('TB-99', ['note.txt']);
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
