import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@wailsio/runtime', () => ({
  Events: { On: () => () => {} },
}));

const apiMocks = vi.hoisted(() => ({
  createTask: vi.fn(),
}));
vi.mock('$lib/api', () => apiMocks);

const toastMock = vi.hoisted(() => ({ pushToast: vi.fn() }));
vi.mock('$lib/stores/toast', () => toastMock);

import CreateTaskDialog from './CreateTaskDialog.svelte';
import Harness from './CreateTaskDialog.harness.test.svelte';
import ParentEsc from './CreateTaskDialog.parentEsc.test.svelte';

let component: ReturnType<typeof mount> | null = null;
let confirmSpy: ReturnType<typeof vi.spyOn>;
let onClose: ReturnType<typeof vi.fn>;
let onCreated: ReturnType<typeof vi.fn>;

beforeEach(() => {
  document.body.innerHTML = '';
  apiMocks.createTask.mockReset();
  toastMock.pushToast.mockReset();
  onClose = vi.fn();
  onCreated = vi.fn();
  confirmSpy = vi.spyOn(window, 'confirm');
});

afterEach(() => {
  if (component) {
    try { unmount(component); } catch { /* ignore */ }
    component = null;
  }
  confirmSpy.mockRestore();
});

function mountDialog(extra: Record<string, unknown> = {}) {
  return mount(CreateTaskDialog, {
    target: document.body,
    props: {
      open: true,
      onClose: onClose as unknown as () => void,
      onCreated: onCreated as unknown as (id: string) => void,
      ...extra,
    },
  });
}

function setInput(name: string, value: string) {
  const el = document.querySelector<HTMLInputElement | HTMLTextAreaElement>(`input[placeholder="${name}"], textarea[placeholder="${name}"]`);
  if (!el) throw new Error(`no input matching ${name}`);
  el.value = value;
  el.dispatchEvent(new Event('input', { bubbles: true }));
}

function clickCancel() {
  const btn = Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === 'Cancel');
  if (!btn) throw new Error('cancel button not found');
  (btn as HTMLButtonElement).click();
}

function clickHeaderClose() {
  const btn = document.querySelector<HTMLButtonElement>('button.close');
  if (!btn) throw new Error('header close not found');
  btn.click();
}

function clickBackdrop() {
  const backdrop = document.querySelector<HTMLDivElement>('.backdrop');
  if (!backdrop) throw new Error('backdrop not found');
  // Svelte sets currentTarget via delegation; a simple click on the backdrop
  // satisfies onBackdropClick's target===currentTarget guard.
  backdrop.click();
}

