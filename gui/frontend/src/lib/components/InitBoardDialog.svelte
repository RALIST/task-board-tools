<script lang="ts">
  import {
    INIT_BOARD_PATH_DEFAULT,
    INIT_PREFIX_DEFAULT,
    errorString,
    initBoard,
    validateInitBoardPath,
    validateInitPrefix,
  } from '$lib/api';

  interface Props {
    open: boolean;
    projectRoot: string;
    onCancel: () => void;
    onInitialized?: () => void;
  }

  let { open, projectRoot, onCancel, onInitialized }: Props = $props();

  let boardPath = $state(INIT_BOARD_PATH_DEFAULT);
  let prefix = $state(INIT_PREFIX_DEFAULT);
  let submitting = $state(false);
  let submitError = $state<string | null>(null);
  let boardPathInput: HTMLInputElement | null = $state(null);

  // Reset whenever the dialog opens against a new folder so a previous
  // failure doesn't leak across invocations.
  $effect(() => {
    if (open) {
      boardPath = INIT_BOARD_PATH_DEFAULT;
      prefix = INIT_PREFIX_DEFAULT;
      submitting = false;
      submitError = null;
      queueMicrotask(() => boardPathInput?.focus());
    }
  });

  const boardPathError = $derived(validateInitBoardPath(boardPath));
  const prefixError = $derived(validateInitPrefix(prefix));
  const canSubmit = $derived(
    !submitting && boardPathError === null && prefixError === null && projectRoot !== '',
  );

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    submitting = true;
    submitError = null;
    try {
      await initBoard(projectRoot, boardPath.trim(), prefix.trim());
      onInitialized?.();
    } catch (err) {
      submitError = errorString(err) || 'Initialization failed.';
      submitting = false;
    }
  }

  function tryClose() {
    if (submitting) return;
    onCancel();
  }

  function onBackdropClick(ev: MouseEvent) {
    if (ev.target === ev.currentTarget) tryClose();
  }
</script>

{#if open}
  <div
    class="backdrop"
    role="dialog"
    aria-modal="true"
    aria-label="Initialize tb board"
    tabindex="-1"
    onclick={onBackdropClick}
    onkeydown={() => {}}>
    <form class="dialog" onsubmit={submit}>
      <header>
        <h2>Initialize tb board</h2>
        <button type="button" class="close" onclick={tryClose} aria-label="Close" disabled={submitting}>×</button>
      </header>

      <p class="lede">
        This folder doesn't have a <code>.tb.yaml</code>. Initialize a board here?
      </p>

      <label class="field">
        <span>Project root</span>
        <input value={projectRoot} readonly />
      </label>

      <label class="field">
        <span>Board path</span>
        <input
          bind:this={boardPathInput}
          bind:value={boardPath}
          placeholder={INIT_BOARD_PATH_DEFAULT}
          aria-invalid={boardPathError !== null}
          disabled={submitting} />
        {#if boardPathError}
          <small class="err">{boardPathError}</small>
        {:else}
          <small class="hint">Relative to the project root (default <code>{INIT_BOARD_PATH_DEFAULT}</code>).</small>
        {/if}
      </label>

      <label class="field">
        <span>Task ID prefix</span>
        <input
          bind:value={prefix}
          placeholder={INIT_PREFIX_DEFAULT}
          aria-invalid={prefixError !== null}
          disabled={submitting} />
        {#if prefixError}
          <small class="err">{prefixError}</small>
        {:else}
          <small class="hint">Letters or digits, letter-led (default <code>{INIT_PREFIX_DEFAULT}</code>).</small>
        {/if}
      </label>

      {#if submitError}
        <p class="err submit-err" role="alert">{submitError}</p>
      {/if}

      <footer>
        <button type="button" class="ghost" onclick={tryClose} disabled={submitting}>Cancel</button>
        <button type="submit" class="primary" disabled={!canSubmit}>
          {submitting ? 'Initializing…' : 'Initialize'}
        </button>
      </footer>
    </form>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 60;
    box-sizing: border-box;
  }
  :global(html.platform-mac) .backdrop {
    padding-top: var(--mac-titlebar-height);
  }
  .dialog {
    background: var(--bg-elev);
    width: min(520px, 92vw);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 8px;
    padding: 18px 20px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 4px;
  }
  header h2 { margin: 0; font-size: 16px; font-weight: 600; }
  .close {
    background: none;
    border: 0;
    font-size: 22px;
    line-height: 1;
    cursor: pointer;
    padding: 4px 8px;
    color: var(--fg-dim);
  }
  .close:hover { color: var(--fg); }
  .close[disabled] { cursor: not-allowed; opacity: 0.5; }

  .lede { margin: 0; color: var(--fg-dim); font-size: 13px; }
  .lede code {
    background: rgba(255, 255, 255, 0.08);
    padding: 1px 5px;
    border-radius: 4px;
    font-family: ui-monospace, monospace;
  }

  .field { display: flex; flex-direction: column; gap: 4px; font-size: 12px; }
  .field span { color: var(--fg-dim); }
  .field input {
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    border-radius: 5px;
    padding: 6px 8px;
    font: inherit;
    font-size: 13px;
  }
  .field input[aria-invalid='true'] {
    border-color: var(--p0);
  }
  .field input[readonly] {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    color: var(--fg-dim);
  }
  .hint { color: var(--fg-dim); font-size: 11px; }
  .hint code {
    background: rgba(255, 255, 255, 0.08);
    padding: 0 4px;
    border-radius: 3px;
    font-family: ui-monospace, monospace;
  }
  .err { color: var(--p0); font-size: 11px; }
  .submit-err { margin: 0; padding: 6px 8px; border: 1px solid rgba(255, 90, 82, 0.5); border-radius: 5px; background: rgba(255, 90, 82, 0.08); font-size: 12px; }

  footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 4px;
  }
  .ghost {
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.12);
    color: var(--fg);
    border-radius: 5px;
    padding: 6px 14px;
    cursor: pointer;
  }
  .ghost:disabled { opacity: 0.5; cursor: not-allowed; }
  .primary {
    background: var(--accent);
    color: white;
    border: 0;
    border-radius: 5px;
    padding: 6px 16px;
    cursor: pointer;
    font-weight: 600;
  }
  .primary:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
