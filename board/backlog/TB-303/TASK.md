# TB-303: Epic: remove generic AgentStatus field

**Type:** tech-debt
**Priority:** P2
**Size:** XL
**Agent:** codex
**AgentStatus:** running
**Tags:** agent-status,per-mode-fields,epic,refactor
**Module:** workflow
**Branch:** —

## Goal

Remove generic `AgentStatus` as a live scheduling/status field across the project. Each autonomous stage must use its own mode-specific status (`GroomStatus`, `ImplementStatus`, `ReviewStatus`) plus JSONL run history for queueing, active, terminal, recovery, and resume behavior, so current CLI, GUI, docs, and board surfaces no longer emit or depend on a generic `AgentStatus` field.

## Subtasks

- **TB-307** (M) — CLI: replace AgentStatus metadata with per-mode status fields
- **TB-308** (L) — GUI: run lifecycle uses per-mode statuses only
- **TB-309** (M) — Frontend: remove generic AgentStatus display dependency
- **TB-310** (M) — Docs and board cleanup for AgentStatus removal

## Context

- TB-237 introduced per-mode attribution fields while keeping `AgentStatus` as a backwards-compatible most-recent-run cursor.
- TB-268 and TB-299 are bridge fixes that clear or narrow the generic cursor for review-failed and auto-implement pickup; they do not remove the field from the project.
- Current documented/code surfaces still include `AgentStatus`: CLI metadata parsing/edit/json/help, `tb assign`, `tb ready`, review fail cleanup, GUI runner/daemon/recovery/cancel/resume flows, frontend task types and chips, generated board conventions/templates, and canonical docs.
- Child tasks for this epic: TB-307 (CLI/storage), TB-308 (GUI runner/daemon lifecycle), TB-309 (frontend display/controls), TB-310 (docs/templates/board cleanup).
- Related bridge/prerequisite tasks: TB-237, TB-268, TB-299, TB-254.

## Constraints

- Remove only the generic status cursor; do not remove `Agent` assignment or the per-mode `GroomedBy` / `ImplementedBy` / `ReviewedBy` attribution fields unless a child task explicitly proves that is required.
- Preserve the canonical stage split: groom, implement, and review remain separate modes with separate status fields.
- Preserve JSONL run history, captured `session_id` resume behavior, file-form and folder-form task support, lock ordering, and atomic task writes.
- Map `queued`, `running`, `success`, `failed`, `cancelled`, `interrupted`, `lost`, and `needs-user` intentionally onto the relevant per-mode status or run-history-derived guard; do not leave a hidden generic fallback.
- Make code changes before docs/board cleanup so generated guidance and live board metadata are updated against the final contract.
- Keep the existing bridge tasks in scope by reference only; do not duplicate TB-268 or TB-299 behavior inside this epic unless their implementation needs to be adjusted for complete removal.

## Acceptance Criteria

- [ ] TB-307, TB-308, TB-309, and TB-310 are completed or explicitly superseded by narrower follow-up tasks linked from this epic.
- [ ] `tb show --json`, `tb ls --json`, and GUI task DTO/types no longer expose an `agentStatus` field for current task state.
- [ ] CLI, GUI runner/daemon, and autonomous coordinators queue, run, finish, cancel, recover, resume, and block tasks using per-mode statuses plus JSONL run history, with no live dependency on generic `AgentStatus`.
- [ ] Frontend cards/drawer/run controls show accurate groom/implement/review state without reading or rendering generic `AgentStatus`; manual GUI smoke is recorded on the UI child task.
- [ ] Canonical docs, generated board templates, local board skill/conventions, and live active task metadata no longer advertise or require `AgentStatus`; any remaining `AgentStatus` string matches an explicit legacy/history allowance documented by TB-310.
- [ ] Verification for the completed epic includes `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run`, and a final `rg -n 'AgentStatus|agentStatus|--agent-status' cli gui docs README.md AGENTS.md board/CONVENTIONS.md board/SKILL.md` audit.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited type=tech-debt, size=XL, module=workflow, tags=agent-status,per-mode-fields,epic,refactor, title=Epic: remove generic AgentStatus field
- 2026-05-20: Edited goal
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance

