# TB-253: GUI Run History shows multiple concurrent RUNNING rows for one task

**Type:** bug
**Priority:** P1
**Size:** M
**Agent:** claude
**AgentStatus:** success
**Module:** gui
**Tags:** agent,gui,task-drawer,run-history,stale-state
**GroomedBy:** claude
**GroomStatus:** success
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

- [ ] Reproduction is documented: a sequence that ends with the GUI Run History panel listing ≥ 2 rows in `RUNNING` state for one task ID (e.g. start a `review` run with `codex`, kill the daemon mid-run before any terminal JSONL event lands, restart the daemon, then start a second `review` run with `claude`).
- [ ] Root cause is identified and recorded in the task Log — which surface still treats the dead run as running: the on-disk JSONL state (`<status>/<ID>/.agent-state.jsonl`), the Wails `ListAgentRuns`/`StreamAgentRuns` response in `gui/app/agent_runs.go`, the frontend store in `gui/frontend/src/lib/stores/runs.ts`, or the stale-recovery path in `gui/app/agent_recovery.go`.
- [ ] At any point in time the Run History panel for a single task shows at most one row whose status badge reads `RUNNING`. Any older same-task run whose process is no longer alive is rendered as `interrupted` (or `failed` if no session id was captured), matching the recovery contract in `docs/ARCHITECTURE.md` → "Session resume".
- [ ] When the daemon (or the GUI) restarts with a JSONL stream that has a `started` but no terminal event and whose recorded PID is dead, `RecoverStale` reconciles the run to a terminal status before `ListAgentRuns` returns it — the Run History panel never displays a `RUNNING` row backed by a dead PID after the next reconcile cycle completes.
- [ ] Concurrent attempts to launch a second run on the same task while another is genuinely in-flight remain blocked by the existing dedup in `AgentService.activeRuns` (TB-47) and the daemon active set (TB-55); no regression of those guards.
- [ ] Manual GUI verification (record steps + screenshots in the task Log): trigger the reproduction above; confirm the Run History panel collapses to a single `RUNNING` row (or zero) and the older row flips to `interrupted`/`failed`. Re-run a successful `tb-gui` restart and confirm history stays consistent.
- [ ] Automated coverage: a Go test under `gui/app` (or `gui/internal/agent`) constructs a JSONL stream with two `started`-only segments for the same task with different mode/agent pairs and dead PIDs, runs `RecoverStale` + `ListAgentRuns`, and asserts both segments come back with non-`running` terminal statuses. Frontend store test in `gui/frontend/src/lib/stores/runs.test.ts` asserts that two `agent:run-finished`/recovery-terminal events correctly clear both rows out of the `running` bucket.
- [ ] `cd gui && go test ./...` and `cd gui/frontend && npm run check && npm test` pass.

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

