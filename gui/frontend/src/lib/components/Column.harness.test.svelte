<script lang="ts">
  import type { Task } from '$lib/api';
  import Column from './Column.svelte';

  type ColumnStatus = 'backlog' | 'ready' | 'in-progress' | 'code-review' | 'done' | 'archive';

  interface Props {
    initialTasks: Task[];
    status?: ColumnStatus;
    title?: string;
    draggable?: boolean;
  }

  let { initialTasks, status: providedStatus, title = 'Ready', draggable = true }: Props = $props();
  let status = $derived<ColumnStatus>(providedStatus ?? 'ready');
  let currentTasks = $state<Task[]>([]);
  let seeded = false;

  $effect(() => {
    if (!seeded) {
      currentTasks = initialTasks;
      seeded = true;
    }
  });

  export function setTasks(next: Task[]) {
    currentTasks = next;
  }
</script>

<Column {title} {status} tasks={currentTasks} {draggable} />
