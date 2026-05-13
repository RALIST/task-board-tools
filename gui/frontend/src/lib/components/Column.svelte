<script lang="ts">
  import type { Task } from '$lib/api';
  import Card from './Card.svelte';

  interface Props {
    title: string;
    tasks: Task[];
    onSelect?: (id: string) => void;
  }

  let { title, tasks, onSelect }: Props = $props();
</script>

<article class="col">
  <header class="col-head">
    <h2>{title}</h2>
    <span class="count">{tasks.length}</span>
  </header>
  {#if tasks.length === 0}
    <p class="empty">No tasks</p>
  {:else}
    <ul>
      {#each tasks as t (t.id)}
        <li>
          <Card task={t} {onSelect} />
        </li>
      {/each}
    </ul>
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
    overflow: hidden;
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
  ul {
    list-style: none;
    padding: 8px;
    margin: 0;
    overflow-y: auto;
    min-height: 0;
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
