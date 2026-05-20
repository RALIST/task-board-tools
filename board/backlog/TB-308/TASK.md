# TB-308: GUI: run lifecycle uses per-mode statuses only

**Type:** tech-debt
**Priority:** P2
**Size:** L
**Module:** gui
**Tags:** agent-status,per-mode-fields,daemon,refactor
**Branch:** —
**Parent:** TB-303

## Goal

Move GUI runner, daemon, recovery, cancel, resume, and auto-stage eligibility from the generic AgentStatus cursor to mode-specific status fields and run-history data.

## Context

- Parent epic: TB-303.
- GUI runner and daemon code currently treat `AgentStatus` as the queue, active, terminal, cancel, stale-recovery, and resume-visible cursor.
- Per-mode fields from TB-237 already exist and TB-299 narrows one auto-implement gate, but complete removal requires the run lifecycle itself to stop writing and reading the generic cursor.
- Surfaces to audit include manual Run/Groom/Review, daemon pickup, `RunQueuedAgentSync`, terminal recording, cancellation, stale recovery, resume eligibility, auto-groom, auto-implement, and auto-review.

## Constraints

- Preserve JSONL event ordering, captured session ids, run logs, resumability rules, shutdown/cancel behavior, and file-form/folder-form artifact paths.
- Queueing must be unambiguous: each queued run has exactly one owning mode (`groom`, `implement`, or `review`) and updates only that mode's status field.
- `needs-user`, `cancelled`, `interrupted`, and `lost` must keep their existing safety semantics, but be represented without a generic task-level status fallback.
- Do not weaken auto-stage guards, epic-order checks, WIP handling, or review-failed retry behavior while changing the status source.

## Acceptance Criteria

- [ ] Manual Run, Groom, and Review queue paths write mode-specific `queued` / `running` state and daemon pickup consumes mode-specific queued runs without requiring `AgentStatus`.
- [ ] Terminal recording writes the matching per-mode status/agent pair for success, failed, cancelled, interrupted, lost, and needs-user outcomes without rewriting a generic cursor.
- [ ] Cancel, stale recovery, daemon shutdown, and resume eligibility continue to work from JSONL plus per-mode status, including captured-session resume and no-session lost recovery.
- [ ] Auto-groom, auto-implement, and auto-review eligibility/skip diagnostics use per-mode status and run history instead of generic `AgentStatus`.
- [ ] Tests cover daemon pickup, terminal writes, cancel precedence, stale recovery, resume eligibility, and auto-stage gating for the new status source.
- [ ] Verification: `cd gui && go test ./...` passes.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
