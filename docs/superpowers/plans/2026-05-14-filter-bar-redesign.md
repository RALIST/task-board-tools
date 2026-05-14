# FilterBar Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the chip-flood FilterBar header with a compact row of per-category dropdown buttons plus a removable "Active filters" chips row that renders only when filters are set.

**Architecture:** Two new Svelte 5 components in `gui/frontend/src/lib/components/` — `FilterDropdown.svelte` (generalized popover with optional in-popover search and single-select mode) and `ActiveFilters.svelte` (row 2). `FilterBar.svelte` becomes a thin composition shell. The `BoardFilter` store, `filtering.ts` selectors, and IPC are untouched; `FILTER_BAR_INLINE_TAG_LIMIT` and `selectInlineTags` are removed.

**Tech Stack:** Svelte 5 (runes, `$state`/`$derived`/`$effect`/`$props`), TypeScript, Vitest + jsdom, `svelte-check`. Pre-existing patterns: alias `$lib` resolves to `gui/frontend/src/lib`; component tests use `mount`/`unmount` from `svelte` with `await tick()` between interactions; `filter` store from `$lib/stores/filter` carries the canonical state.

**Spec:** [docs/superpowers/specs/2026-05-14-filter-bar-redesign-design.md](../specs/2026-05-14-filter-bar-redesign-design.md)

---

## File Structure

**Create:**
- `gui/frontend/src/lib/components/FilterDropdown.svelte` — popover component (trigger button + popover menu with optional search input + single/multi-select modes).
- `gui/frontend/src/lib/components/FilterDropdown.test.ts` — Vitest suite for the dropdown.
- `gui/frontend/src/lib/components/ActiveFilters.svelte` — second-row component rendering removable chips when any filter is active.
- `gui/frontend/src/lib/components/ActiveFilters.test.ts` — Vitest suite for ActiveFilters.

**Modify:**
- `gui/frontend/src/lib/filtering.ts` — remove `FILTER_BAR_INLINE_TAG_LIMIT`, `selectInlineTags`, and the `InlineTagSelection` interface.
- `gui/frontend/src/lib/filtering.test.ts` — drop the two `selectInlineTags`-based tests; keep the others.
- `gui/frontend/src/lib/components/FilterBar.svelte` — full rewrite as a composition shell.
- `gui/frontend/src/lib/components/FilterBar.test.ts` — replace the five tag-overflow tests with five new tests covering the new layout.
- `board/backlog/TB-92.md` — append a Log line marking the task superseded.

Each new component owns one responsibility (popover for `FilterDropdown`, chip row for `ActiveFilters`); `FilterBar` orchestrates them. This keeps each file readable in a single context window and individually unit-testable, addressing the spec's "Design for isolation and clarity" principle.

---

### Task 1: Create the failing FilterDropdown unit test

**Files:**
- Create: `gui/frontend/src/lib/components/FilterDropdown.test.ts`

This is the first task in TDD. Write the full test suite for `FilterDropdown` before any implementation, so the failure shows the component does not yet exist.

- [ ] **Step 1: Write the failing test file**

Create `gui/frontend/src/lib/components/FilterDropdown.test.ts` with:

