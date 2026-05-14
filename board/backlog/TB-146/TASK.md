# TB-146: TB-93/CLI: attach promotion orphans file-form agent state + logs

**Type:** bug
**Priority:** P0
**Size:** M
**Module:** cli
**Tags:** epic-tb93,review-tb93,attach,promotion,agent-artifacts,testing
**Agent:** codex
**AgentStatus:** success
**Branch:** —
**Parent:** TB-93

## Goal

Preserve a legacy file-form task's existing agent run state and logs when `tb attach` promotes that task to folder form.

## Context

- Parent epic `TB-93` requires folder tasks to own task-local agent artifacts while legacy file tasks continue to use board-root `.agent-state/` and `.agent-logs/` paths.
- `docs/ARCHITECTURE.md` defines the storage split: file-form tasks use `<board>/.agent-state/<ID>.jsonl` and `<board>/.agent-logs/<ID>/`, while folder-form tasks use `<status>/<ID>/.agent-state.jsonl` and `<status>/<ID>/.agent-logs/`.
- `promoteFileTaskWithAttachments` in `cli/attach.go` currently stages attachments and `TASK.md`, publishes the folder with `os.Rename(stagingDir, taskDir)`, and removes the legacy markdown file. It does not migrate pre-existing board-root agent artifacts for that task.
- Existing coverage in `TestAttachPromotesLegacyFileTask` checks metadata, attachment bytes, promotion logs in task markdown, and `BOARD.md` rendering; extend this path to cover legacy agent state/log preservation.
- Source: CLI grand review findings #1 and #2 against the TB-93 attach-promotion path.

## Constraints

- Keep legacy file-form behavior unchanged until explicit file-to-folder promotion: file tasks still use board-root `.agent-state/<ID>.jsonl` and `.agent-logs/<ID>/`.
- During promotion, copy existing root artifacts into the staging folder before publishing it, then remove the root originals only after the promoted task directory is successfully published.
- Preserve JSONL bytes, log file names, log bytes, permissions where practical, and existing attachment behavior.
- Missing root agent artifacts are valid; promotion should not fail or invent placeholder run history when one or both paths are absent.
- Stay inside CLI attach/promotion code and focused tests. Do not change daemon path resolution, GUI attachment behavior, or the TB-93 architecture docs as part of this task.

## Acceptance Criteria

- [ ] `tb attach <ID> <path>` on a legacy file-form task migrates existing `<board>/.agent-state/<ID>.jsonl` to `<status>/<ID>/.agent-state.jsonl` in the promoted task folder.
- [ ] The same promotion migrates existing `<board>/.agent-logs/<ID>/` to `<status>/<ID>/.agent-logs/` with every run-log filename and file content preserved.
- [ ] Root cleanup is success-gated: after successful promotion, the old board-root state file and log directory for that task are gone; if promotion fails before publish, the legacy markdown file and root agent artifacts remain available.
- [ ] Promotion still succeeds when either or both legacy root artifact paths are absent, without creating fake state/log history.
- [ ] `TestAttachPromotesLegacyFileTask` or a focused companion test seeds board-root state JSONL plus at least one run log for a file-form task, runs `attachTask`, and asserts promoted task metadata, attachment bytes, board rendering, task-local artifacts, and absence of root artifacts.
- [ ] `cd cli && go test ./...` passes.

## Related Tasks

- **TB-93** — parent epic for folder-backed tasks, attachment promotion, and task-local agent artifacts.
- **TB-99** — original CLI attach/promotion implementation path this bug hardens.
- **TB-102** — completed daemon-side task-local agent state/log behavior that depends on promoted folder tasks owning their artifacts.
- **TB-106** — final mixed-board smoke should no longer be able to leave promoted-task agent artifacts orphaned at board root.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited tags=epic-tb93,review-tb93,attach,promotion,agent-artifacts,testing, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success
- 2026-05-15: Edited agentstatus=success

