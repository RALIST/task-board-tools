# TB-317: CLI: add --agent-status filter for running agent tasks

**Type:** feature
**Priority:** P2
**Size:** S
**Agent:** codex
**Module:** cli
**Tags:** cli,filter,agent-status
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Add an `--agent-status` filter to `tb ls` so users and automation can quickly list tasks with an active running agent, for example `tb ls --status active --agent-status running`, without grepping task markdown or generated board output.

## Context

- `cli/list.go` already owns the multi-value `tb ls` filter pipeline from TB-289: type, priority, module, size, tags, parent, assignment agent, search, status, and JSON output.
- Current lifecycle state is exposed as task metadata `AgentStatus` with values `queued`, `running`, `success`, `failed`, `cancelled`, `interrupted`, `lost`, and `needs-user`; blank/missing status is also common.
- The requested fast path is the active-run query: `tb ls --status active --agent-status running`.
- TB-303/TB-307 may remove the generic `AgentStatus` cursor. If that refactor lands first, implement this filter against the replacement per-mode/run-history status source instead of reintroducing generic metadata.

## Related Tasks

- **TB-289** — Established the current `tb ls` multi-value filter-flag pattern.
- **TB-303** — Parent epic for removing the generic `AgentStatus` cursor.
- **TB-307** — CLI child of TB-303; may change the status source this filter should read.

## Constraints

- Keep `--agent` as the assignment filter; `--agent-status` filters lifecycle/run status and must be documented distinctly.
- Preserve existing `tb ls` semantics: OR within one comma-separated flag, AND across populated flags, priority/ID sorting, status-directory filtering, and unchanged JSON task shape.
- Validate `--agent-status` against the existing lifecycle status enum plus a `none` sentinel for blank/missing status.
- Do not parse `BOARD.md`; use parsed task metadata or the post-TB-303 replacement state source.
- Rebuild and relink the local `tb` binary after CLI changes.

## Acceptance Criteria

- [ ] `tb ls --status active --agent-status running` lists only active-column tasks whose current agent lifecycle status is `running`; it excludes queued, terminal, `needs-user`, blank-status, and archived tasks unless the caller changes `--status`.
- [ ] `--agent-status` accepts comma-separated status values with whitespace trimming and OR-within-flag semantics; `none` matches tasks with no lifecycle status.
- [ ] Invalid `--agent-status` values fail with a clear error that lists the accepted values.
- [ ] `--agent-status` composes with existing filters, including `--agent`, so `tb ls --status active --agent codex --agent-status running` returns the assignment/status intersection.
- [ ] Text output and `--json` output keep the existing shape and ordering for unfiltered and existing-filter calls.
- [ ] CLI help and `cli/README.md` document the new flag and include a running-agent example.
- [ ] Tests cover single running status, comma-separated statuses, `none`, invalid status, combination with `--agent`, and JSON output for the new filter.
- [ ] Verification: `cd cli && go test ./...`; `cd cli && go build -o tb .`; relink the local `tb` binary used by this repo.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited agent=codex
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited type=feature, size=S, module=cli, tags=cli,filter,agent-status, groomed-by=codex, groom-status=success, title=CLI: add --agent-status filter for running agent tasks
- 2026-05-21: Edited goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Edited context
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Committed — moved to ready

