<script lang="ts">
  import type { BoardSnapshot } from '$lib/api';
  import Column from './Column.svelte';

  interface Props {
    snapshot: BoardSnapshot;
    showArchive?: boolean;
    onSelect?: (id: string) => void;
    onDrop?: (taskId: string, target: 'backlog' | 'in-progress' | 'done') => void;
  }

  let { snapshot, showArchive = false, onSelect, onDrop }: Props = $props();
</script>

<section class="board" class:with-archive={showArchive} aria-label="kanban">
  <Column title="Backlog" status="backlog" tasks={snapshot.backlog} {onSelect} {onDrop} />
  <Column title="In progress" status="in-progress" tasks={snapshot.inProgress} {onSelect} {onDrop} />
  <Column title="Done" status="done" tasks={snapshot.done} {onSelect} {onDrop} />
  {#if showArchive}
    <Column title="Archive" status="archive" tasks={snapshot.archive ?? []} draggable={false} {onSelect} />
  {/if}
</section>

<style>
  .board {
    flex: 1;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 10px;
    padding: 12px;
    min-height: 0;
    min-width: 0;
  }
  .board.with-archive {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
  @media (max-width: 1023px) {
    .board, .board.with-archive {
      grid-template-columns: 1fr;
      overflow-y: auto;
    }
  }
</style>
