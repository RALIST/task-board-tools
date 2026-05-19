# TB-256: Test TestRunQueuedAgentSync_ResumeRehydratesParentContext should assert per-mode write on daemon replay

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** agent,testing,metadata
**Agent:** claude
**AgentStatus:** success
**GroomedBy:** claude
**GroomStatus:** success
**Branch:** —

## Goal

Extend `TestRunQueuedAgentSync_ResumeRehydratesParentContext` in `gui/app/agent_run_test.go` to seed `Mode: agent.ModeGroom.String()` in the parent's queued (and started) JSONL events so the daemon-replay resume path is asserted to write `**GroomedBy:** claude` / `**GroomStatus:** success` — not the `ModeImplement` fallback — closing the TB-237 review nit on the daemon-replay branch.

## Context

- TB-237 introduced per-mode agent attribution (`GroomedBy`/`GroomStatus`, `ImplementedBy`/`ImplementStatus`, `ReviewedBy`/`ReviewStatus`); reviewer noted the resume rehydration test never exercised the per-mode write because the parent's queued event lacked a `Mode` field, so `runModeFor` returned the `ModeImplement` fallback and any per-mode assertion would have been vacuous against the originating action.
- Test under change: `TestRunQueuedAgentSync_ResumeRehydratesParentContext` in `gui/app/agent_run_test.go` (around the parent-event seeding block; the function ends with the `RunQueuedAgentSync` replay assertions).
- Production code that must light up on replay (do not modify): `runModeFor` in `gui/app/agent_finish.go` reads `Mode` from the parent's queued event; `applyPerModeAttribution` in `gui/app/agent_run.go` routes `ModeGroom` → `GroomedBy`/`GroomStatus`.
- Status note: this work appears to already be implemented in commit `861d474` ("test: TB-237: extend resume rehydrate test to assert per-mode write"). Verify on pickup; if green and matching the criteria below, just move to done.

## Constraints / non-goals

- Test-only change; no production-code edits.
- Keep all existing assertions intact (`finalStatus == "success"`, `RunInput.Mode == ModeResume`, `SessionID`, `ProjectRoot`, `Prompt == PromptResume`, `TB_BOARD_PATH` env).
- Resume must not introduce a fourth action — the assertion must positively confirm the per-mode write lands only on the parent's slot (groom), and explicitly that `**ImplementedBy:**` and `**ReviewedBy:**` are NOT written by the replay.

## Acceptance Criteria

- [ ] Parent's queued event in `TestRunQueuedAgentSync_ResumeRehydratesParentContext` (`gui/app/agent_run_test.go`) is seeded with `Mode: agent.ModeGroom.String()` (so `runModeFor` returns `ModeGroom` instead of falling back to `ModeImplement`).
- [ ] Parent's started event is also seeded with `Mode: agent.ModeGroom.String()` for JSONL consistency.
- [ ] After `RunQueuedAgentSync` replay, the test asserts the task body contains `**GroomedBy:** claude`.
- [ ] After `RunQueuedAgentSync` replay, the test asserts the task body contains `**GroomStatus:** success`.
- [ ] The test asserts the task body does NOT contain `**ImplementedBy:**` or `**ReviewedBy:**` (resume must update only the parent action's slot).
- [ ] All existing assertions in the test remain in place (`finalStatus`, `RunInput.Mode`, `SessionID`, `ProjectRoot`, `Prompt`, `TB_BOARD_PATH` env).
- [ ] `cd gui && go test ./app/ -run TestRunQueuedAgentSync_ResumeRehydratesParentContext` passes.
- [ ] `cd gui && go test ./app/...` stays green (no other test regressed by the JSONL change).

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=success, groomed-by=claude, groom-status=success

