# TB-256: Test TestRunQueuedAgentSync_ResumeRehydratesParentContext should assert per-mode write on daemon replay

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** agent,testing,metadata
**Branch:** —

## Goal

TB-237 follow-up nit: TestRunQueuedAgentSync_ResumeRehydratesParentContext in gui/app/agent_run_test.go seeds a parent queued event without a Mode field, so runModeFor falls back to ModeImplement and the test never asserts the per-mode write lands on the originating action. Acceptance: extend the test to seed Mode: 'groom' in the parent's queued JSONL event, then assert that the replayed resume run writes **GroomedBy:** (not **ImplementedBy:**) to close the regression gap on the daemon-replay branch.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-19: Created
