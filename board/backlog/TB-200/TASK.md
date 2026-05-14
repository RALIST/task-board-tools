# TB-200: Docs: document code-review workflow

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** docs
**Tags:** epic-tb194,code-review,docs,prompts
**Branch:** —
**Parent:** TB-194

## Goal

Document the Code Review workflow for humans and agents so the new column, review sections, commands, failure marker, and automation rules have one shared contract.

## Context

- Parent epic: TB-194.
- Board workflow docs live in `board/CONVENTIONS.md` and `board/SKILL.md`; repo-level agent instructions live in `AGENTS.md` / `CLAUDE.md` where present.
- Product/architecture docs live in `docs/`, especially `docs/FEATURES.md`, `docs/IMPLEMENTATION.md`, and `docs/ARCHITECTURE.md`.
- Agent prompt templates live under `gui/internal/agent/prompts/` and are embedded in the GUI backend.

## Constraints / Non-goals

- Docs must name exact commands and section names after TB-195/TB-196/TB-198/TB-199 choose them.
- Humans and agents should both understand the happy path: implement -> set Review Target/Reviewer Notes -> submit to Code Review -> run/human review -> done.
- Docs must explain the failure path: write Review Findings -> mark `review-failed` -> move back to backlog -> prioritize rework.
- Make clear that `code-review` is a board status and `review-failed` is a tag, not an `AgentStatus`.
- Do not duplicate long implementation internals across docs; link to architecture/source references where useful.

## Related Tasks

- **TB-194** - Parent epic.
- **TB-195** - CLI status and submit flow to document.
- **TB-196** - Review metadata commands/sections to document.
- **TB-197** - GUI behavior/manual test notes to document.
- **TB-198** - Review agent mode and prompt rules to document.
- **TB-199** - Failed-review marker and automation priority to document.
- **TB-177** - Auto-implement interaction.
- **TB-182** - Separate user-attention protocol to distinguish from review failure.

## Acceptance Criteria

- [ ] `board/CONVENTIONS.md` describes the `code-review` column, when to move tasks there, required Review Target/Reviewer Notes guidance, and the Review Findings/failure loop.
- [ ] Agent-facing repo rules and board skill text tell implementer agents to submit only after tests/verification, include implementation refs, and leave reviewer notes when useful.
- [ ] Review-mode prompt/docs tell reviewer agents to inspect implementation refs, write actionable findings to `## Review Findings`, and use the failed-review handoff when appropriate.
- [ ] Product/architecture docs describe status semantics, CLI command surfaces, GUI column behavior, and `review-failed` automation priority without drifting from implemented command names.
- [ ] Docs explicitly distinguish `code-review` status, `review-failed` tag, and unrelated `AgentStatus` values such as `needs-user` from TB-182.
- [ ] Examples include both happy path and failed-review path using concrete commands.
- [ ] Manual test note from TB-197/TB-198/TB-199 is captured in docs or release notes so a human can smoke the full workflow.
- [ ] Verification includes spelling/link checks by inspection plus the relevant docs-adjacent tests, if any.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