describe('CreateTaskDialog dirty-close guard', () => {
  it('closes an empty form on Cancel without prompting', async () => {
    component = mountDialog();
    await tick();
    clickCancel();
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('closes an empty form on the header close button without prompting', async () => {
    component = mountDialog();
    await tick();
    clickHeaderClose();
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('closes an empty form on a backdrop click without prompting', async () => {
    component = mountDialog();
    await tick();
    clickBackdrop();
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('prompts on Cancel when the title field is dirty', async () => {
    component = mountDialog();
    await tick();
    setInput('What needs doing?', 'New thing');
    await tick();
    confirmSpy.mockReturnValue(true);
    clickCancel();
    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('keeps the dialog open and preserves values when discard is rejected', async () => {
    component = mountDialog();
    await tick();
    setInput('What needs doing?', 'Keep me');
    setInput('comma,separated', 'tag1,tag2');
    await tick();
    confirmSpy.mockReturnValue(false);
    clickHeaderClose();
    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
    const titleEl = document.querySelector<HTMLInputElement>('input[placeholder="What needs doing?"]');
    expect(titleEl?.value).toBe('Keep me');
    const tagsEl = document.querySelector<HTMLInputElement>('input[placeholder="comma,separated"]');
    expect(tagsEl?.value).toBe('tag1,tag2');
  });

  it('also prompts when description (non-title field) is the only edit', async () => {
    component = mountDialog();
    await tick();
    setInput('One-sentence goal', 'a goal');
    await tick();
    confirmSpy.mockReturnValue(false);
    clickBackdrop();
    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('submitting a valid task closes without prompting and resets fields', async () => {
    apiMocks.createTask.mockResolvedValue({ id: 'TB-123' });
    component = mountDialog();
    await tick();
    setInput('What needs doing?', 'Real task');
    setInput('optional', 'core');
    await tick();
    const form = document.querySelector<HTMLFormElement>('form.dialog');
    form!.requestSubmit();
    await Promise.resolve();
    await Promise.resolve();
    await tick();
    expect(apiMocks.createTask).toHaveBeenCalledWith(expect.objectContaining({ title: 'Real task', module: 'core' }));
    expect(toastMock.pushToast).toHaveBeenCalledWith('Created TB-123', 'success');
    expect(onCreated).toHaveBeenCalledWith('TB-123');
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('updates bound dirty prop as fields change', async () => {
    // Vitest's vi.fn() type doesn't satisfy the strict `() => void` /
    // `(id: string) => void` props shape; cast for type-check, runtime
    // behavior is identical.
    component = mount(Harness, {
      target: document.body,
      props: {
        open: true,
        onClose: onClose as unknown as () => void,
        onCreated: onCreated as unknown as (id: string) => void,
      },
    });
    await tick();
    const harness = component as { getDirty: () => boolean };
    expect(harness.getDirty()).toBe(false);
    setInput('What needs doing?', 'Hello');
    await tick();
    expect(harness.getDirty()).toBe(true);
    setInput('What needs doing?', '');
    await tick();
    expect(harness.getDirty()).toBe(false);
  });
});

describe('CreateTaskDialog parent-global Escape flow (TB-181 review)', () => {
  // The dialog removed its own Escape handler so a single global handler
  // (in +page.svelte) owns the close-on-Escape path and reads `dirty`
  // back via `bind:dirty`. ParentEsc mimics that wiring: it renders the
  // dialog, binds dirty, and routes window keydown through a local
  // tryCloseCreate() that mirrors +page.svelte:close-create.

  function dispatchEscape() {
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  }

  it('Escape closes the empty dialog immediately, no confirm', async () => {
    component = mount(ParentEsc, {
      target: document.body,
      props: {
        onClose: onClose as unknown as () => void,
      },
    });
    await tick();
    dispatchEscape();
    await tick();
    const harness = component as { getOpen: () => boolean; getConfirmCalls: () => number };
    expect(harness.getConfirmCalls()).toBe(0);
    expect(harness.getOpen()).toBe(false);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('Escape prompts for discard when a field has been edited', async () => {
    component = mount(ParentEsc, {
      target: document.body,
      props: {
        onClose: onClose as unknown as () => void,
        confirmFn: () => true,
      },
    });
    await tick();
    setInput('What needs doing?', 'Pending work');
    await tick();
    dispatchEscape();
    await tick();
    const harness = component as { getOpen: () => boolean; getConfirmCalls: () => number };
    expect(harness.getConfirmCalls()).toBe(1);
    expect(harness.getOpen()).toBe(false);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('rejecting discard on Escape keeps the dialog open and values intact', async () => {
    component = mount(ParentEsc, {
      target: document.body,
      props: {
        onClose: onClose as unknown as () => void,
        confirmFn: () => false,
      },
    });
    await tick();
    setInput('What needs doing?', 'Keep this');
    await tick();
    dispatchEscape();
    await tick();
    const harness = component as { getOpen: () => boolean; getConfirmCalls: () => number };
    expect(harness.getConfirmCalls()).toBe(1);
    expect(harness.getOpen()).toBe(true);
    expect(onClose).not.toHaveBeenCalled();
    const titleEl = document.querySelector<HTMLInputElement>('input[placeholder="What needs doing?"]');
    expect(titleEl?.value).toBe('Keep this');
  });
});