```ts
import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import FilterDropdown from './FilterDropdown.svelte';

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
});

afterEach(async () => {
  if (component) await unmount(component);
  component = null;
  document.body.innerHTML = '';
});

function trigger(): HTMLButtonElement {
  const t = document.querySelector<HTMLButtonElement>('.fd-trigger');
  if (!t) throw new Error('trigger not found');
  return t;
}

function options(): HTMLButtonElement[] {
  return [...document.querySelectorAll<HTMLButtonElement>('.fd-option')];
}

describe('FilterDropdown', () => {
  it('renders the bare label when nothing is selected', async () => {
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Type', options: ['bug', 'feature'], selected: [], onToggle: () => {} },
    });
    await tick();
    expect(trigger().textContent?.trim()).toBe('Type');
  });

  it('renders a count badge in the label when selections exist', async () => {
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Type', options: ['bug', 'feature', 'spike'], selected: ['bug', 'feature'], onToggle: () => {} },
    });
    await tick();
    expect(trigger().textContent?.replace(/\s+/g, ' ').trim()).toBe('Type (2)');
  });

  it('opens the popover on click and lists every option', async () => {
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Type', options: ['bug', 'feature', 'spike'], selected: [], onToggle: () => {} },
    });
    await tick();
    expect(document.querySelector('.fd-menu')).toBeNull();
    trigger().click();
    await tick();
    expect(document.querySelector('.fd-menu')).not.toBeNull();
    expect(options().map((b) => b.textContent?.trim())).toEqual(['bug', 'feature', 'spike']);
  });

  it('fires onToggle with the clicked value and keeps the popover open in multi-select mode', async () => {
    const onToggle = vi.fn();
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Type', options: ['bug', 'feature'], selected: [], onToggle },
    });
    await tick();
    trigger().click();
    await tick();
    options()[0].click();
    await tick();
    expect(onToggle).toHaveBeenCalledWith('bug');
    expect(document.querySelector('.fd-menu')).not.toBeNull();
  });

  it('closes the popover after picking an option in single-select mode', async () => {
    const onToggle = vi.fn();
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Epic', options: ['TB-1', 'TB-2'], selected: [], onToggle, single: true },
    });
    await tick();
    trigger().click();
    await tick();
    options()[1].click(); // skip the leading "(any)" option, pick TB-1; index 0 is "(any)"
    await tick();
    expect(onToggle).toHaveBeenCalledWith('TB-1');
    expect(document.querySelector('.fd-menu')).toBeNull();
  });

  it('exposes an "(any)" leading option in single-select mode that clears selection', async () => {
    const onToggle = vi.fn();
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Epic', options: ['TB-1'], selected: ['TB-1'], onToggle, single: true },
    });
    await tick();
    trigger().click();
    await tick();
    const opts = options();
    expect(opts[0].textContent?.trim()).toBe('(any)');
    opts[0].click();
    await tick();
    expect(onToggle).toHaveBeenCalledWith('');
    expect(document.querySelector('.fd-menu')).toBeNull();
  });

  it('renders an in-popover search input when options.length > 10 and filters options as you type', async () => {
    const opts = Array.from({ length: 11 }, (_, i) => `mod-${String(i).padStart(2, '0')}`);
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Module', options: opts, selected: [], onToggle: () => {} },
    });
    await tick();
    trigger().click();
    await tick();
    const search = document.querySelector<HTMLInputElement>('.fd-search');
    expect(search).not.toBeNull();
    search!.value = '01';
    search!.dispatchEvent(new Event('input', { bubbles: true }));
    await tick();
    expect(options().map((b) => b.textContent?.trim())).toEqual(['mod-01']);
  });

  it('hides the search input when options.length <= 10', async () => {
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Type', options: ['bug', 'feature'], selected: [], onToggle: () => {} },
    });
    await tick();
    trigger().click();
    await tick();
    expect(document.querySelector('.fd-search')).toBeNull();
  });

  it('closes on outside pointerdown and on Escape', async () => {
    const outside = document.createElement('button');
    outside.textContent = 'outside';
    document.body.append(outside);
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Type', options: ['bug', 'feature'], selected: [], onToggle: () => {} },
    });
    await tick();
    trigger().click();
    await tick();
    expect(document.querySelector('.fd-menu')).not.toBeNull();
    outside.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    await tick();
    expect(document.querySelector('.fd-menu')).toBeNull();

    trigger().click();
    await tick();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await tick();
    expect(document.querySelector('.fd-menu')).toBeNull();
  });

  it('marks selected options with aria-checked=true', async () => {
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Type', options: ['bug', 'feature'], selected: ['bug'], onToggle: () => {} },
    });
    await tick();
    trigger().click();
    await tick();
    const opts = options();
    expect(opts[0].getAttribute('aria-checked')).toBe('true');
    expect(opts[1].getAttribute('aria-checked')).toBe('false');
  });
});
```

- [ ] **Step 2: Run the suite and verify it fails because the component does not exist**

```bash
cd gui/frontend && npx vitest run src/lib/components/FilterDropdown.test.ts
```

Expected output: the test file fails to resolve `./FilterDropdown.svelte` with an error similar to `Failed to load url ./FilterDropdown.svelte`. **Do not commit yet** — the tests fail at module-resolution time, not assertion time.

---

### Task 2: Implement FilterDropdown.svelte

**Files:**
- Create: `gui/frontend/src/lib/components/FilterDropdown.svelte`
- Test: `gui/frontend/src/lib/components/FilterDropdown.test.ts` (already exists from Task 1)

- [ ] **Step 1: Create `FilterDropdown.svelte` with the full implementation**

