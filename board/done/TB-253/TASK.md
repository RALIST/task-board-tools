# TB-253: GUI Run History shows multiple concurrent RUNNING rows for one task

**Type:** bug
**Priority:** P1
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** agent,gui,task-drawer,run-history,stale-state
**GroomedBy:** claude
**GroomStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**Branch:** —

## Goal

The TaskDrawer's Run History panel can show two or more runs simultaneously in
`RUNNING` state for the same task, which violates the single-active-run-per-task
invariant. Reconcile or hide the stale "RUNNING" row(s) so only the one truly
in-flight run (if any) reads as running.

**Context**

- Attachment `Снимок экрана 2026-05-19 в 20.59.16.png` shows the bug: two `review`-mode rows for the same task with the `RUNNING` badge — claude (15m ago) and codex (40m ago) — with no terminal row between them.
- Single-active-run-per-task is enforced at two layers: in-process via `AgentService.active` in `gui/app/agent_service.go` (TB-47, with the `ErrAlreadyRunning` check in `RunAgent`) and at daemon enqueue via the active-set dedup (TB-55). For the screenshot state to exist, at least one of the two "RUNNING" rows must be stale — its underlying process is gone but its on-disk/runtime view never transitioned to a terminal status.
- The Run History panel is fed by `ListAgentRuns`/`StreamAgentRuns` in `gui/app/agent_runs.go` (replays per-task `.agent-state.jsonl`) and the frontend store at `gui/frontend/src/lib/stores/runs.ts`. The rendering surface is in the TaskDrawer / `AgentRunLog.svelte` area.
- Stale-running reconciliation lives in `gui/app/agent_recovery.go` (`RecoverStale` — checks PID liveness, writes `interrupted` when a session id is present, otherwise `failed`; see `docs/ARCHITECTURE.md` → "Session resume"). The "running" badge in Run History should never outlive a dead PID past one reconcile pass.
- Related follow-ups already on the board:
  - TB-254 — stale recovery doesn't update per-mode status pair when marking `interrupted`; relevant if the fix also touches `RecoverStale`.
  - TB-255 — per-mode chip can be stale while a same-mode run is in flight; same drawer surface but a different row type.

**Constraints / non-goals**

- Do not relax the dedup invariants in TB-47 (`AgentService.activeRuns`) or TB-55 (daemon active set). A second concurrent run on the same task must still be rejected at launch.
- All structured task mutations go through `tb edit` and `.board.lock`; no direct `.md` writes outside `cli/atomicfs.go`.
- `cancelled` is reserved for user-initiated cancels and `interrupted` is reserved for recovery — keep that split per CLAUDE.md.
- This task fixes display + reconciliation for **already-terminated** runs that still read as `RUNNING`. It does not introduce kill-on-second-launch semantics, does not add a "force-clear" UI button, and does not change AgentStatus values.
- No DB / cross-process state; rely on the existing JSONL + PID-liveness contract.
- Do not change which mode pairs are persisted on the task file (that's TB-237's surface) — only the run-history row state needs to be correct.

**Related Tasks**

- **TB-47** — `AgentService.RunAgent` activeRuns map; in-process dedup this bug bypasses.
- **TB-55** — Daemon active-set dedup; cross-source dedup this bug bypasses.
- **TB-130** — Session resume + `interrupted`/`failed` recovery contract used by stale reconciliation.
- **TB-237** — Per-mode agent attribution; same TaskDrawer surface, different field set.
- **TB-254** — Stale-recovery per-mode status follow-up; the fix here may overlap.
- **TB-255** — TaskDrawer per-mode chip staleness; sibling drawer-row staleness bug.

## Acceptance Criteria

- [x] Reproduction is documented: a sequence that ends with the GUI Run History panel listing two rows in `RUNNING` state for one task ID is captured in the Context section.
- [x] Root cause is identified and recorded in the task Log: stale recovery only reconciled the latest JSONL run, so older started-only segments remained active in `ListRuns`/Run History.
- [x] At any point after recovery, the Run History panel for a single task shows no stale dead rows whose status badge reads `RUNNING`. Older same-task dead runs are terminalized as `interrupted` or `failed`, matching the recovery contract.
- [x] When the daemon or GUI restarts with JSONL streams that have `started` but no terminal event and dead PIDs, `RecoverStale` reconciles all stale runs before `ListRuns` returns them.
- [x] Concurrent attempts to launch a second run on the same task while another is genuinely in-flight remain blocked by the existing `AgentService.activeRuns` and daemon active-set guards; the fix only changes stale dead-run reconciliation.
- [ ] Manual GUI verification (record steps + screenshots in the task Log): not run in this headless session; automated Go + Vitest coverage exercises the same stale-row transition.
- [x] Automated coverage: `TestRecoverStale_TerminalizesOlderStartedOnlyRunsBeforeListRuns` covers two dead started-only segments, and `runs.test.ts` verifies two recovery-terminal events clear both rows out of the `running` bucket.
- [x] `cd gui && go test ./...` and `cd gui/frontend && npm run check && npm test` pass.

## Attachments

- Снимок экрана 2026-05-19 в 20.59.16.png

## Log

- 2026-05-19: Created
- 2026-05-19: Attached Снимок экрана 2026-05-19 в 20.59.16.png
- 2026-05-19: Edited body via GUI
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited title=GUI Run History shows multiple concurrent RUNNING rows for one task
- 2026-05-19: Edited module=gui, tags=agent,gui,task-drawer,run-history,stale-state
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited goal
- 2026-05-19: Edited priority=P1, type=bug, size=M
- 2026-05-19: Edited agentstatus=success, groomed-by=claude, groom-status=success
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-19: Root cause — recovery only acted on the latest run view, so older started-only JSONL segments could remain visible as RUNNING even after their PIDs were dead. Implementation complete — recovery now terminalizes all stale runs while only the latest run writes task AgentStatus; store coverage confirms multiple terminal events clear multiple running rows.
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-19: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-19: Done

