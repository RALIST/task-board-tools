import { describe, expect, it } from 'vitest';
import {
  COLUMN_TASK_BATCH_SIZE,
  hiddenTaskCount,
  nextVisibleTaskLimit,
  shouldBatchRenderColumn,
  visibleTaskCount,
} from './columnVisibility';

describe('column visibility helpers', () => {
  it('shows the whole column when it fits in the first batch', () => {
    expect(visibleTaskCount(12)).toBe(12);
    expect(hiddenTaskCount(12)).toBe(0);
  });

  it('caps large columns to the initial batch', () => {
    expect(visibleTaskCount(COLUMN_TASK_BATCH_SIZE + 1)).toBe(COLUMN_TASK_BATCH_SIZE);
    expect(hiddenTaskCount(COLUMN_TASK_BATCH_SIZE + 1)).toBe(1);
  });

  it('advances by one batch without exceeding the total', () => {
    expect(nextVisibleTaskLimit(COLUMN_TASK_BATCH_SIZE, COLUMN_TASK_BATCH_SIZE * 3 + 5))
      .toBe(COLUMN_TASK_BATCH_SIZE * 2);
    expect(nextVisibleTaskLimit(COLUMN_TASK_BATCH_SIZE * 3, COLUMN_TASK_BATCH_SIZE * 3 + 5))
      .toBe(COLUMN_TASK_BATCH_SIZE * 3 + 5);
  });

  it('only batch-renders completed history columns', () => {
    expect(shouldBatchRenderColumn('backlog')).toBe(false);
    expect(shouldBatchRenderColumn('ready')).toBe(false);
    expect(shouldBatchRenderColumn('in-progress')).toBe(false);
    expect(shouldBatchRenderColumn('code-review')).toBe(false);
    expect(shouldBatchRenderColumn('done')).toBe(true);
    expect(shouldBatchRenderColumn('archive')).toBe(true);
  });
});
