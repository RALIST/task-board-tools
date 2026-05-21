import { get } from 'svelte/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  _resetStartupGraceForTesting,
  cancelStartupGrace,
  startStartupGrace,
  startupGraceStore,
} from './startupGrace';

describe('startupGraceStore', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-21T12:00:00Z'));
    _resetStartupGraceForTesting();
  });

  afterEach(() => {
    _resetStartupGraceForTesting();
    vi.useRealTimers();
  });

  it('shows the initial remaining time for the active board', () => {
    startStartupGrace('/boards/a', 3);

    expect(get(startupGraceStore)).toMatchObject({
      active: true,
      boardKey: '/boards/a',
      remainingSeconds: 3,
    });
  });

  it('decrements each second and hides when the grace window expires', () => {
    startStartupGrace('/boards/a', 3);

    vi.advanceTimersByTime(1000);
    expect(get(startupGraceStore).remainingSeconds).toBe(2);

    vi.advanceTimersByTime(1000);
    expect(get(startupGraceStore).remainingSeconds).toBe(1);

    vi.advanceTimersByTime(1000);
    expect(get(startupGraceStore).active).toBe(false);
  });

  it('keeps a new board countdown active when an old board deadline passes', () => {
    startStartupGrace('/boards/a', 2);
    vi.advanceTimersByTime(1000);

    startStartupGrace('/boards/b', 5);
    vi.advanceTimersByTime(1100);

    expect(get(startupGraceStore)).toMatchObject({
      active: true,
      boardKey: '/boards/b',
      remainingSeconds: 4,
    });
  });

  it('hides immediately when cancelled', () => {
    startStartupGrace('/boards/a', 3);
    cancelStartupGrace();

    expect(get(startupGraceStore).active).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not show or keep timers when the configured grace is zero', () => {
    startStartupGrace('/boards/a', 0);

    expect(get(startupGraceStore).active).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
  });
});
