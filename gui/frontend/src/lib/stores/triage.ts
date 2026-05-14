import { derived, get, writable } from 'svelte/store';
import { getTriage } from '$lib/api';

type TriageMap = Map<string, string[]>;

const triage = writable<TriageMap>(new Map());
let seeded = false;

export const triageStore = {
  subscribe(run: (value: TriageMap) => void) {
    const off = triage.subscribe(run);
    ensureSeeded();
    return off;
  },
};

export function needsGrooming(id: string): boolean {
  return get(triage).has(id);
}

export function reasonsFor(id: string): string[] {
  return [...(get(triage).get(id) ?? [])];
}

export function triageForTask(id: string) {
  ensureSeeded();
  return derived(triage, ($triage) => [...($triage.get(id) ?? [])]);
}

export async function refreshTriage(): Promise<void> {
  try {
    const next = await getTriage();
    triage.set(recordToMap(next));
  } catch {
    seeded = false;
  }
}

export async function refreshTriageTask(id: string): Promise<void> {
  if (!id) return;
  let next: TriageMap;
  try {
    next = recordToMap(await getTriage());
  } catch {
    return;
  }
  triage.update((current) => {
    const copy = new Map(current);
    if (next.has(id)) {
      copy.set(id, [...(next.get(id) ?? [])]);
    } else {
      copy.delete(id);
    }
    return copy;
  });
}

export function registerTriageEventHandlers(
  on: (event: string, handler: (e: { data: unknown[] }) => void) => () => void,
): () => void {
  const off = on('board:reloaded', () => {
    void refreshTriage();
  });
  return () => {
    try {
      off();
    } catch {
      /* ignore */
    }
  };
}

export function registerTaskTriageEventHandler(
  id: string,
  on: (event: string, handler: (e: { data: unknown[] }) => void) => () => void,
): () => void {
  const off = on(`task:updated:${id}`, () => {
    void refreshTriageTask(id);
  });
  return () => {
    try {
      off();
    } catch {
      /* ignore */
    }
  };
}

export function _resetTriageStoreForTesting(): void {
  seeded = false;
  triage.set(new Map());
}

function recordToMap(input: Record<string, string[]>): TriageMap {
  const out = new Map<string, string[]>();
  for (const [id, reasons] of Object.entries(input ?? {})) {
    out.set(id, [...reasons]);
  }
  return out;
}

function ensureSeeded(): void {
  if (seeded) return;
  seeded = true;
  void refreshTriage();
}
