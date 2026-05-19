# TB-235: Require ReviewRef metadata before code-review moves

**Type:** improvement
**Priority:** P1
**Size:** M
**Agent:** claude
**AgentStatus:** success
**Module:** workflow
**Tags:** code-review,metadata,validation,ux
**ReviewRef:** main@b3795ea
**Branch:** —

## Goal

Require every task entering `code-review` to carry a machine-readable `ReviewRef` metadata value that points reviewers at the branch, PR, commit, worktree, or other concrete ref they should inspect.

## Context

- Follow-up to TB-194/TB-195/TB-197: `code-review` is now a first-class status, and `tb review --submit` currently only warns when `## Review Target` is missing.
- Existing task metadata includes `Branch`, but the review workflow needs a required review target that can be a branch, PR URL, commit SHA, worktree path, or short ref.
- Relevant implementation surfaces: `cli/task.go`, `cli/edit.go`, `cli/move.go`, `cli/review.go`, `cli/json_output.go`, `gui/app/board_service.go`, `gui/internal/cli/mutations.go`, `gui/frontend/src/lib/api.ts`, and `gui/frontend/src/lib/components/TaskDrawer.svelte`.

**Constraints / non-goals**

- Preserve board invariants: `.board.lock` for structured mutations, atomic task writes, and regenerated `BOARD.md`.
- Validate only moves into `code-review`; do not require `ReviewRef` for backlog, in-progress, done, archive, or unrelated metadata edits.
- Keep `## Review Target` / `## Reviewer Notes` / `## Review Findings` as prose review sections; this task adds the minimal header metadata needed for deterministic gating.
- A non-empty `ReviewRef` is enough for this task; do not validate remote existence, git reachability, PR status, or whether the commit is pushed.
- Surface CLI validation errors through the existing GUI move/submit flows instead of adding a separate review workflow.

## Acceptance Criteria

- [ ] Top-level `**ReviewRef:** <value>` metadata is parsed into the CLI `Task` model, emitted as `reviewRef` by `tb show --json`, `tb ls --json`, and `tb board --json`, and is editable/clearable through `tb edit --review-ref <value|none>` without disturbing other metadata or body sections.
- [ ] `tb review --submit <ID>` rejects in-progress and review-failed backlog tasks that have no non-placeholder `ReviewRef`, prints an actionable error naming `tb edit <ID> --review-ref ...`, and leaves the task in its source status with tags, log history, and review sections unchanged.
- [ ] `tb mv <ID> code-review|cr|review` applies the same validation for any source status; moving out of `code-review` or among other statuses keeps existing behavior.
- [ ] Tasks with a valid `ReviewRef` can enter `code-review` through both `tb review --submit` and `tb mv`, and `BOARD.md` plus JSON/status output still reflect the move.
- [ ] GUI board/detail models expose `reviewRef`; TaskDrawer displays and edits it with the existing metadata autosave path, and Submit for review plus drag/drop to Code Review show the CLI validation error when it is missing.
- [ ] Existing `## Review Target` prose sections remain supported and are not treated as a substitute for missing `ReviewRef`.
- [ ] Tests cover CLI parsing/JSON/edit, submit and move failure/no-mutation paths, successful code-review moves with `ReviewRef`, GUI backend edit/model propagation, and frontend missing-ref error/display behavior.
- [ ] Verification includes `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.
- [ ] Manual test note: in the desktop GUI, try Submit for review and drag/drop to Code Review with `ReviewRef` empty (expect validation toast/no move), then set `ReviewRef` to a branch or commit and confirm both paths move the task to Code Review.

## Review Target

branch: main
ReviewRef metadata: main@b3795ea

Surface area to verify:
- cli/task.go, cli/edit.go, cli/move.go, cli/review.go, cli/json_output.go, cli/create.go (flagsWithValue), cli/templates.go, cli/main.go (help)
- gui/internal/cli/mutations.go, gui/app/board_service.go
- gui/frontend/bindings/** (regenerated)
- gui/frontend/src/lib/components/TaskDrawer.svelte (+ tests across .test.ts files)

Tests added:
- cli/review_ref_test.go (parse, JSON, edit set/clear, mv-to-code-review with alias coverage + noop + success + non-code-review unaffected, reviewSubmit happy/sad + review-failed preservation)
- gui/internal/cli/mutations_test.go (Edit --review-ref forwarding, Edit "none" clear, Move missing-ReviewRef → ErrKindValidation)
- gui/app/board_service_test.go (EditTask forwarding, LoadBoard propagation)
- gui/frontend/src/lib/components/TaskDrawer.test.ts (ReviewRef render/autosave/clear; submitReview error toast surfaces --review-ref hint)

Verification (all green):
- cd cli && go test ./...
- cd gui && go test ./...
- cd gui/frontend && npm run check
- cd gui/frontend && npm test -- --run

Manual smoke (in a scratch board): mv TST-1 to code-review without ReviewRef → blocked with actionable error. After `tb edit TST-1 --review-ref feat/x` → move succeeds.

## Related Tasks

- **TB-194** — Parent code-review workflow; this follow-up tightens the original warning-only submit behavior.
- **TB-195** — CLI status/submit flow that needs the hard gate.
- **TB-197** — GUI column/drawer surfaces that need to display/edit the new metadata and pass through validation.
- **TB-234** — Sibling daemon guard for tasks already in code-review; separate from this metadata gate.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited size=M, module=workflow, tags=code-review,metadata,validation,ux, title=Require ReviewRef metadata before code-review moves, goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited reviewref=main@b3795ea
- 2026-05-19: Edited review-target
- 2026-05-19: Edited agentstatus=success

