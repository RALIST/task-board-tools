# TB-291: Auto-resume interrupted tasks in auto-groom and auto-implement coordinators

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** agent,daemon,resume,recovery,automation
**Branch:** —

## Goal

Auto-resume crashed agent runs without requiring user intervention. When the daemon's stale-recovery path marks a task `AgentStatus: interrupted` (PID dead, session_id captured), the relevant coordinator should automatically call `AgentService.ResumeAgent` so the run continues from the captured `session_id` in the agent CLI. Each coordinator owns interrupts in its watched column: auto-groom resumes `interrupted` tasks in `backlog`; auto-implement resumes `interrupted` tasks in `in-progress`.

## Acceptance Criteria

- A task in `backlog` with `AgentStatus: interrupted` and a captured `session_id` is automatically resumed by the auto-groom coordinator on the next scan (no GUI click required).
- A task in `in-progress` with `AgentStatus: interrupted` and a captured `session_id` is automatically resumed by the auto-implement coordinator on the next scan.
- `lost` tasks (no session_id) are NOT auto-restarted from scratch — the coordinator skips them defensively.
- A persistent resume failure (e.g., `ErrNotResumable`) does not produce a tight loop. A per-task cooldown of ~30s prevents repeated attempts when other watcher events fire.
- Both coordinators emit a coordinator-specific Wails event on a successful resume so the frontend can surface "auto-resumed" diagnostics.
- Existing tests pass; new tests cover: interrupted-in-backlog auto-resumes (auto-groom), interrupted-in-progress auto-resumes (auto-implement), `lost` task is skipped, cooldown prevents repeat attempts within the window.
- `make lint-go` clean.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited priority=P1, size=S, tags=agent,daemon,resume,recovery,automation
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Done

