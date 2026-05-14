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
