# TB-196: CLI: add review target and notes commands

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** cli
**Tags:** epic-tb194,code-review,cli,metadata
**Branch:** —
**Parent:** TB-194

## Goal

Add managed CLI support for review metadata sections so implementers and reviewers can record where the implementation lives and what reviewers should know.

## Context

- Parent epic: TB-194.
- Existing narrow body-edit support is limited to `tb edit --goal` and `--acceptance` in `cli/edit.go`; review metadata needs the same lock/atomic/regenerate guarantees without hand-editing markdown.
- The desired task sections are `## Review Target`, `## Reviewer Notes`, and `## Review Findings`.
- TB-195's submit command should warn when `## Review Target` is absent or blank.

## Constraints / Non-goals

- Use managed `tb` commands only; no direct markdown writes.
- Proposed CLI surface: `tb review --target <ID> -`, `tb review --notes <ID> -`, and `tb review --findings <ID> -` to replace the corresponding section from stdin. Equivalent file-path input is acceptable if it follows existing `tb edit --goal file|-` semantics.
- `## Review Target` should accept free-form text and common implementation refs: branch, PR URL, commit SHA, worktree path, or a short note.
- `## Reviewer Notes` is implementer-facing guidance for reviewers.
- `## Review Findings` is reviewer/user-facing output and may be replaced by this task; append/history behavior can be deferred unless implemented cleanly.
- Do not move tasks or set `review-failed` here; TB-195 and TB-199 own movement/failure flow.

## Related Tasks

- **TB-194** - Parent epic.
- **TB-195** - Submit command validates/warns on missing review target.
- **TB-198** - Review agents write findings through this surface.
- **TB-199** - Failed reviews use findings plus the `review-failed` marker.

## Acceptance Criteria

- [ ] `tb review --target <ID> -` creates or replaces `## Review Target` from stdin and regenerates `BOARD.md`.
- [ ] `tb review --notes <ID> -` creates or replaces `## Reviewer Notes` from stdin and regenerates `BOARD.md`.
- [ ] `tb review --findings <ID> -` creates or replaces `## Review Findings` from stdin and regenerates `BOARD.md`.
- [ ] The commands reject empty stdin after trimming with a clear error and leave the task unchanged.
- [ ] `tb show <ID>` and `tb show <ID> --json` expose the sections in the body/detail output without corrupting metadata parsing.
- [ ] Section parsing ignores fenced code and quoted markdown headings consistently with existing task-section helpers.
- [ ] CLI tests cover create, replace, empty input, folder-form tasks, legacy file-form tasks, lock/regenerate behavior, and interaction with TB-195's missing-target warning helper.
- [ ] Verification includes `cd cli && go test ./...`.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-19: Done

