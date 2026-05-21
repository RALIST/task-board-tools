# TB-322: GUI: window close must not kill agent runs

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** gui,daemon,agent,lifecycle,shutdown,manual-qa
**GroomedBy:** codex
**GroomStatus:** success
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** 6a7ec39
**Branch:** —

## Goal

Ensure desktop GUI lifecycle matches the product contract: closing the main window on tray-capable platforms keeps active agent runs alive, while explicit Quit/app shutdown cancels active runs once with durable `finished{status: cancelled, reason: "shutdown"}` JSONL and matching task metadata.

## Context

- `docs/ARCHITECTURE.md` → Native shell: tray-capable platforms set `ApplicationShouldTerminateAfterLastWindowClosed=false`; closing the main window must not kill daemon work. Quit from menu/tray still exits the app and runs daemon shutdown.
- `docs/FEATURES.md` → F5.4: app shutdown cancels daemon root context, writes `finished{status: cancelled, reason: "shutdown"}`, updates task metadata, waits up to 5s, then leaves leftovers for recovery.
- `gui/main.go`: Wails app config sets `ApplicationShouldTerminateAfterLastWindowClosed: !shell.TraySupported()` and defers `d.Close()` during real app shutdown.
- `gui/internal/shell/controller.go`: menu/tray expose Quit; tray support exists on macOS, Linux, and Windows.
- `gui/internal/daemon/daemon.go`: `Close()` and shutdown grace own daemon cancellation; `Deactivate()`/board-switch behavior is separate.
- `gui/app/agent_run.go` and `gui/app/agent_finish.go`: `RunQueuedAgentSync`, active-run cancellation, process signalling, JSONL terminal writes, and task metadata updates are the core seams.
- Related tasks: TB-302/TB-311 cover board-switch cancellation smoke; TB-102 covers task-local agent artifacts and stale recovery; TB-308 may later move status fields but must preserve this lifecycle behavior.

## Constraints

- Distinguish main-window close/hide from explicit Quit/app shutdown. Do not fix this by disabling tray behavior or cancelling runs when the window merely closes.
- Preserve CLI/task markdown format, JSONL event schema, Wails event names, and file-form/folder-form agent artifact paths.
- Preserve existing process-group termination semantics, 5s shutdown grace, and the `cancelled` / `interrupted` / `lost` recovery contract.
- Keep scope to GUI lifecycle for active agent runs. Do not change board-switch behavior beyond avoiding regressions in TB-302/TB-311 coverage.
- If Wails window-close behavior cannot be fully unit-tested, capture the untestable boundary explicitly and require a real desktop manual smoke.

## Acceptance Criteria

- [ ] Backend coverage proves real app shutdown/daemon `Close()` cancels an active agent run exactly once, records `finished{status: cancelled, reason: "shutdown"}`, updates the task's agent status to `cancelled`, and does not overwrite that terminal state during recovery.
- [ ] Coverage or a documented seam check proves closing the main window on tray-capable platforms does not invoke daemon shutdown, does not call the cancel path, and leaves active agent runs/log streaming alive.
- [ ] Explicit Quit from the app menu or tray follows the shutdown path: active run receives cancellation, process group termination keeps existing grace/escalation behavior, JSONL is written before task metadata, and repeated Quit/shutdown races stay idempotent.
- [ ] Manual test note: in the desktop GUI on a scratch board, start a deliberately slow agent run, close the main window via window chrome, restore from tray/menu and confirm the run is still active or finishes naturally; then repeat and choose Quit, confirming the task shows one cancelled shutdown run and no orphaned running row.
- [ ] Existing board-switch cancellation behavior from TB-302/TB-311 still passes; window close must not be conflated with board switch cancellation reason `board switch`.
- [ ] Verification includes `cd gui && go test ./internal/daemon ./app` and `cd gui && go test ./...`; run frontend checks if visible run/tray/window state changes.

## Review Target

commit: 6a7ec39
scope: GUI lifecycle seam and backend coverage for window close vs app shutdown
files:
- gui/main.go
- gui/main_test.go
- gui/app/daemon_integration_test.go
- gui/app/agent_recovery_test.go
summary:
- Added testable Wails window-close policy seam: tray-capable platforms keep app alive after last window closes.
- Added daemon shutdown race coverage proving concurrent Close writes one finished{cancelled, reason:"shutdown"} and AgentStatus cancelled.
- Added recovery coverage proving finished{cancelled, reason:"shutdown"} is preserved and not overwritten.
- Existing board-switch tests still assert reason "board switch".
verification:
- cd gui && go test . ./app -run 'TestWindowCloseTerminationPolicy|TestDaemonShutdown_ConcurrentClose|TestRecoverStale_ShutdownCancelled' (RED failed before helper; GREEN passed after helper)
- cd gui && go test ./internal/daemon ./app
- cd gui && go test ./...
notes:
- No frontend checks run because no Svelte/run/tray UI state changed.
- Full git diff --check still has pre-existing unrelated board/backlog/TB-319/TASK.md EOF whitespace; scoped/cached diff check passed.

## Reviewer Notes

Automated review: subagent Einstein found no Critical or Important issues and no Minor items in the scoped diff.

Manual desktop smoke still required at review time because Wails native window-close behavior cannot be fully exercised by Go unit tests here:
1. On a scratch board, start a deliberately slow agent run in the desktop GUI.
2. Close the main window via window chrome on a tray-capable platform; restore from tray/menu and confirm the run remains active or finishes naturally, with no shutdown cancellation.
3. Repeat and choose explicit Quit from menu/tray; confirm exactly one shutdown-cancelled run and no orphaned running row.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited agent=codex
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited priority=P2, type=bug, size=M, module=gui, tags=gui,daemon,agent,lifecycle,shutdown,manual-qa, title=GUI: window close must not kill agent runs, goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited review-target
- 2026-05-21: Edited reviewer-notes
- 2026-05-21: Edited implemented-by=codex, implement-status=success, reviewref=6a7ec39
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited agentstatus=success
