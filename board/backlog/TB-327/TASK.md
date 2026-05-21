# TB-327: Capture frontend runtime crashes on board

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui-frontend
**Tags:** frontend,error-capture,crash,ui
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —
**Parent:** TB-324

## Goal

Capture frontend unhandled runtime errors and unhandled promise rejections and send them to the shared crash-capture sink as reproducible backlog bug tasks.

## Context

- Parent epic: TB-324. Prerequisite/shared infrastructure: TB-326.
- Frontend entry point is `gui/frontend/src/routes/+page.svelte`; Wails wrappers live in `gui/frontend/src/lib/api.ts`; stores live under `gui/frontend/src/lib/stores/`.
- User-visible failure feedback already goes through `Toast.svelte` and TB-286 `errorString()` formatting; crash capture should reuse that pattern for capture failures rather than dumping raw objects.
- Current concrete sibling crash: TB-323 covers the kanban drag-start crash itself. This task is only the frontend capture infrastructure.

## Constraints

- Capture only unhandled frontend runtime failures (`error` and `unhandledrejection`); do not turn every handled rejected API call or validation toast into a task.
- Do not add a second task-creation path in the frontend. Call the backend sink from TB-326 through a typed API wrapper.
- Handler setup must be idempotent and test-safe so hot reloads, remounts, or repeated store initialization do not register duplicate listeners.
- If capture fails, surface a concise toast/log and leave the original browser error visible for debugging.
- Do not include raw local file paths, credentials, full store dumps, or unrestricted board/task bodies in the report.

## Acceptance Criteria

- [ ] Frontend registers idempotent `window` handlers for `error` and `unhandledrejection` during app startup and removes or avoids duplicates in tests/HMR.
- [ ] ErrorEvent, PromiseRejectionEvent, thrown `Error`, thrown string/object, and missing-stack cases normalize into the TB-326 crash report shape.
- [ ] Reports include useful breadcrumbs available in the frontend, such as current board root/status if exposed, selected task id, route, user action context when available, and browser timestamp, within the sink's redaction/length limits.
- [ ] A successful capture calls the shared backend sink exactly once for a unique unhandled crash.
- [ ] Capture failure does not recurse; it produces concise existing-style feedback and leaves the app as usable as the original crash allows.
- [ ] Frontend tests cover unhandled error, unhandled rejection, non-Error rejection, and duplicate-listener prevention.
- [ ] Verification passes: `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run`, and `cd gui/frontend && npm run lint`.
- [ ] Manual test note: run `cd gui && task dev`, trigger a frontend test crash from the dev console or a temporary test hook, verify one backlog task is created with stack/breadcrumbs and no duplicate task appears on repeated identical trigger within the dedupe window.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited groomed-by=codex, groom-status=success

