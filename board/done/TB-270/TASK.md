# TB-270: Align agent prompts with staged kanban workflow

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** agent
**Tags:** agent,prompt,kanban,automation
**Agent:** codex
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewedBy:** codex
**ReviewStatus:** success
**ReviewRef:** ce50c51
**Branch:** —

## Goal

Update implement/review prompt contracts so agents follow ready/pull/code-review semantics and review pass/fail instructions are not contradictory.

## Context

- `gui/internal/agent/prompts/implement.md` still ends with `tb start {{TASK_ID}}`, which can skip the canonical ready -> in-progress pull flow added by TB-239.
- `gui/internal/agent/prompts/review.md` tells review-mode agents both not to run `tb done` and to run `tb done` on pass. The pass/fail contract must be unambiguous before auto-review scales it.
- TB-272 adds the preferred managed pass flow so review agents do not have to compose `tb review --findings` + `tb done` themselves.
- `gui/internal/agent/prompts/groom.md` also has a responsibility contradiction: it forbids `tb close`/status commands, then says outdated tasks should be closed.
- The staged autonomous flow relies on prompts and daemon reconciliation agreeing about which actor owns each move:
  - auto-groom: daemon/coordinator may promote backlog -> ready after clean grooming.
  - auto-implement: daemon/coordinator moves ready -> in-progress; implement agent submits to code-review with ReviewRef.
  - auto-review: review agent records findings and either passes to done via the managed pass flow or fails back to ready.

## Constraints / Non-goals

- Prompt-only task unless tests reveal a prompt renderer fixture that must be updated.
- Do not change runner behavior or daemon selection logic here.
- Keep user-attention handoff wording aligned with TB-182.
- Preserve the ReviewRef guidance from TB-238/TB-235.

## Acceptance Criteria

- [x] `implement.md` no longer instructs agents to call `tb start {{TASK_ID}}` unconditionally; it states the task should already be in-progress and that auto-implement/human pull owns the ready -> in-progress transition.
- [x] `implement.md` keeps the code-review submission block requiring both `tb review --target` and `tb edit --review-ref` before `tb review --submit`.
- [x] `groom.md` has one clear stale/outdated-task path through User Attention; it no longer mixes stale-task handling with `tb close`.
- [x] `implement.md` rewrites the direct-done escape hatch so implementation normally submits to code-review unless the task/user explicitly authorizes bypassing review.
- [x] `implement.md` routes clarification through User Attention instead of unsupported comment/wait language.
- [x] `review.md` names TB-272's managed pass flow and the temporary two-step fallback explicitly.
- [x] `review.md` treats top-level `**ReviewRef:**` as the machine-readable review target and `## Review Target` as supplementary prose; missing `ReviewRef` triggers User Attention.
- [x] `review.md` has one clear fail path: blocking findings use `tb review --fail`, returning the task to `ready` with `review-failed`.
- [x] `review.md` no longer tells review agents to commit code and no longer has contradictory done/review wording.
- [x] Prompt tests were updated in `gui/internal/agent/runner_test.go` and `gui/internal/agent/groom_test.go`.
- [x] Verification passed: `cd gui && go test ./...`; targeted prompt greps found no stale unconditional `tb start {{TASK_ID}}` or unsupported comment/wait instruction in the prompt paths.

## Related Tasks

- **TB-177** — Auto-implement depends on canonical ready -> in-progress semantics.
- **TB-238** — Existing implement prompt ReviewRef update.
- **TB-262** — Auto-review depends on unambiguous review pass/fail instructions.
- **TB-266** — Daemon reconciliation should match prompt ownership.
- **TB-269** — Docs for the staged autonomous workflow.
- **TB-272** — Managed review pass command for the review prompt.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agent=codex
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=codex, review-status=success, reviewref=ce50c51, acceptance
- 2026-05-20: Done