```svelte
<script lang="ts">
  import { tick } from 'svelte';

  interface Props {
    label: string;
    options: string[];
    selected: string[];
    onToggle: (value: string) => void;
    onClear?: () => void;
    single?: boolean;
    nullLabel?: string;
  }
  let { label, options, selected, onToggle, onClear, single = false, nullLabel = '(any)' }: Props = $props();

  const SEARCH_THRESHOLD = 10;

  let open = $state(false);
  let query = $state('');
  let rootEl: HTMLElement | null = $state(null);

  let showSearch = $derived(options.length > SEARCH_THRESHOLD);
  let filteredOptions = $derived(
    query.trim() === ''
      ? options
      : options.filter((o) => o.toLowerCase().includes(query.trim().toLowerCase())),
  );

  let triggerText = $derived(
    single
      ? selected.length > 0 ? `${label}: ${selected[0]}` : label
      : selected.length > 0 ? `${label} (${selected.length})` : label,
  );

  $effect(() => {
    if (!open) return;

    function onDocumentPointerDown(event: PointerEvent) {
      if (rootEl?.contains(event.target as Node)) return;
      open = false;
    }
    function onDocumentKeydown(event: KeyboardEvent) {
      if (event.key !== 'Escape') return;
      open = false;
      focusTrigger();
    }
    document.addEventListener('pointerdown', onDocumentPointerDown, true);
    document.addEventListener('keydown', onDocumentKeydown);
    return () => {
      document.removeEventListener('pointerdown', onDocumentPointerDown, true);
      document.removeEventListener('keydown', onDocumentKeydown);
    };
  });

  async function openMenu(focusIndex: number | 'search' = 'search') {
    open = true;
    await tick();
    if (focusIndex === 'search' && showSearch) {
      rootEl?.querySelector<HTMLInputElement>('.fd-search')?.focus();
    } else if (typeof focusIndex === 'number') {
      focusOption(focusIndex);
    } else {
      focusOption(0);
    }
  }

  function toggleMenu() {
    if (open) {
      open = false;
      query = '';
    } else {
      void openMenu();
    }
  }

  function onTriggerKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown') {
      event.preventDefault();
      void openMenu('search');
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      void openMenu(filteredOptions.length - 1);
    } else if (event.key === 'Escape') {
      open = false;
    }
  }

  function onMenuKeydown(event: KeyboardEvent) {
    const opts = optionButtons();
    const current = opts.indexOf(document.activeElement as HTMLButtonElement);
    let next = current;
    switch (event.key) {
      case 'ArrowDown':
        next = current < opts.length - 1 ? current + 1 : 0;
        break;
      case 'ArrowUp':
        next = current > 0 ? current - 1 : opts.length - 1;
        break;
      case 'Home':
        next = 0;
        break;
      case 'End':
        next = opts.length - 1;
        break;
      case 'Escape':
        open = false;
        focusTrigger();
        event.preventDefault();
        return;
      default:
        return;
    }
    event.preventDefault();
    opts[next]?.focus();
  }

  function onSearchKeydown(event: KeyboardEvent) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      focusOption(0);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      open = false;
      focusTrigger();
    }
  }

  function pick(value: string) {
    onToggle(value);
    if (single) {
      open = false;
      query = '';
    }
  }

  function pickNull() {
    onToggle('');
    open = false;
    query = '';
  }

  function clear() {
    onClear?.();
  }

  function optionButtons(): HTMLButtonElement[] {
    return [...(rootEl?.querySelectorAll<HTMLButtonElement>('.fd-option') ?? [])];
  }
  function focusOption(index: number) {
    optionButtons()[index]?.focus();
  }
  function focusTrigger() {
    rootEl?.querySelector<HTMLButtonElement>('.fd-trigger')?.focus();
  }
</script>

<div class="fd" bind:this={rootEl}>
  <button
    class="fd-trigger"
    type="button"
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label={`Filter by ${label}`}
    onclick={toggleMenu}
    onkeydown={onTriggerKeydown}>
    {triggerText} <span class="fd-caret" aria-hidden="true">▾</span>
  </button>
  {#if open}
    <div class="fd-menu" role="menu" aria-label={`${label} options`} tabindex="-1" onkeydown={onMenuKeydown}>
      {#if showSearch}
        <input
          class="fd-search"
          type="search"
          placeholder="Filter…"
          bind:value={query}
          onkeydown={onSearchKeydown} />
      {/if}
      {#if single}
        <button
          class="fd-option fd-null"
          role="menuitemradio"
          aria-checked={selected.length === 0}
          type="button"
          onclick={pickNull}>{nullLabel}</button>
      {/if}
      {#each filteredOptions as opt}
        <button
          class="fd-option"
          role={single ? 'menuitemradio' : 'menuitemcheckbox'}
          aria-checked={selected.includes(opt)}
          class:on={selected.includes(opt)}
          type="button"
          onclick={() => pick(opt)}>{opt}</button>
      {/each}
      {#if onClear && selected.length > 0}
        <button class="fd-clear" type="button" onclick={clear}>Clear</button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .fd { position: relative; display: inline-block; }
  .fd-trigger {
    background: rgba(255, 255, 255, 0.05);
    color: var(--fg-dim);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 999px;
    padding: 2px 9px;
    font: inherit;
    font-size: 11px;
    line-height: 1.35;
    cursor: pointer;
    white-space: nowrap;
  }
  .fd-trigger:hover { background: rgba(255, 255, 255, 0.1); color: var(--fg); }
  .fd-caret { margin-left: 2px; font-size: 9px; opacity: 0.7; }
  .fd-menu {
    position: absolute;
    z-index: 20;
    top: calc(100% + 6px);
    left: 0;
    display: grid;
    gap: 3px;
    min-width: 200px;
    max-height: 280px;
    overflow: auto;
    padding: 6px;
    background: var(--bg);
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 6px;
    box-shadow: 0 10px 24px rgba(0, 0, 0, 0.35);
  }
  .fd-search {
    background: rgba(0, 0, 0, 0.25);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--fg);
    border-radius: 5px;
    padding: 4px 8px;
    font: inherit;
    font-size: 12px;
    margin-bottom: 4px;
  }
  .fd-option {
    width: 100%;
    background: transparent;
    color: var(--fg-dim);
    border: 1px solid transparent;
    border-radius: 5px;
    padding: 4px 8px;
    font: inherit;
    font-size: 11px;
    font-family: ui-monospace, monospace;
    text-align: left;
    cursor: pointer;
  }
  .fd-option:hover,
  .fd-option:focus-visible {
    color: var(--fg);
    background: rgba(255, 255, 255, 0.08);
    outline: none;
  }
  .fd-option.on {
    background: var(--accent);
    border-color: var(--accent);
    color: white;
  }
  .fd-null { font-style: italic; font-family: inherit; }
  .fd-clear {
    margin-top: 4px;
    background: transparent;
    color: var(--fg-dim);
    border: 1px dashed rgba(255, 255, 255, 0.15);
    border-radius: 5px;
    padding: 3px 8px;
    font: inherit;
    font-size: 11px;
    cursor: pointer;
  }
  .fd-clear:hover { color: var(--fg); }
</style>
```

- [ ] **Step 2: Run the unit suite**

```bash
cd gui/frontend && npx vitest run src/lib/components/FilterDropdown.test.ts
```

Expected: all 10 tests pass.

- [ ] **Step 3: Run svelte-check on the new component**

