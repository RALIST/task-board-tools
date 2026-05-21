<script lang="ts">
  import { untrack } from 'svelte';
  import type { Task } from '$lib/api';
  import Card from './Card.svelte';
  import { dndzone, TRIGGERS, type DndEvent } from 'svelte-dnd-action';
  import {
    shouldVirtualizeColumn,
    virtualTaskRange,
    type VirtualTaskRange,
  } from '$lib/columnVisibility';

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
  let dragStartupPending = $state(false);
  let scrollTop = $state(0);
  let viewportHeight = $state(0);
  let scrollList = $state<HTMLUListElement | null>(null);
  let lastTasks = $state<Task[] | null>(null);
  let pendingDragItem: HTMLElement | null = null;
  let pendingDragZone: HTMLUListElement | null = null;
  let virtualized = $derived(shouldVirtualizeColumn(status, tasks.length));
  let virtualRange = $derived.by<VirtualTaskRange>(() => {
    if (!virtualized) return { start: 0, end: tasks.length, paddingTop: 0, paddingBottom: 0 };
    return virtualTaskRange(tasks.length, scrollTop, viewportHeight);
  });
  let visibleTasks = $derived(tasks.slice(virtualRange.start, virtualRange.end));
  let dndEnabled = $derived(draggable && status !== 'archive' && !virtualized);
  let dndGuarded = $derived(draggable && status !== 'archive' && virtualized);

  $effect(() => {
    const previousTasks = untrack(() => lastTasks);
    if (tasks !== previousTasks) {
      scrollTop = 0;
      viewportHeight = scrollList?.clientHeight ?? 0;
      if (scrollList) scrollList.scrollTop = 0;
      lastTasks = tasks;
    }
  });

  // svelte-dnd-action requires stable DOM from pointer-down through finalize.
  // Freeze the handed-off items while drag startup is pending too; the library
  // dereferences the source item's parent before it emits the first consider.
  let items = $state<Array<{ id: string; task: Task }>>([]);
  // Re-seed from the `tasks` prop whenever it changes — unless a drag is in
  // flight. We can't use `$derived` directly because we MUST keep the same
  // array identity through the drag (svelte-dnd-action stores DOM refs by
  // index against the array we hand it).
  $effect(() => {
    if (dragging || dragStartupPending) return;
    if (sameItems(untrack(() => items), visibleTasks)) return;
    items = visibleTasks.map((t) => ({ id: t.id, task: t }));
  });

  function sameItems(current: Array<{ id: string; task: Task }>, next: Task[]): boolean {
    if (current.length !== next.length) return false;
    return current.every((item, i) => item.id === next[i]?.id && item.task === next[i]);
  }

  function handleConsider(e: CustomEvent<DndEvent<{ id: string; task: Task }>>) {
    clearDragStartupPending();
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
    clearDragStartupPending();
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

  function handleScroll(e: Event) {
    const node = e.currentTarget as HTMLUListElement;
    scrollTop = node.scrollTop;
    viewportHeight = node.clientHeight;
  }

  function dragStartupGuard(node: HTMLUListElement) {
    const handlePointerDown = (event: MouseEvent | TouchEvent) => {
      beginDragStartupPending(event, node);
    };
    node.addEventListener('mousedown', handlePointerDown, true);
    node.addEventListener('touchstart', handlePointerDown, true);
    return {
      destroy: () => {
        node.removeEventListener('mousedown', handlePointerDown, true);
        node.removeEventListener('touchstart', handlePointerDown, true);
        if (pendingDragZone === node) clearDragStartupPending();
      },
    };
  }

  function beginDragStartupPending(event: MouseEvent | TouchEvent, zone: HTMLUListElement) {
    if ('button' in event && event.button !== 0) return;
    const item = draggableItemFromEvent(event, zone);
    if (!item) return;
    dragStartupPending = true;
    pendingDragItem = item;
    pendingDragZone = zone;
    addDragStartupWindowGuards();
  }

  function draggableItemFromEvent(event: Event, zone: HTMLUListElement): HTMLElement | null {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return null;
    const item = target.closest('li');
    if (!(item instanceof HTMLElement) || item.parentElement !== zone) return null;
    if (target !== item && ('value' in target || target.isContentEditable)) return null;
    return item;
  }

  function addDragStartupWindowGuards() {
    window.removeEventListener('mousemove', cancelDetachedDragStartup, true);
    window.removeEventListener('touchmove', cancelDetachedDragStartup, true);
    window.removeEventListener('mouseup', clearDragStartupPending, true);
    window.removeEventListener('touchend', clearDragStartupPending, true);
    window.removeEventListener('touchcancel', clearDragStartupPending, true);
    window.addEventListener('mousemove', cancelDetachedDragStartup, true);
    window.addEventListener('touchmove', cancelDetachedDragStartup, true);
    window.addEventListener('mouseup', clearDragStartupPending, true);
    window.addEventListener('touchend', clearDragStartupPending, true);
    window.addEventListener('touchcancel', clearDragStartupPending, true);
  }

  function clearDragStartupPending() {
    if (!dragStartupPending && !pendingDragItem && !pendingDragZone) return;
    dragStartupPending = false;
    pendingDragItem = null;
    pendingDragZone = null;
    window.removeEventListener('mousemove', cancelDetachedDragStartup, true);
    window.removeEventListener('touchmove', cancelDetachedDragStartup, true);
    window.removeEventListener('mouseup', clearDragStartupPending, true);
    window.removeEventListener('touchend', clearDragStartupPending, true);
    window.removeEventListener('touchcancel', clearDragStartupPending, true);
  }

  function cancelDetachedDragStartup(event: Event) {
    if (!dragStartupPending || !pendingDragItem || !pendingDragZone) return;
    if (pendingDragItem.isConnected && pendingDragItem.parentElement === pendingDragZone) return;
    event.preventDefault();
    event.stopImmediatePropagation();
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
  {#if dndEnabled}
    <ul
      bind:this={scrollList}
      use:dragStartupGuard
      use:dndzone={{ items, type: 'task', flipDurationMs: 150, dropTargetStyle: {} }}
      onscroll={handleScroll}
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
      <ul
        class="static"
        class:virtualized
        bind:this={scrollList}
        onscroll={handleScroll}
        aria-label={`${title} tasks`}>
        {#if virtualized && virtualRange.paddingTop > 0}
          <li
            class="virtual-spacer"
            style={`height: ${virtualRange.paddingTop}px`}
            aria-hidden="true"></li>
        {/if}
        {#each visibleTasks as t (t.id)}
          <li><Card task={t} {onSelect} /></li>
        {/each}
        {#if virtualized && virtualRange.paddingBottom > 0}
          <li
            class="virtual-spacer"
            style={`height: ${virtualRange.paddingBottom}px`}
            aria-hidden="true"></li>
        {/if}
      </ul>
      {#if dndGuarded}
        <p class="virtual-dnd-note">Drag disabled while this large column uses lazy rendering.</p>
      {/if}
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
  .virtualized {
    scrollbar-gutter: stable;
  }
  .virtual-spacer {
    margin: 0;
    padding: 0;
    pointer-events: none;
  }
  .empty {
    color: var(--fg-dim);
    text-align: center;
    margin: 16px 0;
    font-size: 11px;
    font-style: italic;
  }
  .virtual-dnd-note {
    border: 0;
    border-top: 1px solid rgba(255, 255, 255, 0.06);
    background: rgba(255, 255, 255, 0.035);
    color: var(--fg-muted);
    margin: 0;
    font-size: 12px;
    padding: 8px 10px;
    text-align: center;
  }
</style>
