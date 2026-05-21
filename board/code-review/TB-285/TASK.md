# TB-285: CLI: tb scan --apply creates folder-form tasks

**Type:** bug
**Priority:** P0
**Size:** S
**Agent:** codex
**Module:** cli
**Tags:** scan,folder-form,quick-win
**GroomedBy:** codex
**GroomStatus:** success
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** 9ee67bb
**Branch:** —

## Goal

Make `tb scan --apply` create each generated backlog task in folder form (`<board>/backlog/<ID>/TASK.md`) instead of the legacy `<board>/backlog/<ID>.md` file form, while preserving scan's source-comment tagging and board-regeneration behavior.

## Context

- `cli/scan.go` currently allocates an ID, builds scan task markdown, and writes scan-created backlog tasks through the legacy file-form path.
- `cli/create.go` and TB-97 made normal task creation default to `<status>/<ID>/TASK.md` with an empty `## Attachments` section; `--legacy-file` remains the explicit opt-in for file form.
- `docs/ARCHITECTURE.md` -> "Folder-form tasks" defines folder tasks as first-class, `TASK.md` as the canonical markdown file, and `<ID>.md` as legacy / explicit opt-in.
- `tb scan` tasks intentionally remain ungroomed so `tb triage` can find generated follow-up tasks after source-marker conversion.

## Constraints

- CLI-only bug fix; do not change GUI behavior or migrate existing file-form tasks.
- Preserve scan semantics unrelated to the storage path: marker detection, dry-run output, ID allocation, source comment replacement, `.board.lock`, and `BOARD.md` regeneration.
- New scan-created task markdown should use the folder-form task skeleton, including an empty `## Attachments` section, while preserving the existing triage-recognized scan log marker.
- Do not add a new legacy-file mode to `tb scan` unless a separate product decision asks for it.

## Acceptance Criteria

- [ ] `tb scan --apply` creates each new task at `board/backlog/<ID>/TASK.md` and does not create sibling `board/backlog/<ID>.md` files.
- [ ] Scan-created task markdown keeps expected metadata (`Type`, `Priority: P2`, `Size: S`, inferred `Module` when available), has `## Goal`, placeholder `## Acceptance Criteria`, empty `## Attachments`, and a `## Log` entry using the existing scan-created marker consumed by triage.
- [ ] Apply path still updates matching source comments with allocated task IDs, advances `.next-id`, holds the board lock while mutating board files, writes task markdown with `writeFileAtomic`, and regenerates `BOARD.md`.
- [ ] Dry-run mode remains read-only: no task folder or legacy task file is created, no source comment is rewritten, and existing summary output still reports would-create hits.
- [ ] CLI tests cover apply path on a temp board/source file and assert folder path, no legacy file, source comment rewrite, generated board visibility, and continued `tb triage` detection of generated scan tasks.
- [ ] `cd cli && go test ./...` passes.
- [ ] `cd cli && go build -o tb .` has been run so local untracked `tb` binary matches CLI change.

## User Attention

**Reason** — resolved prior managed-edit blocker.

**Specific question/action** — No current user action needed.

**Attempted context** — Re-read `tb show TB-285`, `board/CONVENTIONS.md`, `board/SKILL.md`, and `tb edit --help`. Current `tb edit` supports managed `--context` and `--constraints` rewrites, so duplicate section cleanup can stay inside board tooling.

**Unblock condition** — Grooming resumed and task sections were cleaned with managed edits.

## Review Target

branch: main
commit: 9ee67bb
summary: tb scan --apply now writes generated backlog tasks as folder-form TASK.md files and includes the empty Attachments section while preserving scan triage marker/source rewrite behavior.
verification:
- cd cli && go test ./... -count=1
- cd cli && go build -o tb .
- subagent code review: no findings, Ready to merge: Yes

## Related Tasks

- **TB-93** — parent folder-form task milestone and storage contract.
- **TB-96** — read parity for folder-form tasks; scan output must remain readable by the shared discovery path.
- **TB-97** — normal `tb create` folder-form default that `tb scan --apply` should match.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited priority=P0, type=bug, size=S, module=cli, tags=scan,folder-form,quick-win, title=CLI: tb scan --apply creates folder-form tasks
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited goal
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited user-attention
- 2026-05-20: Edited agentstatus=needs-user
- 2026-05-21: Edited agentstatus=none
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=lost, groomed-by=codex, groom-status=lost
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=failed, groomed-by=codex, groom-status=failed
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited user-attention
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited review-target
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewref=9ee67bb
- 2026-05-21: Submitted to code-review

