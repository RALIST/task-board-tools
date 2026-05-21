# TB-272: CLI: add managed review pass flow

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** workflow
**Tags:** code-review,auto-review,automation,cli
**Branch:** —
**Parent:** TB-262

## Goal

Add a managed review pass command so review agents can atomically record no-blocking-findings and move code-review tasks to done without contradictory prompt guidance.

## Context

Implemented `tb review --pass <ID> file|-` with managed Review Findings write, code-review-only guard, pass log, done move, and GUI wrapper/service exposure.

Verification:
- `cd cli && go test ./...`
- `cd cli && go build -o tb .`
- `cd gui && go test ./...`
- code review pass: No CRITICAL issues found. No MAJOR issues found.

## Acceptance Criteria

- [x] Add a managed CLI surface such as `tb review --pass <ID> file|-` that accepts review findings / a no-blocking-findings note from stdin or a file.
- [x] The pass command accepts only tasks currently in `code-review`; other statuses are rejected without mutation.
- [x] The pass command writes/replaces `## Review Findings`, appends an explicit pass log entry, moves the task to `done`, and regenerates `BOARD.md` under the board lock.
- [x] Empty pass findings are rejected, matching `tb review --fail` empty-input behavior.
- [x] Existing `tb review --findings` and `tb done` flows keep working for manual users.
- [x] Review prompt guidance can collapse to one clear choice: `tb review --pass` for no blocking findings, `tb review --fail` for blocking findings.
- [x] CLI tests cover pass happy path, non-code-review rejection, empty findings rejection, folder-form task movement, and no regression to `tb review --fail`.
- [x] Verification includes `cd cli && go test ./...`.

## Related Tasks

- **TB-262** — Parent auto-review epic.
- **TB-270** — Prompt cleanup should adopt the managed pass command.
- **TB-266** — Daemon reconciliation can use the explicit pass/fail command outcomes instead of prose.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited context
- 2026-05-21: Done

