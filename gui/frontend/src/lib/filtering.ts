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
