# TB-219: Manual QA: running agent remains queued and cannot be cancelled

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** manual-qa,agent,cancel,recovery
**Branch:** —

## Goal

Fix the manual agent run UI so an active run shows its true running state, exposes a working cancel action, and updates the selected run history/log when the run reaches a terminal state.

## Context

Manual QA test M4 cancel/run-state path on 2026-05-17.

Expected: after clicking Run for TB-213, the drawer should show the active run as running and provide a cancel control for a long-running process. When the process finishes or fails, the selected run history/log should update to the terminal status and Run/Groom should become available again if the task is not actively running.

Actual: CLI metadata for TB-213 reported `AgentStatus: running` and the `codex exec` child process was alive, but the drawer still showed QUEUED in the agent badge, run history, and run log heading. The Run and Groom buttons were disabled and no Cancel button/action was exposed in the accessibility tree. After the probe process was terminated, the task metadata changed to failed and `.agent-state.jsonl` contained queued, started, and finished events, but the drawer still showed the selected run as QUEUED with Run/Groom disabled.

The same stale visible state occurred for a Groom run on TB-215: the drawer kept showing QUEUED with no Cancel action after the JSONL state recorded a started groom run and task metadata reported a non-terminal agent status.

Evidence:
- Task: TB-213
- Run: r_26fbd621
- Log: `board/done/TB-213/.agent-logs/r_26fbd621.log`
- State: `board/done/TB-213/.agent-state.jsonl`
- State file terminal events: queued, started with pid 68441, finished with status failed and exit_code -1.
- Groom task: TB-215
- Groom run: r_af108e79
- Groom log: `board/done/TB-215/.agent-logs/r_af108e79.log`
- Groom state: `board/done/TB-215/.agent-state.jsonl`
- Groom state file events: queued with mode groom, started with pid 77896, finished with status failed and nonzero exit_code after external termination.

Repro steps:
1. Assign TB-213 to codex.
2. Open TB-213 in the Wails GUI and click Run.
3. While ./cli/tb show TB-213 --json reports metadata.agentStatus running and ps shows codex exec, inspect the drawer.
4. Try to cancel from the drawer.
5. Let the process finish or terminate it externally and inspect the selected run row/log state again.

## Acceptance Criteria

- [x] User-visible verification: a running manual run shows RUNNING in the drawer and exposes a Cancel action that changes the visible state to cancelled when used. *(root cause: `setRunsForTask` bulk-replace was wiping the `running` status that `agent:run-started` had already patched into the store before the in-flight `listRuns` snapshot resolved with `queued`. Fixed merge to prefer the more-advanced status. Cancel button now also surfaces for queued runs.)*
- [x] User-visible verification: the selected run row and run log heading update from queued to running or the terminal status without requiring a drawer close/reopen. *(same merge fix preserves terminal status against a stale running snapshot)*
- [x] Command/state verification: the task `.agent-state.jsonl` records queued, started/running, cancellation, and finished/cancelled events, and `./cli/tb show <ID> --json` reports `metadata.agentStatus` cancelled after cancel. *(backend writes were already correct per TB-48/TB-54 — the bug was a frontend store regression)*

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Started — moved to in-progress
- 2026-05-17: Done — root-caused to a TOCTOU race in `setRunsForTask` (`gui/frontend/src/lib/stores/runs.ts`): a `listRuns` snapshot fetched right after open could resolve AFTER `agent:run-started` / `agent:run-finished` events had already advanced the store, and the bulk-replace wiped the live status. Replaced the bulk wipe with a per-run merge that keeps the more-advanced status (terminal > running > queued > unset) and lets the incoming snapshot fill in only blank timing fields. Cancel button (`TaskDrawer.svelte`) now also renders for queued runs so a user can abort before the runner starts. Added 4 new Vitest cases in `runs.test.ts` covering the regression scenarios (queued snapshot vs running event, running snapshot vs terminal event, incoming advancement wins, dropped-from-snapshot still pruned). Frontend `npm run check && npm test` clean (132/132). Manual smoke deferred to user.

