# TB-269: Docs: define staged autonomous agent workflow

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** docs
**Tags:** automation,docs,kanban,agent
**Agent:** codex
**AgentStatus:** running
**ImplementedBy:** codex
**ImplementStatus:** failed
**ReviewRef:** 54d5549
**Branch:** —

## Goal

Document the three independently toggleable automation stages, their kanban transitions, daemon housekeeping limits, and epic-ordering rule across architecture/features/board guidance.

## Context

- The desired workflow is now a staged system, not one monolithic "agent automation" toggle.
- Existing docs already describe ready/pull, code-review, daemon pickup, user-attention, per-mode attribution, and review-failed behavior, but the end-to-end autonomous flow is scattered.
- The implementation tasks need a single written contract so prompts, daemon selection, GUI toggles, and CLI docs do not drift.

## Constraints / Non-goals

- Docs must reflect the actual task split and existing code surfaces; do not promise behavior not captured by tasks.
- Keep stages independently toggleable: auto-groom, auto-implement, auto-review.
- Document daemon reconciliation as soft/deterministic, not semantic guessing.
- Document the epic-order rule as the first hard dependency policy for auto-implement, with numeric child ID ordering.

## Acceptance Criteria

- [ ] `docs/ARCHITECTURE.md` describes the staged autonomous flow: auto-groom backlog -> ready, auto-implement ready -> in-progress -> code-review, auto-review code-review -> done or ready.
- [ ] `docs/FEATURES.md` or `docs/IMPLEMENTATION.md` records the product behavior and user-facing toggles for all three stages.
- [ ] `board/CONVENTIONS.md` and generated templates are updated if needed so future boards know the autonomous flow and daemon housekeeping limits.
- [ ] Agent prompts (`groom.md`, `implement.md`, `review.md`) are checked for consistency with the staged contract; any prompt changes needed for status moves or handoff wording are made in the relevant implementation task or documented as follow-up.
- [ ] Docs explicitly say failed review returns to `ready` with `review-failed` and clears retry-blocking generic agent state per TB-268.
- [ ] Docs explicitly say auto-implement must not pick a later child in an epic while an earlier child is not done.
- [ ] Verification includes a doc grep/smoke check for stale "backlog-only auto-implement" wording and `cd cli && go test ./...` if templates or CLI help text change.

## Review Target

branch: main
commit: 54d5549 (TB-269 document staged autonomous workflow)

Scope:
- Added the staged autonomous workflow contract to architecture, product, implementation, board conventions, and generated board templates.
- Documented independent auto-groom / auto-implement / auto-review toggles, failed-review retry state cleanup, epic child ordering, and deterministic daemon reconciliation limits.
- Checked groom/implement/review prompt consistency; prompt wording cleanup is tracked by TB-270 and was not folded into the TB-269 commit.

Verification:
- stale wording smoke grep over current docs/prompts/guidance: no matches
- git diff --check -- docs/ARCHITECTURE.md docs/FEATURES.md docs/IMPLEMENTATION.md board/CONVENTIONS.md board/SKILL.md cli/templates.go cli/templates_test.go
- cd cli && go test ./...
- cd cli && go build -o tb .; cp cli/tb tb
- scoped subagent review: No CRITICAL or MAJOR issues found.

## Related Tasks

- **TB-172** — Auto-groom stage.
- **TB-177** — Auto-implement stage.
- **TB-262** — Auto-review stage.
- **TB-266** — Daemon reconciliation limits.
- **TB-267** — Epic child ordering rule.
- **TB-268** — Review-failed retry state.
- **TB-270** — Agent prompt cleanup for staged kanban semantics.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agent=codex
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-20: Edited review-target
- 2026-05-20: Edited reviewref=54d5549
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running

