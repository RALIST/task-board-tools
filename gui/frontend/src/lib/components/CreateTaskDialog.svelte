<script lang="ts">
  import { createTask, type Task } from '$lib/api';
  import { pushToast } from '$lib/stores/toast';

  interface Props {
    open: boolean;
    epics?: Task[];
    onClose: () => void;
    onCreated?: (id: string) => void;
    /** Bindable read-out of whether any field differs from defaults. The
     * parent reads this to apply the same discard-confirmation when its
     * global Escape shortcut closes the dialog. */
    dirty?: boolean;
  }

  let {
    open,
    epics = [],
    onClose,
    onCreated,
    dirty = $bindable(false),
  }: Props = $props();

  const DEFAULTS = {
    title: '',
    module: '',
    type: 'bug' as const,
    priority: 'P2' as const,
    size: 'M' as const,
    tags: '',
    description: '',
    parent: '',
    isEpic: false,
  };

  let title = $state(DEFAULTS.title);
  let module = $state(DEFAULTS.module);
  let type = $state<'bug' | 'feature' | 'tech-debt' | 'improvement' | 'spike'>(DEFAULTS.type);
  let priority = $state<'P0' | 'P1' | 'P2' | 'P3'>(DEFAULTS.priority);
  let size = $state<'S' | 'M' | 'L' | 'XL'>(DEFAULTS.size);
  let tags = $state(DEFAULTS.tags);
  let description = $state(DEFAULTS.description);
  let parent = $state(DEFAULTS.parent);
  let isEpic = $state(DEFAULTS.isEpic);
  let submitting = $state(false);
  let titleInput: HTMLInputElement | null = $state(null);

  function resetFields() {
    title = DEFAULTS.title;
    module = DEFAULTS.module;
    type = DEFAULTS.type;
    priority = DEFAULTS.priority;
    size = DEFAULTS.size;
    tags = DEFAULTS.tags;
    description = DEFAULTS.description;
    parent = DEFAULTS.parent;
    isEpic = DEFAULTS.isEpic;
    submitting = false;
  }

  // When the dialog opens, focus the title field and reset state.
  $effect(() => {
    if (open) {
      resetFields();
      // Slight delay to wait for the DOM transition before focus.
      queueMicrotask(() => titleInput?.focus());
    }
  });

  /** Form is dirty when any user-editable field differs from the create-task
   * defaults. Submit clears these before calling onClose so the next reopen
   * still starts empty. */
  const isDirty = $derived(
    title !== DEFAULTS.title
      || module !== DEFAULTS.module
      || type !== DEFAULTS.type
      || priority !== DEFAULTS.priority
      || size !== DEFAULTS.size
      || tags !== DEFAULTS.tags
      || description !== DEFAULTS.description
      || parent !== DEFAULTS.parent
      || isEpic !== DEFAULTS.isEpic,
  );

  // Publish dirty state to the parent so its global Esc handler can apply
  // the same discard guard without re-deriving from individual fields.
  $effect(() => {
    dirty = isDirty;
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!title.trim() || submitting) return;
    submitting = true;
    try {
      const res = await createTask({
        title: title.trim(),
        module,
        type,
        priority,
        size,
        tags,
        description,
        parent,
        epic: isEpic,
      });
      pushToast(`Created ${res.id}`, 'success');
      // Reset BEFORE closing so the dirty flag is false by the time the
      // parent (or its global Esc handler) reacts to onClose.
      resetFields();
      onCreated?.(res.id);
      onClose();
    } catch (err) {
      pushToast(`Create failed: ${err instanceof Error ? err.message : String(err)}`);
      submitting = false;
    }
  }

  /** Close the dialog, prompting for discard confirmation if the form has
   * been edited. Used by Cancel, the header close button, and the backdrop
   * click. The global Escape shortcut is owned by the parent (+page.svelte),
   * which performs the same dirty check via the bindable `dirty` prop. */
  export function tryClose() {
    if (isDirty) {
      const ok = window.confirm('Discard this unsaved task?');
      if (!ok) return;
    }
    onClose();
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
    aria-label="Create task"
    tabindex="-1"
    onclick={onBackdropClick}
    onkeydown={() => {}}>
    <form class="dialog" onsubmit={submit}>
      <header>
        <h2>New task</h2>
        <button type="button" class="close" onclick={tryClose} aria-label="Close">×</button>
      </header>

      <label class="field">
        <span>Title</span>
        <input
          bind:this={titleInput}
          bind:value={title}
          required
          placeholder="What needs doing?" />
      </label>

      <div class="row">
        <label class="field">
          <span>Module</span>
          <input bind:value={module} placeholder="optional" />
        </label>
        <label class="field">
          <span>Tags</span>
          <input bind:value={tags} placeholder="comma,separated" />
        </label>
      </div>

      <div class="row">
        <label class="field">
          <span>Type</span>
          <select bind:value={type}>
            <option value="bug">bug</option>
            <option value="feature">feature</option>
            <option value="tech-debt">tech-debt</option>
            <option value="improvement">improvement</option>
            <option value="spike">spike</option>
          </select>
        </label>
        <label class="field">
          <span>Priority</span>
          <select bind:value={priority}>
            <option>P0</option>
            <option>P1</option>
            <option>P2</option>
            <option>P3</option>
          </select>
        </label>
        <label class="field">
          <span>Size</span>
          <select bind:value={size}>
            <option>S</option>
            <option>M</option>
            <option>L</option>
            <option>XL</option>
          </select>
        </label>
      </div>

      <label class="field">
        <span>Parent epic</span>
        <select bind:value={parent}>
          <option value="">(none)</option>
          {#each epics as e}
            <option value={e.id}>{e.id} — {e.title}</option>
          {/each}
        </select>
      </label>

      <label class="check">
        <input type="checkbox" bind:checked={isEpic} />
        <span>This is an epic</span>
      </label>

      <label class="field">
        <span>Description (Goal)</span>
        <textarea bind:value={description} rows="3" placeholder="One-sentence goal"></textarea>
      </label>

      <footer>
        <button type="button" class="ghost" onclick={tryClose}>Cancel</button>
        <button type="submit" class="primary" disabled={!title.trim() || submitting}>
          {submitting ? 'Creating…' : 'Create'}
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
    width: min(540px, 92vw);
    max-height: 90vh;
    overflow-y: auto;
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

  .field { display: flex; flex-direction: column; gap: 4px; font-size: 12px; }
  .field span { color: var(--fg-dim); }
  .field input, .field select, .field textarea {
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    border-radius: 5px;
    padding: 6px 8px;
    font: inherit;
    font-size: 13px;
  }
  .field textarea { resize: vertical; font-family: ui-monospace, monospace; }
  .row { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 8px; }
  .row .field:has(input:not([type='checkbox'])) { width: 100%; }
  @media (max-width: 540px) { .row { grid-template-columns: 1fr; } }

  .check { display: flex; align-items: center; gap: 6px; font-size: 12px; color: var(--fg-dim); }

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
