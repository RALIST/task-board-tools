# TB-183: CLI: add user-attention agent status and note section

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** cli
**Tags:** user-attention,agent,metadata
**Branch:** —
**Parent:** TB-182

## Goal

Define the CLI-level board contract for tasks waiting on human input: support `AgentStatus: needs-user` plus a managed `## User Attention` section so agents can record the exact question, blocker, or user action needed without hand-editing markdown.

## Context

- Existing agent metadata is parsed in `cli/task.go`, validated in `cli/edit.go`, and exposed through `cli/json_output.go`.
- Existing agent runs use `AgentStatus` for queueing and terminal state; current values are `queued`, `running`, `success`, `failed`, and `cancelled`.
- `tb edit` already owns metadata writes and section upserts for `## Goal` and `## Acceptance Criteria`; the attention note needs the same managed-write behavior.
- Auto-groom and auto-implement need a durable machine-readable marker so unresolved tasks are skipped instead of retried silently.

## Constraints / Non-goals

- Use `AgentStatus: needs-user` as the machine-readable marker; do not rely on tags, task type, or kanban column for this state.
- `needs-user` means the agent stopped because user input is required. It is not equivalent to `failed`, `cancelled`, or task completion.
- Provide a CLI-managed way to create or replace `## User Attention`; agents must not be instructed to edit task markdown directly.
- Existing status values, JSON shape, atomic writes, board locking, and generated `BOARD.md` behavior must remain compatible.

## Acceptance Criteria

- [x] `AgentStatus` supports `needs-user` everywhere task metadata is parsed, validated, rendered, and emitted as JSON, while existing statuses keep their current behavior.
- [x] `tb edit <ID> --agent-status needs-user` validates and writes the status; `--agent-status none` still clears `AgentStatus` for the user's resolution path.
- [x] A managed CLI path can create or replace `## User Attention` from stdin with the specific user ask and unblock condition, and section parsing treats it as a first-class task section.
- [x] Manual run, groom run, daemon queue, auto-groom, and auto-implement entry points reject or skip `needs-user` tasks with a clear message rather than immediately retrying them.
- [x] Go tests cover enum validation, metadata parsing, JSON output, attention-section upsert, clearing the status, and run/queue skip behavior.
- [x] Verification includes `cd cli && go test ./...` and `cd gui && go test ./...` if GUI backend status handling is touched.

## Related Tasks

- **TB-182** — Parent epic for the shared user-attention protocol.
- **TB-184** — Documents the protocol for agents and board users.
- **TB-185** — Surfaces the status and request in the GUI.
- **TB-174** — Auto-groom queueing must skip unresolved user-attention tasks.
- **TB-179** — Auto-implement queueing must skip unresolved user-attention tasks.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done

