<script lang="ts">
  import type { Task } from '$lib/api';

  interface Props {
    task: Task;
    onSelect?: (id: string) => void;
  }

  let { task, onSelect }: Props = $props();

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
</script>

<button
  class="card"
  class:epic={isEpic}
  onclick={() => onSelect?.(task.id)}
  type="button"
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
</button>

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
    text-align: left;
    color: inherit;
    font: inherit;
    /* truncation: clamp title to 2 lines */
  }
  .card:hover { border-color: rgba(255, 255, 255, 0.18); background: rgba(34, 44, 64, 0.95); }
  .card:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
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
  }
  .id {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 11px;
    color: var(--fg-dim);
    display: inline-flex;
    align-items: center;
    gap: 4px;
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
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
    color: var(--fg-dim);
    font-size: 10px;
    margin-bottom: 4px;
  }
  .mod, .size {
    background: rgba(255, 255, 255, 0.05);
    padding: 1px 5px;
    border-radius: 3px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
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
  }
  .tag {
    font-size: 10px;
    background: rgba(255, 255, 255, 0.05);
    color: var(--fg-dim);
    border-radius: 3px;
    padding: 1px 5px;
    font-family: ui-monospace, monospace;
  }
  .tag.overflow { background: rgba(255, 255, 255, 0.10); color: var(--fg); }
</style>
