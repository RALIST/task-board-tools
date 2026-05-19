# TB-251: Distinguish agent-failed from daemon-lost in recovery

**Type:** improvement
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** agent,daemon,recovery,reliability
**Branch:** —

## Goal

Make recovery surface daemon-side run loss as a distinct, resumable state rather than overloading `AgentStatus=failed`, which today conflates "agent CLI exited non-zero" with "daemon never saw a `finished` event".

## Acceptance Criteria

- [ ] Audit `RecoveryService.recoverOne` in `gui/app/agent_recovery.go:115-181`. Recovery rules 5 (PID dead + no session) and 6 (no JSONL at all) currently call `markFailed`; rule 2 (JSONL recorded `finished{status: failed}`) also produces `failed`. These three cases must be distinguishable on disk.
- [ ] Pick one of the two designs and document the choice in `docs/ARCHITECTURE.md` and `CLAUDE.md` under the AgentStatus invariants:
  - (a) Widen `interrupted` to cover rule 5 (and possibly rule 6). The `## User Attention` invariant in CLAUDE.md already reserves `interrupted` for "recovery-initiated"; absent session_id still fits that meaning.
  - (b) Introduce a new `lost` AgentStatus value. Update the validator in `cli/task.go` (`validAgentStatuses`), the `tb edit --agent-status` enum, the JSON output, and the GUI status badge styling.
- [ ] `markFailed` is reserved for rule 2 only (the agent CLI itself reported failure). Rule 5 (and 6, if chosen) routes to the new/widened state.
- [ ] The Wails `agent:run-finished` payload distinguishes the cases so the frontend can render a distinct badge ("Lost"/"Interrupted") and the Resume button visibility flows from the same signal.
- [ ] Tests: stale-recovery test cases in `gui/app/agent_recovery_test.go` cover all six routing rules and assert the new on-disk + emit shape.
- [ ] `cd cli && go test ./...` and `cd gui && go test ./...` pass.
- [ ] Coordinate with TB-252 — once `failed` is reserved for real agent failures, the resume eligibility rule there can stop treating recovery-failed runs specially.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance

