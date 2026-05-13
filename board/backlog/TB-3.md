# TB-3: M3: GUI mutations, DnD, and inline editor

**Type:** feature
**Priority:** P1
**Size:** XL
**Module:** gui
**Tags:** milestone-m3,gui,dnd,editor,epic
**Branch:** —

## Goal

Make the GUI write-capable: create/edit/move/close via exec tb, optimistic drag-and-drop with revert-on-conflict, CodeMirror body editor with direct-write-under-flock for body sections, and FilterBar.

## Context

Add the write side to the GUI. Mutations go through `exec tb` so the CLI stays the only path that takes `.board.lock`. The single explicit exception is `EditTaskBody` (free-form `## Goal`, `## Context`, `## Acceptance Criteria` text) which acquires `.board.lock` directly, preserves the header + first 15 metadata lines, appends a log entry, atomically renames, then triggers `tb regenerate`. DnD is optimistic with conflict-revert. See plan M3 and `docs/ARCHITECTURE.md` → "Locking and atomic writes".

## Acceptance Criteria

- [ ] `CreateTask`, `EditTask`, `MoveTask`, `CloseTask`, `Regenerate` services exec the CLI
- [ ] `EditTaskBody` acquires `.board.lock`, preserves header + metadata, appends log entry, writes atomically, then triggers `tb regenerate`
- [ ] `svelte-dnd-action` moves cards between columns with optimistic UI
- [ ] Concurrent `tb mv` during a drag produces a conflict toast and reverts
- [ ] `CreateTaskDialog` creates a new backlog task end-to-end
- [ ] `FilterBar` filters cards client-side (type, priority, module, tags, parent, agent)
- [ ] Toast component surfaces errors from any mutation

## Related Tasks

- **TB-2** — Prerequisite (skeleton + watcher)
- **TB-4** — Builds on this (needs assign UI hooks)

## Log

- 2026-05-13: Created
