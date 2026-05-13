// Typed wrappers over the auto-generated Wails bindings.
//
// The generator emits a CancellablePromise from @wailsio/runtime; for app code
// we just want plain Promises, so each wrapper awaits-and-returns. Co-locating
// these here also gives the rest of the frontend a single import surface
// instead of poking at the generated `bindings/tools/tb-gui/app` paths.

import {
  CloseTask,
  CreateTask,
  EditTask,
  EditTaskBody,
  GetTask,
  LoadBoard,
  LoadBoardWithMode,
  MoveTask,
  Regenerate,
} from '../../bindings/tools/tb-gui/app/boardservice';
import {
  GetBoardInfo,
  GetProjectRoot,
  ListRecentBoards,
  OpenBoard,
  PickBoardDialog,
} from '../../bindings/tools/tb-gui/app/settingsservice';
import type {
  BoardInfo,
  BoardSnapshot,
  CreateTaskInput,
  CreateTaskResult,
  EditTaskInput,
  RecentBoard,
  Task,
  TaskDetail,
} from '../../bindings/tools/tb-gui/app/models';

export type {
  BoardInfo,
  BoardSnapshot,
  CreateTaskInput,
  CreateTaskResult,
  EditTaskInput,
  RecentBoard,
  Task,
  TaskDetail,
};

export type StatusMode = 'active' | 'all';

export async function loadBoard(mode: StatusMode = 'active'): Promise<BoardSnapshot> {
  if (mode === 'all') return await LoadBoardWithMode('all');
  return await LoadBoard();
}

export async function getTask(id: string): Promise<TaskDetail> {
  return await GetTask(id);
}

export async function createTask(input: CreateTaskInput): Promise<CreateTaskResult> {
  return await CreateTask(input);
}

export async function editTask(id: string, fields: EditTaskInput): Promise<void> {
  await EditTask(id, fields);
}

export async function moveTask(id: string, status: 'backlog' | 'in-progress' | 'done'): Promise<void> {
  await MoveTask(id, status);
}

export async function closeTask(id: string): Promise<void> {
  await CloseTask(id);
}

export async function editTaskBody(id: string, newBody: string): Promise<void> {
  await EditTaskBody(id, newBody);
}

export async function regenerate(): Promise<void> {
  await Regenerate();
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
