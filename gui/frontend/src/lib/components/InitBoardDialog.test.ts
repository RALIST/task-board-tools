import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@wailsio/runtime', () => ({
  Events: { On: () => () => {} },
}));

const apiMocks = vi.hoisted(() => ({
  initBoard: vi.fn(),
}));
// Stub the whole api surface used by the dialog: validation helpers and
// constants are reproduced verbatim from gui/frontend/src/lib/api.ts so the
// test doesn't pull the auto-generated Wails bindings (and their runtime
// helpers) into the module graph.
vi.mock('$lib/api', () => {
  const INIT_PREFIX_PATTERN = /^[A-Za-z][A-Za-z0-9]*$/;
  const INIT_PREFIX_MAX_LEN = 10;
  return {
    INIT_BOARD_PATH_DEFAULT: 'board',
    INIT_PREFIX_DEFAULT: 'PR',
    INIT_PREFIX_MAX_LEN,
    INIT_PREFIX_PATTERN,
    initBoard: apiMocks.initBoard,
    errorString(err: unknown): string {
      if (err == null) return '';
      if (err instanceof Error) return err.message;
      return String(err);
    },
    validateInitBoardPath(raw: string): string | null {
      const trimmed = raw.trim();
      if (trimmed === '') return null;
      if (/^[/\\]/.test(trimmed)) return 'Board path must be relative to the project root.';
      if (trimmed === '.' || trimmed === '..') {
        return 'Board path must point to a directory inside the project root.';
      }
      if (trimmed.split(/[/\\]/).some((part) => part === '..')) {
        return 'Board path may not escape the project root.';
      }
      return null;
    },
    validateInitPrefix(raw: string): string | null {
      const trimmed = raw.trim();
      if (trimmed === '') return null;
      if (trimmed.length > INIT_PREFIX_MAX_LEN) return 'Prefix is too long (max 10).';
      if (!INIT_PREFIX_PATTERN.test(trimmed)) {
        return 'Prefix must start with a letter and contain only letters or digits.';
      }
      return null;
    },
  };
});

import InitBoardDialog from './InitBoardDialog.svelte';

let component: ReturnType<typeof mount> | null = null;
let onCancel: ReturnType<typeof vi.fn>;
let onInitialized: ReturnType<typeof vi.fn>;

beforeEach(() => {
  document.body.innerHTML = '';
  apiMocks.initBoard.mockReset();
  onCancel = vi.fn();
  onInitialized = vi.fn();
});

afterEach(() => {
  if (component) {
    try { unmount(component); } catch { /* ignore */ }
    component = null;
  }
});

function mountDialog(extra: Record<string, unknown> = {}) {
  return mount(InitBoardDialog, {
    target: document.body,
    props: {
      open: true,
      projectRoot: '/tmp/new-project',
      onCancel: onCancel as unknown as () => void,
      onInitialized: onInitialized as unknown as () => void,
      ...extra,
    },
  });
}

function getInput(placeholder: string): HTMLInputElement {
  const el = document.querySelector<HTMLInputElement>(`input[placeholder="${placeholder}"]`);
  if (!el) throw new Error(`no input matching ${placeholder}`);
  return el;
}

function setInput(placeholder: string, value: string) {
  const el = getInput(placeholder);
  el.value = value;
  el.dispatchEvent(new Event('input', { bubbles: true }));
}

function clickButton(label: string): HTMLButtonElement {
  const btn = Array.from(document.querySelectorAll('button')).find(
    (b) => b.textContent?.trim() === label,
  );
  if (!btn) throw new Error(`button "${label}" not found`);
  (btn as HTMLButtonElement).click();
  return btn as HTMLButtonElement;
}

describe('InitBoardDialog', () => {
  it('renders the selected project root as read-only', async () => {
    component = mountDialog();
    await tick();
    const root = document.querySelector<HTMLInputElement>('input[readonly]');
    expect(root?.value).toBe('/tmp/new-project');
    expect(getInput('board').value).toBe('board');
    expect(getInput('PR').value).toBe('PR');
  });

  it('does not call initBoard when Cancel is pressed', async () => {
    component = mountDialog();
    await tick();
    clickButton('Cancel');
    expect(apiMocks.initBoard).not.toHaveBeenCalled();
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('does not call initBoard when the backdrop is clicked', async () => {
    component = mountDialog();
    await tick();
    const backdrop = document.querySelector<HTMLDivElement>('.backdrop');
    backdrop?.click();
    expect(apiMocks.initBoard).not.toHaveBeenCalled();
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('blocks submit when the prefix is invalid', async () => {
    component = mountDialog();
    await tick();
    setInput('PR', '1bad');
    await tick();
    const submit = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Initialize',
    ) as HTMLButtonElement | undefined;
    expect(submit?.disabled).toBe(true);
    const form = document.querySelector<HTMLFormElement>('form.dialog');
    form!.requestSubmit();
    await Promise.resolve();
    expect(apiMocks.initBoard).not.toHaveBeenCalled();
  });

  it('blocks submit when the board path escapes the project root', async () => {
    component = mountDialog();
    await tick();
    setInput('board', '../etc');
    await tick();
    const submit = Array.from(document.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Initialize',
    ) as HTMLButtonElement | undefined;
    expect(submit?.disabled).toBe(true);
    expect(apiMocks.initBoard).not.toHaveBeenCalled();
  });

  it('confirming defaults calls initBoard then onInitialized', async () => {
    apiMocks.initBoard.mockResolvedValue(undefined);
    component = mountDialog();
    await tick();
    const form = document.querySelector<HTMLFormElement>('form.dialog');
    form!.requestSubmit();
    await Promise.resolve();
    await Promise.resolve();
    await tick();
    expect(apiMocks.initBoard).toHaveBeenCalledWith('/tmp/new-project', 'board', 'PR');
    expect(onInitialized).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();
  });

  it('passes trimmed custom values to initBoard', async () => {
    apiMocks.initBoard.mockResolvedValue(undefined);
    component = mountDialog();
    await tick();
    setInput('board', '  tasks  ');
    setInput('PR', '  WS  ');
    await tick();
    const form = document.querySelector<HTMLFormElement>('form.dialog');
    form!.requestSubmit();
    await Promise.resolve();
    await Promise.resolve();
    await tick();
    expect(apiMocks.initBoard).toHaveBeenCalledWith('/tmp/new-project', 'tasks', 'WS');
  });

  it('surfaces backend errors inline and stays open', async () => {
    apiMocks.initBoard.mockRejectedValue(new Error('boom: tb init failed'));
    component = mountDialog();
    await tick();
    const form = document.querySelector<HTMLFormElement>('form.dialog');
    form!.requestSubmit();
    await Promise.resolve();
    await Promise.resolve();
    await tick();
    const alert = document.querySelector('[role="alert"]');
    expect(alert?.textContent).toContain('boom: tb init failed');
    expect(onInitialized).not.toHaveBeenCalled();
    expect(onCancel).not.toHaveBeenCalled();
  });
});
