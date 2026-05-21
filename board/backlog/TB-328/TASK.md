# TB-328: Capture backend GUI panics on board

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** backend,error-capture,crash,wails
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —
**Parent:** TB-324

## Goal

Capture recovered backend GUI panics at the selected Wails service or daemon boundaries and send them to the shared crash-capture sink without hiding the original failure.

## Context

- Parent epic: TB-324. Prerequisite/shared infrastructure: TB-326.
- Backend GUI code lives in `gui/app/` exported Wails services (`BoardService`, `AgentService`, `SettingsService`, and related controllers) plus daemon/agent goroutines.
- Wails binding failures currently reach the frontend as rejected calls; TB-286 makes normal user-facing errors readable, but a backend panic still needs capture evidence for developers.
- Agent stale-run crash recovery is already a separate domain; do not rewrite M5/TB-130 recovery semantics while adding crash reporting.

## Constraints

- Prefer the narrowest central recover boundary available for Wails service calls; if Wails cannot provide a single wrapper, add explicit wrappers only around the exported service/goroutine paths chosen in the task log.
- Capture panics and crash-like backend failures, not all expected validation errors or ordinary returned CLI errors.
- Recovered panics must still return a clear error to the caller and keep existing toast/error behavior intact.
- Capture must use the TB-326 sink and must not perform direct markdown writes.
- Do not change agent run stale-recovery status semantics (`interrupted`, `lost`, `cancelled`) except to add crash-capture breadcrumbs if a chosen boundary observes a real panic.

## Acceptance Criteria

- [ ] The implementation identifies and records the backend recover boundary used for Wails service calls or daemon goroutines; any intentionally uncovered backend paths are listed with rationale before review.
- [ ] A recovered panic produces a TB-326 crash report with service/method or goroutine name, panic value, stack trace, board root, task id/run id when available, and timestamp.
- [ ] The original caller still receives a clear error after recovery; frontend error/toast behavior remains compatible with TB-286 formatting.
- [ ] Normal handled errors such as validation failures, missing board, missing task, and expected CLI non-zero exits are not auto-captured as crashes.
- [ ] Capture-sink failure does not panic, recurse, or mask the original recovered panic error.
- [ ] Backend tests inject a panic through the chosen boundary and assert that one sanitized crash task request is made; tests also cover sink failure and a normal returned error.
- [ ] Verification passes: `cd gui && go test ./app/... ./internal/...`.
- [ ] Manual test note: run `cd gui && task dev`, trigger a backend capture test hook or panic fixture, confirm the GUI remains open, caller sees a readable error, and one backlog crash task is created.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited groomed-by=codex, groom-status=success

