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

- [x] Top-level `**ReviewRef:** <value>` metadata is parsed into the CLI `Task` model, emitted as `reviewRef` by `tb show --json`, `tb ls --json`, and `tb board --json`, and is editable/clearable through `tb edit --review-ref <value|none>` without disturbing other metadata or body sections.
- [x] `tb review --submit <ID>` rejects in-progress and review-failed backlog tasks that have no non-placeholder `ReviewRef`, prints an actionable error naming `tb edit <ID> --review-ref ...`, and leaves the task in its source status with tags, log history, and review sections unchanged.
- [x] `tb mv <ID> code-review|cr|review` applies the same validation for any source status; moving out of `code-review` or among other statuses keeps existing behavior.
- [x] Tasks with a valid `ReviewRef` can enter `code-review` through both `tb review --submit` and `tb mv`, and `BOARD.md` plus JSON/status output still reflect the move.
- [x] GUI board/detail models expose `reviewRef`; TaskDrawer displays and edits it with the existing metadata autosave path, and Submit for review plus drag/drop to Code Review show the CLI validation error when it is missing.
- [x] Existing `## Review Target` prose sections remain supported and are not treated as a substitute for missing `ReviewRef`.
- [x] Tests cover CLI parsing/JSON/edit, submit and move failure/no-mutation paths, successful code-review moves with `ReviewRef`, GUI backend edit/model propagation, and frontend missing-ref error/display behavior.
- [x] Verification includes `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.
- [x] Manual test note: in the desktop GUI, try Submit for review and drag/drop to Code Review with `ReviewRef` empty (expect validation toast/no move), then set `ReviewRef` to a branch or commit and confirm both paths move the task to Code Review. _(Equivalent automated coverage in `TaskDrawer.test.ts` for the toast + autosave + clear flows; CLI gate manually smoked on scratch TST board per Review Findings.)_

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

## Review Findings

- No blocking findings — the CLI/GUI gate matches every acceptance criterion. Manually verified on a scratch TST board: `tb mv` with code-review|cr|review aliases rejects identically with the actionable hint; `tb edit --review-ref feat/x` and `--review-ref none` round-trip; `tb show --json` and `tb ls --json` expose `reviewRef`; placeholder values `—`, `-`, and whitespace normalize to "". CLI tests, GUI Go tests, `npm run check`, and `npm test -- --run` all pass.
- The validation gate placement is correct: `moveTaskOnBoardWithLog` runs `ensureReviewRefForCodeReview` after `lockBoard` + source resolution and before any FS mutation (`cli/move.go:145-155`), so a rejected move leaves source mtime, content, tags, and log entries untouched. `reviewSubmit`'s outside-lock pre-flight is correctly justified by the comment (`cli/review.go:145-153`) and does not weaken the in-lock authoritative gate.
- (nit) `gui/internal/agent/prompts/implement.md:24-31` still tells autonomous agents to set `## Review Target` via `tb review --target` and then `tb review --submit`. With TB-235, that two-step now fails because the `**ReviewRef:**` metadata is also required. The CLI error message is actionable, so a resilient agent will recover via `tb edit --review-ref`, but the prompt no longer reflects the workflow. Worth a follow-up to add a `tb edit {{TASK_ID}} --review-ref <branch|PR|commit>` step before `tb review --submit` and to clarify that `## Review Target` is the human-readable section while `**ReviewRef:**` is the gating metadata.
- (nit) `cli/templates.go:48-50` documents metadata order with `**Branch:**` before `**ReviewRef:**`, but `setField` in `cli/edit.go:619-630` inserts `**ReviewRef:**` *immediately before* `**Branch:**` when missing. So a fresh task edited with `tb edit --review-ref X` ends up with `**ReviewRef:** X` then `**Branch:** —`, the opposite of the template order. Purely cosmetic — both orders parse identically — but worth aligning the template to what `setField` actually produces (or vice versa) for consistency.
- (nit) `cli/edit.go:217` checks `len(changes) == 0 && len(bodyEdits) == 0 && !titleProvided && !reviewRefProvided`. When `reviewRefProvided` is true, the matching change has already been appended to `changes`, so `!reviewRefProvided` is implied by `len(changes) == 0`. Same applies to the older `!titleProvided` guard. Defensive but redundant.
- (nit) `TaskDrawer.svelte`'s Submit-for-review button doesn't pre-check `formReviewRef` and relies on the CLI to surface the toast — by design ("CLI is the source of truth"), and the toast text passes through unchanged (verified in `TaskDrawer.test.ts:1295-1317`). A future UX polish could disable the button (or show an inline hint) when `detail.metadata.reviewRef` is empty so the user discovers the requirement without round-tripping through a failed submit. Not blocking.
- (nit) `tb ls` / `tb board` text output don't include a ReviewRef column. Only JSON output exposes the new field. That matches the AC, but operators auditing a code-review backlog from the terminal will need `--json | jq` to see which tasks have a ref set.

## Related Tasks

- **TB-194** — Parent code-review workflow; this follow-up tightens the original warning-only submit behavior.
- **TB-195** — CLI status/submit flow that needs the hard gate.
- **TB-197** — GUI column/drawer surfaces that need to display/edit the new metadata and pass through validation.
- **TB-234** — Sibling daemon guard for tasks already in code-review; separate from this metadata gate.
- **TB-238** — Follow-up: update `gui/internal/agent/prompts/implement.md` so autonomous agents set `ReviewRef` via `tb edit --review-ref` before `tb review --submit` (review nit).

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
- 2026-05-19: Submitted to code-review
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited review-findings
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Done
- 2026-05-19: Edited agentstatus=success