```bash
cd gui/frontend && npx svelte-check --tsconfig ./tsconfig.json --no-tsconfig --workspace src/lib/components/FilterDropdown.svelte
```
(If the `--workspace` form errors on this svelte-check version, fall back to `npx svelte-check --tsconfig ./tsconfig.json` — the same check, broader scope, still acceptable.)
Expected: 0 errors, 0 warnings on `FilterDropdown.svelte`.

- [ ] **Step 4: Commit**

```bash
git add gui/frontend/src/lib/components/FilterDropdown.svelte gui/frontend/src/lib/components/FilterDropdown.test.ts
git commit -m "gui: add FilterDropdown component (popover trigger with optional search, multi/single-select)"
```

---

### Task 3: Create the failing ActiveFilters unit test

**Files:**
- Create: `gui/frontend/src/lib/components/ActiveFilters.test.ts`

- [ ] **Step 1: Write the failing test file**

```ts
import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { BoardFilter } from '$lib/stores/filter';
import { initialFilter } from '$lib/stores/filter';
import ActiveFilters from './ActiveFilters.svelte';

let component: ReturnType<typeof mount> | null = null;

beforeEach(() => {
  document.body.innerHTML = '';
});

afterEach(async () => {
  if (component) await unmount(component);
  component = null;
  document.body.innerHTML = '';
});

function chipTexts(): string[] {
  return [...document.querySelectorAll<HTMLButtonElement>('.af-chip')].map(
    (b) => b.textContent?.replace(/\s+/g, ' ').trim() ?? '',
  );
}

describe('ActiveFilters', () => {
  it('renders nothing when every filter category is empty', async () => {
    component = mount(ActiveFilters, {
      target: document.body,
      props: { filter: { ...initialFilter }, onRemove: () => {} },
    });
    await tick();
    expect(document.body.textContent?.trim()).toBe('');
  });

  it('renders one chip per selected value across categories with an "Active:" label', async () => {
    const f: BoardFilter = {
      ...initialFilter,
      types: ['bug'],
      priorities: ['P1', 'P0'],
      tags: ['cli'],
      modules: ['gui'],
    };
    component = mount(ActiveFilters, {
      target: document.body,
      props: { filter: f, onRemove: () => {} },
    });
    await tick();
    expect(document.querySelector('.af-label')?.textContent?.trim()).toBe('Active:');
    expect(chipTexts()).toEqual([
      'bug ×',
      'P1 ×',
      'P0 ×',
      'gui ×',
      'cli ×',
    ]);
  });

  it('renders the epic chip when parentEpic is set', async () => {
    const f: BoardFilter = { ...initialFilter, parentEpic: 'TB-5' };
    component = mount(ActiveFilters, {
      target: document.body,
      props: { filter: f, onRemove: () => {} },
    });
    await tick();
    expect(chipTexts()).toEqual(['TB-5 ×']);
  });

  it('fires onRemove with the correct category and value when a chip is clicked', async () => {
    const onRemove = vi.fn();
    const f: BoardFilter = { ...initialFilter, priorities: ['P1', 'P0'], tags: ['cli'] };
    component = mount(ActiveFilters, {
      target: document.body,
      props: { filter: f, onRemove },
    });
    await tick();
    const chips = [...document.querySelectorAll<HTMLButtonElement>('.af-chip')];
    chips[0].click(); // P1
    await tick();
    chips[2].click(); // cli
    await tick();
    expect(onRemove).toHaveBeenNthCalledWith(1, 'priorities', 'P1');
    expect(onRemove).toHaveBeenNthCalledWith(2, 'tags', 'cli');
  });

  it('applies the priority chip class for priority filters and the mod class for modules', async () => {
    const f: BoardFilter = { ...initialFilter, priorities: ['P1'], modules: ['gui'], tags: ['cli'] };
    component = mount(ActiveFilters, {
      target: document.body,
      props: { filter: f, onRemove: () => {} },
    });
    await tick();
    const chips = [...document.querySelectorAll<HTMLButtonElement>('.af-chip')];
    expect(chips[0].classList.contains('pri')).toBe(true);
    expect(chips[1].classList.contains('mod')).toBe(true);
    expect(chips[2].classList.contains('tag')).toBe(true);
  });
});
```

- [ ] **Step 2: Verify it fails for the right reason**

```bash
cd gui/frontend && npx vitest run src/lib/components/ActiveFilters.test.ts
```

Expected: module-resolution error against `./ActiveFilters.svelte`.

---

### Task 4: Implement ActiveFilters.svelte

**Files:**
- Create: `gui/frontend/src/lib/components/ActiveFilters.svelte`

- [ ] **Step 1: Create the component**

```svelte
<script lang="ts">
  import type { BoardFilter } from '$lib/stores/filter';

  type Category = 'types' | 'priorities' | 'modules' | 'tags' | 'agents' | 'parentEpic';

  interface Chip {
    category: Category;
    value: string;
    extraClass: string; // appended to .af-chip for category coloring
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
    {#each chips as chip}
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
    background: var(--accent);
    color: white;
    border: 1px solid var(--accent);
    border-radius: 999px;
    padding: 2px 9px;
    font-size: 11px;
    line-height: 1.35;
    cursor: pointer;
    font: inherit;
    white-space: nowrap;
  }
  .af-chip:hover { filter: brightness(1.15); }
  .af-chip.pri { background: var(--p1); color: black; border-color: var(--p1); }
  .af-chip.tag { font-family: ui-monospace, monospace; }
  .af-chip.mod { font-family: ui-monospace, monospace; }
</style>
```

- [ ] **Step 2: Run the test suite**

