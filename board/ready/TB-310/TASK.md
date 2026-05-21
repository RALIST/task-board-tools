# TB-310: Docs and board cleanup for AgentStatus removal

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Module:** docs
**Tags:** agent-status,docs,migration
**Branch:** —
**Parent:** TB-303

## Goal

After CLI and GUI no longer require AgentStatus, update canonical docs/templates and clean live board/task fixtures so the project no longer advertises or emits the generic field.

BLOCKED_BY TB-303

## Context

- Parent epic: TB-303.
- Docs and generated templates still describe generic `AgentStatus` as an architecture invariant, daemon queue source, resume/cancel state, board convention, and CLI command surface.
- Live board task metadata and historical task bodies contain many `AgentStatus` mentions because earlier milestones used it as the canonical cursor.
- This cleanup should run after TB-307/TB-308/TB-309 define and implement the final replacement contract.

## Constraints

- Do not update docs ahead of shipped behavior; docs/templates must describe the live CLI/GUI contract.
- Do not hand-edit generated `board/BOARD.md`; use board tooling for task metadata cleanup and regeneration.
- Preserve useful historical audit text when needed, but make every remaining `AgentStatus` occurrence intentional and easy to classify as legacy/history rather than current behavior.
- Keep `board/CONVENTIONS.md`, `board/SKILL.md`, `cli/templates.go`, AGENTS guidance, and canonical docs aligned.

## Acceptance Criteria

- [ ] `docs/ARCHITECTURE.md`, `docs/FEATURES.md`, `docs/IMPLEMENTATION.md`, `README.md`, AGENTS guidance, `board/CONVENTIONS.md`, `board/SKILL.md`, and `cli/templates.go` describe per-mode statuses and no longer present generic `AgentStatus` as current behavior.
- [ ] Generated board docs for new boards are refreshed through `tb init` / template changes and match the current repository board docs.
- [ ] Live active task metadata is cleaned with managed board commands or a reviewed migration path so active work no longer carries generic `AgentStatus` lines.
- [ ] Tests/fixtures/docs that intentionally retain `AgentStatus` for legacy parsing or changelog history mark that allowance explicitly.
- [ ] Verification includes `tb regenerate`, `tb triage`, and a final `rg -n 'AgentStatus|agentStatus|--agent-status' cli gui docs README.md AGENTS.md board/CONVENTIONS.md board/SKILL.md` audit with each remaining match classified.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Edited body via GUI
