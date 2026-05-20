# TB-252: Allow Resume when session_id is present regardless of AgentStatus

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** agent,resume,session,ux
**Agent:** codex
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewedBy:** codex
**ReviewStatus:** success
**ReviewRef:** ce50c51
**Branch:** —

## Goal

Surface the Resume action whenever the latest run has a captured `session_id`, regardless of `AgentStatus`, so users can recover any daemon-lost run without first having to manually flip the status.

## Acceptance Criteria

- [x] `ResumeAgent` in `gui/app/agent_run.go` no longer gates on `AgentStatus == "interrupted"` alone. Eligibility is latest captured session from `resumableSessionID` plus terminal status in `{interrupted, lost, failed, cancelled, success}`; `queued`, `running`, and `needs-user` remain blocked.
- [x] `ErrCannotResume` is kept for the no-session case and `gui/app/agent_service.go` now documents the captured-session plus terminal-status policy.
- [x] Frontend resume affordances are driven by backend `agentResumable` instead of a hardcoded status string.
- [x] Resuming from failed/success/cancelled-style terminal states is intentional: drawer/card copy labels the source status, for example `Resume failed run`.
- [x] Backend tests cover resume rejection without a session, rejection for non-terminal statuses, eligibility for terminal statuses, and `ResumedFromRun` linkage.
- [x] Frontend tests cover failed resumable runs and failed runs without a captured session.
- [x] Verification passed: `cd cli && go test ./...`; `cd gui && go test ./...`; `cd gui/frontend && npm run lint`; `cd gui/frontend && npm run check`; `cd gui/frontend && npm test -- --run`.
- [x] Coordinated with TB-251: recovery taxonomy remains unchanged; this task only widens the resume policy.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=codex, review-status=success, reviewref=ce50c51, acceptance
- 2026-05-20: Done
