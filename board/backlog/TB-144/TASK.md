# TB-144: Append logs realtime when task is opened

**Type:** bug
**Priority:** P1
**Size:** S
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** gui,frontend,agent,logs,drawer,quick-win
**Branch:** —

## Goal

When a task is opened while its selected agent run is still queued or running, the drawer's run-log pane should immediately show the log text already written to disk and then keep appending new live lines for that same run.

## Context

Current surface:
- `gui/frontend/src/lib/components/TaskDrawer.svelte` loads `ListRuns(taskId)` when the task detail opens, selects the newest run, and renders `AgentRunLog` in the rail.
- `gui/frontend/src/lib/components/AgentRunLog.svelte` currently fetches `GetRunLog(taskId, runId)` only for terminal runs; for live runs it clears its local buffer and listens for future `agent:run-log` events.
- `gui/app/agent_runs.go` already exposes `GetRunLog(ctx, taskID, runID)` and resolves both legacy file-task logs and folder-task-local logs through the backend.

Implementation constraints and non-goals:
- Keep the log path backend-owned: the frontend must call `getRunLog(taskId, runId)`, not read `Run.logPath` or construct `.agent-logs` paths.
- Do not put log lines into `runsStore`; it should remain lifecycle/run-summary state only.
- Do not change JSONL event names, watcher ignore rules, daemon lifecycle, or the backend log storage contract.
- Do not implement live reattach after a GUI/process restart; this task only covers opening/reopening the task while the current GUI process is receiving live run events.

## Acceptance Criteria

- [ ] Opening `TaskDrawer` for a task whose selected run has status `queued` or `running` hydrates `AgentRunLog` from `getRunLog(taskId, runId)` so previously written output is visible immediately instead of starting from an empty pane.
- [ ] After the initial log snapshot is shown, matching `agent:run-log` events for the same `run_id` append within about 1 second; events for other run IDs are ignored.
- [ ] The snapshot-to-live handoff does not duplicate or drop visible lines when a log event arrives while the initial `getRunLog` request is in flight; switching task or selected run clears the old buffer before rendering the next run.
- [ ] Terminal/past-run behavior stays unchanged: finished runs still render through `GetRunLog` and do not start a polling loop or subscribe to unrelated live events.
- [ ] Frontend coverage exercises live-run hydration, live append/filtering, fetch-error/not-found handling for a live run, and task/run switch reset; run `cd gui/frontend && npm test -- AgentRunLog` or the closest focused Vitest target plus `npm run check`.
- [ ] Manual GUI smoke: start a long-running Run or Groom action, close the task detail, wait until several log lines are written, reopen the same task, and confirm old lines are visible immediately and new lines continue appending until the run finishes.

## Related Tasks

- **TB-4** — Baseline M4 agent-run epic that introduced drawer streaming logs.
- **TB-51** — Original `AgentRunLog.svelte` live/past-run component this task tightens.
- **TB-102** — Backend log path resolution for file-form and folder-form tasks; this task should keep using that service boundary.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited priority=P1, type=bug, size=S, module=gui, tags=gui,frontend,agent,logs,drawer,quick-win, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success