```bash
cd gui/frontend && npx vitest run src/lib/components/ActiveFilters.test.ts
```

Expected: all 5 tests pass.

- [ ] **Step 3: Run svelte-check**

```bash
cd gui/frontend && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: 0 errors on `ActiveFilters.svelte`. Pre-existing errors elsewhere are out of scope; ensure no *new* errors appear.

- [ ] **Step 4: Commit**

```bash
git add gui/frontend/src/lib/components/ActiveFilters.svelte gui/frontend/src/lib/components/ActiveFilters.test.ts
git commit -m "gui: add ActiveFilters component (removable chip row, renders only when filters active)"
```

---

### Task 5: Rewrite FilterBar.svelte and remove selectInlineTags

**Files:**
- Modify: `gui/frontend/src/lib/components/FilterBar.svelte` (full rewrite)
- Modify: `gui/frontend/src/lib/filtering.ts` (remove dead exports)
- Modify: `gui/frontend/src/lib/filtering.test.ts` (remove obsolete tests)

This task swaps three files together to keep the tree green between commits.

- [ ] **Step 1: Replace the body of `gui/frontend/src/lib/filtering.ts` with**

```ts
// Client-side filtering over a BoardSnapshot. Pure functions so the result
// can be `$derived` in components without subscribing here.

import type { BoardSnapshot, Task } from './api';
import type { BoardFilter } from './stores/filter';

function allTasks(snap: BoardSnapshot): Task[] {
  return [...snap.backlog, ...snap.inProgress, ...snap.done, ...(snap.archive ?? [])];
}

function passes(t: Task, f: BoardFilter): boolean {
  if (f.types.length > 0 && !f.types.includes(t.type)) return false;
  if (f.priorities.length > 0 && !f.priorities.includes(t.priority)) return false;
  if (f.modules.length > 0 && !f.modules.includes(t.module)) return false;
  if (f.agents.length > 0) {
    if (!t.agent || !f.agents.includes(t.agent)) return false;
  }
  if (f.parentEpic !== '' && t.parent !== f.parentEpic && t.id !== f.parentEpic) return false;
  if (f.tags.length > 0) {
    const tags = t.tags ?? [];
    const hit = f.tags.some((needle) => tags.includes(needle));
    if (!hit) return false;
  }
  if (f.search.trim() !== '') {
    const needle = f.search.toLowerCase();
    const hay = `${t.id} ${t.title}`.toLowerCase();
    if (!hay.includes(needle)) return false;
  }
  return true;
}

export function applyFilter(snap: BoardSnapshot, f: BoardFilter): BoardSnapshot {
  return {
    backlog: snap.backlog.filter((t) => passes(t, f)),
    inProgress: snap.inProgress.filter((t) => passes(t, f)),
    done: snap.done.filter((t) => passes(t, f)),
    archive: (snap.archive ?? []).filter((t) => passes(t, f)),
  } as BoardSnapshot;
}

export function observedValues(snap: BoardSnapshot, field: 'type' | 'priority' | 'module' | 'agent'): string[] {
  const set = new Set<string>();
  for (const t of allTasks(snap)) {
    const v = (t as unknown as Record<string, string>)[field];
    if (v) set.add(v);
  }
  return [...set].sort();
}

export function observedTags(snap: BoardSnapshot): string[] {
  const counts = new Map<string, number>();
  for (const t of allTasks(snap)) {
    for (const tag of t.tags ?? []) counts.set(tag, (counts.get(tag) ?? 0) + 1);
  }
  return [...counts.entries()]
    .sort(([tagA, countA], [tagB, countB]) => countB - countA || tagA.localeCompare(tagB))
    .map(([tag]) => tag);
}

export function observedEpics(snap: BoardSnapshot): Task[] {
  return allTasks(snap).filter((t) => (t.tags ?? []).includes('epic')).sort((a, b) => a.id.localeCompare(b.id));
}
```

The diff vs. the current file: drop the `FILTER_BAR_INLINE_TAG_LIMIT` export, the `InlineTagSelection` interface, and the `selectInlineTags` function. Everything else is byte-identical.

- [ ] **Step 2: Replace the body of `gui/frontend/src/lib/filtering.test.ts` with**

```ts
import { describe, expect, it } from 'vitest';
import type { BoardSnapshot, Task } from './api';
import { applyFilter, observedTags } from './filtering';
import type { BoardFilter } from './stores/filter';

const baseFilter: BoardFilter = {
  search: '',
  types: [],
  priorities: [],
  modules: [],
  tags: [],
  agents: [],
  parentEpic: '',
  showArchive: false,
};

function task(id: string, overrides: Partial<Task> = {}): Task {
  return {
    id,
    title: id,
    type: 'task',
    priority: 'P2',
    size: 'M',
    module: 'core',
    tags: [],
    branch: '',
    parent: '',
    status: 'backlog',
    filePath: '',
    agent: '',
    agentStatus: '',
    ...overrides,
  };
}

function snapshot(columns: Partial<BoardSnapshot>): BoardSnapshot {
  return {
    backlog: columns.backlog ?? [],
    inProgress: columns.inProgress ?? [],
    done: columns.done ?? [],
    archive: columns.archive ?? [],
  };
}

function ids(snap: BoardSnapshot): string[] {
  return [...snap.backlog, ...snap.inProgress, ...snap.done, ...(snap.archive ?? [])].map((t) => t.id);
}

