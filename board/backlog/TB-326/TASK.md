# TB-326: Crash capture sink creates sanitized board tasks

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** error-capture,crash,epic-child
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —
**Parent:** TB-324

## Goal

Build a shared GUI crash-capture sink that accepts a normalized crash report and creates a sanitized backlog bug task through the existing `tb create` path.

## Context

- Parent epic: TB-324.
- `gui/app/board_service.go` exposes `BoardService.CreateTask`, which delegates to `gui/internal/cli` and ultimately `tb create`.
- Crash capture needs a single backend-owned formatting and safety path so frontend and backend callers do not each invent their own task body, redaction, dedupe, or recursion behavior.
- Existing docs require all structured board mutations to go through managed CLI paths so `.board.lock`, folder-form tasks, atomic writes, and generated board output stay consistent.
- Downstream callers: TB-327 will call this sink from frontend runtime handlers; TB-328 will call it from backend recover boundaries.

## Constraints

- Keep sink local to the selected board; no external telemetry, no network calls, no new database.
- Use existing CLI mutation plumbing rather than direct markdown writes.
- Capture reports must be sanitized before writing: redact obvious secrets/tokens, omit raw environment variables, and cap stack/breadcrumb length.
- Add a recursion guard so sink failures do not create more crash reports or mask the original failure.
- Add lightweight fingerprint dedupe or rate limiting for identical crashes in one app session so one bug cannot flood the backlog.
- Do not wire frontend or backend crash handlers in this task except minimal test doubles needed to prove the sink contract.

## Acceptance Criteria

- [ ] A typed crash report shape exists with source (`frontend` or `backend`), message, stack, module hint, board/task/run breadcrumbs, timestamp, and optional compact breadcrumbs.
- [ ] The sink creates backlog tasks via the existing `tb create` / GUI CLI client path with type `bug`, priority `P1` unless explicitly downgraded, module from the report or `gui`, and tags including `crash`, `captured`, and `error-capture`.
- [ ] Generated task bodies include Goal, Context, Constraints or non-goals, and Acceptance Criteria so captured tasks do not land as empty triage placeholders.
- [ ] Redaction and length-cap tests prove secrets/tokens and oversized stacks are not written verbatim.
- [ ] Dedupe/rate-limit tests prove repeated identical reports in one app session create at most one task or return the existing captured result.
- [ ] Create-failure tests prove sink failure is logged/reported without recursive capture and without swallowing the caller's original error path.
- [ ] Verification passes: `cd gui && go test ./app/... ./internal/...`.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited groomed-by=codex, groom-status=success

