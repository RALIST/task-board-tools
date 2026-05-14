# TB-184: Docs: define user-attention handoff protocol

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** docs
**Tags:** user-attention,agent,prompts,docs
**Branch:** —
**Parent:** TB-182

## Goal

Document the user-attention handoff protocol so autonomous implement and groom agents know exactly when to stop, what status to set, and where to write the question or blocker for the user.

## Context

- `board/CONVENTIONS.md` defines the canonical board format and status vocabulary.
- `board/SKILL.md` tells agents how to work with the board manually.
- `gui/internal/agent/prompts/implement.md` currently says to add a comment and wait for clarification, but there is no structured section or status.
- `gui/internal/agent/prompts/groom.md` carries the grooming mutation contract and must explain the same handoff path for unclear grooming work.
- `cli/templates.go` generates default board conventions/skill text for new boards, so template text must stay in sync with the repo docs.

## Constraints / Non-goals

- The protocol must require a specific user ask, attempted context, and unblock condition; vague "blocked" notes are not enough.
- Agents must use the managed CLI path from TB-183 and must not write directly to task markdown.
- Do not change the meaning of kanban columns: user attention is an agent state, not a board-column move.
- Keep guidance short enough for prompts while preserving concrete examples.

## Acceptance Criteria

- [ ] `board/CONVENTIONS.md` defines `AgentStatus: needs-user` and a `## User Attention` section with required content: reason/category, specific question or action, relevant attempted context, and unblock condition.
- [ ] `board/SKILL.md`, `gui/internal/agent/prompts/implement.md`, and `gui/internal/agent/prompts/groom.md` tell agents to set `needs-user` and fill `## User Attention` when they cannot continue safely.
- [ ] Prompt guidance covers unclear requirements, external/manual blockers, conflicting instructions, failed verification that needs a user decision, and stale/outdated tasks.
- [ ] `cli/templates.go` generated conventions/skill templates include the same status and protocol so newly initialized boards inherit it.
- [ ] `docs/ARCHITECTURE.md` and `docs/FEATURES.md` mention the lifecycle semantics and automation skip behavior.
- [ ] Existing prompt/template tests are updated or added so the protocol text cannot drift silently.
- [ ] Verification includes `cd cli && go test ./...` and `cd gui && go test ./...`.

## Related Tasks

- **TB-182** — Parent epic for the shared user-attention protocol.
- **TB-183** — Provides the CLI status and managed attention-section mutation path documented here.
- **TB-185** — Uses the documented protocol in the GUI.
- **TB-172** — Auto-groom epic that should follow the protocol when grooming cannot finish.
- **TB-177** — Auto-implement epic that should follow the protocol when implementation cannot finish.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

