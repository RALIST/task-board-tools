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

- Review failure already has `tb review --fail <ID> -`, which writes findings and moves `code-review` -> `ready` with `review-failed`.
- Review pass currently relies on a two-step prompt contract: write "no blocking findings" with `tb review --findings`, then run `tb done`. That is easy for agents to miss and hard for the daemon to reconcile deterministically.
- Auto-review needs one symmetric command pair: pass -> done, fail -> ready.

## Constraints / Non-goals

- Preserve existing `tb done` for humans and non-review workflows.
- Do not infer pass/fail from free-form findings text.
- The pass flow must reject non-code-review tasks.
- The command must use existing board locking, atomic writes, and `BOARD.md` regeneration patterns.
- Keep `review-failed` clearing behavior on resubmit unchanged.

## Acceptance Criteria

- [ ] Add a managed CLI surface such as `tb review --pass <ID> file|-` that accepts review findings / a no-blocking-findings note from stdin or a file.
- [ ] The pass command accepts only tasks currently in `code-review`; other statuses are rejected without mutation.
- [ ] The pass command writes/replaces `## Review Findings`, appends an explicit pass log entry, moves the task to `done`, and regenerates `BOARD.md` under the board lock.
- [ ] Empty pass findings are rejected, matching `tb review --fail` empty-input behavior.
- [ ] Existing `tb review --findings` and `tb done` flows keep working for manual users.
- [ ] Review prompt guidance can collapse to one clear choice: `tb review --pass` for no blocking findings, `tb review --fail` for blocking findings.
- [ ] CLI tests cover pass happy path, non-code-review rejection, empty findings rejection, folder-form task movement, and no regression to `tb review --fail`.
- [ ] Verification includes `cd cli && go test ./...`.

## Related Tasks

- **TB-262** — Parent auto-review epic.
- **TB-270** — Prompt cleanup should adopt the managed pass command.
- **TB-266** — Daemon reconciliation can use the explicit pass/fail command outcomes instead of prose.

## Attachments

## Log

- 2026-05-19: Created
