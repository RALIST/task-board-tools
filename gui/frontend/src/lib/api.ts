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
  GetTask,
  ListAttachments,
  LoadBoard,
  LoadBoardWithMode,
  MoveTask,
  OpenAttachment,
  PullTask,
  ReadyTask,
  RemoveAttachments,
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
import { Status as AutoGroomStatusBinding } from '../../bindings/tools/tb-gui/app/autogroomcoordinator';
import { Status as AutoReviewStatusBinding } from '../../bindings/tools/tb-gui/app/autoreviewcoordinator';
import * as SettingsService from '../../bindings/tools/tb-gui/app/settingsservice';
import type { AutoImplementFilter } from '$lib/autoImplementFilter';
import type {
  Attachment,
  AutoGroomStatus,
  AutoReviewStatus,
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
  AutoGroomStatus,
  AutoReviewStatus,
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
  GetMaxWorkersLimit: () => Promise<number>;
  SetMaxWorkers: (n: number) => Promise<void>;
  GetAgentTimeoutMinutes: () => Promise<number>;
  SetAgentTimeoutMinutes: (n: number) => Promise<void>;
  GetDefaultAgent: () => Promise<string>;
  SetDefaultAgent: (agent: string) => Promise<void>;
  GetCLIPath: () => Promise<string>;
  SetCLIPath: (path: string) => Promise<void>;
  GetPeriodicRecoveryEnabled: () => Promise<boolean>;
  SetPeriodicRecoveryEnabled: (enabled: boolean) => Promise<void>;
  GetAutoGroomEnabled: () => Promise<boolean>;
  SetAutoGroomEnabled: (enabled: boolean) => Promise<void>;
  GetAutoGroomSettleMinutes: () => Promise<number>;
  SetAutoGroomSettleMinutes: (n: number) => Promise<void>;
  GetAutoImplementEnabled: () => Promise<boolean>;
  SetAutoImplementEnabled: (enabled: boolean) => Promise<void>;
  GetAutoImplementQuery: () => Promise<AutoImplementFilter>;
  SetAutoImplementQuery: (filter: AutoImplementFilter) => Promise<void>;
  GetAutoReviewEnabled: () => Promise<boolean>;
  SetAutoReviewEnabled: (enabled: boolean) => Promise<void>;
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

// pullTask pulls a specific ready task into in-progress. Rejects when
// the task is not currently in ready.
export async function pullTask(id: string): Promise<void> {
  await PullTask(id);
}

export async function submitReview(id: string): Promise<void> {
  await SubmitReview(id);
}

export async function closeTask(id: string): Promise<void> {
  await CloseTask(id);
}

export async function editTaskBody(id: string, newBody: string): Promise<void> {
  await EditTaskBody(id, newBody);
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

// resumeAgent continues the latest captured terminal agent session
// (TB-130/TB-252). The Wails binding rejects with ErrCannotResume when
// there is no captured session or the task is queued/running/non-terminal;
// `needs-user` is rejected separately so the user clears that handoff first.
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

function normalizeProjectPath(path: string): string {
  const trimmed = path.trim().replace(/\\/g, '/').replace(/\/+$/, '');
  if (trimmed === '' && path.startsWith('/')) return '/';
  return trimmed;
}

function parentDirectory(path: string): string {
  const normalized = normalizeProjectPath(path);
  if (!normalized || normalized === '/') return '';
  const idx = normalized.lastIndexOf('/');
  if (idx < 0) return '';
  if (idx === 0) return '/';
  return normalized.slice(0, idx);
}

export function suggestBoardDialogDirectory(
  projectRoot: string,
  recents: RecentBoard[] = [],
): string | undefined {
  const activeRoot = normalizeProjectPath(projectRoot);
  for (const recent of recents) {
    const recentRoot = normalizeProjectPath(recent.projectRoot ?? '');
    if (!recentRoot || recentRoot === activeRoot) continue;
    const parent = parentDirectory(recentRoot);
    if (parent) return parent;
  }

  return parentDirectory(activeRoot) || undefined;
}

export async function pickBoardDialog(directory?: string): Promise<string> {
  // Use the runtime dialog from the click handler so Wails attaches each fresh
  // picker to the active window; the Go service method remains for native menu
  // actions that already run outside the webview.
  const options: Parameters<typeof Dialogs.OpenFile>[0] & { Directory?: string } = {
    CanChooseDirectories: true,
    CanChooseFiles: false,
    CanCreateDirectories: false,
    AllowsMultipleSelection: false,
    Title: 'Open tb board',
    Message: 'Pick a directory that contains .tb.yaml',
    ButtonText: 'Open',
  };
  const startDirectory = normalizeProjectPath(directory ?? '');
  if (startDirectory) options.Directory = startDirectory;

  const result = await Dialogs.OpenFile(options);
  return Array.isArray(result) ? (result[0] ?? '') : result;
}

export async function listRecentBoards(): Promise<RecentBoard[]> {
  return await SettingsService.ListRecentBoards();
}

export async function getProjectRoot(): Promise<string> {
  return await SettingsService.GetProjectRoot();
}

export async function getAutoGroomStatus(): Promise<AutoGroomStatus> {
  return await AutoGroomStatusBinding();
}

export async function getAutoReviewStatus(): Promise<AutoReviewStatus> {
  return await AutoReviewStatusBinding();
}

export async function getMaxWorkers(): Promise<number> {
  return await requireSettingsMethod('GetMaxWorkers')();
}

export async function getMaxWorkersLimit(): Promise<number> {
  return await requireSettingsMethod('GetMaxWorkersLimit')();
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

export async function getPeriodicRecoveryEnabled(): Promise<boolean> {
  return await requireSettingsMethod('GetPeriodicRecoveryEnabled')();
}

export async function setPeriodicRecoveryEnabled(enabled: boolean): Promise<void> {
  await requireSettingsMethod('SetPeriodicRecoveryEnabled')(enabled);
}

export async function getAutoGroomEnabled(): Promise<boolean> {
  return await requireSettingsMethod('GetAutoGroomEnabled')();
}

export async function setAutoGroomEnabled(enabled: boolean): Promise<void> {
  await requireSettingsMethod('SetAutoGroomEnabled')(enabled);
}

export async function getAutoGroomSettleMinutes(): Promise<number> {
  return await requireSettingsMethod('GetAutoGroomSettleMinutes')();
}

export async function setAutoGroomSettleMinutes(n: number): Promise<void> {
  await requireSettingsMethod('SetAutoGroomSettleMinutes')(n);
}

export async function getAutoImplementEnabled(): Promise<boolean> {
  return await requireSettingsMethod('GetAutoImplementEnabled')();
}

export async function setAutoImplementEnabled(enabled: boolean): Promise<void> {
  await requireSettingsMethod('SetAutoImplementEnabled')(enabled);
}

export async function getAutoImplementQuery(): Promise<AutoImplementFilter> {
  return await requireSettingsMethod('GetAutoImplementQuery')();
}

export async function setAutoImplementQuery(filter: AutoImplementFilter): Promise<void> {
  await requireSettingsMethod('SetAutoImplementQuery')(filter);
}

export async function getAutoReviewEnabled(): Promise<boolean> {
  return await requireSettingsMethod('GetAutoReviewEnabled')();
}

export async function setAutoReviewEnabled(enabled: boolean): Promise<void> {
  await requireSettingsMethod('SetAutoReviewEnabled')(enabled);
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
  const message = extractErrorMessage(err, new Set(), 0);
  return message || 'unknown error';
}

function extractErrorMessage(value: unknown, seen: Set<object>, depth: number): string {
  if (value == null) return '';
  if (typeof value === 'string') return errorTextFromString(value, seen, depth);
  if (value instanceof Error) {
    const cause = (value as Error & { cause?: unknown }).cause;
    const ownMessage = errorTextFromString(value.message, seen, depth);
    if (cause !== undefined && isCliValidationEnvelope(ownMessage)) {
      const causeStderr = extractStderrMessage(cause);
      if (causeStderr) return causeStderr;
      return ownMessage;
    }
    if (cause !== undefined && isRuntimeEnvelope(value.name, ownMessage)) {
      const causeMessage = extractErrorMessage(cause, seen, depth + 1);
      if (causeMessage) return causeMessage;
    }
    return ownMessage;
  }
  if (typeof value === 'object') {
    if (seen.has(value)) return '';
    seen.add(value);
    const record = value as Record<string, unknown>;
    const stderr = stringField(record, 'Stderr') ?? stringField(record, 'stderr');
    if (stderr) return errorTextFromString(stderr, seen, depth + 1);

    const cause = record.cause ?? record.Cause;
    const ownMessage = firstStringField(record, ['message', 'Message', 'error', 'Error']);
    const message = ownMessage ? errorTextFromString(ownMessage, seen, depth + 1) : '';
    const name = firstStringField(record, ['name', 'Name']);
    if (cause !== undefined) {
      const causeStderr = extractStderrMessage(cause);
      if (causeStderr && (!message || isRuntimeEnvelope(name, message) || isCliValidationEnvelope(message))) {
        return causeStderr;
      }
      const causeMessage = extractErrorMessage(cause, seen, depth + 1);
      if (causeMessage && (!message || isRuntimeEnvelope(name, message))) return causeMessage;
      if (!message) return causeMessage;
    }
    return message;
  }
  try {
    return errorTextFromString(String(value), seen, depth);
  } catch {
    return '';
  }
}

function errorTextFromString(text: string, seen: Set<object>, depth: number): string {
  const trimmed = text.trim();
  if (!trimmed) return '';

  const parsed = parseStructuredErrorString(trimmed);
  if (isStructuredErrorValue(parsed)) {
    const extracted = extractErrorMessage(parsed, seen, depth + 1);
    if (extracted) return extracted;
  }

  const stderr = extractStderrFromDebugString(trimmed);
  if (stderr) return stderr.trim();

  const withoutRuntime = trimmed.replace(/^RuntimeError:?\s*/i, '').trim();
  if (!withoutRuntime || withoutRuntime === '[object Object]') return '';
  if (looksLikeDebugEnvelope(withoutRuntime)) return '';
  return withoutRuntime;
}

function parseStructuredErrorString(text: string): unknown | undefined {
  for (const candidate of jsonCandidates(text)) {
    try {
      return JSON.parse(candidate);
    } catch {
      // Try the next candidate below.
    }
  }
  return undefined;
}

function isStructuredErrorValue(value: unknown): value is object {
  return typeof value === 'object' && value !== null;
}

function extractStderrMessage(value: unknown): string {
  const stderr = findStderr(value, new Set(), 0);
  return stderr ? errorTextFromString(stderr, new Set(), 0) : '';
}

function findStderr(value: unknown, seen: Set<object>, depth: number): string {
  if (value == null || depth > 8) return '';
  if (typeof value === 'string') return extractStderrFromDebugString(value);
  if (value instanceof Error) {
    return findStderr((value as Error & { cause?: unknown }).cause, seen, depth + 1);
  }
  if (typeof value !== 'object') return '';
  if (seen.has(value)) return '';
  seen.add(value);

  if (Array.isArray(value)) {
    for (const item of value) {
      const nested = findStderr(item, seen, depth + 1);
      if (nested) return nested;
    }
    return '';
  }

  const record = value as Record<string, unknown>;
  const direct = stringField(record, 'Stderr') ?? stringField(record, 'stderr');
  if (direct) return direct;
  return findStderr(record.cause ?? record.Cause, seen, depth + 1);
}

function jsonCandidates(text: string): string[] {
  const candidates = [text];
  const objectStart = text.indexOf('{');
  if (objectStart > 0) candidates.push(text.slice(objectStart));
  const arrayStart = text.indexOf('[');
  if (arrayStart > 0) candidates.push(text.slice(arrayStart));
  return candidates;
}

function extractStderrFromDebugString(text: string): string {
  const match = text.match(/["']?Stderr["']?\s*[:=]\s*["']?([\s\S]*?)(?:["']?\s+["']?(?:Cause|Kind|Op|Args)["']?\s*[:=]|$)/);
  return match?.[1]?.trim() ?? '';
}

function firstStringField(record: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const value = stringField(record, key);
    if (value) return value;
  }
  return '';
}

function stringField(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key];
  return typeof value === 'string' && value.trim() ? value : undefined;
}

function isRuntimeEnvelope(name: string | undefined, message: string): boolean {
  return name === 'RuntimeError' || /^RuntimeError\b/i.test(message);
}

function isCliValidationEnvelope(message: string): boolean {
  return /^tb\s+\S+:\s+validation:/i.test(message);
}

function looksLikeDebugEnvelope(message: string): boolean {
  if (message === '[object Object]') return true;
  if (!message.startsWith('{') && !message.startsWith('[')) return false;
  return /\b(?:Kind|Op|Args|Cause|Stderr)\b/.test(message);
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
