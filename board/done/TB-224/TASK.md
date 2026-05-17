# TB-224: Support task-root attachments

**Type:** improvement
**Priority:** P2
**Size:** L
**Agent:** codex
**AgentStatus:** success
**Module:** cli/gui
**Tags:** attachments,folder-form,compatibility
**Branch:** —

## Goal

Let users attach and manage files directly in the task directory (`<status>/<ID>/`) instead of requiring them to know about or create a special `attachments/` subdirectory.

## Context

The current folder-form contract stores user attachments under `<status>/<ID>/attachments/<filename>` and writes `## Attachments` entries as `attachments/<filename>`. That behavior is implemented across `cli/attach.go`, `gui/app/attachments.go`, `gui/internal/watcher/watcher.go`, the frontend attachment drawer/drop flows, and the docs/CLI help.

This task changes the user-facing attachment location for new attachments to the task root while preserving existing boards that already contain `attachments/` directories.

## Constraints

Keep task internals reserved: `TASK.md`, `.agent-state.jsonl`, `.agent-logs/`, hidden temp/staging paths, and any other implementation-owned files must never be listed, opened, or removed as user attachments.

Preserve compatibility with existing `attachments/<name>` files until a separate explicit migration removes that legacy layout; do not strand, silently delete, or hide existing attachment files.

All attachment mutations must still take `.board.lock`, publish files atomically, regenerate `BOARD.md`, and update `## Attachments` only through structured CLI mutation paths.

Legacy file-form tasks should continue to promote to folder form when attaching unless the implementation explicitly documents and tests a narrower behavior.

The GUI must continue delegating attachment add/remove mutations to the CLI instead of writing or deleting attachment files directly.

## Acceptance Criteria

- [x] Folder-form attachment docs and CLI help state that new user attachments live directly under `<status>/<ID>/`, and document the reserved internal names that are excluded from attachment behavior.
- [x] `tb attach <ID> <path>...` writes new attachments to the task root, rejects basename collisions with task internals or existing attachments, preserves file-form auto-promotion atomicity, writes root-relative `## Attachments` entries, and leaves no half-published files on failure.
- [x] Existing legacy `attachments/<name>` entries and files remain readable, openable, and removable during the compatibility period; tests cover a task containing both legacy `attachments/` files and new task-root files.
- [x] `tb attach --rm` removes only user attachments, refuses reserved names, traversal, symlink escapes, and unrelated directories, and cannot delete `TASK.md`, agent artifacts, hidden temp/staging paths, or other implementation-owned files.
- [x] GUI list/open/add/remove and picker/drag-and-drop flows treat task-root files as attachments, still delegate mutations to the CLI, and refresh from watcher events without duplicate manual refreshes.
- [x] Watcher coverage proves task-root attachment create/remove/rename events produce the expected board/task refresh behavior while ignored internal files remain ignored.
- [x] Verification includes CLI tests, GUI backend tests, watcher tests, frontend API/TaskDrawer tests if UI or bindings change, plus `cd cli && go test ./...`, `cd gui && go test ./...`, and `cd gui/frontend && npm test && npm run check`.
- [x] Manual smoke note: in the real GUI, attach via picker and drag-and-drop, open and remove a task-root file, confirm existing legacy `attachments/` files still display/open/remove, and verify reserved/internal files do not appear in the drawer.

## Related Tasks

- **TB-93** — Folder-form tasks + attachments epic (relationship: changes the shipped attachment storage contract)
- **TB-94** — Spec folder-task contract in docs/ARCHITECTURE.md (relationship: source contract to update)
- **TB-99** — CLI attach with auto-promotion (relationship: CLI add/promotion behavior to change)
- **TB-100** — CLI remove attachments safely (relationship: removal safety behavior to preserve)
- **TB-103** — GUI TaskDrawer attachments list/add/remove (relationship: drawer surface to update)
- **TB-104** — GUI drag-and-drop attachments (relationship: drop surface to update)

## Log

- 2026-05-17: Created
- 2026-05-17: Edited agent=codex
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Edited type=improvement, size=L, module=cli/gui, tags=attachments,folder-form,compatibility, title=Support task-root attachments, goal
- 2026-05-17: Edited acceptance
- 2026-05-17: Edited agentstatus=success
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Started — moved to in-progress
- 2026-05-17: Edited agentstatus=failed
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Edited agentstatus=failed
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Attached tb224-picker.txt
- 2026-05-17: Removed attachments: attachments/tb224-picker.txt
- 2026-05-17: Edited agentstatus=failed
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Attached tb224-picker-root.txt
- 2026-05-17: Removed attachments: attachments/tb224-picker-root.txt
- 2026-05-17: Attached tb224-picker-root.txt
- 2026-05-17: Removed attachments: attachments/tb224-picker-root.txt
- 2026-05-17: Attached tb224-picker-root.txt
- 2026-05-17: Attached tb224-drop-root.txt
- 2026-05-17: Removed attachments: attachments/tb224-legacy.txt
- 2026-05-17: Removed attachments: tb224-drop-root.txt
- 2026-05-17: Removed attachments: tb224-picker-root.txt
- 2026-05-17: Edited agentstatus=success, acceptance
- 2026-05-17: Completed task-root attachment support; verified CLI, GUI backend, watcher, frontend tests, code review, and real GUI picker/drag-drop/open/remove smoke including legacy compatibility and reserved/internal-file exclusion.
- 2026-05-17: Done
