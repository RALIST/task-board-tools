# TB-270: Align agent prompts with staged kanban workflow

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** agent
**Tags:** agent,prompt,kanban,automation
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

- [ ] `implement.md` no longer instructs agents to call `tb start {{TASK_ID}}` unconditionally at the end; it states that auto-implement/daemon or the human pull path owns ready -> in-progress, and an implementation agent should first confirm the task is already in-progress.
- [ ] `implement.md` keeps the code-review submission block requiring both `tb review --target` and `tb edit --review-ref` before `tb review --submit`.
- [ ] `groom.md` has one clear stale/outdated-task path: either update the task into a groomed state or use User Attention; it does not both forbid and require `tb close`.
- [ ] `implement.md` removes or rewrites the "small direct done" escape hatch so autonomous implementation normally submits to code-review unless the task/user explicitly authorizes bypassing review.
- [ ] `implement.md` routes clarification through User Attention instead of telling agents to "add a comment and wait" without a supported comment command.
- [ ] `review.md` has one clear pass path using TB-272's managed pass flow once available; before TB-272 lands, the prompt names the temporary two-step fallback explicitly.
- [ ] `review.md` treats top-level `**ReviewRef:**` as the machine-readable review target and `## Review Target` as supplementary human prose; missing `ReviewRef` triggers User Attention rather than guessing from prose alone.
- [ ] `review.md` has one clear fail path: blocking findings use `tb review --fail`, returning the task to `ready` with `review-failed`.
- [ ] `review.md` no longer tells review agents to commit code or contains contradictory "do not run `tb done`" versus "run `tb done`" wording.
- [ ] Prompt tests or renderer snapshots are updated if present; otherwise note that direct file review was the verification.
- [ ] Verification includes `cd gui && go test ./...` if prompt render tests exist; otherwise a targeted grep for stale prompt strings is recorded in the Log.

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
