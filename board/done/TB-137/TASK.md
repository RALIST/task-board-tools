# TB-137: Recovery: dead-PID + SessionID -> interrupted (markInterrupted)

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb130,agent,resume,recovery,daemon
**Branch:** —
**Parent:** TB-130

## Goal

Split `recoverOne`'s dead-PID branch in two: when a captured SessionID
is present, transition the task to the new `interrupted` AgentStatus
(via the new `markInterrupted` helper); otherwise keep the existing
`failed{stale after restart}` path. Preserves the TB-61 cancelled
carve-out (cancelled still wins).

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 7 (recovery transition), § 12 task G.

Today `gui/app/agent_recovery.go:154` always marks dead-PID runs as
`failed`. The change is a single conditional: if the latest run has
a `session_id`, it can be resumed → mark `interrupted` instead.

The cancelled carve-out (`agent_recovery.go:116-125`) MUST still
fire first; recovery never overwrites a user-initiated cancel.

## Acceptance Criteria

- [ ] New helper `markInterrupted` in `gui/app/agent_recovery.go`,
      mirroring `markFailed`:
  - Appends synthetic `finished{StatusInterrupted, exit_code=-1,
    reason=<reason>}` JSONL.
  - Edits `--agent-status interrupted` via `c.Edit(...)` (validator
    accepts it after TB-131; convention enforces "recovery only").
  - Emits Wails `agent:run-finished{status:"interrupted", ...}`.
- [ ] `recoverOne` line 154 changes to:
      ```go
      if latest.SessionID != "" {
          return r.markInterrupted(ctx, c, boardDir, t, latest.RunID,
              "interrupted by daemon restart")
      }
      return r.markFailed(ctx, c, boardDir, t, latest.RunID,
          "stale after restart")
      ```
- [ ] `gui/internal/agent/state.go` adds
      `StatusInterrupted Status = "interrupted"` (cross-check with
      TB-131 — likely lands in TB-131; if not, here).
- [ ] `agent.Status` switches in
      `gui/app/agent_finish.go:mapRunnerOutcome`-adjacent code (and
      anywhere else statuses are switched on) handle `interrupted`
      gracefully.
- [ ] Test: dead PID + SessionID present → status flips to
      `interrupted`, JSONL has synthetic `finished{interrupted}`.
- [ ] Test: dead PID + no SessionID → status flips to `failed`
      (existing behaviour, regression guard).
- [ ] Test: latest event is `finished{cancelled}` AND a SessionID
      exists → status stays `cancelled` (carve-out wins).
- [ ] Test: live PID (regardless of SessionID) → recovery skips
      (existing behaviour).
- [ ] Wails event payload for `interrupted` runs includes the
      session_id so the frontend can render the Resume button
      without re-reading the JSONL.

## Related Tasks

- **TB-130** — parent epic.
- Depends on **TB-131** (status enum), **TB-132** (`SessionID` on
  `runRecoveryView`), transitively on **TB-133** + **TB-135** (so
  the JSONL actually carries `session_id`).
- Blocks **TB-138** + **TB-139** (resume only fires from
  `interrupted`), **TB-140** (frontend renders the new pill).

## Log

- 2026-05-14: Created
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done
