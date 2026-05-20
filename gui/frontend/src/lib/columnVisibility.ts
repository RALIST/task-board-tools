export const COLUMN_TASK_BATCH_SIZE = 120;

export function shouldBatchRenderColumn(status: string): boolean {
  return status === 'done' || status === 'archive';
}

export function visibleTaskCount(total: number, limit: number = COLUMN_TASK_BATCH_SIZE): number {
  if (total <= 0) return 0;
  return Math.min(total, Math.max(0, limit));
}

export function hiddenTaskCount(total: number, limit: number = COLUMN_TASK_BATCH_SIZE): number {
  return Math.max(0, total - visibleTaskCount(total, limit));
}

export function nextVisibleTaskLimit(currentLimit: number, total: number): number {
  return Math.min(total, Math.max(COLUMN_TASK_BATCH_SIZE, currentLimit) + COLUMN_TASK_BATCH_SIZE);
}
