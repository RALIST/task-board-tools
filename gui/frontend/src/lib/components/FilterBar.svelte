<script lang="ts">
  import type { BoardSnapshot } from '$lib/api';
  import { clearFilter, filter, type BoardFilter } from '$lib/stores/filter';
  import { observedEpics, observedTags, observedValues } from '$lib/filtering';

  interface Props {
    snapshot: BoardSnapshot;
    onShowArchiveChange?: (show: boolean) => void;
  }
  let { snapshot, onShowArchiveChange }: Props = $props();

  let types = $derived(observedValues(snapshot, 'type'));
  let priorities = $derived(observedValues(snapshot, 'priority'));
  let modules = $derived(observedValues(snapshot, 'module'));
  let agents = $derived(observedValues(snapshot, 'agent'));
  let tags = $derived(observedTags(snapshot));
  let epics = $derived(observedEpics(snapshot));

  // Mirror the store into local state so the bind:value controls work
  // cleanly. The $effect auto-unsubscribes when the component unmounts.
  let f = $state<BoardFilter>({ ...$filter });
  $effect(() => {
    f = { ...$filter };
  });

  function commit() {
    filter.set({ ...f });
  }

  function toggleMember(arr: string[], v: string): string[] {
    return arr.includes(v) ? arr.filter((x) => x !== v) : [...arr, v];
  }

  function toggle(category: 'types' | 'priorities' | 'modules' | 'tags' | 'agents', v: string) {
    f = { ...f, [category]: toggleMember(f[category], v) };
    commit();
  }

  function clear() {
    clearFilter();
  }

  function onArchiveToggle(ev: Event) {
    const checked = (ev.currentTarget as HTMLInputElement).checked;
    f = { ...f, showArchive: checked };
    commit();
    onShowArchiveChange?.(checked);
  }

  function onSearchInput(ev: Event) {
    f = { ...f, search: (ev.currentTarget as HTMLInputElement).value };
    commit();
  }
</script>

<section class="filter" aria-label="Filters">
  <input
    class="search"
    type="search"
    placeholder="Search id or title…"
    value={f.search}
    oninput={onSearchInput} />

  {#if types.length > 1}
    <div class="group" aria-label="type">
      {#each types as t}
        <button class="chip" class:on={f.types.includes(t)} onclick={() => toggle('types', t)} type="button">{t}</button>
      {/each}
    </div>
  {/if}
  {#if priorities.length > 1}
    <div class="group" aria-label="priority">
      {#each priorities as p}
        <button class="chip pri" class:on={f.priorities.includes(p)} onclick={() => toggle('priorities', p)} type="button">{p}</button>
      {/each}
    </div>
  {/if}
  {#if modules.length > 0}
    <div class="group" aria-label="modules">
      {#each modules as m}
        <button class="chip mod" class:on={f.modules.includes(m)} onclick={() => toggle('modules', m)} type="button">{m}</button>
      {/each}
    </div>
  {/if}
  {#if tags.length > 0}
    <div class="group tags" aria-label="tags">
      {#each tags.slice(0, 16) as tg}
        <button class="chip tag" class:on={f.tags.includes(tg)} onclick={() => toggle('tags', tg)} type="button">{tg}</button>
      {/each}
    </div>
  {/if}
  {#if epics.length > 0}
    <select class="dropdown" aria-label="parent epic" bind:value={f.parentEpic} onchange={commit}>
      <option value="">(any epic)</option>
      {#each epics as e}
        <option value={e.id}>{e.id} {e.title.slice(0, 40)}</option>
      {/each}
    </select>
  {/if}
  {#if agents.length > 0}
    <div class="group" aria-label="agents">
      {#each agents as a}
        <button class="chip" class:on={f.agents.includes(a)} onclick={() => toggle('agents', a)} type="button">{a}</button>
      {/each}
    </div>
  {/if}

  <label class="check">
    <input type="checkbox" checked={f.showArchive} onchange={onArchiveToggle} />
    <span>Show archived</span>
  </label>

  <button class="clear" onclick={clear} type="button">Clear</button>
</section>

<style>
  .filter {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 10px;
    padding: 8px 14px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    background: var(--bg);
  }
  .search {
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    border-radius: 5px;
    padding: 4px 8px;
    font: inherit;
    font-size: 12px;
    min-width: 180px;
  }
  .group { display: inline-flex; flex-wrap: wrap; gap: 4px; }
  .chip {
    background: rgba(255, 255, 255, 0.05);
    color: var(--fg-dim);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 999px;
    padding: 2px 9px;
    font-size: 11px;
    cursor: pointer;
    font: inherit;
  }
  .chip:hover { background: rgba(255, 255, 255, 0.1); color: var(--fg); }
  .chip.on { background: var(--accent); border-color: var(--accent); color: white; }
  .chip.pri.on { background: var(--p1); color: black; border-color: var(--p1); }
  .chip.tag { font-family: ui-monospace, monospace; }
  .dropdown {
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    border-radius: 5px;
    padding: 3px 6px;
    font-size: 12px;
    max-width: 220px;
  }
  .chip.mod { font-family: ui-monospace, monospace; }
  .check { display: inline-flex; gap: 4px; align-items: center; font-size: 12px; color: var(--fg-dim); }
  .clear {
    margin-left: auto;
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: var(--fg-dim);
    border-radius: 5px;
    padding: 3px 10px;
    font-size: 11px;
    cursor: pointer;
    font: inherit;
  }
  .clear:hover { color: var(--fg); }
</style>
