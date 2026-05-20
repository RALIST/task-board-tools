# TB-307: CLI: replace AgentStatus metadata with per-mode status fields

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Module:** cli
**Tags:** agent-status,per-mode-fields,refactor
**Branch:** —
**Parent:** TB-303

## Goal

Remove the generic AgentStatus CLI/storage surface after replacing each lifecycle write/read with the appropriate GroomStatus, ImplementStatus, or ReviewStatus field.

## Context

- Parent epic: TB-303.
- TB-237 added `GroomedBy` / `GroomStatus`, `ImplementedBy` / `ImplementStatus`, and `ReviewedBy` / `ReviewStatus`, but the CLI still exposes the generic cursor for compatibility.
- Current CLI seams to audit include task parsing/serialization, `tb edit --agent-status`, `tb assign`, `tb ready` cleanup, `tb review --fail` cleanup, JSON output, help text, templates, and tests that assert `AgentStatus` lines.
- TB-299 may already have changed `tb ready` behavior; preserve any shipped bridge behavior while removing the generic field from the final CLI contract.

## Constraints

- Keep markdown as source of truth and preserve file-form and folder-form task support.
- All task mutations must continue through locked atomic writes; do not introduce direct `.md` writes outside the existing atomic helpers.
- Define legacy handling explicitly: existing `**AgentStatus:**` lines must either be removed by a managed migration/cleanup path or tolerated only as legacy input with tests proving they are not re-emitted.
- Do not remove the `Agent` assignment field as part of this task unless the parent epic is updated with that expanded scope.

## Acceptance Criteria

- [ ] CLI task parsing/writing and JSON output no longer emit generic `AgentStatus` / `agentStatus` for current task state.
- [ ] `tb edit --agent-status` and `tb assign` are replaced, migrated, or retired with clear help/error behavior that routes queue/status changes to the appropriate per-mode field.
- [ ] `tb ready`, `tb review --fail`, triage/list/show flows, fixtures, and CLI tests are updated so no current behavior depends on a generic status cursor.
- [ ] Legacy task files containing `**AgentStatus:**` are covered by tests for the chosen migration/compatibility behavior and do not cause data loss.
- [ ] Verification: `cd cli && go test ./...` passes.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
