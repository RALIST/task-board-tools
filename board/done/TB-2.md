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

- [x] All M2 sub-tasks (TB-16..TB-24) closed
- [x] `wails3 doctor` passes on Go 1.26.2 + Wails CLI `v3.0.0-alpha.91`
- [x] `wails3 task darwin:package` produces a working `.app` bundle; launches and serves the SvelteKit frontend (verified via `/ui-test`)
- [x] Three columns render against the live board snapshot — visually verified with `task-board-tools` itself (10 backlog, 0 in-progress, 19 done)
- [x] Empty board renders cleanly — In Progress column shows "No tasks"
- [x] `tb mv TB-26 ip` from the terminal → GUI moved TB-26 to In Progress within ~1s; counts updated (Backlog 10→9, In Progress 0→1)
- [x] Same code path covers `tb create` (single debounced `board:reloaded` event regardless of which Create/Remove/Rename triggered it)
- [x] Click on a card opens TaskDrawer with metadata grid + rendered markdown body (via `marked`); visually verified for TB-1 + TB-2
- [x] Esc + click-outside close the TaskDrawer (Esc via `<svelte:window onkeydown>`; backdrop target check)
- [x] Second GUI launch focuses the existing window — verified: launching `tb-gui.app` while one was running kept the same PID; `OnSecondInstanceLaunch` triggered Restore+Focus
- [x] `SettingsService.OpenBoard(projectRoot)` swaps the BoardService client and calls `watcher.Switch`; emits `board:opened` + `board:reloaded`; state commits gated behind every fallible step (no partial state on failure). Tested.
- [x] First-launch picker → selection persisted to `$XDG_CONFIG_HOME/tb-gui/recent.json` (or `~/.config/tb-gui/recent.json` fallback); dedup + cap-20; tested
- [x] Missing `tb` binary → typed `ErrBinaryNotFound` bubbles through; frontend renders an error toast (no silent empty board)
- [x] Opening a path without `.tb.yaml` → `ErrNoTbYaml` → toast in `+page.svelte`; previous board stays active
- [x] docs/IMPLEMENTATION.md M2 markers flipped to ☑

## Related Tasks

- **TB-1** — Prerequisite (needs `--json`, atomic writes, regenerate fix)
- **TB-3** — Builds on this (adds mutations)

## Log

- 2026-05-13: Created
- 2026-05-13: Groomed — decomposed into TB-16..TB-24; added edge-case acceptance criteria (empty board, missing tb binary, invalid project root, drawer dismiss)
- 2026-05-13: Started — moved to in-progress
- 2026-05-13: All 9 sub-tasks closed. Backend modules: `gui/internal/cli` (exec wrapper, 7 tests), `gui/internal/watcher` (fsnotify + 200ms debounce + pump-goroutine swap design, 8 unit + 1 integration tests against real `tb`), `gui/app/board_service.go` (LoadBoard/GetTask, status bucketing, 7 tests), `gui/app/settings_service.go` (OpenBoard/PickBoardDialog/recents, atomic gating, 8 tests). Frontend: `gui/frontend/src/lib/api.ts` (typed wrappers + error helpers), `stores/{board,selection,filter,toast}.ts`, `components/{Board,Column,Card,TaskDrawer}.svelte`, `routes/+page.svelte` orchestrator with empty-state + recent boards + Wails event subscriptions. Markdown via `marked`; theme is dark with macOS translucent title bar. `go build ./gui` + `npm run check` (0/0) + `npm run build` all green. Runtime acceptance (window opening, live updates, single-instance focus) deferred to end-of-epic `/ui-test`.
- 2026-05-13: docs/IMPLEMENTATION.md M2 markers flipped to ☑.
- 2026-05-13: `/ui-test` pass against `task-board-tools` itself: built `.app` via `wails3 task darwin:package`; window opened; empty-state with picker → "Open board…" → Cmd+Shift+G → `/Users/ralist/projects/task-board-tools` → board rendered (10/0/19); TB-2 card click → drawer → Esc → closed; `tb mv TB-26 ip` from terminal → GUI moved card within 1s; second `open tb-gui.app` did not spawn a new process (single-instance focus). Two issues found + fixed during the test: (1) scaffold `static/style.css` had a leftover `#app { max-width: 1280px; ... }` that broke topbar layout; replaced with a minimal reset. (2) Esc-to-close was bound via `window.addEventListener('keydown')` inside `$effect` which didn't fire reliably; switched to Svelte's `<svelte:window onkeydown>` directive (rebuilt + re-verified).
- 2026-05-13: Done
