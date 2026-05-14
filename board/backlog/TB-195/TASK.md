# TB-195: CLI: add code-review status and submit flow

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** cli
**Tags:** epic-tb194,code-review,cli,status
**Branch:** —
**Parent:** TB-194

## Goal

Add `code-review` as a first-class active board status and provide a submit shortcut for implementation work that is ready to review.

## Context

- Parent epic: TB-194.
- CLI status taxonomy currently lives in `cli/board.go` (`statusDirs`, `allStatusDirs`, `resolveStatusFilter`) and movement lives in `cli/move.go`.
- `tb init`, `tb ls --status active|all`, `tb mv`, `tb start`, `tb done`, `tb board`, JSON output, and `BOARD.md` regeneration all depend on the status directory list.
- GUI support is a sibling task; this task owns the CLI/data-plane contract only.

## Constraints / Non-goals

- Preserve the directory-is-status invariant and all `.board.lock` / atomic-write behavior.
- `code-review` is an active status: active = backlog + in-progress + code-review + done; all = active + archive.
- Support status aliases `cr` and `review` for filtering and moving.
- Add a quick submit command, proposed surface: `tb review --submit <ID>`, that moves an in-progress task to `code-review`.
- If no review target is present yet, submit prints a warning to stderr and still moves the task for this MVP.
- Do not add reviewer notes/findings editing in this task; TB-196 owns review sections and metadata commands.

## Related Tasks

- **TB-194** - Parent epic.
- **TB-196** - Provides the review target section that submit warns about when missing.
- **TB-197** - Consumes the new `code-review` status in the GUI.
- **TB-200** - Documents the workflow and command names.

## Acceptance Criteria

- [ ] `tb init` creates `board/code-review/` for new boards without disrupting existing boards.
- [ ] `tb mv <ID> code-review`, `tb mv <ID> cr`, and `tb mv <ID> review` move a task into the `code-review` directory under the board lock and regenerate `BOARD.md`.
- [ ] `tb ls --status active --json` and `tb board --json` include `code-review` tasks; `tb ls --status code-review|cr|review` filters only that status.
- [ ] `tb review --submit <ID>` moves only an `in-progress` task to `code-review`; tasks in backlog, done, archive, or missing directories get a clear validation error.
- [ ] When `tb review --submit <ID>` runs without a review target section from TB-196, stderr contains a warning and the command still exits successfully after moving the task.
- [ ] `tb done <ID>` works from `code-review` so a passed review can use the existing completion path.
- [ ] Table-driven CLI tests cover status filtering, aliases, submit warning, invalid-source validation, JSON status output, and `BOARD.md` regeneration.
- [ ] Verification includes `cd cli && go test ./...`.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

