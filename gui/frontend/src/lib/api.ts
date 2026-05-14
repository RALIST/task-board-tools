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
  Triage,
} from '../../bindings/tools/tb-gui/app/boardservice';
import {
  AssignAgent,
  CancelRun,
  GetRunLog,
  GroomTask,
  ListRuns,
  RunAgent,
} from '../../bindings/tools/tb-gui/app/agentservice';
import {
  GetBoardInfo,
} from '../../bindings/tools/tb-gui/app/settingsservice';
import * as SettingsService from '../../bindings/tools/tb-gui/app/settingsservice';
import type {
  BoardInfo,
  BoardSnapshot,
  CreateTaskInput,
  CreateTaskResult,
  EditTaskInput,
  RecentBoard,
  Run as BoundRun,
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
type SettingsServiceBindings = typeof SettingsService & {
  GetMaxWorkers: () => Promise<number>;
  SetMaxWorkers: (n: number) => Promise<void>;
  GetAgentTimeoutMinutes: () => Promise<number>;
  SetAgentTimeoutMinutes: (n: number) => Promise<void>;
  GetDefaultAgent: () => Promise<string>;
  SetDefaultAgent: (agent: string) => Promise<void>;
  GetCLIPath: () => Promise<string>;
  SetCLIPath: (path: string) => Promise<void>;
};

const settingsService = SettingsService as unknown as SettingsServiceBindings;

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

export async function getTriage(): Promise<Record<string, string[]>> {
  const raw = await Triage();
  const out: Record<string, string[]> = {};
  for (const [id, reasons] of Object.entries(raw ?? {})) {
    out[id] = [...(reasons ?? [])];
  }
  return out;
}

// --- Agent service wrappers ---

export async function assignAgent(id: string, agent: string): Promise<void> {
  await AssignAgent(id, agent);
}

export async function runAgent(id: string): Promise<string> {
  return await RunAgent(id);
}

export async function groomTask(id: string): Promise<string> {
  return await GroomTask(id);
}

export async function cancelRun(id: string): Promise<void> {
  await CancelRun(id);
}

export async function listRuns(id: string): Promise<BoundRun[]> {
  return await ListRuns(id);
}

export async function getRunLog(taskID: string, runID: string): Promise<string> {
  return await GetRunLog(taskID, runID);
}

export type AgentName = 'claude' | 'codex' | 'none' | '';

export function isRunLogNotFoundError(err: unknown): boolean {
  return errorString(err).includes('run log not found');
}

export async function openBoard(projectRoot: string): Promise<void> {
  await SettingsService.OpenBoard(projectRoot);
}

export async function pickBoardDialog(): Promise<string> {
  return await SettingsService.PickBoardDialog();
}

export async function listRecentBoards(): Promise<RecentBoard[]> {
  return await SettingsService.ListRecentBoards();
}

export async function getProjectRoot(): Promise<string> {
  return await SettingsService.GetProjectRoot();
}

export async function getBoardInfo(): Promise<BoardInfo> {
  return await GetBoardInfo();
}

export async function getMaxWorkers(): Promise<number> {
  return await requireSettingsMethod('GetMaxWorkers')();
}

export async function setMaxWorkers(n: number): Promise<void> {
  await requireSettingsMethod('SetMaxWorkers')(n);
}

export async function getAgentTimeoutMinutes(): Promise<number> {
  return await requireSettingsMethod('GetAgentTimeoutMinutes')();
}

export async function setAgentTimeoutMinutes(n: number): Promise<void> {
  await requireSettingsMethod('SetAgentTimeoutMinutes')(n);
}

export async function getDefaultAgent(): Promise<string> {
  return await requireSettingsMethod('GetDefaultAgent')();
}

export async function setDefaultAgent(agent: string): Promise<void> {
  await requireSettingsMethod('SetDefaultAgent')(agent);
}

export async function getCLIPath(): Promise<string> {
  return await requireSettingsMethod('GetCLIPath')();
}

export async function setCLIPath(path: string): Promise<void> {
  await requireSettingsMethod('SetCLIPath')(path);
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

function requireSettingsMethod<K extends keyof SettingsServiceBindings>(
  name: K,
): SettingsServiceBindings[K] {
  const method = settingsService[name];
  if (typeof method !== 'function') {
    throw new Error(`${String(name)} binding is missing; regenerate Wails frontend bindings`);
  }
  return method;
}