describe('applyFilter', () => {
  it('matches the selected parent epic and its child tasks', () => {
    const snap = snapshot({
      backlog: [
        task('TB-1', { title: 'Parent epic', tags: ['epic'], type: 'epic' }),
        task('TB-3', { title: 'Unrelated task' }),
      ],
      inProgress: [
        task('TB-2', { title: 'Child task', parent: 'TB-1' }),
      ],
    });

    const filtered = applyFilter(snap, { ...baseFilter, parentEpic: 'TB-1' });

    expect(ids(filtered)).toEqual(['TB-1', 'TB-2']);
  });

  it('composes parent epic matching with the other filter predicates', () => {
    const snap = snapshot({
      backlog: [
        task('TB-1', { title: 'Checkout epic', tags: ['epic', 'ui'], type: 'epic' }),
        task('TB-2', { title: 'Checkout child UI', parent: 'TB-1', tags: ['ui'], type: 'task' }),
        task('TB-3', { title: 'Checkout child backend', parent: 'TB-1', tags: ['backend'], type: 'task' }),
        task('TB-4', { title: 'Other child UI', parent: 'TB-9', tags: ['ui'], type: 'task' }),
      ],
    });

    const filtered = applyFilter(snap, {
      ...baseFilter,
      parentEpic: 'TB-1',
      tags: ['ui'],
      types: ['task'],
      search: 'child',
    });

    expect(ids(filtered)).toEqual(['TB-2']);
  });
});

