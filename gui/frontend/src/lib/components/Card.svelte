<script lang="ts">
  import { Events } from '@wailsio/runtime';
  import type { Task } from '$lib/api';
  import { suggestGroom } from '$lib/stores/groomSuggestion';
  import { registerTaskTriageEventHandler, triageForTask } from '$lib/stores/triage';

  interface Props {
    task: Task;
    onSelect?: (id: string) => void;
  }

  let { task, onSelect }: Props = $props();
  let triageReasons = $state<string[]>([]);

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

  function onCardKeydown(ev: KeyboardEvent) {
    if (ev.key !== 'Enter' && ev.key !== ' ') return;
    ev.preventDefault();
    onSelect?.(task.id);
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
  </header>

  <p class="ttl">{task.title}</p>

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
  }

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
