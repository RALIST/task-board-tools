# TB-176: Track PID of launched agents

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** agent,daemon,recovery,ui
**Branch:** —

## Goal

Keep recovered live agent processes represented accurately after a GUI/daemon restart: if stale recovery finds an unfinished run whose recorded PID still belongs to the expected agent, keep the task visible as running and monitor that orphaned PID until it can be reconciled instead of immediately marking it failed or leaving it running forever.

## Context

- `gui/app/agent_run.go` writes `AgentStatus=running` and appends `started{run_id, task_id, agent, mode, pid}` from the runner's `OnStarted` callback.
- `gui/app/agent_recovery.go` already scans `AgentStatus=running` tasks on daemon activation, replays the latest JSONL run, and skips recovery when `pidAlive(pid, expectedAgent)` says the recorded PID is still alive.
- `gui/internal/daemon/pid.go` is the liveness guard and includes the command-name / npm node-wrapper check; reuse it rather than adding a raw `kill(0)` probe.
- `docs/FEATURES.md` F5.2 and TB-5 explicitly say M5 does not re-attach to live runs. This task is the deferred follow-up for recovered live PIDs, not a rewrite of M5 crash recovery.
- The draft resume design at `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md` treats hot stdout/stderr reattachment and session resume as separate concerns.

## Constraints

- Do not spawn a second agent process for the recovered run, and do not start a fresh session automatically.
- Do not attempt hot stdout/stderr reattachment; after the GUI process died, old pipes are gone. The UI may show the run as detached/running, but live log streaming is not required here.
- Preserve the cancelled carve-out: `AgentStatus=cancelled` or latest `finished{status: cancelled}` must never become failed/running because of this monitor.
- Use the existing file-form and folder-form agent artifact resolution; do not introduce another `.agent-state` or `.agent-logs` layout.
- When the only durable fact is that the orphaned PID later disappeared and there is still no `finished` JSONL event, reconcile conservatively as failed with an explicit recovery reason rather than inventing success.

## Acceptance Criteria

- [ ] On daemon activation, unfinished `AgentStatus=running` runs still use the latest JSONL `started.pid` plus expected agent name to decide liveness; live matching PIDs remain `running`, dead/mismatched PIDs continue through existing stale-failed recovery.
- [ ] When recovery skips a task because the PID is alive, the daemon registers an idempotent recovered-run monitor keyed by board path + task ID + run ID; repeated activation, watcher reloads, and board switches must not create duplicate monitors or duplicate terminal JSONL events.
- [ ] While the recovered PID stays alive, the card badge and drawer run history continue to show `running`, and Run/Groom actions stay disabled for that task.
- [ ] When the monitored PID later stops being alive and the latest JSONL run still has no `finished` event, append exactly one synthetic `finished{status: failed, reason: "orphaned process exited after restart"}` event, write `AgentStatus=failed` via the existing CLI edit path, and emit `agent:run-finished` so the open UI updates without a manual refresh.
- [ ] If the latest run already has a terminal `finished` event before the monitor fires, the monitor only reconciles the task metadata to that terminal status and does not append another finished line; `finished{status: cancelled}` and durable `AgentStatus=cancelled` remain cancelled.
- [ ] File-form tasks (`board/.agent-state/<ID>.jsonl`) and folder-form tasks (`<status>/<ID>/.agent-state.jsonl`) are both covered by backend tests for live-PID skip, later-dead monitor reconciliation, duplicate-monitor prevention, and cancelled carve-out preservation.
- [ ] The drawer must not show a working Cancel button for a recovered/detached live run unless the backend can actually signal that recovered PID/process group safely; otherwise disable or hide Cancel for that run state and cover it with a frontend test.
- [ ] Verification commands pass: `cd gui && go test ./...`; `cd gui/frontend && npm run check`; `cd gui/frontend && npm test`.

## Related Tasks

- **TB-5** — prerequisite daemon and stale-running recovery contract; live-PID reattach was explicitly deferred there.
- **TB-102** — shares folder/file agent-state path resolution and stale-recovery coverage.

## Manual Test (UI)

Start a long-running fake agent from the GUI, force-kill the GUI process so the child PID survives, restart the GUI on the same board, and confirm the task remains visibly running instead of failed. Leave the child alive long enough to verify Run/Groom stay disabled and Cancel is not a broken action; then terminate the child process and confirm the card/drawer update to failed with the recovery reason without restarting the GUI again.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited module=gui, tags=agent,daemon,recovery,ui, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success

