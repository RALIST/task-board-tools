import { get } from 'svelte/store';
import { mount, tick, unmount } from 'svelte';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { BoardSnapshot, Task } from '$lib/api';
import { FILTER_BAR_INLINE_TAG_LIMIT } from '$lib/filtering';
import { filter, initialFilter } from '$lib/stores/filter';
import FilterBar from './FilterBar.svelte';

let component: ReturnType<typeof mount> | null = null;

function task(id: string, tag: string): Task {
  return {
    id,
    title: tag,
    type: 'task',
    priority: 'P2',
    size: 'M',
    module: 'core',
    tags: [tag],
    branch: '',
    parent: '',
    status: 'backlog',
    filePath: '',
    agent: '',
    agentStatus: '',
  };
}

function snapshot(tags: string[]): BoardSnapshot {
  return {
    backlog: tags.map((tag, index) => task(`TB-${index + 1}`, tag)),
    inProgress: [],
    done: [],
    archive: [],
  };
}

function inlineTagTexts(): string[] {
  return [...document.querySelectorAll<HTMLButtonElement>('.group.tags > button.chip.tag')]
    .map((button) => button.textContent?.trim() ?? '');
}

function inlineTagButton(tag: string): HTMLButtonElement {
  const button = [...document.querySelectorAll<HTMLButtonElement>('.group.tags > button.chip.tag')]
    .find((el) => el.textContent?.trim() === tag);
  if (!button) throw new Error(`inline tag button ${tag} not found`);
  return button;
}

function moreButton(): HTMLButtonElement {
  const button = document.querySelector<HTMLButtonElement>('.tag-more-trigger');
  if (!button) throw new Error('more button not found');
  return button;
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

describe('FilterBar tag overflow', () => {
  it('collapses low-ranked tags and promotes selected overflow tags inline', async () => {
    const tags = Array.from(
      { length: FILTER_BAR_INLINE_TAG_LIMIT + 1 },
      (_, index) => `tag-${String(index + 1).padStart(2, '0')}`,
    );
    const overflowTag = tags[FILTER_BAR_INLINE_TAG_LIMIT];
    component = mount(FilterBar, {
      target: document.body,
      props: { snapshot: snapshot(tags) },
    });
    await tick();

    expect(inlineTagTexts()).toEqual(tags.slice(0, FILTER_BAR_INLINE_TAG_LIMIT));
    expect(moreButton().textContent?.replace(/\s+/g, ' ').trim()).toBe('+1 more');
    expect(moreButton().getAttribute('aria-expanded')).toBe('false');
    moreButton().click();
    await tick();
    expect(moreButton().getAttribute('aria-expanded')).toBe('true');

    const overflowButton = document.querySelector<HTMLButtonElement>('.tag-menu .tag-option');
    expect(overflowButton?.textContent?.trim()).toBe(overflowTag);
    overflowButton?.click();
    await tick();

    expect(get(filter).tags).toEqual([overflowTag]);
    expect(inlineTagTexts()).toContain(overflowTag);
    expect(document.querySelector('.tag-more')).toBeNull();

    inlineTagButton(overflowTag).click();
    await tick();

    expect(get(filter).tags).toEqual([]);
    expect(inlineTagTexts()).toEqual(tags.slice(0, FILTER_BAR_INLINE_TAG_LIMIT));
    expect(moreButton().textContent?.replace(/\s+/g, ' ').trim()).toBe('+1 more');
  });

  it('closes the overflow menu on outside click, Escape, and focus leaving the menu', async () => {
    const tags = Array.from(
      { length: FILTER_BAR_INLINE_TAG_LIMIT + 2 },
      (_, index) => `tag-${String(index + 1).padStart(2, '0')}`,
    );
    const outside = document.createElement('button');
    outside.textContent = 'outside';
    document.body.append(outside);
    component = mount(FilterBar, {
      target: document.body,
      props: { snapshot: snapshot(tags) },
    });
    await tick();

    moreButton().click();
    await tick();
    expect(document.querySelector('.tag-menu')).not.toBeNull();

    outside.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    await tick();
    expect(document.querySelector('.tag-menu')).toBeNull();

    moreButton().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    await tick();
    expect(document.querySelector('.tag-menu')).not.toBeNull();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await tick();
    expect(document.querySelector('.tag-menu')).toBeNull();

    moreButton().click();
    await tick();
    const option = document.querySelector<HTMLButtonElement>('.tag-option');
    option?.focus();
    option?.dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: outside }));
    await tick();
    expect(document.querySelector('.tag-menu')).toBeNull();
  });

  it('moves keyboard focus through overflow tags with arrow keys', async () => {
    const tags = Array.from(
      { length: FILTER_BAR_INLINE_TAG_LIMIT + 3 },
      (_, index) => `tag-${String(index + 1).padStart(2, '0')}`,
    );
    component = mount(FilterBar, {
      target: document.body,
      props: { snapshot: snapshot(tags) },
    });
    await tick();

    moreButton().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    await tick();
    await tick();

    const options = [...document.querySelectorAll<HTMLButtonElement>('.tag-option')];
    expect(document.activeElement).toBe(options[0]);

    options[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    await tick();
    expect(document.activeElement).toBe(options[1]);

    options[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'End', bubbles: true }));
    await tick();
    expect(document.activeElement).toBe(options[options.length - 1]);
  });
});
