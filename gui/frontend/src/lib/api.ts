// Typed wrappers over the auto-generated Wails bindings.
//
// The generator emits a CancellablePromise from @wailsio/runtime; for app code
// we just want plain Promises, so each wrapper awaits-and-returns. Co-locating
// these here also gives the rest of the frontend a single import surface
// instead of poking at the generated `bindings/tools/tb-gui/app` paths.

import {
  AddAttachments,
  CloseTask,
  CreateTask,
  EditTask,
  EditTaskBody,
  FailReview,
  GetTask,
  ListAttachments,
  LoadBoard,
  LoadBoardWithMode,
  MoveTask,
  OpenAttachment,
  PullNext,
  PullTask,
  ReadyTask,
  Regenerate,
  RemoveAttachments,
  SetReviewFindings,
  SetReviewTarget,
  SetReviewerNotes,
  SubmitReview,
  Triage,
} from '../../bindings/tools/tb-gui/app/boardservice';
import { Dialogs } from '@wailsio/runtime';
import {
  AssignAgent,
  CancelRun,
  GetRunLog,
  GroomTask,
  ListRuns,
  ResumeAgent,
  ReviewTask,
  RunAgent,
} from '../../bindings/tools/tb-gui/app/agentservice';
import {
  GetBoardInfo,
} from '../../bindings/tools/tb-gui/app/settingsservice';
import * as SettingsService from '../../bindings/tools/tb-gui/app/settingsservice';
import type {
  Attachment,
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
  Attachment,
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

// Rename a task's title via the structured edit path. Whitespace is trimmed
// client-side so the CLI never sees a value that would trigger its
// "must not be empty" validation. Callers should detect no-op renames before
// invoking — passing the unchanged title still resolves successfully (the
// CLI treats it as a silent no-op), but it's wasted work.
export async function renameTask(id: string, newTitle: string): Promise<void> {
  const trimmed = newTitle.trim();
  if (trimmed === '') {
    throw new Error('Title cannot be empty');
  }
  await EditTask(id, { title: trimmed } as EditTaskInput);
}

export async function moveTask(
  id: string,
  status: 'backlog' | 'ready' | 'in-progress' | 'code-review' | 'done',
): Promise<void> {
  await MoveTask(id, status);
}

// readyTask promotes a backlog task into ready (canonical kanban
// commitment column). The CLI enforces the triage gate, so a task missing
// priority or with a placeholder goal is rejected here with a stderr
// message in the Wails error.
export async function readyTask(id: string): Promise<void> {
  await ReadyTask(id);
}

// pullNext pulls the highest-priority oldest task from ready into
// in-progress. No-op (resolves successfully) when the ready column is
// empty.
export async function pullNext(): Promise<void> {
  await PullNext();
}

// pullTask pulls a specific ready task into in-progress. Rejects when
// the task is not currently in ready.
export async function pullTask(id: string): Promise<void> {
  await PullTask(id);
}

export async function submitReview(id: string): Promise<void> {
  await SubmitReview(id);
}

export async function setReviewTarget(id: string, body: string): Promise<void> {
  await SetReviewTarget(id, body);
}

export async function setReviewerNotes(id: string, body: string): Promise<void> {
  await SetReviewerNotes(id, body);
}

export async function setReviewFindings(id: string, body: string): Promise<void> {
  await SetReviewFindings(id, body);
}

export async function failReview(id: string, findings: string): Promise<void> {
  await FailReview(id, findings);
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

// --- Attachment wrappers ---

export async function listAttachments(id: string): Promise<Attachment[]> {
  // Pass the binding rows through unchanged so any new field Wails adds
  // (modTime, owner, …) survives. The earlier re-map silently stripped
  // anything beyond {name, size}.
  return (await ListAttachments(id)) ?? [];
}

export async function addAttachments(id: string, paths: string[]): Promise<void> {
  await AddAttachments(id, paths);
}

export async function removeAttachments(id: string, names: string[]): Promise<void> {
  await RemoveAttachments(id, names);
}

export async function openAttachment(id: string, name: string): Promise<void> {
  await OpenAttachment(id, name);
}

export async function pickAttachmentFiles(): Promise<string[]> {
  // Multi-select file picker rooted at the user's home. The Wails dialog
  // returns absolute paths, which is what `tb attach` expects.
  const result: unknown = await Dialogs.OpenFile({
    CanChooseDirectories: false,
    CanChooseFiles: true,
    CanCreateDirectories: false,
    AllowsMultipleSelection: true,
    Title: 'Add attachments',
    Message: 'Pick one or more files to attach',
    ButtonText: 'Attach',
  });
  if (Array.isArray(result)) {
    return result.filter((p): p is string => typeof p === 'string' && p.length > 0);
  }
  if (typeof result === 'string' && result.length > 0) return [result];
  return [];
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

export async function reviewTask(id: string): Promise<string> {
  return await ReviewTask(id);
}

// resumeAgent continues an `interrupted` task's prior agent session
// (TB-130). The Wails binding rejects with ErrCannotResume when
// AgentStatus != "interrupted" and ErrNotResumable when the latest run
// has no captured session id — surface those upstream as the toast
// message via the standard error handling.
export async function resumeAgent(id: string): Promise<string> {
  return await ResumeAgent(id);
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

// Defaults mirror `tb init` and the Go-side InitBoardPathDefault /
// InitPrefixDefault constants so the dialog and the backend agree when the
// user accepts the empty placeholders.
export const INIT_BOARD_PATH_DEFAULT = 'board';
export const INIT_PREFIX_DEFAULT = 'PR';
export const INIT_PREFIX_MAX_LEN = 10;
export const INIT_PREFIX_PATTERN = /^[A-Za-z][A-Za-z0-9]*$/;

// validateInitBoardPath mirrors normalizeInitBoardPath in
// gui/app/settings_service.go so the dialog can surface validation errors
// before paying for a round-trip to the Wails service.
export function validateInitBoardPath(raw: string): string | null {
  const trimmed = raw.trim();
  if (trimmed === '') return null; // empty → backend defaults to "board"
  if (/^[/\\]/.test(trimmed)) {
    return 'Board path must be relative to the project root.';
  }
  if (trimmed === '.' || trimmed === '..') {
    return 'Board path must point to a directory inside the project root.';
  }
  if (trimmed.split(/[/\\]/).some((part) => part === '..')) {
    return 'Board path may not escape the project root.';
  }
  return null;
}

export function validateInitPrefix(raw: string): string | null {
  const trimmed = raw.trim();
  if (trimmed === '') return null; // empty → backend defaults to "PR"
  if (trimmed.length > INIT_PREFIX_MAX_LEN) {
    return `Prefix is too long (max ${INIT_PREFIX_MAX_LEN}).`;
  }
  if (!INIT_PREFIX_PATTERN.test(trimmed)) {
    return 'Prefix must start with a letter and contain only letters or digits.';
  }
  return null;
}

export async function initBoard(
  projectRoot: string,
  boardPath: string,
  prefix: string,
): Promise<void> {
  await SettingsService.InitBoard(projectRoot, boardPath, prefix);
}

export function isAlreadyInitializedError(err: unknown): boolean {
  return errorString(err).includes('.tb.yaml already exists');
}

export async function pickBoardDialog(): Promise<string> {
  // Use the runtime dialog from the click handler so Wails attaches each fresh
  // picker to the active window; the Go service method remains for native menu
  // actions that already run outside the webview.
  const result = await Dialogs.OpenFile({
    CanChooseDirectories: true,
    CanChooseFiles: false,
    CanCreateDirectories: false,
    AllowsMultipleSelection: false,
    Title: 'Open tb board',
    Message: 'Pick a directory that contains .tb.yaml',
    ButtonText: 'Open',
  });
  return Array.isArray(result) ? (result[0] ?? '') : result;
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
