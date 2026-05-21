# TB-324: Epic: Capture GUI crashes as board tasks

**Type:** feature
**Priority:** P1
**Size:** L
**Agent:** codex
**Tags:** epic,error-capture,crash,frontend,backend
**Module:** gui
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Track the crash-to-board work as an epic: turn unhandled GUI frontend crashes and recovered GUI backend panics into sanitized backlog bug tasks with enough context to reproduce and fix them.

## Subtasks

- **TB-326** (M) — Crash capture sink creates sanitized board tasks
- **TB-327** (M) — Capture frontend runtime crashes on board
- **TB-328** (M) — Capture backend GUI panics on board

## Context

- Current board data model is local markdown; GUI structured mutations delegate to the `tb` CLI through `BoardService` / `gui/internal/cli`, so crash capture should create normal backlog tasks rather than a separate database or external service.
- `BoardService.CreateTask` already wraps `tb create`; task writes must preserve `.board.lock`, folder-form task storage, atomic writes, and `BOARD.md` regeneration through existing CLI paths.
- Frontend crash surface lives under `gui/frontend/src/`: Wails binding wrappers in `lib/api.ts`, stores, `routes/+page.svelte`, and UI feedback through `Toast.svelte` / TB-286 readable error formatting.
- Backend GUI surface lives under `gui/app/` exported Wails services plus daemon/agent goroutines that already have crash and stale-run recovery rules.
- Child tasks:
  - TB-326: shared crash-capture sink, task format, redaction, dedupe, and loop protection.
  - TB-327: frontend `window.onerror` / `unhandledrejection` capture wired to the sink.
  - TB-328: backend recovered-panic capture wired to the sink.
- Related tasks: TB-323 handles one concrete kanban drag-start crash; TB-286 handles readable user-facing error toasts. Keep those scopes separate from crash capture infrastructure.

## Constraints

- Do not try to capture every expected `if err != nil` branch. Scope is unhandled frontend runtime failures and backend panics/errors that would otherwise crash the GUI or surface as opaque binding failures.
- Do not auto-fix captured tasks. Created tasks enter backlog with enough evidence for grooming, implementation, or auto-implement to decide separately.
- Do not add external telemetry, cloud services, accounts, or network reporting. Capture stays local to the selected board.
- Redact secrets and cap payload sizes before writing task markdown; never persist environment variables, API keys, access tokens, or full unrestricted process state.
- Crash capture must be loop-safe: failure to create the crash task must not trigger another crash task, block the UI, or hide the original error path.
- Created tasks should use existing board conventions: type `bug`, appropriate module, priority, tags such as `crash` / `captured`, a clear Goal, Context with stack/breadcrumbs, and verifiable acceptance criteria.

## Acceptance Criteria

- [ ] TB-326 is done: shared crash-capture sink creates sanitized backlog bug tasks and is covered by backend tests for formatting, redaction, dedupe, and create-failure behavior.
- [ ] TB-327 is done: frontend unhandled errors and unhandled promise rejections are captured through the shared sink without duplicate listeners or recursive failure loops.
- [ ] TB-328 is done: backend GUI panics at the chosen service/goroutine boundaries are recovered, reported through the shared sink, and still surface a clear error to the caller.
- [ ] Captured crash tasks include title, type, priority, module, tags, timestamp, source surface, stack trace or panic stack, current board/task breadcrumbs when available, and a concrete reproduction/fix acceptance checklist.
- [ ] Manual test note: run `cd gui && task dev`, trigger one frontend crash and one backend capture test hook, confirm exactly one backlog task per unique crash appears, app remains usable, and captured markdown contains no secrets or raw environment dump.
- [ ] Parent epic is complete only after child tasks pass their verification commands and the manual smoke evidence is recorded on the relevant child or parent task.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited agent=codex
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited type=feature, size=L, module=gui, tags=epic,error-capture,crash,frontend,backend, title=Epic: Capture GUI crashes as board tasks
- 2026-05-21: Edited goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Committed — moved to ready