describe('tag helpers', () => {
  it('ranks observed tags by frequency with deterministic tie sorting', () => {
    const snap = snapshot({
      backlog: [
        task('TB-1', { tags: ['ui', 'backend', 'cli'] }),
        task('TB-2', { tags: ['ui', 'backend'] }),
        task('TB-3', { tags: ['ui', 'docs'] }),
        task('TB-4', { tags: ['docs'] }),
      ],
    });

    expect(observedTags(snap)).toEqual(['ui', 'backend', 'docs', 'cli']);
  });
});
```

The two removed cases (`'strictly caps inline tags at the limit in ranked order'` and `'caps the filter bar tag header at 10 inline chips'`) referenced the deleted exports.

- [ ] **Step 3: Replace the body of `gui/frontend/src/lib/components/FilterBar.svelte` with**

```svelte
<script lang="ts">
  import type { BoardSnapshot } from '$lib/api';
  import { clearFilter, filter, type BoardFilter } from '$lib/stores/filter';
  import { observedEpics, observedTags, observedValues } from '$lib/filtering';
  import FilterDropdown from './FilterDropdown.svelte';
  import ActiveFilters from './ActiveFilters.svelte';

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
  let epicTasks = $derived(observedEpics(snapshot));
  let epicOptions = $derived(epicTasks.map((e) => e.id));

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

  function setEpic(v: string) {
    f = { ...f, parentEpic: v === f.parentEpic ? '' : v };
    commit();
  }

  function removeFilter(category: 'types' | 'priorities' | 'modules' | 'tags' | 'agents' | 'parentEpic', value: string) {
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
</script>

<section class="filter" aria-label="Filters">
  <input
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
  {#if tags.length > 0}
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
```

- [ ] **Step 4: Run the full frontend suite (FilterBar tests will fail — that's expected; we rewrite them in Task 7)**

```bash
cd gui/frontend && npx vitest run
```

Expected:
- `filtering.test.ts` — all pass.
- `FilterDropdown.test.ts` — all pass.
- `ActiveFilters.test.ts` — all pass.
- `FilterBar.test.ts` — failures (the old tests assert on `.group.tags`, `.tag-more-trigger`, etc. that no longer exist).

This is the expected red state before Task 7. Do **not** commit yet.

- [ ] **Step 5: Defer the commit to Task 6**

We do not want a commit where `vitest` is red on `main`. Move directly to Task 6, which rewrites the FilterBar test, then commit Tasks 5 + 6 together.

---

### Task 6: Rewrite FilterBar.test.ts for the new layout

**Files:**
- Modify: `gui/frontend/src/lib/components/FilterBar.test.ts`

- [ ] **Step 1: Replace the body of `gui/frontend/src/lib/components/FilterBar.test.ts` with**

```ts
import { get } from 'svelte/store';
import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { BoardSnapshot, Task } from '$lib/api';
import { filter, initialFilter } from '$lib/stores/filter';
import FilterBar from './FilterBar.svelte';

let component: ReturnType<typeof mount> | null = null;

function task(id: string, overrides: Partial<Task> = {}): Task {
  return {
    id,
    title: id,
    type: 'task',
    priority: 'P2',
    size: 'M',
    module: 'core',
    tags: [],
    branch: '',
    parent: '',
    status: 'backlog',
    filePath: '',
    agent: '',
    agentStatus: '',
    ...overrides,
  };
}

function snapshot(tasks: Task[]): BoardSnapshot {
  return { backlog: tasks, inProgress: [], done: [], archive: [] };
}

function triggers(): HTMLButtonElement[] {
  return [...document.querySelectorAll<HTMLButtonElement>('.fd-trigger')];
}

function triggerByLabel(label: string): HTMLButtonElement {
  for (const t of triggers()) {
    const text = t.textContent?.replace(/\s+/g, ' ').trim() ?? '';
    if (text === label || text.startsWith(`${label} `) || text.startsWith(`${label}:`)) return t;
  }
  throw new Error(`trigger for label "${label}" not found; saw: ${triggers().map((t) => t.textContent?.trim()).join(', ')}`);
}

beforeEach(() => {
  filter.set({ ...initialFilter });
});

afterEach(async () => {
  if (component) await unmount(component);
  component = null;
  document.body.innerHTML = '';
  filter.set({ ...initialFilter });
});

describe('FilterBar layout', () => {
  it('renders one dropdown per category that has more than one observed value', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug', priority: 'P1', module: 'cli', agent: 'claude', tags: ['ui'] }),
      task('TB-2', { type: 'feature', priority: 'P0', module: 'gui', agent: 'codex', tags: ['docs'] }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();

    const labels = triggers().map((t) => t.textContent?.replace(/\s+/g, ' ').trim() ?? '');
    expect(labels).toEqual(expect.arrayContaining(['Type ▾', 'Priority ▾', 'Module ▾', 'Tags ▾', 'Agent ▾']));
    expect(labels.find((l) => l.startsWith('Epic'))).toBeUndefined(); // no epic tasks
  });

  it('hides categories with one or zero distinct values', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug', priority: 'P1', module: 'cli' }),
      task('TB-2', { type: 'bug', priority: 'P1', module: 'cli' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();

    const labels = triggers().map((t) => t.textContent?.replace(/\s+/g, ' ').trim() ?? '');
    // Single distinct value per category → no dropdowns rendered at all.
    expect(labels.find((l) => l.startsWith('Type'))).toBeUndefined();
    expect(labels.find((l) => l.startsWith('Priority'))).toBeUndefined();
    expect(labels.find((l) => l.startsWith('Module'))).toBeUndefined();
  });

  it('shows a count badge on the trigger label when filters are selected', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
    ]);
    filter.set({ ...initialFilter, types: ['bug', 'feature'] });
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();

    expect(triggerByLabel('Type').textContent?.replace(/\s+/g, ' ').trim()).toBe('Type (2) ▾');
  });

  it('renders ActiveFilters only when at least one filter is set', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    expect(document.querySelector('.af')).toBeNull();

    filter.set({ ...initialFilter, types: ['bug'] });
    await tick();
    expect(document.querySelector('.af')).not.toBeNull();
    expect(document.querySelector('.af-chip')?.textContent?.replace(/\s+/g, ' ').trim()).toBe('bug ×');
  });

  it('removes a single filter when its active chip is clicked', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug', priority: 'P1' }),
      task('TB-2', { type: 'feature', priority: 'P0' }),
    ]);
    filter.set({ ...initialFilter, types: ['bug', 'feature'], priorities: ['P1'] });
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    const bugChip = [...document.querySelectorAll<HTMLButtonElement>('.af-chip')].find(
      (b) => b.textContent?.includes('bug'),
    );
    bugChip?.click();
    await tick();
    expect(get(filter).types).toEqual(['feature']);
    expect(get(filter).priorities).toEqual(['P1']);
  });

  it('clear button resets every filter category but preserves showArchive', async () => {
    const snap = snapshot([task('TB-1', { type: 'bug' }), task('TB-2', { type: 'feature' })]);
    filter.set({ ...initialFilter, types: ['bug'], priorities: ['P1'], showArchive: true });
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();

    document.querySelector<HTMLButtonElement>('button.clear')?.click();
    await tick();
    expect(get(filter).types).toEqual([]);
    expect(get(filter).priorities).toEqual([]);
    expect(get(filter).showArchive).toBe(true);
  });

  it('multi-toggles values inside a Type dropdown popover', async () => {
    const snap = snapshot([
      task('TB-1', { type: 'bug' }),
      task('TB-2', { type: 'feature' }),
      task('TB-3', { type: 'spike' }),
    ]);
    component = mount(FilterBar, { target: document.body, props: { snapshot: snap } });
    await tick();
    triggerByLabel('Type').click();
    await tick();
    const opts = [...document.querySelectorAll<HTMLButtonElement>('.fd-option')];
    opts.find((b) => b.textContent?.trim() === 'bug')?.click();
    await tick();
    opts.find((b) => b.textContent?.trim() === 'spike')?.click();
    await tick();
    expect(get(filter).types).toEqual(['bug', 'spike']);
  });
});
```

- [ ] **Step 2: Run the full frontend suite**

```bash
cd gui/frontend && npx vitest run
```

Expected: every test in `filtering.test.ts`, `FilterDropdown.test.ts`, `ActiveFilters.test.ts`, and `FilterBar.test.ts` passes.

- [ ] **Step 3: Run svelte-check across the frontend**

```bash
cd gui/frontend && npx svelte-check --tsconfig ./tsconfig.json
```

Expected: 0 new errors. If pre-existing warnings exist elsewhere, they are out of scope — verify the count matches the count before this change.

- [ ] **Step 4: Commit Tasks 5 + 6 together**

```bash
git add gui/frontend/src/lib/filtering.ts \
        gui/frontend/src/lib/filtering.test.ts \
        gui/frontend/src/lib/components/FilterBar.svelte \
        gui/frontend/src/lib/components/FilterBar.test.ts
git commit -m "$(cat <<'EOF'
gui: redesign FilterBar (dropdowns + active chips row)

Replaces the inline chip flood (types/priorities/modules/agents/tags
rendered as wrap-flowing pill rows) with one compact dropdown per
category plus a removable "Active filters" chip row that only
renders when filters are set. Drops selectInlineTags /
FILTER_BAR_INLINE_TAG_LIMIT — the cap is unnecessary now that long
option lists live in searchable popovers instead of the header.

