# TB-255: TaskDrawer per-mode chip stale while same-mode run in flight

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** agent,metadata,attribution,ux
**Branch:** —

## Goal

TB-237 follow-up nit: per-mode (GroomedBy/ImplementedBy/ReviewedBy) is only written at terminal state, so when a new run of the same action starts on a task that already has a per-mode pair, the drawer keeps showing the previous run's status (e.g. 'Groomed: claude · success') while AgentStatus reads 'running'. The legacy chip still tells the truth, but the per-mode row is misleading. Acceptance: TaskDrawer surfaces a hint (e.g. dim/strike or 'stale — re-running' badge) on the per-mode row whose mode matches AgentStatus=running.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-19: Created
