<script lang="ts">
  import type { BoardFilter } from '$lib/stores/filter';

  type Category =
    | 'types'
    | 'priorities'
    | 'modules'
    | 'sizes'
    | 'tags'
    | 'agents'
    | 'parentEpic';

  interface Chip {
    category: Category;
    value: string;
    extraClass: string;
    ariaLabel: string;
  }

  interface Props {
    filter: BoardFilter;
    onRemove: (category: Category, value: string) => void;
  }
  let { filter, onRemove }: Props = $props();

  let chips = $derived<Chip[]>(buildChips(filter));

  function buildChips(f: BoardFilter): Chip[] {
    const out: Chip[] = [];
    for (const v of f.types) out.push({ category: 'types', value: v, extraClass: '', ariaLabel: `Remove type filter ${v}` });
    for (const v of f.priorities) out.push({ category: 'priorities', value: v, extraClass: 'pri', ariaLabel: `Remove priority filter ${v}` });
    for (const v of f.modules) out.push({ category: 'modules', value: v, extraClass: 'mod', ariaLabel: `Remove module filter ${v}` });
    for (const v of f.sizes) out.push({ category: 'sizes', value: v, extraClass: '', ariaLabel: `Remove size filter ${v}` });
    for (const v of f.tags) out.push({ category: 'tags', value: v, extraClass: 'tag', ariaLabel: `Remove tag filter ${v}` });
    for (const v of f.agents) out.push({ category: 'agents', value: v, extraClass: '', ariaLabel: `Remove agent filter ${v}` });
    if (f.parentEpic !== '') out.push({ category: 'parentEpic', value: f.parentEpic, extraClass: '', ariaLabel: `Remove epic filter ${f.parentEpic}` });
    return out;
  }

  function remove(chip: Chip) {
    onRemove(chip.category, chip.value);
  }
</script>

{#if chips.length > 0}
  <section class="af" aria-label="Active filters">
    <span class="af-label">Active:</span>
    {#each chips as chip (chip.category + ':' + chip.value)}
      <button
        class={`af-chip ${chip.extraClass}`.trim()}
        type="button"
        aria-label={chip.ariaLabel}
        onclick={() => remove(chip)}>
        {chip.value} <span aria-hidden="true">×</span>
      </button>
    {/each}
  </section>
{/if}

<style>
  .af {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    padding: 6px 14px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    background: var(--bg);
  }
  .af-label {
    font-size: 11px;
    color: var(--fg-dim);
    margin-right: 4px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .af-chip {
    font: inherit;
    background: var(--accent);
    color: white;
    border: 1px solid var(--accent);
    border-radius: 999px;
    padding: 2px 9px;
    font-size: 11px;
    line-height: 1.35;
    cursor: pointer;
    white-space: nowrap;
  }
  .af-chip:hover { filter: brightness(1.15); }
  .af-chip:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .af-chip.pri { background: var(--p1); color: black; border-color: var(--p1); }
  .af-chip.tag { font-family: ui-monospace, monospace; }
  .af-chip.mod { font-family: ui-monospace, monospace; }
</style>
