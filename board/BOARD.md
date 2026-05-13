# Board

## Epics

| ID | Title | Progress | Status | Module |
|----|-------|----------|--------|--------|
| TB-4 | M4: Agent assignment and manual runs from GUI | 0/10 | backlog | gui |
| TB-5 | M5: Agent daemon with autopickup and crash recovery | 0/0 | backlog | gui |
| TB-6 | M6: Groom flow for AI-assisted task refinement | 0/0 | backlog | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | 0/0 | backlog | gui |

## Finished Epics

| ID | Title | Progress | Module |
|----|-------|----------|--------|
| TB-1 | M1: CLI extensions for GUI integration | 8/8 | cli |
| TB-2 | M2: Wails3 skeleton with read-only kanban GUI | 9/9 | gui |
| TB-3 | M3: GUI mutations, DnD, and inline editor | 10/10 | gui |

## In Progress

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| — | — | — | — | — |

## Backlog

| ID | Title | Type | Priority | Size | Module |
|----|-------|------|----------|------|--------|
| TB-4 | M4: Agent assignment and manual runs from GUI | feature | P1 | XL | gui |
| TB-5 | M5: Agent daemon with autopickup and crash recovery | feature | P1 | XL | gui |
| TB-6 | M6: Groom flow for AI-assisted task refinement | feature | P2 | L | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | feature | P2 | L | gui |
| TB-26 | Atomic write for .next-id under board lock | tech-debt | P2 | S | cli |
| TB-27 | cmdRegenerate should take .board.lock | bug | P2 | S | cli |
| TB-28 | collectAllTasks / findChildren: archive inclusion semantics | improvement | P3 | M | cli |
| TB-29 | parseTaskFile should reject malformed task files | bug | P3 | S | cli |
| TB-30 | tb assign sugar for paired Agent + AgentStatus writes | feature | P3 | S | cli |
| TB-39 | addChildToSubtasks corrupts task body when '## ' headers appear inside backticks | bug | P2 | S | cli |
| TB-43 | Agent Runner interface, Mode type, and embedded implement.md prompt | feature | P1 | S | gui |
| TB-44 | ClaudeRunner and CodexRunner: exec.CommandContext with own process group | feature | P1 | M | gui |
| TB-45 | Agent state writer: JSONL events + per-run log file | feature | P1 | M | gui |
| TB-46 | AgentService.AssignAgent via tb edit -a | feature | P1 | S | gui |
| TB-47 | AgentService.RunAgent: enqueue, spawn runner, bridge Wails events | feature | P1 | M | gui |
| TB-48 | AgentService.CancelRun: SIGTERM/SIGKILL, JSONL cancelled, AgentStatus cancelled | feature | P1 | M | gui |
| TB-49 | AgentService.ListRuns: parse JSONL into Run summaries | feature | P1 | S | gui |
| TB-50 | TaskDrawer agent dropdown + Run/Cancel buttons + Card agent badge | feature | P1 | M | gui |
| TB-51 | AgentRunLog.svelte: live streaming and past-run log rendering | feature | P1 | M | gui |
| TB-52 | runsStore.ts keyed by run_id + drawer past-runs list | feature | P1 | S | gui |

## Recently Done

| ID | Title | Type | Module |
|----|-------|------|--------|
| TB-41 | Frontend Toast.svelte component | feature | gui |
| TB-40 | BoardService.LoadBoard: archive-aware status mode | feature | gui |
| TB-38 | FilterBar with archive column toggle | feature | gui |
| TB-37 | CodeMirror body editor in TaskDrawer | feature | gui |
| TB-36 | TaskDrawer: inline metadata editing and Archive button | feature | gui |
| TB-35 | CreateTaskDialog: modal form for new tasks | feature | gui |
| TB-34 | Drag-and-drop between columns with optimistic UI and conflict revert | feature | gui |
| TB-33 | BoardService: EditTaskBody direct-write under .board.lock | feature | gui |
| TB-32 | BoardService: CreateTask, EditTask, MoveTask, CloseTask, Regenerate via exec tb | feature | gui |
| TB-31 | CLI wrapper: mutation commands (create/edit/mv/close/regenerate) | feature | gui |
| TB-24 | Frontend TaskDrawer: read-only markdown body | feature | gui |
| TB-23 | Frontend kanban: Board, Column, Card components (read-only) | feature | gui |
| TB-22 | Frontend skeleton: api.ts, stores, +page.svelte layout | feature | gui |
| TB-21 | SettingsService: project root, recent boards, folder picker | feature | gui |
| TB-20 | fsnotify watcher with debounce and Wails events | feature | gui |
| TB-19 | BoardService: LoadBoard and GetTask via exec tb | feature | gui |
| TB-18 | CLI exec wrapper in gui/internal/cli | feature | gui |
| TB-17 | Scaffold gui/ Wails3 + SvelteKit project with single-instance lock | feature | gui |
| TB-16 | Verify Wails3 alpha works on current Go toolchain | spike | gui |
| TB-15 | Add flag.NewFlagSet and reorderArgs to tb show | tech-debt | cli |
| TB-14 | Implement active/archive/all status semantics | improvement | cli |
| TB-13 | Call regenerateBoard at end of create and edit | bug | cli |
| TB-12 | Add --json output to ls, show, and board | feature | cli |
| TB-11 | Add Agent and AgentStatus task metadata fields | feature | cli |
| TB-10 | Migrate task .md writes to writeFileAtomic | tech-debt | cli |
| TB-9 | Add cli/atomicfs.go writeFileAtomic helper | feature | cli |
| TB-8 | Rename tb/ to cli/ and add go.work | tech-debt | cli |
| TB-3 | M3: GUI mutations, DnD, and inline editor | feature | gui |
| TB-2 | M2: Wails3 skeleton with read-only kanban GUI | feature | gui |
| TB-1 | M1: CLI extensions for GUI integration | feature | cli |