Supersedes TB-92.
EOF
)"
```

---

### Task 7: Mark TB-92 superseded

**Files:**
- Modify: `board/backlog/TB-92.md`

The CLI knows how to mutate task files via `tb`, but the Log section is free-form markdown — a single appended line is the lowest-risk path here.

- [ ] **Step 1: Read the file to know the exact tail**

```bash
tail -5 board/backlog/TB-92.md
```

- [ ] **Step 2: Append a Log entry**

Use the Edit tool to add this line directly after the existing `- 2026-05-14: Edited agentstatus=failed` line (or after the last Log line, whichever is later):

```
- 2026-05-14: Superseded by FilterBar redesign — header cap is moot now that all filter values live in per-category dropdowns. See docs/superpowers/specs/2026-05-14-filter-bar-redesign-design.md.
```

- [ ] **Step 3: Regenerate `BOARD.md` so the generated view does not lag**

```bash
go run ./cli regenerate
```

Expected: no error; `git status` shows `M board/BOARD.md` and `M board/backlog/TB-92.md`.

- [ ] **Step 4: Commit**

```bash
git add board/backlog/TB-92.md board/BOARD.md
git commit -m "board: mark TB-92 superseded by FilterBar redesign"
```

---

### Task 8: Manual GUI verification

**Files:**
- (No file changes — verification only.)

The Wails3 dev binary is the only place we can verify the result is visually correct against the screenshots that prompted this work. Vitest + jsdom cannot exercise positioning, overflow, or the actual feel of opening 5 popovers in a row.

- [ ] **Step 1: Build the dev binary**

From the repo root:

```bash
cd gui && wails3 task dev
```

If `wails3 task dev` is not the correct invocation in this repo, fall back to `wails3 build` and run the produced binary. Consult `gui/CLAUDE.md` or `gui/Taskfile.yml` for the project's exact dev command.

- [ ] **Step 2: Open the `task-board-tools` board (the small one)**

Expected:
- Header row 1 shows: Search, Type ▾, Priority ▾, Module ▾, Tags ▾, Agent ▾ (no Epic dropdown if no `epic`-tagged tasks), Show archived, Clear.
- No row 2 visible.

- [ ] **Step 3: Open the `writer-studio` board (the big one with ≈150 modules + ≈250 tags)**

Expected:
- Header is still a single row (or two wrap-rows on narrow windows, but never the wall of chips from the screenshot).
- Clicking `Module ▾` opens a popover with a `Filter…` search box at the top; typing a substring narrows the list.

- [ ] **Step 4: Set a few filters in writer-studio (e.g. one type, one priority, two modules, one tag)**

Expected:
- Row 2 appears with chips: `feature ×  P1 ×  ai/scaffold ×  backend/git ×  cli ×` (chips colored by category).
- Each trigger shows a count badge: `Type (1) ▾`, `Priority (1) ▾`, `Module (2) ▾`, `Tags (1) ▾`.
- Clicking the × on a chip removes that single filter; row 2 collapses when the last chip is removed.

- [ ] **Step 5: Test keyboard accessibility**

- Tab through row 1: focus moves Search → each dropdown trigger → archived checkbox → Clear.
- On a focused trigger, press Enter / Space / ArrowDown: popover opens. Press Escape: popover closes, focus returns to the trigger.
- Inside an open popover with a search input (Module on writer-studio), the search input has initial focus; ArrowDown moves focus into the option list.

- [ ] **Step 6: Quit the binary and log results in a board task or PR description**

Note in `Log` (on the merged task or PR comment): tag-count observed in writer-studio, that header stayed at 1 row at rest and 2 when filters were set, screenshot if possible.

No commit for this task — verification only.

---

## Self-Review Notes (author)

- **Spec coverage:**
  - "Layout (2 rows max)" → Tasks 5, 6, 8.
  - "FilterDropdown component (props/behavior/threshold/single-select/aria)" → Tasks 1, 2.
  - "ActiveFilters component (renders nothing when empty, Active: label, category coloring)" → Tasks 3, 4.
  - "FilterBar.svelte composition + drop selectInlineTags imports" → Task 5.
  - "Remove selectInlineTags + FILTER_BAR_INLINE_TAG_LIMIT" → Task 5 (filtering.ts + filtering.test.ts together).
  - "FilterBar.test.ts replaces 5 tag-overflow cases with new layout cases" → Task 6.
  - "TB-92 marked superseded" → Task 7.
  - "Manual verification in writer-studio + task-board-tools" → Task 8.
- **Placeholder scan:** No "TBD" / "implement later" / "similar to Task N" — all code blocks are concrete.
- **Type consistency:** `BoardFilter` type used unchanged from `$lib/stores/filter`; the `removeFilter` category union in FilterBar matches the one in ActiveFilters (`'types' | 'priorities' | 'modules' | 'tags' | 'agents' | 'parentEpic'`).
- **Known minor UX regression — documented, not blocked:** the old `<select>` showed `{id} {title.slice(0, 40)}` per epic option. The new `FilterDropdown` takes a plain `string[]` and shows only the ID. Epic counts are typically small (single digits in both target projects), and the trigger label becomes `Epic: TB-5` when one is selected. If this proves disruptive in dogfooding, the lightweight follow-up is to extend `FilterDropdown` props with an optional `optionLabel?: (value: string) => string` — but that is **out of scope for this plan**.
- **One known intentional intermediate red state:** at end of Task 5 the `FilterBar.test.ts` is red. Task 6 closes the gap before any commit lands.
