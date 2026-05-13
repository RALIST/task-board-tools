# TB-2: M2: Wails3 skeleton with read-only kanban GUI

**Type:** feature
**Priority:** P1
**Size:** XL
**Module:** gui
**Tags:** milestone-m2,gui,read-only,epic
**Branch:** —

## Goal

Bootstrap the Wails3 alpha + Svelte 5 desktop app under gui/. Render the kanban board (backlog/in-progress/done) read-only from existing tb data via tb ls --json. Wire fsnotify watcher, board picker, recent boards, and single-instance lock.

## Context

First GUI deliverable: read-only kanban over existing board data using Wails3 alpha + Svelte 5/SvelteKit. GUI reads via `exec tb ls --json --status active` and watches the filesystem via fsnotify for live updates. Single-instance lock keeps the daemon unique once M5 lands. Reference: plan M2 and `docs/ARCHITECTURE.md`.

## Subtasks

- **TB-16** (S) — Verify Wails3 alpha works on current Go toolchain
- **TB-17** (S) — Scaffold gui/ Wails3 + SvelteKit project with single-instance lock
- **TB-18** (S) — CLI exec wrapper in gui/internal/cli
- **TB-19** (M) — BoardService: LoadBoard and GetTask via exec tb
- **TB-20** (M) — fsnotify watcher with debounce and Wails events
- **TB-21** (M) — SettingsService: project root, recent boards, folder picker
- **TB-22** (S) — Frontend skeleton: api.ts, stores, +page.svelte layout
- **TB-23** (M) — Frontend kanban: Board, Column, Card components (read-only)
- **TB-24** (S) — Frontend TaskDrawer: read-only markdown body

## Acceptance Criteria

- [ ] All M2 sub-tasks (TB-16..TB-24) closed
- [ ] `wails3 doctor` passes on the current Go toolchain (or Wails3 tag pinned if not)
- [ ] `cd gui && wails3 dev` starts the app without errors
- [ ] Three columns (backlog / in-progress / done) render with cards from the live board
- [ ] Empty board renders cleanly (just empty columns, no errors)
- [ ] `tb mv 1 ip` in the terminal → GUI updates within 1s
- [ ] `tb create "X"` in the terminal → new card appears in the GUI without a manual refresh
- [ ] Click on a card opens TaskDrawer with metadata + read-only markdown body
- [ ] `Esc` and click-outside both close the TaskDrawer
- [ ] Second GUI launch focuses the existing window (single-instance lock)
- [ ] `SettingsService.OpenBoard(projectRoot)` switches active board and reloads UI; watcher follows
- [ ] First-launch folder picker stores selection in `~/.config/tb-gui/recent.json`
- [ ] Missing `tb` binary surfaces a clear error state (instead of a silent empty board)
- [ ] Opening a path without `.tb.yaml` shows a toast and leaves the previous board active
- [ ] docs/IMPLEMENTATION.md M2 markers flipped to ☑

## Related Tasks

- **TB-1** — Prerequisite (needs `--json`, atomic writes, regenerate fix)
- **TB-3** — Builds on this (adds mutations)

## Log

- 2026-05-13: Created
- 2026-05-13: Groomed — decomposed into TB-16..TB-24; added edge-case acceptance criteria (empty board, missing tb binary, invalid project root, drawer dismiss)
