<script lang="ts">
  import { flushSync } from 'svelte';

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

  function close() {
    open = false;
    query = '';
  }

  $effect(() => {
    if (!open) return;

    function onDocumentPointerDown(event: PointerEvent) {
      if (rootEl?.contains(event.target as Node)) return;
      close();
    }
    function onDocumentKeydown(event: KeyboardEvent) {
      if (event.key !== 'Escape') return;
      close();
      focusTrigger();
    }
    document.addEventListener('pointerdown', onDocumentPointerDown, true);
    document.addEventListener('keydown', onDocumentKeydown);
    return () => {
      document.removeEventListener('pointerdown', onDocumentPointerDown, true);
      document.removeEventListener('keydown', onDocumentKeydown);
    };
  });

  function openMenu(focusIndex: number | 'search' = 'search') {
    open = true;
    flushSync();
    clampMenuToViewport();
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
      close();
    } else {
      openMenu();
    }
  }

  function onTriggerKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown') {
      event.preventDefault();
      openMenu('search');
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      openMenu(filteredOptions.length - 1);
    } else if (event.key === 'Escape') {
      close();
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
        close();
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
      event.stopPropagation();
      focusOption(0);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      close();
      focusTrigger();
    }
  }

  function pick(value: string) {
    onToggle(value);
    if (single) {
      close();
    }
  }

  function pickNull() {
    onToggle('');
    close();
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

  function clampMenuToViewport() {
    const menu = rootEl?.querySelector<HTMLElement>('.fd-menu');
    const triggerEl = rootEl?.querySelector<HTMLElement>('.fd-trigger');
    if (!menu || !triggerEl) return;
    // Reset before measuring so successive opens compute against a clean baseline.
    menu.style.left = '';
    const triggerRect = triggerEl.getBoundingClientRect();
    const menuWidth = menu.offsetWidth;
    const margin = 8;
    const overflowRight = triggerRect.left + menuWidth - (window.innerWidth - margin);
    if (overflowRight > 0) {
      menu.style.left = `-${overflowRight}px`;
    }
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
    {triggerText}
  </button>
  {#if open}
    <div class="fd-menu" role="menu" aria-label={`${label} options`} tabindex="-1" onkeydown={onMenuKeydown}>
      {#if showSearch}
        <input
          class="fd-search"
          type="search"
          aria-label={`Search ${label}`}
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
  .fd-trigger:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .fd-trigger::after { content: ' ▾'; margin-left: 2px; font-size: 9px; opacity: 0.7; }
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
  .fd-clear:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
</style>
