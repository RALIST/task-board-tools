import { writable } from 'svelte/store';

export interface StartupGraceState {
  active: boolean;
  boardKey: string;
  remainingSeconds: number;
}

const inactiveState: StartupGraceState = {
  active: false,
  boardKey: '',
  remainingSeconds: 0,
};

export const startupGraceStore = writable<StartupGraceState>(inactiveState);

let timer: ReturnType<typeof setInterval> | null = null;
let generation = 0;

export function startStartupGrace(boardKey: string, seconds: number): void {
  generation += 1;
  clearTimer();

  const durationSeconds = Math.max(0, Math.floor(seconds));
  if (!boardKey || durationSeconds === 0) {
    startupGraceStore.set(inactive());
    return;
  }

  const runGeneration = generation;
  const endsAtMs = Date.now() + durationSeconds * 1000;
  startupGraceStore.set({
    active: true,
    boardKey,
    remainingSeconds: durationSeconds,
  });

  timer = setInterval(() => {
    if (runGeneration !== generation) return;
    const remainingSeconds = Math.max(0, Math.ceil((endsAtMs - Date.now()) / 1000));
    if (remainingSeconds === 0) {
      clearTimer();
      startupGraceStore.set(inactive());
      return;
    }
    startupGraceStore.set({
      active: true,
      boardKey,
      remainingSeconds,
    });
  }, 1000);
}

export function cancelStartupGrace(): void {
  generation += 1;
  clearTimer();
  startupGraceStore.set(inactive());
}

export function _resetStartupGraceForTesting(): void {
  cancelStartupGrace();
}

function clearTimer(): void {
  if (timer === null) return;
  clearInterval(timer);
  timer = null;
}

function inactive(): StartupGraceState {
  return { ...inactiveState };
}
