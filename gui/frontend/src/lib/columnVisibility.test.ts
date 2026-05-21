import { describe, expect, it } from 'vitest';
import {
  VIRTUAL_COLUMN_ITEM_HEIGHT,
  VIRTUAL_COLUMN_OVERSCAN,
  shouldVirtualizeColumn,
  virtualTaskRange,
} from './columnVisibility';

describe('column visibility helpers', () => {
  it('virtualizes only large completed-history columns', () => {
    expect(shouldVirtualizeColumn('done', 1000)).toBe(true);
    expect(shouldVirtualizeColumn('archive', 1000)).toBe(true);
    expect(shouldVirtualizeColumn('done', 12)).toBe(false);

    expect(shouldVirtualizeColumn('backlog', 1000)).toBe(false);
    expect(shouldVirtualizeColumn('ready', 1000)).toBe(false);
    expect(shouldVirtualizeColumn('in-progress', 1000)).toBe(false);
    expect(shouldVirtualizeColumn('code-review', 1000)).toBe(false);
  });

  it('keeps rendered range bounded around viewport and overscan', () => {
    const range = virtualTaskRange(3000, 0, VIRTUAL_COLUMN_ITEM_HEIGHT * 4);

    expect(range.start).toBe(0);
    expect(range.end - range.start).toBeLessThanOrEqual(4 + VIRTUAL_COLUMN_OVERSCAN);
    expect(range.paddingTop).toBe(0);
    expect(range.paddingBottom).toBeGreaterThan(0);
  });

  it('maps scroll offset to the same task order without rendering earlier cards', () => {
    const range = virtualTaskRange(
      3000,
      VIRTUAL_COLUMN_ITEM_HEIGHT * 1000,
      VIRTUAL_COLUMN_ITEM_HEIGHT * 4,
    );

    expect(range.start).toBe(1000 - VIRTUAL_COLUMN_OVERSCAN);
    expect(range.end - range.start).toBeLessThanOrEqual(4 + VIRTUAL_COLUMN_OVERSCAN * 2);
    expect(range.paddingTop).toBe(range.start * VIRTUAL_COLUMN_ITEM_HEIGHT);
    expect(range.paddingBottom).toBe((3000 - range.end) * VIRTUAL_COLUMN_ITEM_HEIGHT);
  });
});
