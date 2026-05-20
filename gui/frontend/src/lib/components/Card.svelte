<script lang="ts">
  import { Events } from '@wailsio/runtime';
  import { renameTask, resumeAgent, type Task } from '$lib/api';
  import { suggestGroom } from '$lib/stores/groomSuggestion';
  import { pushToast } from '$lib/stores/toast';
  import { upsertRun } from '$lib/stores/runs';
  import { registerTaskTriageEventHandler, triageForTask } from '$lib/stores/triage';
  import { board } from '$lib/stores/board';
  import { epicProgress } from '$lib/filtering';
  import { preferencesStore } from '$lib/stores/preferences';
  import { autoGroomStore } from '$lib/stores/autoGroom';
  import { runsForTask, type Run } from '$lib/stores/runs';

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
  // Epic progress is computed from the live board store rather than props so
  // a child status change reflows here without a backend round-trip per card.
  let progress = $derived(isEpic ? epicProgress($board, task.id) : null);

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
  // Auto-groom chip is the live state of the most-recent mode=groom run
  // for this task (TB-175). Shown only when auto-groom is enabled — when
  // disabled, we hide the chip entirely so the M6 manual flow stays
  // visually unchanged.
  let autoGroomEnabledForCard = $derived($preferencesStore.autoGroomEnabled);
  // Use the reactive runsForTask store (vs the plain runsByTask snapshot)
  // so the chip status updates when an in-flight groom run advances
  // through queued → running → success without requiring a re-mount.
  let cardRuns = $derived(runsForTask(task.id));
  let groomRunStatus = $derived.by<Run['status'] | ''>(() => {
    if (!autoGroomEnabledForCard) return '';
    const runs = $cardRuns.filter((r) => r.mode === 'groom');
    return runs.length > 0 ? runs[0].status : '';
  });
  let showAutoGroomChip = $derived(
    autoGroomEnabledForCard && task.status === 'backlog' && groomRunStatus !== '',
  );
  // Settle-waiting pill: the coordinator skipped this task because the
  // settle window has not yet elapsed. Render only on backlog and only
  // when the most-recent skip reason was specifically "settle".
  let settleSkipReason = $derived($autoGroomStore.lastSkipReasons[task.id] ?? '');
  let showSettleWaiting = $derived(
    autoGroomEnabledForCard && task.status === 'backlog' && settleSkipReason === 'settle',
  );
  // Compose a single tooltip across all three slot-resident chips so the
  // user can read the state without opening the drawer.
  let groomSlotTitle = $derived.by<string>(() => {
    if (showAutoGroomChip) {
      return `Auto-groom run ${groomRunStatus}`;
    }
    if (showSettleWaiting) {
      return 'Auto-groom waiting for the settle window to end.';
    }
    return triageTitle;
  });
  let showAnyGroomSlotContent = $derived(showGroomIndicator || showAutoGroomChip || showSettleWaiting);
  // TB-182: surface a "user attention" indicator on the card so a needs-user
  // task is immediately visible on the kanban without opening the drawer.
  let needsUserAttention = $derived(task.agentStatus === 'needs-user');
  // TB-199 / M10 (TB-239): review-failed marker — visible on tasks that got
  // bounced back from code-review with actionable findings. Lives on `ready`
  // tasks post-M10 (legacy `backlog` tasks may still carry the tag). Distinct
  // from needs-user (an autonomous-agent pause); review-failed is a workflow
  // signal for human and auto-implement triage.
  let isReviewFailed = $derived(task.tags?.includes('review-failed') ?? false);

  // TB-237: per-action attribution chips. One chip per action that has at
  // least one of (agent, status) recorded — missing actions render
  // nothing. Glyphs match the mode initials (G/I/R) and inherit the
  // status-coloured palette via .per-action-<status>.
  type PerActionChip = { mode: string; label: string; glyph: string; agent: string; status: string };
  let perActionChips = $derived.by<PerActionChip[]>(() => {
    const chips: PerActionChip[] = [
      { mode: 'groom', label: 'Groomed', glyph: 'G', agent: task.groomedBy ?? '', status: task.groomStatus ?? '' },
      { mode: 'implement', label: 'Implemented', glyph: 'I', agent: task.implementedBy ?? '', status: task.implementStatus ?? '' },
      { mode: 'review', label: 'Reviewed', glyph: 'R', agent: task.reviewedBy ?? '', status: task.reviewStatus ?? '' },
    ];
    return chips.filter((c) => c.agent !== '' || c.status !== '');
  });

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

  // Resume is driven by the backend's latest-run session lookup instead of
  // a frontend status string. The task status still appears in the tooltip
  // so resuming a failed/cancelled/successful run is an intentional action.
  let resumeBusy = $state(false);
  let hasRunnableAgent = $derived(!!task.agent);
  let showResumeOnCard = $derived(task.agentResumable);
  let resumeSourceStatus = $derived(task.agentStatus || 'previous');
  let canResumeOnCard = $derived(showResumeOnCard && hasRunnableAgent && !resumeBusy);
  let resumeTitle = $derived(`Resume ${resumeSourceStatus} run for ${task.id}`);
  let resumeDisabledTitle = $derived(!hasRunnableAgent
    ? 'No runnable agent assigned'
    : resumeTitle);

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
    <span class="groom-slot" aria-hidden={!showAnyGroomSlotContent} title={groomSlotTitle}>
      {#if showGroomIndicator}
        <button
          class="groom-indicator"
          type="button"
          title={triageTitle}
          aria-label={`Needs grooming: ${triageTitle}`}
          onclick={openForGroom}>!</button>
      {/if}
      {#if showAutoGroomChip}
        <span
          class={`auto-groom-chip per-action-${groomRunStatus}`}
          aria-label={`Auto-groom ${groomRunStatus}`}>
          G
        </span>
      {/if}
      {#if showSettleWaiting}
        <span class="auto-groom-settle" aria-label="Auto-groom waiting on settle window">⏳</span>
      {/if}
    </span>
    {#if showResumeOnCard}
      <button
        class="resume-indicator"
        type="button"
        title={canResumeOnCard ? resumeTitle : resumeDisabledTitle}
        aria-label={resumeTitle}
        disabled={!canResumeOnCard}
        onclick={onCardResume}>↻</button>
    {/if}
    {#if needsUserAttention}
      <span
        class="needs-user-indicator"
        title={`${task.id} needs user input — open the drawer to see the question`}
        aria-label={`${task.id} needs user input`}>?</span>
    {/if}
    {#if isReviewFailed}
      <span
        class="review-failed-indicator"
        title={`${task.id} failed code review — see Review Findings in the drawer`}
        aria-label={`${task.id} failed code review`}>↩</span>
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
    <!-- Plain <div> rather than <button>: svelte-dnd-action refuses to start
         a drag when pointerdown lands on an element exposing `.value` (true
         for HTMLButtonElement) or `isContentEditable`, which made the title
         (the largest target on the card) undraggable. role="presentation"
         already silences the relevant Svelte a11y warnings on the <div>. -->
    <div
      class="ttl"
      role="presentation"
      onclick={onTitleClick}
      ondblclick={onTitleDblClick}
      title="Click to open, double-click to rename"
      aria-label={`${task.title} — click to open, double-click to rename`}>{task.title}</div>
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
    {#each perActionChips as chip (chip.mode)}
      <span
        class={`per-action per-action-${chip.status || 'idle'}`}
        title={`${chip.label}${chip.agent ? `: ${chip.agent}` : ''}${chip.status ? ` · ${chip.status}` : ''}`}
        aria-label={[chip.label, chip.agent, chip.status].filter(Boolean).join(' ')}>
        <span class="per-action-glyph">{chip.glyph}</span>
      </span>
    {/each}
  </footer>

  {#if isEpic && progress}
    <div
      class="epic-progress"
      class:complete={progress.total > 0 && progress.done === progress.total}
      class:empty={progress.total === 0}
      title={progress.total === 0
        ? 'No child tasks yet'
        : `Epic progress: ${progress.done} of ${progress.total} done (${progress.percent}%)`}
      aria-label={progress.total === 0
        ? 'Epic progress: no child tasks yet'
        : `Epic progress ${progress.done} of ${progress.total}`}>
      <span class="epic-progress-label">{progress.done}/{progress.total}</span>
      <span class="epic-progress-bar" aria-hidden="true">
        <span class="epic-progress-fill" style={`width: ${progress.total === 0 ? 0 : progress.percent}%`}></span>
      </span>
    </div>
  {/if}

  {#if visibleTags.length > 0 || hiddenTagCount > 0}
    <div class="tags">
      {#each visibleTags as tag (tag)}
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
    min-width: 18px;
    height: 18px;
    display: inline-flex;
    align-items: center;
    justify-content: flex-start;
    gap: 3px;
    flex: 0 0 auto;
  }
  .auto-groom-chip {
    display: inline-flex;
    width: 16px;
    height: 16px;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    font-size: 9.5px;
    font-weight: 700;
    line-height: 1;
    letter-spacing: 0.02em;
  }
  .auto-groom-settle {
    display: inline-flex;
    width: 16px;
    height: 16px;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    color: var(--fg-dim);
    background: rgba(255, 255, 255, 0.06);
    border-radius: 4px;
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
  .needs-user-indicator {
    margin-left: 4px;
    display: inline-flex;
    width: 18px;
    height: 18px;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    background: rgba(168, 85, 247, 0.22);
    color: #a855f7;
    font-size: 12px;
    font-weight: 800;
    line-height: 1;
    cursor: help;
  }
  .review-failed-indicator {
    margin-left: 4px;
    display: inline-flex;
    width: 18px;
    height: 18px;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    background: rgba(255, 90, 82, 0.22);
    color: var(--p0);
    font-size: 12px;
    font-weight: 700;
    line-height: 1;
    cursor: help;
  }
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
  .agent-lost { background: rgba(236, 72, 153, 0.18); color: #ec4899; }
  .agent-needs-user { background: rgba(168, 85, 247, 0.22); color: #a855f7; }
  .agent-idle { background: rgba(110, 118, 134, 0.10); color: var(--fg-dim); }
  .agent-glyph {
    display: inline-block;
    width: 12px;
    text-align: center;
    margin-right: 3px;
    font-weight: 700;
    color: inherit;
  }

  /* TB-237: compact per-action chips. Same colour palette as .agent-* so
     the user can read groom/implement/review status at a glance. */
  .per-action {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    padding: 0 4px;
    border-radius: 8px;
    font-size: 10px;
    line-height: 1;
    height: 16px;
  }
  .per-action-queued { background: rgba(255, 184, 108, 0.18); color: var(--p1); }
  .per-action-running { background: rgba(74, 141, 248, 0.18); color: var(--p2); }
  .per-action-success { background: rgba(80, 200, 120, 0.18); color: #50c878; }
  .per-action-failed { background: rgba(255, 90, 82, 0.18); color: var(--p0); }
  .per-action-cancelled { background: rgba(110, 118, 134, 0.18); color: var(--p3); }
  .per-action-interrupted { background: rgba(245, 158, 11, 0.22); color: #f59e0b; }
  .per-action-lost { background: rgba(236, 72, 153, 0.18); color: #ec4899; }
  .per-action-needs-user { background: rgba(168, 85, 247, 0.22); color: #a855f7; }
  .per-action-idle { background: rgba(110, 118, 134, 0.10); color: var(--fg-dim); }
  .per-action-glyph {
    display: inline-block;
    text-align: center;
    font-weight: 700;
    color: inherit;
  }

  .epic-progress {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 4px;
    /* Fixed-height row keeps epic cards visually consistent regardless of
       done/total — avoids a layout shift between e.g. 0/0 and 3/4 epics. */
    height: 14px;
  }
  .epic-progress-label {
    font-size: 10px;
    color: var(--fg-dim);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-variant-numeric: tabular-nums;
    flex: 0 0 auto;
    /* Reserve room for 4 digits ("99/99") so the bar's left edge doesn't
       drift between e.g. 1/3 and 11/30. Tabular-nums alone only equalises
       digit width; the digit *count* still varies without a min-width. */
    min-width: 34px;
    text-align: left;
  }
  .epic-progress-bar {
    flex: 1 1 auto;
    min-width: 0;
    height: 4px;
    background: rgba(255, 255, 255, 0.06);
    border-radius: 2px;
    overflow: hidden;
    position: relative;
  }
  .epic-progress-fill {
    display: block;
    height: 100%;
    background: var(--accent);
    border-radius: 2px;
    transition: width 160ms ease;
  }
  .epic-progress.complete .epic-progress-fill { background: #50c878; }
  /* 0/0 epics: muted label colour and an empty bar — avoids the misleading
     "100% complete" look of a full bar when there are no children yet. */
  .epic-progress.empty .epic-progress-label { color: var(--fg-dim); opacity: 0.7; }
  .epic-progress.empty .epic-progress-fill { background: transparent; }

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
