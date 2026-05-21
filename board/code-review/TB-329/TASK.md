# TB-329: Needs-user handoff should return in-progress task to ready

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** agent,automation,kanban,needs-user
**GroomedBy:** codex
**GroomStatus:** success
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** efc9d88
**Branch:** —

## Goal

When an implement-mode agent run stops for user attention while the task is in `in-progress`, return the task to `ready` while preserving the `AgentStatus: needs-user` handoff so it no longer consumes an in-progress WIP slot.

## Context

- `needs-user` is the user-attention protocol from TB-182: unresolved tasks keep `AgentStatus: needs-user`, carry a `## User Attention` section, and are skipped until the user clears the status.
- Current bug: an implement run can stop in `in-progress` with `needs-user`; the task then cannot proceed automatically but still counts against the in-progress WIP limit.
- Relevant contracts: `board/CONVENTIONS.md` Agent Handoffs, `docs/ARCHITECTURE.md` AgentStatus and autonomous stage flow, TB-182 user-attention protocol, TB-266 daemon reconciliation, and TB-299 ready/AgentStatus gating.
- Implemented in `StageReconciler`: a finished implement-mode run with `AgentStatus: needs-user` and a non-empty structured `## User Attention` marker moves from `in-progress` back to `ready` through `BoardService.MoveTask`, unless strict ready WIP blocks it and records a durable reconciliation skip.
- Verification: targeted app tests and GUI golangci-lint pass. Full `cd gui && go test ./...` remains blocked by existing TB-338 PromptGroom placeholder failure. Full `make lint-go` remains blocked by existing TB-339 CLI `init.go` errorlint failure.

## Constraints

- Preserve `AgentStatus: needs-user` and the `## User Attention` body; moving to `ready` is only WIP-slot relief, not the primary handoff marker.
- Scope the move to implement-mode handoffs from `in-progress`; do not move backlog groom handoffs, code-review review handoffs, done/archive tasks, or unrelated statuses to `ready`.
- Use managed board mutation paths so `.board.lock`, atomic writes, `BOARD.md` regeneration, status directories, and configured WIP behavior remain intact.
- Ready tasks with `AgentStatus: needs-user` must remain ineligible for auto-implement/manual retry until the user resolves the request and clears the status.

## Acceptance Criteria

- [x] An in-progress task whose implement-mode run records `## User Attention` and terminal `AgentStatus: needs-user` is moved to `ready/` after terminal handling or deterministic reconciliation.
- [x] The ready task still shows `AgentStatus: needs-user` and the full `## User Attention` section in `tb show`; neither is cleared by the move.
- [x] Auto-implement and manual run/groom guards still skip or block the ready `needs-user` task until `tb edit <ID> --agent-status none` clears the handoff.
- [x] Non-implement `needs-user` handoffs are not moved to ready: backlog groom handoffs stay in backlog, code-review review handoffs stay in code-review, and done/archive tasks stay put.
- [x] Managed move behavior is covered for WIP settings: normal/default warn mode frees the in-progress slot; strict ready-WIP blockage leaves the task `needs-user` and reports a visible error/diagnostic without clearing the handoff.
- [x] Automated tests cover the in-progress -> ready handoff, preservation of `AgentStatus` and `## User Attention`, unchanged non-target statuses, and the relevant WIP behavior. Verification includes targeted `cd gui && go test ./app -run 'Test(Daemon_ImplementNeedsUserHandoffMovesToReady|StageReconciler_(ImplementNeedsUser|NonImplementNeedsUser|ProtectedAgentStatuses))'`; full `cd gui && go test ./...` is blocked by existing TB-338. CLI move behavior was not changed, so no CLI tests required.
- [x] Manual smoke note: covered by `TestDaemon_ImplementNeedsUserHandoffMovesToReady`, which simulates the daemon running an implement-mode needs-user handoff, confirms the card/task moves from In Progress to Ready with user-attention state, then clears/requeues and confirms pickup is available again.

## Review Target

commit: efc9d88 gui: return needs-user handoffs to ready

Scope:
- docs/ARCHITECTURE.md
- gui/app/stage_reconciler.go
- gui/app/stage_reconciler_test.go
- gui/app/daemon_integration_test.go

Verification:
- cd gui && go test ./app -run 'Test(Daemon_ImplementNeedsUserHandoffMovesToReady|StageReconciler_(ImplementNeedsUser|NonImplementNeedsUser|ProtectedAgentStatuses))'
- cd gui && golangci-lint run --config ../.golangci.yml ./...
- cd gui && go test ./... attempted; blocked by existing TB-338 PromptGroom placeholder failure.
- make lint-go attempted; GUI lint passed, blocked by existing TB-339 CLI init.go errorlint.

Review notes:
- Two subagent code reviews found no Critical/Important findings.
- Minor doc wording finding fixed and verify-only review passed.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited agent=codex
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited priority=P2, type=bug, size=M, module=gui, tags=agent,automation,kanban,needs-user, title=Needs-user handoff should return in-progress task to ready
- 2026-05-21: Edited goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited implemented-by=codex, implement-status=success
- 2026-05-21: Edited context
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited agentstatus=success
- 2026-05-21: Edited review-target
- 2026-05-21: Edited reviewref=efc9d88
- 2026-05-21: Submitted to code-review

