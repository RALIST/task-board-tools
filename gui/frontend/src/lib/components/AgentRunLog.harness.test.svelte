<script lang="ts">
  import AgentRunLog from './AgentRunLog.svelte';

  let runId = $state<string | null>(null);
  let taskId = $state<string | null>(null);

  export function setProps(newRunId: string | null, newTaskId: string | null) {
    runId = newRunId;
    taskId = newTaskId;
  }
</script>

<AgentRunLog {runId} {taskId} />
