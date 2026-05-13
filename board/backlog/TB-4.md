# TB-4: M4: Agent assignment and manual runs from GUI

**Type:** feature
**Priority:** P1
**Size:** XL
**Module:** gui
**Tags:** milestone-m4,agent,epic
**Branch:** —

## Goal

Wire `claude` and `codex` CLIs into the GUI as manual, drawer-driven runs. Assign an agent to a task, click Run, watch streaming logs, Cancel mid-flight — all while keeping `AgentStatus` in the task `.md` as the canonical state and JSONL under `board/.agent-state/` as the run history.

## Context

`AgentService` lives next to `BoardService` and owns four entry points: `AssignAgent`, `RunAgent`, `CancelRun`, `ListRuns`. Runs shell out via `exec.CommandContext` to `claude -p <prompt>` or `codex exec`, cwd = project root, env = whitelist, own process group (so SIGKILL cascades to children), 30-minute default timeout. Stdout/stderr are teed to `board/.agent-logs/<ID>/<run_id>.log` and forwarded as Wails events `agent:run-started` / `agent:run-log` / `agent:run-finished`. Lifecycle events (`queued`, `started`+pid+run_id, `stdout`, `stderr`, `finished`+status) are appended to `board/.agent-state/<ID>.jsonl`. `AgentStatus` round-trips through `tb edit --agent-status` so the state in the `.md` matches reality and is durable across crashes (M5 will recover stale `running`; M4 just guarantees the write order so cancel sticks). See plan M4 in `docs/IMPLEMENTATION.md` and `docs/ARCHITECTURE.md` → "Agent state (hybrid)".

## Subtasks

- **TB-43** (S) — `Runner` interface, `Mode` type, `RunResult`; embedded `prompts/implement.md`
- **TB-44** (M) — `ClaudeRunner` + `CodexRunner` via `exec.CommandContext` (own pgid, env whitelist, timeout, line-scanning stdout/stderr)
- **TB-45** (M) — JSONL writer + log-file rotation in `gui/internal/agent/state.go`; locks down event names
- **TB-46** (S) — `AgentService.AssignAgent` via `tb edit -a`
- **TB-47** (M) — `AgentService.RunAgent` + Wails event bridge; sets `AgentStatus: running` then `success|failed`
- **TB-48** (M) — `AgentService.CancelRun` with kill → JSONL `finished{cancelled}` → `AgentStatus: cancelled` ordering
- **TB-49** (S) — `AgentService.ListRuns` parses JSONL
- **TB-50** (M) — TaskDrawer agent dropdown + Run/Cancel buttons + Card agent badge
- **TB-51** (M) — `AgentRunLog.svelte` streams `agent:run-log` lines; can render a past run's log file
- **TB-52** (S) — `runsStore.ts` keyed by `run_id`; drawer past-run list

## Acceptance Criteria

- [ ] **F4.1** Drawer has an Agent dropdown (`none | claude | codex`) → `AgentService.AssignAgent` → `tb edit -a`. The card surfaces an agent badge. Reload — assignment persists; `tb show <ID>` shows `**Agent:** claude`.
- [ ] **F4.2** Drawer **Run agent** button (enabled when an agent is assigned) → `AgentService.RunAgent(id)` generates a `run_id` (`r_<8 hex>`), writes JSONL `queued` and `started`+pid, sets `AgentStatus: running` via `tb edit`, spawns the runner via `exec.CommandContext` with cwd = project root, env whitelist, own process group, default 30-min timeout. Stdout/stderr are teed to `board/.agent-logs/<ID>/<run_id>.log` and emitted as Wails events. On exit the JSONL `finished{status, exit_code}` event is appended and `AgentStatus` becomes `success` or `failed`.
- [ ] **F4.3** `AgentRunLog.svelte` panel inside the drawer streams stdout/stderr lines within ~1s of the agent emitting them, shows the current status pill (`queued | running | success | failed | cancelled`), and (re-)renders the selected run's `.log` file content for past runs.
- [ ] **F4.4** Drawer Cancel button on a running task does, **in this order**: (1) cancels the runner context (SIGTERM, then SIGKILL after 5s grace; kill propagates via the process group), (2) appends JSONL `finished{status: cancelled}`, (3) writes `**AgentStatus:** cancelled` via `tb edit --agent-status cancelled`. After a restart the task remains `cancelled` — M5 stale-recovery never overwrites it.
- [ ] **F4.5** Drawer shows a list of past runs parsed from `board/.agent-state/<ID>.jsonl` with timestamp + status; clicking a past run loads its log file into the `AgentRunLog` panel.
- [ ] All M4 sub-tasks (TB-43..TB-52) closed.
- [ ] `docs/IMPLEMENTATION.md` M4 markers flipped to ☑.

## Related Tasks

- **TB-3** — Prerequisite (mutations + drawer UI; toast component)
- **TB-11** — Prerequisite (`Agent`/`AgentStatus` fields on the task model)
- **TB-5** — Builds on this (daemon auto-pickup reuses Runner/JSONL/log-file plumbing)
- **TB-6** — Builds on this (groom decorator wraps these runners)

## Log

- 2026-05-13: Created
- 2026-05-13: Groomed — aligned acceptance criteria 1:1 with `docs/FEATURES.md` F4.1–F4.5; decomposed into TB-43..TB-52 (Runner interface + embedded implement.md; ClaudeRunner/CodexRunner with own process group; JSONL+log-file writer with locked-down event names; AssignAgent backend; RunAgent + Wails event bridge; CancelRun with explicit kill→JSONL→AgentStatus ordering for M5 cancel-stickiness; ListRuns; drawer dropdown + Run/Cancel + Card badge; AgentRunLog streamer; runsStore keyed by run_id)
- 2026-05-13: Review fixes from Codex — TB-43 locks down `{{TASK_ID}}/{{TASK_TITLE}}/{{TASK_BODY}}` placeholders and adds `RenderPrompt`; TB-44 commits to `codex exec --prompt` (with stdin fallback), drops the SIGKILL-grace duplication (signal *mechanism* lives here via `OnStarted` exposing pgid; *policy* lives in TB-48), adds the deadline-vs-cancel split; TB-45 adds `task_id` to every JSONL event; TB-46 adds a real-`tb` integration test proving `tb show <ID>` shows `**Agent:** claude` after assignment (F4.1 persistence); TB-47 specifies the `activeRun` struct fields, the async return semantics (returns `run_id` after queued+started writes; goroutine continues), the JSONL↔Wails event mapping table (adds `agent:run-queued` so TB-52 can render the queued pill; `task_id` on every payload), and an error→status mapping (binary-not-found / timeout / IO → `failed` with `reason`); TB-48 makes the cancel ordering 5 steps (mark→kill→JSONL→Wails→AgentStatus) and emits `agent:run-finished{cancelled}` so the UI updates without waiting on the next disk poll; TB-49 adds `TaskID` to the `Run` model and changes `GetRunLog` to `(taskID, runID)` so the file path is resolvable; TB-50 stops claiming backend lifecycle truth (frontend Vitest only — disk truth is TB-46/TB-47/TB-48); TB-51 adds the F4.3 ~1s latency acceptance and goes through `GetRunLog` instead of `Run.LogPath` directly; TB-52 listens for `agent:run-queued`, renders timestamp+status+agent per row (F4.5)
