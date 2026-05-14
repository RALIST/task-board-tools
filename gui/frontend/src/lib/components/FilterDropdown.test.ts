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
    options()[1].click(); // skip leading "(any)"; index 0 is "(any)", index 1 is TB-1
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

  it('ArrowDown from the search input lands on the first option, not the second', async () => {
    const opts = Array.from({ length: 11 }, (_, i) => `mod-${String(i).padStart(2, '0')}`);
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Module', options: opts, selected: [], onToggle: () => {} },
    });
    await tick();
    trigger().click();
    await tick();
    const search = document.querySelector<HTMLInputElement>('.fd-search')!;
    search.focus();
    search.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    await tick();
    const firstOption = document.querySelector<HTMLButtonElement>('.fd-option');
    expect(document.activeElement).toBe(firstOption);
  });

  it('clears the search query when the popover closes via outside click', async () => {
    const opts = Array.from({ length: 11 }, (_, i) => `mod-${String(i).padStart(2, '0')}`);
    const outside = document.createElement('button');
    outside.textContent = 'outside';
    document.body.append(outside);
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Module', options: opts, selected: [], onToggle: () => {} },
    });
    await tick();
    trigger().click();
    await tick();
    const search = document.querySelector<HTMLInputElement>('.fd-search')!;
    search.value = '01';
    search.dispatchEvent(new Event('input', { bubbles: true }));
    await tick();
    // Confirm filter is applied (only mod-01 visible).
    expect(options().map((b) => b.textContent?.trim())).toEqual(['mod-01']);
    // Close via outside click.
    outside.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    await tick();
    expect(document.querySelector('.fd-menu')).toBeNull();
    // Reopen — search should be empty and all options visible.
    trigger().click();
    await tick();
    const search2 = document.querySelector<HTMLInputElement>('.fd-search')!;
    expect(search2.value).toBe('');
    expect(options()).toHaveLength(11);
  });

  it('exposes an aria-label on the search input', async () => {
    const opts = Array.from({ length: 11 }, (_, i) => `mod-${String(i).padStart(2, '0')}`);
    component = mount(FilterDropdown, {
      target: document.body,
      props: { label: 'Module', options: opts, selected: [], onToggle: () => {} },
    });
    await tick();
    trigger().click();
    await tick();
    expect(document.querySelector<HTMLInputElement>('.fd-search')?.getAttribute('aria-label')).toBe('Search Module');
  });
});
