export const VIRTUAL_COLUMN_THRESHOLD = 240;
export const VIRTUAL_COLUMN_ITEM_HEIGHT = 112;
export const VIRTUAL_COLUMN_OVERSCAN = 8;
export const VIRTUAL_COLUMN_FALLBACK_VIEWPORT_HEIGHT = 672;

export type VirtualTaskRange = {
  start: number;
  end: number;
  paddingTop: number;
  paddingBottom: number;
};

// Active workflow columns stay fully mounted because their DnD zones depend on
// complete column membership. Completed-history columns are browse-only at
// large sizes, so lazy viewport rendering is safe there.
export function shouldVirtualizeColumn(status: string, total: number): boolean {
  return (status === 'done' || status === 'archive') && total > VIRTUAL_COLUMN_THRESHOLD;
}

export function virtualTaskRange(total: number, scrollTop: number, viewportHeight: number): VirtualTaskRange {
  const count = Math.max(0, Math.floor(total));
  if (count === 0) return { start: 0, end: 0, paddingTop: 0, paddingBottom: 0 };

  const safeScrollTop = Math.max(0, scrollTop);
  const viewport = viewportHeight > 0 ? viewportHeight : VIRTUAL_COLUMN_FALLBACK_VIEWPORT_HEIGHT;
  const firstVisible = Math.min(count - 1, Math.floor(safeScrollTop / VIRTUAL_COLUMN_ITEM_HEIGHT));
  const visibleSlots = Math.max(1, Math.ceil(viewport / VIRTUAL_COLUMN_ITEM_HEIGHT));
  const start = Math.max(0, firstVisible - VIRTUAL_COLUMN_OVERSCAN);
  const end = Math.min(count, firstVisible + visibleSlots + VIRTUAL_COLUMN_OVERSCAN);

  return {
    start,
    end,
    paddingTop: start * VIRTUAL_COLUMN_ITEM_HEIGHT,
    paddingBottom: (count - end) * VIRTUAL_COLUMN_ITEM_HEIGHT,
  };
}
