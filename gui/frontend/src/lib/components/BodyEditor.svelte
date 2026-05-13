<script lang="ts">
  // CodeMirror 6 wrapper for the task body.
  //
  // The editor buffer contains ONLY the editable body slice — everything from
  // the first `## ` section heading onward. The protected prefix (title line +
  // bold-field metadata block) is rendered above the editor as a read-only
  // chrome strip and is never reachable from the editor itself. On every doc
  // change we re-emit the full file as `header + editedBody` so the parent's
  // `value` binding stays the same shape that `EditTaskBody` expects.

  import { onDestroy, onMount } from 'svelte';
  import { EditorState } from '@codemirror/state';
  import { EditorView, keymap, lineNumbers, drawSelection, highlightActiveLine } from '@codemirror/view';
  import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
  import { markdown } from '@codemirror/lang-markdown';

  interface Props {
    value: string;
    originalBody: string;
    onDirtyChange?: (dirty: boolean) => void;
  }

  let { value = $bindable(''), originalBody, onDirtyChange }: Props = $props();

  let host: HTMLDivElement | null = $state(null);
  let view: EditorView | null = null;
  // Set while we're propagating a doc change outward via the value binding.
  // The $effect that watches `value` uses this to skip re-dispatching what
  // we just emitted (which would otherwise clobber the cursor/selection).
  let internalChange = false;

  // The boundary into `originalBody` where the editable region begins.
  // Anything before the first `## ` body heading (within metadataScanCap
  // lines) is the immutable header + metadata block. Mirrors the Go-side
  // `protectedPrefix` in gui/app/edit_body.go.
  function findBodyStart(s: string): number {
    const lines = s.split('\n');
    const cap = 30;
    let offset = 0;
    for (let i = 0; i < lines.length && i < cap; i++) {
      if (lines[i].trimStart().startsWith('## ')) {
        return offset;
      }
      offset += lines[i].length + 1; // +1 for the newline
    }
    return s.length;
  }

  let boundary = $derived(findBodyStart(originalBody));
  let headerStrip = $derived(originalBody.slice(0, boundary).replace(/\s+$/, ''));
  let editableInitial = $derived(value.slice(boundary));

  onMount(() => {
    if (!host) return;
    const state = EditorState.create({
      doc: editableInitial,
      extensions: [
        lineNumbers(),
        history(),
        drawSelection(),
        highlightActiveLine(),
        markdown(),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        EditorView.lineWrapping,
        EditorView.theme({
          '&': { fontSize: '13px', backgroundColor: 'transparent', color: 'var(--fg)' },
          '.cm-content': { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' },
          '.cm-gutters': { backgroundColor: 'rgba(0,0,0,0.15)', borderRight: '1px solid rgba(255,255,255,0.05)', color: 'var(--fg-dim)' },
          '.cm-activeLine': { backgroundColor: 'rgba(255,255,255,0.03)' },
          '.cm-cursor': { borderLeftColor: 'var(--accent)' },
          '.cm-scroller': { maxHeight: '360px' },
        }),
        EditorView.updateListener.of((u) => {
          if (!u.docChanged) return;
          const editedBody = u.state.doc.toString();
          const next = originalBody.slice(0, boundary) + editedBody;
          internalChange = true;
          value = next;
          onDirtyChange?.(next !== originalBody);
          queueMicrotask(() => { internalChange = false; });
        }),
      ],
    });
    view = new EditorView({ state, parent: host });
  });

  // When the parent swaps `value` externally (e.g., Discard), reset the doc
  // to the new editable slice. Skip when the change originated from our own
  // updateListener — otherwise we'd echo every keystroke back into the editor.
  $effect(() => {
    if (!view || internalChange) return;
    const wantEditable = value.slice(boundary);
    if (view.state.doc.toString() !== wantEditable) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: wantEditable },
      });
    }
  });

  onDestroy(() => {
    view?.destroy();
    view = null;
  });
</script>

<div class="editor-wrap">
  {#if headerStrip}
    <pre class="header-strip" aria-label="Read-only header" title="Edit metadata via the fields above">{headerStrip}</pre>
  {/if}
  <div bind:this={host} class="editor"></div>
  <p class="hint">Cmd/Ctrl+S to save · header above is read-only</p>
</div>

<style>
  .editor-wrap {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .header-strip {
    margin: 0;
    padding: 8px 12px;
    background: rgba(0, 0, 0, 0.18);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 6px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 12px;
    color: var(--fg-dim);
    white-space: pre-wrap;
    user-select: text;
    pointer-events: none;
    max-height: 140px;
    overflow: auto;
  }
  .editor {
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 6px;
    overflow: hidden;
    background: rgba(0, 0, 0, 0.12);
  }
  .editor :global(.cm-editor) { outline: none; }
  .editor :global(.cm-editor.cm-focused) { outline: 2px solid var(--accent); }
  .hint {
    margin: 0;
    color: var(--fg-dim);
    font-size: 11px;
  }
</style>
