<script lang="ts">
  import type { Task } from '$lib/api';
  import Card from './Card.svelte';
  import { dndzone, TRIGGERS, type DndEvent } from 'svelte-dnd-action';

  export type DropTarget = 'backlog' | 'ready' | 'in-progress' | 'code-review' | 'done';

  interface Props {
    title: string;
    status: 'backlog' | 'ready' | 'in-progress' | 'code-review' | 'done' | 'archive';
    tasks: Task[];
    draggable?: boolean;
    wipLimit?: number;
    onSelect?: (id: string) => void;
    onDrop?: (taskId: string, target: DropTarget) => void;
  }

  let { title, status, tasks, draggable = true, wipLimit, onSelect, onDrop }: Props = $props();

  let overLimit = $derived(wipLimit !== undefined && wipLimit > 0 && tasks.length >= wipLimit);

  let dragOver = $state(false);
  let dragging = $state(false);

  // svelte-dnd-action requires the SAME array IDENTITY across the consider →
  // finalize lifecycle. We keep a $state-backed `items` array and refresh it
  // from `tasks` only when no drag is in flight, so a board:reloaded mid-drag
  // doesn't blow the library's internal DOM tracking (it crashes with
  // `undefined is not an object (originalDragTarget.parentElement)` otherwise).
  let items = $state<Array<{ id: string; task: Task }>>([]);
  // Re-seed from the `tasks` prop whenever it changes — unless a drag is in
  // flight. We can't use `$derived` directly because we MUST keep the same
  // array identity through the drag (svelte-dnd-action stores DOM refs by
  // index against the array we hand it).
  $effect(() => {
    if (dragging) return;
    items = tasks.map((t) => ({ id: t.id, task: t }));
  });

  function handleConsider(e: CustomEvent<DndEvent<{ id: string; task: Task }>>) {
    dragging = true;
    items = e.detail.items;
    // svelte-dnd-action fires `consider` on every zone the pointer crosses,
    // not just source + destination. Without this trigger check, columns the
    // drag merely passed over would never see a clearing event and stay
    // highlighted forever (finalize only fires on source + destination).
    const trigger = e.detail.info?.trigger;
    if (trigger === TRIGGERS.DRAGGED_ENTERED || trigger === TRIGGERS.DRAGGED_OVER_INDEX) {
      dragOver = true;
    } else if (trigger === TRIGGERS.DRAGGED_LEFT || trigger === TRIGGERS.DRAGGED_LEFT_ALL) {
      dragOver = false;
    }
  }

  function handleFinalize(e: CustomEvent<DndEvent<{ id: string; task: Task }>>) {
    dragOver = false;
    const next = e.detail.items;
    items = next;
    dragging = false;
    if (status === 'archive') return; // can't drop INTO archive via DnD
    if (!onDrop) return;
    const incoming = next.find((n) => !tasks.some((t) => t.id === n.id));
    if (incoming) {
      onDrop(incoming.id, status as DropTarget);
    }
  }
</script>

<article class="col" class:drag-over={dragOver}>
  <header class="col-head">
    <h2>{title}</h2>
    {#if wipLimit !== undefined && wipLimit > 0}
      <span class="count" class:over-limit={overLimit} title="WIP limit {tasks.length}/{wipLimit}">
        {tasks.length}/{wipLimit}{overLimit ? ' ⚠' : ''}
      </span>
    {:else}
      <span class="count">{tasks.length}</span>
    {/if}
  </header>
  {#if draggable && status !== 'archive'}
    <ul
      use:dndzone={{ items, type: 'task', flipDurationMs: 150, dropTargetStyle: {} }}
      onconsider={handleConsider}
      onfinalize={handleFinalize}>
      {#each items as item (item.id)}
        <li>
          <Card task={item.task} {onSelect} />
        </li>
      {/each}
    </ul>
    {#if tasks.length === 0}
      <p class="empty">No tasks</p>
    {/if}
  {:else}
    {#if tasks.length === 0}
      <p class="empty">No tasks</p>
    {:else}
      <ul class="static">
        {#each tasks as t (t.id)}
          <li><Card task={t} {onSelect} /></li>
        {/each}
      </ul>
    {/if}
  {/if}
</article>

<style>
  .col {
    background: var(--bg-elev);
    border-radius: var(--radius);
    border: 1px solid rgba(255, 255, 255, 0.05);
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
    overflow: hidden;
    transition: border-color 120ms ease, background 120ms ease;
  }
  .col.drag-over {
    border-color: var(--accent);
    background: rgba(74, 141, 248, 0.06);
  }
  .col-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 12px 8px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }
  .col-head h2 {
    margin: 0;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--fg-dim);
    font-weight: 600;
  }
  .count {
    background: rgba(255, 255, 255, 0.06);
    color: var(--fg-dim);
    border-radius: 999px;
    padding: 1px 7px;
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }
  .count.over-limit {
    background: rgba(220, 80, 80, 0.18);
    color: #ff9a9a;
  }
  ul {
    list-style: none;
    padding: 8px;
    margin: 0;
    overflow-y: auto;
    overflow-x: hidden;
    min-height: 0;
    min-width: 0;
    flex: 1;
  }
  li { margin: 0; }
  .empty {
    color: var(--fg-dim);
    text-align: center;
    margin: 16px 0;
    font-size: 11px;
    font-style: italic;
  }
</style>
