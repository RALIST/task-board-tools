# Board

## Epics

| ID | Title | Progress | Status | Module |
|----|-------|----------|--------|--------|
| TB-1 | M1: CLI extensions for GUI integration | 0/8 | backlog | cli |
| TB-2 | M2: Wails3 skeleton with read-only kanban GUI | 0/9 | backlog | gui |
| TB-3 | M3: GUI mutations, DnD, and inline editor | 0/0 | backlog | gui |
| TB-4 | M4: Agent assignment and manual runs from GUI | 0/0 | backlog | gui |
| TB-5 | M5: Agent daemon with autopickup and crash recovery | 0/0 | backlog | gui |
| TB-6 | M6: Groom flow for AI-assisted task refinement | 0/0 | backlog | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | 0/0 | backlog | gui |

## In Progress

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| — | — | — | — | — |

## Backlog

| ID | Title | Type | Priority | Size | Module |
|----|-------|------|----------|------|--------|
| TB-1 | M1: CLI extensions for GUI integration | feature | P1 | XL | cli |
| TB-2 | M2: Wails3 skeleton with read-only kanban GUI | feature | P1 | XL | gui |
| TB-3 | M3: GUI mutations, DnD, and inline editor | feature | P1 | XL | gui |
| TB-4 | M4: Agent assignment and manual runs from GUI | feature | P1 | XL | gui |
| TB-5 | M5: Agent daemon with autopickup and crash recovery | feature | P1 | XL | gui |
| TB-6 | M6: Groom flow for AI-assisted task refinement | feature | P2 | L | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | feature | P2 | L | gui |
| TB-8 | Rename tb/ to cli/ and add go.work | tech-debt | P0 | S | cli |
| TB-9 | Add cli/atomicfs.go writeFileAtomic helper | feature | P1 | S | cli |
| TB-10 | Migrate task .md writes to writeFileAtomic | tech-debt | P1 | S | cli |
| TB-11 | Add Agent and AgentStatus task metadata fields | feature | P1 | M | cli |
| TB-12 | Add --json output to ls, show, and board | feature | P1 | M | cli |
| TB-13 | Call regenerateBoard at end of create and edit | bug | P1 | S | cli |
| TB-14 | Implement active/archive/all status semantics | improvement | P1 | S | cli |
| TB-15 | Add flag.NewFlagSet and reorderArgs to tb show | tech-debt | P2 | S | cli |
| TB-16 | Verify Wails3 alpha works on current Go toolchain | spike | P0 | S | gui |
| TB-17 | Scaffold gui/ Wails3 + SvelteKit project with single-instance lock | feature | P1 | S | gui |
| TB-18 | CLI exec wrapper in gui/internal/cli | feature | P1 | S | gui |
| TB-19 | BoardService: LoadBoard and GetTask via exec tb | feature | P1 | M | gui |
| TB-20 | fsnotify watcher with debounce and Wails events | feature | P1 | M | gui |
| TB-21 | SettingsService: project root, recent boards, folder picker | feature | P1 | M | gui |
| TB-22 | Frontend skeleton: api.ts, stores, +page.svelte layout | feature | P1 | S | gui |
| TB-23 | Frontend kanban: Board, Column, Card components (read-only) | feature | P1 | M | gui |
| TB-24 | Frontend TaskDrawer: read-only markdown body | feature | P1 | S | gui |

## Recently Done

| ID | Title | Type | Module |
|----|-------|------|--------|
| — | — | — | — |
