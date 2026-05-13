<script lang="ts">
  import type { BoardSnapshot } from '$lib/api';
  import Column from './Column.svelte';

  interface Props {
    snapshot: BoardSnapshot;
    onSelect?: (id: string) => void;
  }

  let { snapshot, onSelect }: Props = $props();
</script>

<section class="board" aria-label="kanban">
  <Column title="Backlog" tasks={snapshot.backlog} {onSelect} />
  <Column title="In progress" tasks={snapshot.inProgress} {onSelect} />
  <Column title="Done" tasks={snapshot.done} {onSelect} />
</section>

<style>
  .board {
    flex: 1;
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
    padding: 12px;
    min-height: 0;
    /* Below 1024px collapse to a single column for narrow windows. The
     * minimum window width is 720px but users can resize. */
  }
  @media (max-width: 1023px) {
    .board {
      grid-template-columns: 1fr;
      overflow-y: auto;
    }
  }
</style>
