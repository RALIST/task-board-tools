// Typed wrappers over the auto-generated Wails bindings.
//
// The generator emits a CancellablePromise from @wailsio/runtime; for app code
// we just want plain Promises, so each wrapper awaits-and-returns. Co-locating
// these here also gives the rest of the frontend a single import surface
// instead of poking at the generated `bindings/tools/tb-gui/app` paths.

import {
  GetTask,
  LoadBoard,
} from '../../bindings/tools/tb-gui/app/boardservice';
import {
  GetBoardInfo,
  GetProjectRoot,
  ListRecentBoards,
  OpenBoard,
  PickBoardDialog,
} from '../../bindings/tools/tb-gui/app/settingsservice';
import type { BoardInfo, BoardSnapshot, RecentBoard, Task, TaskDetail } from '../../bindings/tools/tb-gui/app/models';

export type { BoardInfo, BoardSnapshot, RecentBoard, Task, TaskDetail };

export async function loadBoard(): Promise<BoardSnapshot> {
  return await LoadBoard();
}

export async function getTask(id: string): Promise<TaskDetail> {
  return await GetTask(id);
}

export async function openBoard(projectRoot: string): Promise<void> {
  await OpenBoard(projectRoot);
}

export async function pickBoardDialog(): Promise<string> {
  return await PickBoardDialog();
}

export async function listRecentBoards(): Promise<RecentBoard[]> {
  return await ListRecentBoards();
}

export async function getProjectRoot(): Promise<string> {
  return await GetProjectRoot();
}

export async function getBoardInfo(): Promise<BoardInfo> {
  return await GetBoardInfo();
}

// Error-message heuristics — the Wails bridge stringifies Go errors, so
// callers can branch on them for toast styling.
export function isNoTbYamlError(err: unknown): boolean {
  return errorString(err).includes('no .tb.yaml');
}

export function isCancelledError(err: unknown): boolean {
  const s = errorString(err);
  return s.includes('cancelled') || s.includes('user cancelled');
}

export function isNoBoardError(err: unknown): boolean {
  return errorString(err).includes('no board selected');
}

export function errorString(err: unknown): string {
  if (err == null) return '';
  if (err instanceof Error) return err.message;
  if (typeof err === 'string') return err;
  try {
    return String(err);
  } catch {
    return 'unknown error';
  }
}
