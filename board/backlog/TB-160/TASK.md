# TB-160: TB-93/GUI: TestRecoverStale_DurableCancelledTaskIgnored tests the wrong code path

**Type:** bug
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,testing,recovery
**Branch:** —
**Parent:** TB-93

## Goal

gui/app/agent_recovery_test.go:319-387 - the test claims to verify the 'never overwrite cancelled' carve-out but the task it creates has agentStatus: cancelled, which makes the recovery loop skip it at line 87-93 of agent_recovery.go (the filter is t.AgentStatus == 'running'). The test passes for the wrong reason - it tests the candidate filter, not the carve-out at line 116-125. The correct cancelled-carve-out test is TestRecoverStale_FolderCancelledCarveOut (line 322) which uses agentStatus: running and an in-JSONL finished{cancelled}. Fix: rename or rescope the test to TestRecoverStale_CancelledAgentStatusSkipsCandidate so the intent matches what's actually under test. Source: GUI backend review finding #9.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
