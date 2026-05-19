<script lang="ts">
  import type { BoardSnapshot } from '$lib/api';
  import Column from './Column.svelte';

  export type DropTarget = 'backlog' | 'ready' | 'in-progress' | 'code-review' | 'done';

  interface Props {
    snapshot: BoardSnapshot;
    showArchive?: boolean;
    // Wails generates Record<string, number | undefined> for Go's
    // map[string]int when the JSON omits zero/missing entries, so the
    // prop type has to allow undefined values.
    wipLimits?: Record<string, number | undefined>;
    onSelect?: (id: string) => void;
    onDrop?: (taskId: string, target: DropTarget) => void;
  }

  let { snapshot, showArchive = false, wipLimits = {}, onSelect, onDrop }: Props = $props();
</script>

<section class="board" class:with-archive={showArchive} aria-label="kanban">
  <Column title="Backlog" status="backlog" tasks={snapshot.backlog} {onSelect} {onDrop} />
  <Column title="Ready" status="ready" tasks={snapshot.ready ?? []} wipLimit={wipLimits['ready']} {onSelect} {onDrop} />
  <Column title="In progress" status="in-progress" tasks={snapshot.inProgress} wipLimit={wipLimits['in-progress']} {onSelect} {onDrop} />
  <Column title="Code review" status="code-review" tasks={snapshot.codeReview ?? []} wipLimit={wipLimits['code-review']} {onSelect} {onDrop} />
  <Column title="Done" status="done" tasks={snapshot.done} {onSelect} {onDrop} />
  {#if showArchive}
    <Column title="Archive" status="archive" tasks={snapshot.archive ?? []} draggable={false} {onSelect} />
  {/if}
</section>

<style>
  .board {
    flex: 1;
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 10px;
    padding: 12px;
    min-height: 0;
    min-width: 0;
  }
  .board.with-archive {
    grid-template-columns: repeat(6, minmax(0, 1fr));
  }
  @media (max-width: 1023px) {
    .board, .board.with-archive {
      grid-template-columns: 1fr;
      overflow-y: auto;
    }
  }
</style>
