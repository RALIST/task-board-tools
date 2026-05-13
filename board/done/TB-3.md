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

Add the write side to the GUI. Mutations go through `exec tb` so the CLI stays the only path that takes `.board.lock`. The single explicit exception is `EditTaskBody` — free-form body sections (Goal / Context / Acceptance Criteria text) — which acquires `.board.lock` directly, preserves the header + first 15 metadata lines, appends a log entry, atomically renames, then triggers `tb regenerate`. DnD is optimistic with conflict-revert. See plan M3 and `docs/ARCHITECTURE.md` → "Locking and atomic writes".

## Subtasks

- **TB-31** (S) — CLI wrapper: mutation commands (create/edit/mv/close/regenerate)
- **TB-32** (M) — BoardService: CreateTask, EditTask, MoveTask, CloseTask, Regenerate via exec tb
- **TB-33** (M) — BoardService: EditTaskBody direct-write under .board.lock
- **TB-34** (M) — Drag-and-drop between columns with optimistic UI and conflict revert
- **TB-35** (S) — CreateTaskDialog: modal form for new tasks
- **TB-36** (M) — TaskDrawer: inline metadata editing and Archive button
- **TB-37** (M) — CodeMirror body editor in TaskDrawer
- **TB-38** (M) — FilterBar with archive column toggle
- **TB-40** (S) — BoardService.LoadBoard: archive-aware status mode
- **TB-41** (S) — Frontend Toast.svelte component

## Acceptance Criteria

- [ ] **F3.1** Drag-and-drop via `svelte-dnd-action` moves cards between columns; `BoardService.MoveTask` → `tb mv`; optimistic UI; a racing `tb mv` from the terminal causes a revert + toast
- [ ] **F3.2** `CreateTaskDialog` (title, module, type, priority, size, tags, description, optional parent, "is epic") → `tb create` → new card visible in backlog within 1s
- [ ] **F3.3** Drawer fields (priority, type, size, module, tags) editable inline → `tb edit <ID>`; disk file and `BOARD.md` reflect the change
- [ ] **F3.4** CodeMirror body editor → `BoardService.EditTaskBody`: acquires `.board.lock`, rejects header/metadata changes, appends log entry, writes atomically, then triggers `tb regenerate`
- [ ] **F3.5** `FilterBar` filters cards client-side over the loaded snapshot (type, priority, module, tags, parent epic, agent); "Show archived" toggle adds an Archive column
- [ ] **F3.6** Drawer "Archive" button → `tb close <ID>`; card leaves active board (unless "Show archived" is on)
- [ ] All mutations surface failures via the reusable `Toast.svelte` component (TB-41); no silent failures
- [ ] All M3 sub-tasks (TB-31..TB-38, TB-40, TB-41) closed
- [ ] `docs/IMPLEMENTATION.md` M3 markers flipped to ☑

## Related Tasks

- **TB-2** — Prerequisite (skeleton + watcher)
- **TB-4** — Builds on this (needs assign UI hooks)

## Log

- 2026-05-13: Created
- 2026-05-13: Groomed — fixed corrupted title; aligned acceptance criteria 1:1 with `docs/FEATURES.md` F3.1–F3.6; decomposed into TB-31..TB-38 (CLI wrapper, BoardService mutations, EditTaskBody direct-write, DnD frontend, CreateTaskDialog, drawer metadata edit + archive, CodeMirror body editor, FilterBar + archive column)
- 2026-05-13: Review fixes from Codex — TB-31/32 drop the JSON-output claim and parse `Created <path>` instead; TB-33 tightens flock test to real-process POSIX integration; TB-37 fixes self-contradictory title-editability wording; TB-38 now depends on new TB-40 (backend archive load); added TB-41 to own the `Toast.svelte` deliverable from `docs/IMPLEMENTATION.md` M3 task 7
- 2026-05-13: Started — moved to in-progress
- 2026-05-13: Done
