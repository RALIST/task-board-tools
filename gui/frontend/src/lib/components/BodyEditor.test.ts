import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import BodyEditor from './BodyEditor.svelte';

const SAMPLE_BODY = [
  '# TB-1: Sample task',
  '',
  '**Type:** bug',
  '**Priority:** P2',
  '',
  '## Goal',
  '',
  'Do the thing.',
  '',
].join('\n');

const PROTECTED_PREFIX = SAMPLE_BODY.slice(0, SAMPLE_BODY.indexOf('## Goal'));
const EDITABLE_SLICE = SAMPLE_BODY.slice(PROTECTED_PREFIX.length);

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
});

afterEach(async () => {
  if (component) await unmount(component);
  component = null;
  document.body.innerHTML = '';
});

function mountEditor(value: string = SAMPLE_BODY) {
  const state = { value, dirty: false };
  component = mount(BodyEditor, {
    target: document.body,
    props: {
      value: state.value,
      originalBody: SAMPLE_BODY,
      onDirtyChange: (d: boolean) => { state.dirty = d; },
    },
  });
  return state;
}

describe('BodyEditor.svelte (TB-129)', () => {
  it('does not render the read-only header strip above the editor', async () => {
    mountEditor();
    await tick();

    expect(document.querySelector('.header-strip')).toBeNull();
    expect(
      document.querySelector('[aria-label="Read-only header"]'),
    ).toBeNull();
  });

  it('does not leak the protected prefix text into the CodeMirror buffer', async () => {
    mountEditor();
    await tick();

    const cm = document.querySelector('.cm-content');
    expect(cm).not.toBeNull();
    const text = cm!.textContent ?? '';
    // The title line must not appear inside the editor — it is owned by the
    // surrounding TaskDrawer header, never by the CodeMirror document.
    expect(text).not.toContain('TB-1: Sample task');
    expect(text).toContain('## Goal');
    expect(text).toContain('Do the thing.');
  });

  it('omits the "read-only" hint copy now that the strip is gone', async () => {
    mountEditor();
    await tick();

    const hint = document.querySelector('.hint');
    expect(hint?.textContent ?? '').not.toContain('read-only');
    expect(hint?.textContent ?? '').toContain('Cmd/Ctrl+S');
  });

  it('mounts CodeMirror with each editable line (and no metadata rows)', async () => {
    mountEditor();
    await tick();

    // CodeMirror renders each line as a separate `.cm-line`; textContent on
    // the parent strips the joining newlines, so check line-by-line instead.
    const lines = Array.from(document.querySelectorAll('.cm-line')).map(
      (n) => n.textContent ?? '',
    );
    const editableLines = EDITABLE_SLICE.split('\n');
    // Every non-empty editable line should appear as a CM line.
    for (const expected of editableLines.filter((l) => l.length > 0)) {
      expect(lines).toContain(expected);
    }
    // The protected prefix's metadata rows must not appear in any CM line.
    expect(lines.some((l) => l.startsWith('# TB-1'))).toBe(false);
    expect(lines.some((l) => l.startsWith('**Type:**'))).toBe(false);
  });
});
