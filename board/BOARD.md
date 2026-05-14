# Board

## Epics

| ID | Title | Progress | Status | Module |
|----|-------|----------|--------|--------|
| TB-6 | M6: Groom flow for AI-assisted task refinement | 0/11 | in-progress | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | 0/10 | backlog | gui |

## Finished Epics

| ID | Title | Progress | Module |
|----|-------|----------|--------|
| TB-1 | M1: CLI extensions for GUI integration | 8/8 | cli |
| TB-2 | M2: Wails3 skeleton with read-only kanban GUI | 9/9 | gui |
| TB-3 | M3: GUI mutations, DnD, and inline editor | 10/10 | gui |
| TB-4 | M4: Agent assignment and manual runs from GUI | 10/10 | gui |
| TB-5 | M5: Agent daemon with autopickup and crash recovery | 10/10 | gui |

## In Progress

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| TB-6 | M6: Groom flow for AI-assisted task refinement | P2 | gui | — |
| TB-65 | prompts/groom.md template + agent.PromptGroom embed | P2 | gui | — |
| TB-66 | GroomingDecorator: wrap a Runner, swap prompt template | P2 | gui | — |
| TB-67 | AgentService.GroomTask: queue a groom run distinct from a normal run | P2 | gui | — |
| TB-68 | Daemon: honor Mode=groom from queued JSONL event so pickup uses GroomingDecorator | P2 | gui | — |
| TB-69 | CLI: tb triage --json structured output | P2 | cli | — |
| TB-70 | BoardService.Triage(): wrap tb triage --json, cache + invalidate on watcher events | P2 | gui | — |
| TB-71 | Frontend api.ts + triageStore.ts: groomTask, getTriage, watcher-driven refresh | P2 | gui | — |
| TB-72 | TaskDrawer: Groom button next to Run, shared Cancel, mode-labelled past runs | P2 | gui | — |
| TB-73 | Card: 'needs grooming' indicator + click-to-suggest-Groom in drawer | P2 | gui | — |
| TB-75 | CLI: tb edit --goal / --acceptance — body-section writes via writeFileAtomic | P2 | cli | — |

## Backlog

| ID | Title | Type | Priority | Size | Module |
|----|-------|------|----------|------|--------|
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | feature | P2 | L | gui |
| TB-29 | parseTaskFile should reject malformed task files | bug | P3 | S | cli |
| TB-30 | tb assign sugar for paired Agent + AgentStatus writes | feature | P3 | S | cli |
| TB-39 | Make CLI task-section scans ignore quoted Markdown headings | bug | P2 | M | cli |
| TB-63 | GUI tag header overflows when too many tags — collapse to popular + dropdown | bug | P1 | M | gui |
| TB-64 | GUI "Open project" button only works once, subsequent clicks do nothing | bug | P1 | S | gui |
| TB-74 | Docs: flip M6 markers — IMPLEMENTATION.md / FEATURES.md F6.1+F6.2 / ARCHITECTURE.md GroomingDecorator | improvement | P2 | S | docs |
| TB-76 | Preferences struct: add agent_timeout_minutes, default_agent, cli_path with clamps + tests | feature | P2 | S | gui |
| TB-77 | Wire agent_timeout_minutes into agent_run.go (replace agentTimeoutDefault const) | feature | P2 | S | gui |
| TB-78 | Wire cli_path preference into cli.NewClient at board open + reload on change | feature | P2 | S | gui |
| TB-79 | Wire default_agent into AssignAgent dropdown default for unassigned tasks | feature | P2 | S | gui |
| TB-80 | Frontend api.ts settings wrappers + preferencesStore.ts | feature | P2 | S | gui |
| TB-81 | SettingsPanel.svelte: form for timeout/max_workers/default_agent/cli_path with Save + toast | feature | P2 | M | gui |
| TB-82 | Wails3 application menu: File (Open board…, Open Recent ›, Quit), View, Help | feature | P2 | M | gui |
| TB-83 | System tray: idle/running glyph + click to show/hide window | feature | P2 | M | gui |
| TB-84 | Keyboard shortcuts: N (new), / (search), Esc (close drawer), Enter (open card) | feature | P2 | S | gui |
| TB-85 | Docs flip: IMPLEMENTATION.md M7 + FEATURES.md F7.1/F7.2/F7.3 markers + ARCHITECTURE.md if needed | tech-debt | P2 | S | gui |

## Recently Done

| ID | Title | Type | Module |
|----|-------|------|--------|
| TB-86 | Adapt post-edit hook for Codex PostToolUse | improvement | tooling |
| TB-62 | Graceful shutdown: ctx cancel + 5s grace + JSONL flush | feature | gui |
| TB-61 | Stale-recovery cancelled carve-out: never overwrite AgentStatus=cancelled | feature | gui |
| TB-60 | Stale-running recovery: scan AgentStatus=running, JSONL replay, synthetic finished+failed | feature | gui |
| TB-59 | pidAlive(pid, name) probe with command-name cross-check (R10 mitigation) | feature | gui |
| TB-58 | Watcher event sink: enqueue on task changes via emitter fan-out | feature | gui |
| TB-57 | Daemon initial queue scan on startup (AgentStatus=queued → enqueue) | feature | gui |
| TB-56 | Settings field max_workers (1-4) wired to daemon semaphore | feature | gui |
| TB-55 | Daemon active-set dedup keyed by task_id | feature | gui |
| TB-54 | Worker pool calling an internal blocking executor on AgentService | feature | gui |
| TB-53 | Daemon skeleton + Wails OnStartup/OnShutdown lifecycle wiring | feature | gui |
| TB-52 | runsStore.ts keyed by run_id + drawer past-runs list | feature | gui |
| TB-51 | AgentRunLog.svelte: live streaming and past-run log rendering | feature | gui |
| TB-50 | TaskDrawer agent dropdown + Run/Cancel buttons + Card agent badge | feature | gui |
| TB-49 | AgentService.ListRuns: parse JSONL into Run summaries | feature | gui |
| TB-48 | AgentService.CancelRun: SIGTERM/SIGKILL, JSONL cancelled, AgentStatus cancelled | feature | gui |
| TB-47 | AgentService.RunAgent: enqueue, spawn runner, bridge Wails events | feature | gui |
| TB-46 | AgentService.AssignAgent via tb edit -a | feature | gui |
| TB-45 | Agent state writer: JSONL events + per-run log file | feature | gui |
| TB-44 | ClaudeRunner and CodexRunner: exec.CommandContext with own process group | feature | gui |
| TB-43 | Agent Runner interface, Mode type, and embedded implement.md prompt | feature | gui |
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
| TB-28 | collectAllTasks / findChildren: archive inclusion semantics | improvement | cli |
| TB-27 | cmdRegenerate should take .board.lock | bug | cli |
| TB-26 | Atomic write for .next-id under board lock | tech-debt | cli |
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
