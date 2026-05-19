---
name: project-needs-user-carveout
description: TB-182 needs-user carve-out invariant in recordTerminal — scope and rationale
metadata:
  type: project
---

The `needs-user` AgentStatus (TB-182) is preserved over runner exit status inside `gui/app/agent_run.go`'s `recordTerminal` carve-out (currently lines ~774-794), but the carve-out is intentionally scoped to `StatusSuccess` and `StatusFailed` only. Explicit `StatusCancelled` (user Cancel button + daemon shutdown via `finishCancelled`) and `StatusInterrupted` (only ever written by `RecoveryService.markInterrupted`, which does NOT go through `recordTerminal`) still write through.

**Why:** an agent that calls `tb edit --agent-status needs-user` mid-run must not be silently overwritten by the runner's mapped exit status (success/failed). But user-explicit Cancel and recovery-detected interruption are stronger intent than the agent's needs-user marker.

**How to apply:** when reviewing changes to `recordTerminal`, the `mapRunnerOutcome` table, or any new AgentStatus writer, confirm:
- The carve-out gate stays `(status == StatusSuccess || status == StatusFailed)`. Adding cancelled/interrupted would silently swallow user/recovery intent.
- New AgentStatus writers either route through `recordTerminal` (so the carve-out applies) or are explicitly justified bypassing it (currently only `RecoveryService` does — for cancelled carve-out and interrupted).
- `isReadyForDaemon` keeps the `AgentStatus == "queued"` gate so the daemon never enqueues a needs-user task by accident. `IsAutomationEligible` is the future-facing predicate for TB-174/TB-179 auto-loops and explicitly returns false for needs-user.
- The frontend's drawer disables Run/Groom on needs-user and the Clear button goes through `editTask({ agentStatus: 'none' })` → `clearField` in cli/edit.go (drops the line entirely; does not write the literal string "none").

Related: [[project-agent-status-write-paths]] (future memory if a new writer is added).
