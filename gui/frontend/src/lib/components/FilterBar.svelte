<script lang="ts">
  import type { BoardSnapshot } from '$lib/api';
  import {
    clearFilter,
    filter,
    focusFilterBarToken,
    type BoardFilter,
  } from '$lib/stores/filter';
  import { observedEpics, observedTags, observedValues } from '$lib/filtering';
  import FilterDropdown from './FilterDropdown.svelte';
  import ActiveFilters from './ActiveFilters.svelte';
  import { preferencesStore } from '$lib/stores/preferences';
  import { pushToast } from '$lib/stores/toast';
  import {
    autoImplementFilterEquals,
    boardFilterToAutoImplement,
    isBoardFilterActive,
  } from '$lib/autoImplementFilter';

  interface Props {
    snapshot: BoardSnapshot;
    onShowArchiveChange?: (show: boolean) => void;
  }
  let { snapshot, onShowArchiveChange }: Props = $props();

  let searchInput: HTMLInputElement | null = $state(null);

  let types = $derived(observedValues(snapshot, 'type'));
  let priorities = $derived(observedValues(snapshot, 'priority'));
  let modules = $derived(observedValues(snapshot, 'module'));
  let agents = $derived(observedValues(snapshot, 'agent'));
  let tags = $derived(observedTags(snapshot));
  let epicTasks = $derived(observedEpics(snapshot));
  let epicOptions = $derived(epicTasks.map((e) => e.id));
  // Size is a closed enum the CLI accepts; surface every value so the
  // Save-as-auto-implement-query button can produce the original AC
  // fixture `bug + S + gui` end-to-end (TB-288 acceptance criterion).
  const sizeOptions = ['S', 'M', 'L', 'XL'];

  let f: BoardFilter = $derived({ ...$filter });

  let saveable = $derived(isBoardFilterActive(f));
  let saved = $derived(
    autoImplementFilterEquals(
      boardFilterToAutoImplement(f),
      $preferencesStore.autoImplementQuery,
    ),
  );

  function commit() {
    filter.set({ ...f });
  }

  function toggleMember(arr: string[], v: string): string[] {
    return arr.includes(v) ? arr.filter((x) => x !== v) : [...arr, v];
  }

  function toggle(
    category: 'types' | 'priorities' | 'modules' | 'sizes' | 'tags' | 'agents',
    v: string,
  ) {
    f = { ...f, [category]: toggleMember(f[category], v) };
    commit();
  }

  function setEpic(v: string) {
    f = { ...f, parentEpic: v === f.parentEpic ? '' : v };
    commit();
  }

  function removeFilter(
    category: 'types' | 'priorities' | 'modules' | 'sizes' | 'tags' | 'agents' | 'parentEpic',
    value: string,
  ) {
    if (category === 'parentEpic') {
      f = { ...f, parentEpic: '' };
    } else {
      f = { ...f, [category]: f[category].filter((x) => x !== value) };
    }
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

  async function saveAsAutoImplement() {
    if (!saveable) return;
    try {
      await preferencesStore.setAutoImplementQuery(boardFilterToAutoImplement(f));
      pushToast('Saved as auto-implement query');
    } catch {
      // optimisticWrite already surfaced a toast; no further action needed.
    }
  }

  // SettingsPanel's "Edit in board filter" button bumps focusFilterBarToken;
  // refocus the search input so the user lands cursor-ready.
  $effect(() => {
    const token = $focusFilterBarToken;
    if (token === 0) return;
    searchInput?.focus();
  });
</script>

<section class="filter" aria-label="Filters">
  <input
    bind:this={searchInput}
    class="search"
    type="search"
    placeholder="Search id or title…"
    value={f.search}
    oninput={onSearchInput} />

  {#if types.length > 1}
    <FilterDropdown
      label="Type"
      options={types}
      selected={f.types}
      onToggle={(v) => toggle('types', v)} />
  {/if}
  {#if priorities.length > 1}
    <FilterDropdown
      label="Priority"
      options={priorities}
      selected={f.priorities}
      onToggle={(v) => toggle('priorities', v)} />
  {/if}
  {#if modules.length > 1}
    <FilterDropdown
      label="Module"
      options={modules}
      selected={f.modules}
      onToggle={(v) => toggle('modules', v)} />
  {/if}
  <FilterDropdown
    label="Size"
    options={sizeOptions}
    selected={f.sizes}
    onToggle={(v) => toggle('sizes', v)} />
  {#if tags.length > 1}
    <FilterDropdown
      label="Tags"
      options={tags}
      selected={f.tags}
      onToggle={(v) => toggle('tags', v)} />
  {/if}
  {#if agents.length > 1}
    <FilterDropdown
      label="Agent"
      options={agents}
      selected={f.agents}
      onToggle={(v) => toggle('agents', v)} />
  {/if}
  {#if epicOptions.length > 0}
    <FilterDropdown
      label="Epic"
      single
      options={epicOptions}
      selected={f.parentEpic === '' ? [] : [f.parentEpic]}
      onToggle={(v) => setEpic(v)} />
  {/if}

  <label class="check">
    <input type="checkbox" checked={f.showArchive} onchange={onArchiveToggle} />
    <span>Show archived</span>
  </label>

  <button
    class="save-auto-implement"
    class:saved
    data-testid="save-as-auto-implement"
    disabled={!saveable}
    title={saved
      ? 'Auto-implement query matches the current filter'
      : 'Persist this filter as the auto-implement query'}
    onclick={saveAsAutoImplement}
    type="button">
    {saved ? 'Saved' : 'Save as auto-implement'}
  </button>
  <button class="clear" onclick={clear} type="button">Clear</button>
</section>

<ActiveFilters filter={$filter} onRemove={removeFilter} />

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
  .check { display: inline-flex; gap: 4px; align-items: center; font-size: 12px; color: var(--fg-dim); }
  .save-auto-implement {
    font: inherit;
    margin-left: auto;
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: var(--fg-dim);
    border-radius: 5px;
    padding: 3px 10px;
    font-size: 11px;
    cursor: pointer;
  }
  .save-auto-implement:hover:not(:disabled) { color: var(--fg); }
  .save-auto-implement:disabled { opacity: 0.45; cursor: not-allowed; }
  .save-auto-implement.saved { color: var(--fg); border-color: rgba(120, 200, 120, 0.45); }
  .clear {
    font: inherit;
    background: transparent;
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: var(--fg-dim);
    border-radius: 5px;
    padding: 3px 10px;
    font-size: 11px;
    cursor: pointer;
  }
  .clear:hover { color: var(--fg); }
</style>
