# TB-299: Auto-implement: gate on per-mode ImplementStatus, clear AgentStatus on tb ready

**Type:** tech-debt
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** auto-implement,daemon,gate,per-mode-fields
**Branch:** —

## Goal

The auto-implement coordinator gates pickup on `t.AgentStatus != ""`. Because
the generic `AgentStatus` cursor is shared across all modes, a successful
auto-groom run (which writes `AgentStatus=success` alongside
`GroomStatus=success`) leaves the cursor set on backlog tasks; when
auto-groom then promotes the task to ready via `tb ready`, the cursor is
preserved and the implement gate silently skips the task. Every ready task
that auto-groom touches is permanently stranded until a human runs
`tb edit --agent-status none` by hand.

The per-mode attribution fields (TB-237) already exist for exactly this
reason. Use them as the source of truth for in-flight detection; keep the
generic cursor for display and for user-intent statuses (`cancelled`,
`needs-user`) that should block any mode.

Also clean up the kanban commitment point: `tb ready` (and the GUI's
`ReadyTask` that wraps it) should clear the generic AgentStatus on
backlog -> ready, mirroring what TB-268 already does for the
code-review -> ready failed-review path.

## Acceptance Criteria

- [x] `gui/app/auto_implement.go` candidate-pass gate no longer blocks on
  arbitrary non-blank `AgentStatus`. New `implementGateBlocker` helper
  blocks only on user-intent statuses (`cancelled`, `needs-user`) on the
  generic cursor and on in-flight per-mode `ImplementStatus`
  (`queued`, `running`).
- [x] `cli/ready.go` `promoteToReady` clears the generic `AgentStatus` on
  successful backlog -> ready promotion via a new
  `clearReadyAgentStatus` helper that takes the board lock and writes
  atomically. Per-mode attribution lines (`GroomStatus`,
  `ImplementStatus`, `ReviewStatus`) are preserved so history stays
  intact.
- [x] Tests added:
  - GUI: `TestAutoImplementCoordinator_GroomSuccessAgentStatusDoesNotBlock`,
    `_CancelledAgentStatusStillBlocks`, `_NeedsUserAgentStatusStillBlocks`,
    `_InFlightImplementStatusBlocks` (replaces the old
    `NonBlankAgentStatusSkipped`).
  - CLI: `TestPromoteToReadyClearsAgentStatusForFreshPickup` and
    `TestPromoteToReadyNoopOnAlreadyBlankAgentStatus` in `cli/ready_test.go`.
- [x] `make lint-go` clean; both module test suites pass.
- [x] Verified on the live board: TB-247/TB-249/TB-250/TB-286/TB-287 were
  picked up by auto-implement after `tb edit --agent-status none` cleared
  the stale cursor.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited priority=P1, type=tech-debt, size=S, tags=auto-implement,daemon,gate,per-mode-fields
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
