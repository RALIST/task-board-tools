# TB-254: Stale recovery should write per-mode pair when marking interrupted

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** agent,metadata,attribution,recovery
**Branch:** —

## Goal

TB-237 follow-up nit: gui/app/agent_recovery.go marks tasks AgentStatus=interrupted on dead-PID recovery but does not update the per-mode pair (GroomStatus/ImplementStatus/ReviewStatus). If the daemon crashes mid-groom, GroomStatus keeps its prior terminal value (or stays empty) while legacy AgentStatus becomes 'interrupted' — attribution of the interrupted action is lost. The originating mode is already available via ResumeCandidate.Mode (set during recovery scan in agent_recovery.go:506-510). Acceptance: when RecoverStale writes 'interrupted' to AgentStatus, it also writes 'interrupted' to the matching per-mode status field for the action that was running.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-19: Created
