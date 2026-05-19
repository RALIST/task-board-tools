# TB-190: Implement autosave instead of save with buttons

**Type:** improvement
**Priority:** P2
**Size:** M
**Agent:** claude
**AgentStatus:** running
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

- [x] The Details rail no longer renders a metadata Save button; changing priority, type, size, module, or tags schedules an autosave after a short idle debounce and persists through the existing `editTask`/`tb edit` service path.
- [x] Multiple metadata edits made before the debounce fires are coalesced into the latest payload, in-flight saves are serialized or superseded safely, and the UI shows accessible per-section state for dirty/saving/saved/error without losing the user's current field values.
- [x] Description/body edit mode no longer renders a `Save body` button; body changes autosave through `editTaskBody` after idle and are flushed on close/task switch when practical, while `Edit` and `Discard` remain available for entering or abandoning edit mode.
- [x] Save failures are visible via toast and/or inline status, leave the user's draft intact for retry, and do not mark the task as saved until the backend call succeeds and the watcher refresh catches up.
- [x] Existing explicit command buttons remain explicit actions: Run, Groom, Cancel, Archive, Add files, and attachment removal are not converted to autosave.
- [x] The existing CLI clear-field limitation is still handled honestly: attempts to clear unsupported metadata fields are surfaced to the user and do not leave the drawer showing a value that was not persisted. (Pragmatic reading: snap-back after the debounce fires is the intent; the field briefly shows the empty draft inside the 600 ms debounce window.)
- [x] Automated coverage exercises the autosave state machine for debouncing/coalescing, success refresh, backend failure, task switch/close flush, and unsupported clear-field handling; run `cd gui/frontend && npm test` or the closest focused Vitest target plus `npm run check`.
- [ ] Manual GUI smoke: open a task in the desktop app, edit metadata and description text without pressing any Save button, close/reopen the drawer, and verify the task file, regenerated board, toasts/status, and rendered drawer all reflect the persisted changes. *(not run by the agent — please verify in the desktop app before merging)*

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
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Implemented metadata + body autosave in `gui/frontend/src/lib/components/TaskDrawer.svelte`. Replaced the Details Save and `Save body` buttons with a per-section autosave status chip (`Unsaved` / `Saving…` / `Saved` / `Save failed`) plus an inline Retry on failure. The 600 ms debounce coalesces edits; an in-flight save serializes a follow-up via a `pendingResave` flag; the saved chip only promotes after the watcher refresh catches up (per AC #4). A `userTouchedMetadata` flag gates `fetchOnce` from clobbering an in-progress draft on `board:reloaded` / `task:updated`. Body autosave hooks the BodyEditor's `onDirtyChange`. Pending saves flush on task switch and on the × close button (the prior `window.confirm` "unsaved body edits" dialog was removed — UX behavior change with autosave). Cmd/Ctrl+S now flushes the pending body autosave rather than acting as primary save. Reactive-loop traps were hit during initial wiring; both the saved-indicator promotion effects and the form-watcher effect now route writes through `untrack`, and the cleanup uses `untrack` defensively so future refactors can't reintroduce the loop. Added 9 vitest cases (debounce/coalesce, form-reset guard, saved-after-watcher gating, error+Retry, unmount flush, close-button flush, in-flight resave, unsupported clear, "no Save body in edit mode") and reset more api mocks in `beforeEach`. `cd gui/frontend && npm test` → 177/177; `npm run check` clean. Docs: F3.3 / F3.4 in `docs/FEATURES.md` updated to describe autosave behavior. AC #6 read pragmatically — the field snaps back after the debounce, not mid-keystroke. AC #8 (manual GUI smoke) not run by the agent and is left unchecked for the user to verify in the desktop app.

