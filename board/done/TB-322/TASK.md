# TB-322: GUI: window close must not kill agent runs

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** gui,daemon,agent,lifecycle,shutdown,manual-qa
**GroomedBy:** codex
**GroomStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** working-tree
**AgentStatus:** success
**ReviewedBy:** codex
**ReviewStatus:** success
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

working-tree: /Users/ralist/projects/task-board-tools
scope: TB-322 review-fix rework for GUI last-window-close lifecycle
files:
- gui/main.go
- gui/main_test.go
summary:
- Wired tray-capable last-window-close policy into macOS, Windows, and Linux Wails app options.
- Added a Common.WindowClosing hook so normal tray-capable window chrome close cancels Wails' default destroy path and hides the main window instead.
- Preserved explicit app shutdown/Quit by letting the close event proceed once the app context is cancelled.
- Added table coverage for tray/no-tray policy and close-vs-quit hook behavior.
verification:
- cd gui && go test . ./app -run 'TestWindowClosePolicy|TestWindowCloseTerminationPolicy|TestHandleTrayWindowClosing|TestDaemonShutdown_ConcurrentClose|TestRecoverStale_ShutdownCancelled'
- cd gui && go test ./internal/daemon ./app
- cd gui && go test ./... (fails in untouched tools/tb-gui/internal/agent: TestPromptGroom_NonEmptyAndUsesOnlySupportedPlaceholders; captured as TB-338)
- git diff --check -- gui/main.go gui/main_test.go
notes:
- No Svelte/frontend checks run; this change is native shell/Wails lifecycle wiring only.
- Manual desktop smoke is still required for native tray/window behavior.

## Reviewer Notes

Reworked both prior blocking findings:
- Linux/Windows now receive DisableQuitOnLastWindowClosed from the same tray policy as macOS.
- Tray-capable user window close now cancels the default Wails destroy event and hides the window; explicit Quit/app shutdown still proceeds because app context cancellation disables the hide hook.

Verification caveat: full `cd gui && go test ./...` fails in untouched `internal/agent` PromptGroom placeholder coverage. Captured as TB-338 so TB-322 review can distinguish that existing suite failure from this lifecycle fix.

## Review Findings

- Accepted after manual reconciliation: TB-322 was moved from stale `code-review` to `done` with `tb review --pass`.
- Stuck-state evidence is attached as `TB-322-review-state-evidence.md`; it shows the resumed run resubmitted TB-322 to `code-review` and never ran managed pass/fail, while runner metadata later recorded `finished{mode: resume, status: success}`.
- Root prevention follow-up is TB-340.
- Existing verification caveat remains captured in the review target: full `cd gui && go test ./...` is blocked by unrelated TB-338; scoped TB-322 checks passed.

## Attachments

- TB-322-review-state-evidence.md

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
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited agentstatus=none, reviewed-by=codex, review-status=success
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited review-target
- 2026-05-21: Edited reviewer-notes
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewref=working-tree
- 2026-05-21: Submitted to code-review
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited tags=gui,daemon,agent,lifecycle,shutdown,manual-qa, agentstatus=success, reviewed-by=none, review-status=none
- 2026-05-21: Edited review-findings
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited agentstatus=success, reviewed-by=codex, review-status=success
- 2026-05-21: Passed code review
- 2026-05-21: Attached TB-322-review-state-evidence.md
- 2026-05-21: Edited review-findings

