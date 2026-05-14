# TB-190: Implement autosave instead of save with buttons

**Type:** improvement
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** gui,frontend,autosave,drawer
**Branch:** —

## Goal

Make the task detail drawer persist editable metadata and description/body changes automatically, without requiring separate Save buttons for fields that are part of the task record.

## Context

Current task-detail save surfaces:
- `gui/frontend/src/lib/components/TaskDrawer.svelte:375` computes dirty metadata for priority, type, size, module, and tags.
- `gui/frontend/src/lib/components/TaskDrawer.svelte:424` saves metadata through `editTask`, which delegates to `tb edit`.
- `gui/frontend/src/lib/components/TaskDrawer.svelte:491` saves description/body edits through `editTaskBody`, backed by `BoardService.EditTaskBody`.
- `gui/frontend/src/lib/components/TaskDrawer.svelte:754` renders the Details Save button, and `:694` renders the body Save button.
- `docs/FEATURES.md:110` and `:115` describe the current explicit-save behavior for metadata and body editing.
- `docs/ARCHITECTURE.md:242` documents the persistence split: structured edits go through `tb`; body edits use the backend lock/write path.

Constraints and non-goals:
- Preserve the architecture boundary: metadata autosave should still call the existing `editTask`/`tb edit` path, and body autosave should still call `editTaskBody`; do not introduce frontend filesystem writes.
- Debounce or batch autosaves so typing does not run one CLI command per keystroke, and flush pending changes before closing the drawer or switching tasks when practical.
- Keep explicit action buttons for commands that are not field edits, such as Run, Groom, Cancel, Archive, and attachment add/remove.
- Do not change CLI semantics in this task. If `tb edit` still cannot clear a field, autosave must surface the existing limitation instead of silently pretending the clear succeeded.
- Avoid overwriting newer external changes from the CLI or watcher refreshes while the user has an in-progress draft.

## Acceptance Criteria

- [ ] The Details rail no longer renders a metadata Save button; changing priority, type, size, module, or tags schedules an autosave after a short idle debounce and persists through the existing `editTask`/`tb edit` service path.
- [ ] Multiple metadata edits made before the debounce fires are coalesced into the latest payload, in-flight saves are serialized or superseded safely, and the UI shows accessible per-section state for dirty/saving/saved/error without losing the user's current field values.
- [ ] Description/body edit mode no longer renders a `Save body` button; body changes autosave through `editTaskBody` after idle and are flushed on close/task switch when practical, while `Edit` and `Discard` remain available for entering or abandoning edit mode.
- [ ] Save failures are visible via toast and/or inline status, leave the user's draft intact for retry, and do not mark the task as saved until the backend call succeeds and the watcher refresh catches up.
- [ ] Existing explicit command buttons remain explicit actions: Run, Groom, Cancel, Archive, Add files, and attachment removal are not converted to autosave.
- [ ] The existing CLI clear-field limitation is still handled honestly: attempts to clear unsupported metadata fields are surfaced to the user and do not leave the drawer showing a value that was not persisted.
- [ ] Automated coverage exercises the autosave state machine for debouncing/coalescing, success refresh, backend failure, task switch/close flush, and unsupported clear-field handling; run `cd gui/frontend && npm test` or the closest focused Vitest target plus `npm run check`.
- [ ] Manual GUI smoke: open a task in the desktop app, edit metadata and description text without pressing any Save button, close/reopen the drawer, and verify the task file, regenerated board, toasts/status, and rendered drawer all reflect the persisted changes.

## Related Tasks

- **TB-79** — Existing Agent dropdown autosaves on change; preserve its default-agent guard and avoid reintroducing hidden assignment writes.
- **TB-186** — Parent editing from the task page is a complementary field-editing workflow and should follow this autosave model if both tasks touch the drawer.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited priority=P2, type=improvement, size=M, module=gui, tags=gui,frontend,autosave,drawer
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success
- 2026-05-15: Edited agentstatus=success

