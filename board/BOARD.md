# Board

## Finished Epics

| ID | Title | Progress | Module |
|----|-------|----------|--------|
| TB-1 | M1: CLI extensions for GUI integration | 8/8 | cli |
| TB-2 | M2: Wails3 skeleton with read-only kanban GUI | 9/9 | gui |
| TB-3 | M3: GUI mutations, DnD, and inline editor | 10/10 | gui |
| TB-4 | M4: Agent assignment and manual runs from GUI | 10/10 | gui |
| TB-5 | M5: Agent daemon with autopickup and crash recovery | 10/10 | gui |
| TB-6 | M6: Groom flow for AI-assisted task refinement | 11/11 | gui |
| TB-7 | M7: Polish — settings, shortcuts, tray, menus | 10/10 | gui |

## In Progress

| ID | Title | Priority | Module | Branch |
|----|-------|----------|--------|--------|
| — | — | — | — | — |

## Backlog

| ID | Title | Type | Priority | Size | Module |
|----|-------|------|----------|------|--------|
| TB-92 | Limit showed tags in header to 10 | bug | P1 | M |  |

## Recently Done

| ID | Title | Type | Module |
|----|-------|------|--------|
| TB-91 | Task card is bigger then column | improvement |  |
| TB-90 | Board switching is not working | bug |  |
| TB-89 | Click outside should close dropdown | bug | gui |
| TB-88 | GUI triage fails when active tb binary lacks JSON support | bug | gui |
| TB-87 | Parent epic filter hides the epic card itself | bug | gui |
| TB-86 | Adapt post-edit hook for Codex PostToolUse | improvement | tooling |
| TB-85 | Docs flip: IMPLEMENTATION.md M7 + FEATURES.md F7.1/F7.2/F7.3 markers + ARCHITECTURE.md if needed | tech-debt | gui |
| TB-84 | Keyboard shortcuts: N (new), / (search), Esc (close drawer), Enter (open card) | feature | gui |
| TB-83 | System tray: idle/running glyph + click to show/hide window | feature | gui |
| TB-82 | Wails3 application menu: File (Open board…, Open Recent ›, Quit), View, Help | feature | gui |
| TB-81 | SettingsPanel.svelte: form for timeout/max_workers/default_agent/cli_path with Save + toast | feature | gui |
| TB-80 | Frontend api.ts settings wrappers + preferencesStore.ts | feature | gui |
| TB-79 | Wire default_agent into AssignAgent dropdown default for unassigned tasks | feature | gui |
| TB-78 | Wire cli_path preference into cli.NewClient at board open + reload on change | feature | gui |
| TB-77 | Wire agent_timeout_minutes into agent_run.go (replace agentTimeoutDefault const) | feature | gui |
| TB-76 | Preferences struct: add agent_timeout_minutes, default_agent, cli_path with clamps + tests | feature | gui |
| TB-75 | CLI: tb edit --goal / --acceptance — body-section writes via writeFileAtomic | feature | cli |
| TB-74 | Docs: flip M6 markers — IMPLEMENTATION.md / FEATURES.md F6.1+F6.2 / ARCHITECTURE.md GroomingDecorator | improvement | docs |
| TB-73 | Card: 'needs grooming' indicator + click-to-suggest-Groom in drawer | feature | gui |
| TB-72 | TaskDrawer: Groom button next to Run, shared Cancel, mode-labelled past runs | feature | gui |
| TB-71 | Frontend api.ts + triageStore.ts: groomTask, getTriage, watcher-driven refresh | feature | gui |
| TB-70 | BoardService.Triage(): wrap tb triage --json, cache + invalidate on watcher events | feature | gui |
| TB-69 | CLI: tb triage --json structured output | feature | cli |
| TB-68 | Daemon: honor Mode=groom from queued JSONL event so pickup uses GroomingDecorator | feature | gui |
| TB-67 | AgentService.GroomTask: queue a groom run distinct from a normal run | feature | gui |
| TB-66 | GroomingDecorator: wrap a Runner, swap prompt template | feature | gui |
| TB-65 | prompts/groom.md template + agent.PromptGroom embed | feature | gui |
| TB-64 | GUI "Open project" button only works once, subsequent clicks do nothing | bug | gui |
| TB-63 | GUI tag header overflows when too many tags — collapse to popular + dropdown | bug | gui |
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
