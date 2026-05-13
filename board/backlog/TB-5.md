# TB-5: M5: Agent daemon with autopickup and crash recovery

**Type:** feature
**Priority:** P1
**Size:** XL
**Module:** gui
**Tags:** milestone-m5,agent,daemon,epic
**Branch:** —

## Goal

Embed a background daemon in the GUI process that auto-picks up tasks with Agent set and AgentStatus=queued. Worker pool with semaphore, dedup by task_id, stale-running recovery on startup via PID liveness + JSONL replay, graceful shutdown.

## Context

Embed a background daemon in the GUI process that auto-picks up any task with `Agent` set and `AgentStatus=queued`. Worker pool with semaphore (default 1, configurable). Dedup by task_id (in-memory active set). On startup, scan tasks with `AgentStatus=running`: read last run from `board/.agent-state/PREFIX-NNN.jsonl`; if there is no `finished` event and the PID from the `started` event is dead, mark the run as failed (and the task `AgentStatus` as failed). `AgentStatus=cancelled` is user-initiated and stale-recovery must never overwrite it. Graceful shutdown via `context.Cancel` + 5s grace then kill. See plan M5 and `docs/ARCHITECTURE.md` → "Daemon".

## Acceptance Criteria

- [ ] `gui/internal/daemon/daemon.go` starts in Wails `App.OnStartup`
- [ ] Scan on start + fsnotify watch triggers pickup of queued tasks within ~5s
- [ ] Worker pool with configurable max (default 1) serializes runs
- [ ] In-memory active-task set prevents duplicate runs for the same task_id
- [ ] Stale-running recovery: tasks with `AgentStatus=running` whose last run has no `finished` event and a dead PID are marked failed, with a recovery event appended to JSONL
- [ ] PID + `run_id` recorded in the `started` JSONL event
- [ ] `AgentStatus=cancelled` is never overwritten by stale-recovery
- [ ] Single-instance lock (inherited from M2) keeps the daemon unique per user
- [ ] Graceful shutdown: closing the GUI cancels in-flight ctx, waits 5s, then kills

## Related Tasks

- **TB-4** — Prerequisite (runners + JSONL writer)
- **TB-2** — Provides single-instance lock

## Log

- 2026-05-13: Created
