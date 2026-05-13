# TB-4: M4: Agent assignment and manual runs from GUI

**Type:** feature
**Priority:** P1
**Size:** XL
**Module:** gui
**Tags:** milestone-m4,agent,epic
**Branch:** —

## Goal

Let the user assign claude or codex to a task and trigger a run manually from the GUI. Add AgentService, ClaudeRunner/CodexRunner via exec, JSONL run history under board/.agent-state/, full logs under board/.agent-logs/, and streaming logs in the TaskDrawer.

## Context

Let the user assign claude or codex to a task and trigger a run manually from the GUI. The runner shells out to `claude -p <prompt>` or `codex exec`, streams stdout/stderr to JSONL events under `board/.agent-state/`, and a full log under `board/.agent-logs/`. Agent state round-trips through the task `.md` via `tb edit`. Cwd is the project root. Env is a whitelist. Hard timeout default 30 min. See plan M4 and `docs/ARCHITECTURE.md` → "Agent state (hybrid)".

## Acceptance Criteria

- [ ] `AgentService` exposes `AssignAgent`, `RunAgent`, `CancelRun`, `ListRuns`
- [ ] `ClaudeRunner` and `CodexRunner` run via `exec.CommandContext` with cwd = project root
- [ ] `prompts/implement.md` embedded via `go:embed`
- [ ] JSONL events (`queued`, `started`+pid+run_id, `stdout`, `stderr`, `finished`) written append-only
- [ ] Full log written to `board/.agent-logs/PREFIX-NNN/<run_id>.log`
- [ ] Wails events `agent:run-started`, `agent:run-log`, `agent:run-finished` bridge into the frontend
- [ ] TaskDrawer shows Agent dropdown, Run button, and live `AgentRunLog`
- [ ] After a run, `AgentStatus` in the `.md` reflects success/failed and is visible from `tb show`

## Related Tasks

- **TB-3** — Prerequisite (mutations + drawer UI)
- **TB-11** — Prerequisite (Agent/AgentStatus fields exist on the task model)
- **TB-5** — Builds on this (daemon auto-pickup)
- **TB-6** — Builds on this (groom decorator wraps these runners)

## Log

- 2026-05-13: Created
