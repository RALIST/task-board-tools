<script lang="ts">
  import { Events } from '@wailsio/runtime';
  import { renameTask, resumeAgent, type Task } from '$lib/api';
  import { suggestGroom } from '$lib/stores/groomSuggestion';
  import { pushToast } from '$lib/stores/toast';
  import { upsertRun } from '$lib/stores/runs';
  import { registerTaskTriageEventHandler, triageForTask } from '$lib/stores/triage';

  interface Props {
    task: Task;
    onSelect?: (id: string) => void;
  }

  let { task, onSelect }: Props = $props();
  let triageReasons = $state<string[]>([]);

  // Inline rename state. The draft is kept separately so cancellation can
  // restore the original title without round-tripping the parent store.
  let renaming = $state(false);
  let renameDraft = $state('');
  let renameSaving = $state(false);
  let renameInput = $state<HTMLInputElement | null>(null);

  const MAX_VISIBLE_TAGS = 3;

  // Glyphs are kept ASCII so we don't pull an icon font. Each kind is a
  // single character so it sits cleanly inside a 16px square.
  const typeGlyph: Record<string, string> = {
    feature: '✦',
    bug: '✕',
    'tech-debt': '⚙',
    improvement: '↑',
    spike: '?',
  };

  let isEpic = $derived(task.tags?.includes('epic') ?? false);

  let visibleTags = $derived(
    (task.tags ?? []).filter((t) => t !== 'epic').slice(0, MAX_VISIBLE_TAGS),
  );
  let hiddenTagCount = $derived(
    Math.max(0, (task.tags?.length ?? 0) - visibleTags.length - (isEpic ? 1 : 0)),
  );

  let priorityClass = $derived(`pri-${task.priority?.toLowerCase() ?? 'p2'}`);
  let typeKey = $derived(task.type ?? '');
  let showGroomIndicator = $derived(task.status === 'backlog' && triageReasons.length > 0);
  let triageTitle = $derived(triageReasons.join(', '));

  $effect(() => {
    const id = task.id;
    const offStore = triageForTask(id).subscribe((reasons) => {
      triageReasons = reasons;
    });
    const offEvent = registerTaskTriageEventHandler(id, (name, handler) => Events.On(name, handler as any));
    return () => {
      offStore();
      offEvent();
    };
  });

  function openForGroom(ev: MouseEvent | KeyboardEvent) {
    ev.stopPropagation();
    suggestGroom(task.id);
    onSelect?.(task.id);
  }

  // TB-130: Resume icon-button on the card surface — saves the user
  // from opening the drawer when they spot an interrupted task on the
  // kanban. Visibility is gated on agentStatus === 'interrupted' (the
  // task-level status, written by RecoverStale; the agent must
  // resolve to a runnable name before we can issue ResumeAgent).
  let resumeBusy = $state(false);
  let canResumeOnCard = $derived(task.agentStatus === 'interrupted' && !!task.agent && !resumeBusy);

  async function onCardResume(ev: MouseEvent | KeyboardEvent) {
    ev.stopPropagation();
    if (!canResumeOnCard) return;
    resumeBusy = true;
    try {
      const runId = await resumeAgent(task.id);
      // Optimistic queued row matches the drawer's onResumeClick so
      // the Wails event soon refreshes it with the resume chip.
      upsertRun({
        runId,
        taskId: task.id,
        agent: task.agent ?? 'claude',
        mode: 'resume',
        status: 'queued',
        queuedAt: new Date().toISOString(),
      });
      pushToast(`Resumed ${task.id}`, 'success');
    } catch (e) {
      pushToast(`Resume failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      resumeBusy = false;
    }
  }

  function onCardKeydown(ev: KeyboardEvent) {
    // Don't intercept keys when the rename editor is active — Enter/Escape
    // belong to the inline form there.
    if (renaming) return;
    if (ev.key !== 'Enter' && ev.key !== ' ') return;
    ev.preventDefault();
    onSelect?.(task.id);
  }

  function beginRename(ev: MouseEvent) {
    ev.stopPropagation();
    if (renaming || renameSaving) return;
    renameDraft = task.title ?? '';
    renaming = true;
    // Focus + select after the input is rendered. queueMicrotask runs after
    // Svelte's DOM update so the bind:this reference is populated.
    queueMicrotask(() => {
      renameInput?.focus();
      renameInput?.select();
    });
  }

  // Single clicks on the title are deferred so a real double-click (which
  // browsers deliver as click→click→dblclick) doesn't open the drawer behind
  // the rename editor. If a second click arrives within the threshold, we
  // cancel the open-drawer action and let `ondblclick` handle rename.
  // 250ms is short enough that the open-drawer feel is barely affected and
  // long enough to cover typical OS double-click timings.
  let titleClickTimer: ReturnType<typeof setTimeout> | null = null;
  function onTitleClick(ev: MouseEvent) {
    ev.stopPropagation();
    if (renaming) return;
    if (titleClickTimer) {
      clearTimeout(titleClickTimer);
      titleClickTimer = null;
      // Second click of a dblclick sequence — do nothing here; dblclick
      // handler will fire and open the rename editor.
      return;
    }
    titleClickTimer = setTimeout(() => {
      titleClickTimer = null;
      if (renaming) return;
      onSelect?.(task.id);
    }, 250);
  }

  function cancelRename() {
    renaming = false;
    renameDraft = '';
  }

  async function commitRename() {
    if (renameSaving) return;
    const next = renameDraft.trim();
    if (next === '') {
      pushToast('Title cannot be empty', 'info');
      return;
    }
    if (next === (task.title ?? '').trim()) {
      // No-op rename — close the editor without hitting the CLI.
      renaming = false;
      return;
    }
    renameSaving = true;
    try {
      await renameTask(task.id, next);
      // Successful save: drop the draft and close. The watcher will refresh
      // task.title via the parent's board store; no optimistic mutation here.
      renaming = false;
      pushToast(`Renamed ${task.id}`, 'success');
    } catch (e) {
      // Keep the draft and the input open so the user can fix and retry.
      pushToast(`Rename failed: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      renameSaving = false;
    }
  }

  function onInputKeydown(ev: KeyboardEvent) {
    ev.stopPropagation();
    if (ev.key === 'Enter') {
      ev.preventDefault();
      void commitRename();
    } else if (ev.key === 'Escape') {
      ev.preventDefault();
      cancelRename();
    }
  }

  function onInputBlur() {
    // If the user clicks elsewhere we cancel the draft rather than committing
    // silently. Saves go through Enter or the explicit Save button.
    if (renameSaving) return;
    cancelRename();
  }

  function onTitleDblClick(ev: MouseEvent) {
    beginRename(ev);
  }

</script>

<div
  class="card"
  data-task-id={task.id}
  data-file-drop-target
  class:epic={isEpic}
  role="button"
  tabindex="0"
  onclick={() => onSelect?.(task.id)}
  onkeydown={onCardKeydown}
  title={task.title}>
  <header class="head">
    <span class="id">
      {#if typeGlyph[typeKey]}
        <span class={`glyph t-${typeKey}`} aria-label={typeKey}>{typeGlyph[typeKey]}</span>
      {/if}
      {task.id}
    </span>
    {#if task.priority}
      <span class={`pri ${priorityClass}`}>{task.priority}</span>
    {/if}
    <span class="groom-slot" aria-hidden={!showGroomIndicator}>
      {#if showGroomIndicator}
        <button
          class="groom-indicator"
          type="button"
          title={triageTitle}
          aria-label={`Needs grooming: ${triageTitle}`}
          onclick={openForGroom}>!</button>
      {/if}
    </span>
    {#if task.agentStatus === 'interrupted'}
      <button
        class="resume-indicator"
        type="button"
        title={canResumeOnCard ? `Resume agent session for ${task.id}` : 'No runnable agent assigned'}
        aria-label={`Resume agent session for ${task.id}`}
        disabled={!canResumeOnCard}
        onclick={onCardResume}>↻</button>
    {/if}
  </header>

  {#if renaming}
    <input
      class="ttl-input"
      type="text"
      bind:value={renameDraft}
      bind:this={renameInput}
      onkeydown={onInputKeydown}
      onclick={(ev) => ev.stopPropagation()}
      onblur={onInputBlur}
      disabled={renameSaving}
      aria-label="Task title" />
  {:else}
    <button
      type="button"
      class="ttl"
      onclick={onTitleClick}
      ondblclick={onTitleDblClick}
      title="Click to open, double-click to rename"
      aria-label={`${task.title} — click to open, double-click to rename`}>{task.title}</button>
  {/if}

  <footer class="meta">
    {#if task.module}<span class="mod" title={`module: ${task.module}`}>{task.module}</span>{/if}
    {#if task.size}<span class="size">{task.size}</span>{/if}
    {#if isEpic}<span class="epic-badge" title="epic">EPIC</span>{/if}
    {#if task.agent}
      <span class={`agent agent-${task.agentStatus || 'idle'}`} title={`${task.agent} • ${task.agentStatus || 'idle'}`}>
        <span class="agent-glyph">{task.agent === 'codex' ? 'X' : 'C'}</span>
        {task.agent}{task.agentStatus ? ` · ${task.agentStatus}` : ''}
      </span>
    {/if}
  </footer>

  {#if visibleTags.length > 0 || hiddenTagCount > 0}
    <div class="tags">
      {#each visibleTags as tag}
        <span class="tag">{tag}</span>
      {/each}
      {#if hiddenTagCount > 0}
        <span class="tag overflow">+{hiddenTagCount}</span>
      {/if}
    </div>
  {/if}
</div>

<style>
  .card {
    background: var(--bg-card);
    border: 1px solid rgba(255, 255, 255, 0.05);
    border-radius: 6px;
    padding: 9px 10px;
    margin: 0 0 8px;
    cursor: pointer;
    display: block;
    width: 100%;
    box-sizing: border-box;
    min-width: 0;
    max-width: 100%;
    overflow: hidden;
    text-align: left;
    color: inherit;
    font: inherit;
    /* truncation: clamp title to 2 lines */
  }
  .card:hover { border-color: rgba(255, 255, 255, 0.18); background: rgba(34, 44, 64, 0.95); }
  .card:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
  .card:global(.file-drop-target-active) {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px rgba(74, 141, 248, 0.35);
    background: rgba(74, 141, 248, 0.12);
  }
  .card.epic {
    border-left: 3px solid var(--accent);
    padding-left: 8px;
  }

  .head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    margin-bottom: 4px;
    gap: 8px;
    min-width: 0;
  }
  .id {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    color: var(--fg-dim);
    display: inline-flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .glyph {
    display: inline-flex;
    width: 14px;
    height: 14px;
    align-items: center;
    justify-content: center;
    border-radius: 3px;
    font-size: 10px;
    background: rgba(255, 255, 255, 0.05);
    color: var(--fg-dim);
  }
  .glyph.t-bug { background: rgba(255, 90, 82, 0.18); color: var(--p0); }
  .glyph.t-feature { background: rgba(74, 141, 248, 0.18); color: var(--p2); }
  .glyph.t-improvement { background: rgba(80, 200, 120, 0.18); color: #50c878; }
  .glyph.t-spike { background: rgba(255, 184, 108, 0.18); color: var(--p1); }
  .glyph.t-tech-debt { background: rgba(110, 118, 134, 0.20); color: #b0b8c8; }

  .pri {
    font-size: 10px;
    font-weight: 700;
    padding: 1px 5px;
    border-radius: 4px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    flex-shrink: 0;
  }
  .groom-slot {
    width: 18px;
    height: 18px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: 0 0 18px;
  }
  .groom-indicator {
    display: inline-flex;
    width: 16px;
    height: 16px;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    background: rgba(255, 184, 108, 0.18);
    border: 1px solid rgba(255, 184, 108, 0.42);
    color: var(--p1);
    font-size: 11px;
    font-weight: 800;
    line-height: 1;
    cursor: pointer;
  }
  .groom-indicator:hover { background: rgba(255, 184, 108, 0.28); }
  .groom-indicator:focus-visible { outline: 2px solid var(--p1); outline-offset: 1px; }

  .resume-indicator {
    margin-left: 4px;
    width: 18px;
    height: 18px;
    line-height: 18px;
    text-align: center;
    border: 0;
    padding: 0;
    border-radius: 50%;
    background: rgba(245, 158, 11, 0.18);
    color: #f59e0b;
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
  }
  .resume-indicator:hover:not(:disabled) { background: rgba(245, 158, 11, 0.32); }
  .resume-indicator:disabled { opacity: 0.4; cursor: not-allowed; }
  .resume-indicator:focus-visible { outline: 2px solid #f59e0b; outline-offset: 1px; }
  .pri-p0 { background: var(--p0); color: white; }
  .pri-p1 { background: var(--p1); color: black; }
  .pri-p2 { background: rgba(74, 141, 248, 0.18); color: var(--p2); }
  .pri-p3 { background: rgba(110, 118, 134, 0.18); color: var(--p3); }

  .ttl {
    margin: 0 0 6px;
    font-size: 13px;
    line-height: 1.35;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    overflow-wrap: anywhere;
    word-break: break-word;
    min-width: 0;
    border-radius: 3px;
    /* Reset native button chrome so the title still looks like card text. */
    background: none;
    border: 0;
    padding: 0;
    color: inherit;
    font: inherit;
    font-size: 13px;
    line-height: 1.35;
    text-align: left;
    width: 100%;
    cursor: pointer;
  }
  .ttl:focus-visible {
    outline: 1px dashed rgba(74, 141, 248, 0.6);
    outline-offset: 2px;
  }
  .ttl-input {
    margin: 0 0 6px;
    width: 100%;
    min-width: 0;
    box-sizing: border-box;
    background: rgba(0, 0, 0, 0.30);
    border: 1px solid var(--accent);
    color: var(--fg);
    border-radius: 4px;
    padding: 4px 6px;
    font: inherit;
    font-size: 13px;
    line-height: 1.35;
  }
  .ttl-input:disabled { opacity: 0.7; }

  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
    color: var(--fg-dim);
    font-size: 10px;
    margin-bottom: 4px;
    min-width: 0;
  }
  .mod, .size {
    background: rgba(255, 255, 255, 0.05);
    padding: 1px 5px;
    border-radius: 3px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .epic-badge {
    background: rgba(74, 141, 248, 0.18);
    color: var(--accent);
    padding: 1px 5px;
    border-radius: 3px;
    font-weight: 700;
    letter-spacing: 0.08em;
  }
  .agent {
    margin-left: auto;
    padding: 1px 5px;
    border-radius: 3px;
    font-family: ui-monospace, monospace;
    font-size: 10px;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .agent-queued { background: rgba(255, 184, 108, 0.18); color: var(--p1); }
  .agent-running { background: rgba(74, 141, 248, 0.18); color: var(--p2); }
  .agent-success { background: rgba(80, 200, 120, 0.18); color: #50c878; }
  .agent-failed { background: rgba(255, 90, 82, 0.18); color: var(--p0); }
  .agent-cancelled { background: rgba(110, 118, 134, 0.18); color: var(--p3); }
  .agent-interrupted { background: rgba(245, 158, 11, 0.22); color: #f59e0b; }
  .agent-idle { background: rgba(110, 118, 134, 0.10); color: var(--fg-dim); }
  .agent-glyph {
    display: inline-block;
    width: 12px;
    text-align: center;
    margin-right: 3px;
    font-weight: 700;
    color: inherit;
  }

  .tags {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    min-width: 0;
  }
  .tag {
    font-size: 10px;
    background: rgba(255, 255, 255, 0.05);
    color: var(--fg-dim);
    border-radius: 3px;
    padding: 1px 5px;
    font-family: ui-monospace, monospace;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tag.overflow { background: rgba(255, 255, 255, 0.10); color: var(--fg); }
</style>
