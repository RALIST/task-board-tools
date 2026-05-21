# TB-321: GUI: board view misses live running automation tasks

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** gui/frontend
**Tags:** gui,daemon,agents,refresh,status
**Branch:** —

## Goal

The GUI can show a stale board snapshot while GUI-launched automation processes are still running, so active work is not visible in the kanban columns/status counts.

## Context

Observed on 2026-05-21 while GUI-launched `codex exec` processes were still alive for TB-306 and TB-313. `tb board --json` showed both tasks in `inProgress` with `AgentStatus: running`, but the GUI screenshot showed only TB-306 in In Progress and the column header read `1/3`.

This makes automation look idle or missing even though the board source of truth and OS processes say work is running. The bug may be in watcher/event refresh, frontend store patching, board reload coalescing, or card/status rendering. Investigate from live board events rather than assuming OS process polling is needed.

## Constraints

- Markdown task files and `tb board --json` remain the source of truth for displayed board state.
- Do not add direct OS-process polling to the frontend as the primary source of task status.
- Preserve existing watcher debounce and optimistic move behavior unless evidence shows they cause stale snapshots.
- Running/queued agent state should be visually obvious on cards and column counts should match the current board snapshot.
- Avoid broad refactors of board store or watcher code beyond the stale-refresh fix.

## Acceptance Criteria

- [ ] Reproduction test or harness demonstrates a task moving/appearing in `in-progress` with `AgentStatus: running` after daemon/CLI mutation and the GUI store updating to show it.
- [ ] Board column counts and rendered cards match `tb board --json` after `board:reloaded`, `board:opened`, and relevant task update events.
- [ ] Running automation state is visibly distinguishable on task cards; `AgentStatus: running` should not look idle.
- [ ] Refresh coalescing does not drop a queued reload while a previous `refresh()` is in flight.
- [ ] Tests cover missing/new in-progress task appearing after reload and status update from non-running to running.
- [ ] Verification includes `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run`, and a manual GUI smoke against a board with a queued/running task.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance

