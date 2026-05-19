# TB-182: Add special labes\tags\status for user attention

**Type:** feature
**Priority:** P1
**Size:** L
**Agent:** claude
**AgentStatus:** interrupted
**Tags:** user-attention,agent,automation,ux,epic
**Module:** agent
**Branch:** —

## Goal

Ship a shared user-attention protocol that lets autonomous agents stop safely when they need a human decision, record the exact ask on the task, and prevent auto-groom/auto-implement from silently retrying unresolved work.

## Context

- Current agent lifecycle uses `AgentStatus` for `queued`, `running`, `success`, `failed`, and `cancelled`; docs list these in `docs/ARCHITECTURE.md` and task parsing/validation lives in `cli/task.go` and `cli/edit.go`.
- Current implement prompt says to add a comment and wait for clarification, but there is no structured marker, task section, CLI command, or GUI display for user attention.
- Auto-groom (TB-172/TB-174/TB-175) and auto-implement (TB-177/TB-179/TB-180) need a machine-readable unresolved state so automation can skip tasks that require a user response.
- Use one explicit contract across CLI, docs/prompts, GUI, and automation: `AgentStatus: needs-user` plus a `## User Attention` section containing the specific question/action and unblock condition.

## Constraints / Non-goals

- Do not use kanban column moves, task type, or tags as the primary marker; the durable marker is `AgentStatus: needs-user`.
- `needs-user` is not task completion and is not a failure/cancel replacement. It means the last agent run stopped because user input is required.
- Agents must write the marker and attention note through managed `tb` commands, preserving board locks, atomic writes, and regenerated `BOARD.md` behavior.
- Auto-groom and auto-implement must skip unresolved `needs-user` tasks until the user resolves the ask and clears/resets the agent status.
- Keep manual workflows available: the GUI and CLI should tell the user what is needed instead of hiding controls or retrying silently.

## Subtasks

- **TB-183** (M) — CLI: add user-attention agent status and note section
- **TB-184** (S) — Docs: define user-attention handoff protocol
- **TB-185** (M) — GUI: surface user-attention state and automation guard
## Acceptance Criteria

- [x] **TB-183** is done: the CLI supports `AgentStatus: needs-user`, exposes it in parsed/JSON task data, and provides a managed way to write or replace `## User Attention`.
- [x] **TB-184** is done: board conventions, board skill text, generated templates, and implement/groom prompts document when and how agents request user attention.
- [x] **TB-185** is done: the GUI surfaces `needs-user` tasks, shows the attention request, and guards manual/automatic run paths from retrying unresolved work.
- [x] End-to-end CLI behavior: an agent can mark a task as needing user input, include a precise question/action and unblock condition, and `tb show` plus JSON output make that request visible.
- [x] End-to-end automation behavior: auto-groom and auto-implement skip `needs-user` tasks without mutating task metadata, creating run artifacts, or entering retry loops.
- [x] End-to-end resolution behavior: after the user answers and clears/resets the agent status through the supported flow, manual Run/Groom and eligible automation can proceed normally.
- [x] Manual test note: create a sample backlog task, mark it `needs-user` with a `## User Attention` request, confirm CLI output and GUI card/drawer display the ask, confirm manual Run/Groom and auto-groom/auto-implement skip it, then clear/reset the status and confirm normal execution is available again.
- [x] Verification for the epic includes `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Related Tasks

- **TB-172** — Auto-groom epic that needs this unresolved-user-input guard.
- **TB-174** — Auto-groom queueing must skip `needs-user` tasks.
- **TB-175** — Auto-groom feedback UX should stay consistent with user-attention UX.
- **TB-177** — Auto-implement epic that needs this unresolved-user-input guard.
- **TB-179** — Auto-implement queueing must skip `needs-user` tasks.
- **TB-180** — Auto-implement feedback UX should stay consistent with user-attention UX.
- **TB-183** — Child: CLI status and attention-note support.
- **TB-184** — Child: docs and prompt protocol.
- **TB-185** — Child: GUI display and automation guard.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited priority=P1, type=feature, size=L, module=agent, tags=user-attention,agent,automation,ux,epic, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited agentstatus=success
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done
- 2026-05-19: Edited agentstatus=interrupted

