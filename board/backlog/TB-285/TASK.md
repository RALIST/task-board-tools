# TB-285: CLI: tb scan --apply creates folder-form tasks

**Type:** bug
**Priority:** P0
**Size:** S
**Agent:** codex
**AgentStatus:** interrupted
**Module:** cli
**Tags:** scan,folder-form,quick-win
**Branch:** —

## Goal

Make `tb scan --apply` create each generated backlog task in folder form (`<board>/backlog/<ID>/TASK.md`) instead of the legacy `<board>/backlog/<ID>.md` file form, while preserving scan's source-comment tagging and board-regeneration behavior.

## Context

- `cli/scan.go` currently allocates an ID, builds scan task markdown, then writes `board/backlog/<ID>.md` through `writeFileAtomic`.
- `cli/create.go` and TB-97 made normal task creation default to `<status>/<ID>/TASK.md` with an empty `## Attachments` section; `--legacy-file` is the explicit opt-in for file form.
- `docs/ARCHITECTURE.md` -> "Folder-form tasks" defines folder tasks as first-class, `TASK.md` as the canonical markdown file, and `<ID>.md` as legacy / explicit opt-in.
- `tb scan` tasks intentionally remain ungroomed so `tb triage` can find them for follow-up refinement.

## Constraints

- CLI-only bug fix; do not change GUI behavior or migrate existing file-form tasks.
- Preserve scan semantics unrelated to the storage path: marker detection, dry-run output, ID allocation, source comment replacement, `.board.lock`, and `BOARD.md` regeneration.
- New scan-created task markdown should use the folder-form task skeleton, including an empty `## Attachments` section, but still retain the log text with `Created by` and the `tb scan` command span used by triage.
- Do not add a new legacy-file mode to `tb scan` unless a separate product decision asks for it.

## Context

- `cli/scan.go` currently allocates an ID, builds scan task markdown, then writes `board/backlog/<ID>.md` through `writeFileAtomic`.
- `cli/create.go` and TB-97 made normal task creation default to `<status>/<ID>/TASK.md` with an empty `## Attachments` section; `--legacy-file` is the explicit opt-in for file form.
- `docs/ARCHITECTURE.md` -> "Folder-form tasks" defines folder tasks as first-class, `TASK.md` as the canonical markdown file, and `<ID>.md` as legacy / explicit opt-in.
- `tb scan` tasks intentionally remain ungroomed so `tb triage` can find them for follow-up refinement.

## Constraints

- CLI-only bug fix; do not change GUI behavior or migrate existing file-form tasks.
- Preserve scan semantics unrelated to the storage path: marker detection, dry-run output, ID allocation, source comment replacement, `.board.lock`, and `BOARD.md` regeneration.
- New scan-created task markdown should use the folder-form task skeleton, including an empty `## Attachments` section, but still retain the `Created by `tb scan`` log text used by triage.
- Do not add a new legacy-file mode to `tb scan` unless a separate product decision asks for it.

## Acceptance Criteria

- [ ] `tb scan --apply` creates each new task at `board/backlog/<ID>/TASK.md` and does not create a sibling `board/backlog/<ID>.md`.
- [ ] Scan-created task markdown keeps expected metadata (`Type`, `Priority: P2`, `Size: S`, inferred `Module` when available), has `## Goal`, placeholder `## Acceptance Criteria`, empty `## Attachments`, and a `## Log` entry containing `Created by `tb scan``.
- [ ] The apply path still updates matching source comments with allocated task IDs, advances `.next-id`, holds the board lock while mutating board files, writes task markdown with `writeFileAtomic`, and regenerates `BOARD.md`.
- [ ] Dry-run mode remains read-only: no task folder or legacy task file is created, no source comment is rewritten, and the existing summary output still reports the would-create hits.
- [ ] CLI tests cover the apply path on a temp board/source file and assert folder path, no legacy file, source comment rewrite, generated board visibility, and continued `tb triage` detection of scan-created tasks.
- [ ] `cd cli && go test ./...` passes.
- [ ] `cd cli && go build -o tb .` has been run so the local untracked `tb` binary matches the CLI change.

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

