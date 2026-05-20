# TB-302: GUI board switch cancels old-board auto-runs cleanly

**Type:** bug
**Priority:** P1
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** gui,board-switch,daemon,agent,automation,lifecycle
**GroomedBy:** codex
**GroomStatus:** success
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewedBy:** codex
**ReviewStatus:** success
**ReviewRef:** local:Helmholtz-board-switch-review
**Branch:** —

## Goal

When the GUI switches from one board to another while the previous board has an automation-started auto-groom or auto-implement run in progress, the old-board run must be stopped and terminalized on the old board without leaking terminal writes, run events, promotion/review follow-up actions, active-run state, or UI state into the newly opened board.

## Context

Current board-switch lifecycle to verify and harden:

- `gui/app/settings_service.go`: `OpenBoard` validates the candidate board, then switches the watcher and `BoardService` client, then calls the activator's `Deactivate()` / `Activate(newBoardDir)` hook.
- `gui/adapters.go`: the composite board activator deactivates auto-implement, auto-groom, and daemon coordinators, then reactivates the daemon/coordinators for the new board.
- `gui/internal/daemon/daemon.go`: `Deactivate()` currently stops periodic recovery and clears `boardDir` / daemon active-set state, but it does not cancel the daemon root context or in-flight worker runs. The queue carries only task IDs, so queued old-board work can be consumed after `BoardService` points at the new board. `Close()` is the existing path that cancels workers and lets `RunQueuedAgentSync` write a terminal cancellation.
- `gui/app/agent_run.go`: `RunQueuedAgentSync` captures the old board client and board dir at run start, watches its caller context, and maps daemon shutdown cancellation to `finished{status: cancelled, reason: "shutdown"}`. Its terminal re-read / carve-outs still consult `BoardService`, so board switching while the run is alive is a risk surface.
- `gui/app/auto_groom.go` and `gui/app/auto_implement.go`: coordinator `Deactivate()` paths cancel timers/scans only. Auto-groom and auto-implement start runs through `AgentService` directly (`StartGroomWithTriageHashAs` / `RunAgentAs`) with run contexts that are not tied to coordinator deactivation.
- `gui/main.go` + `gui/app/auto_groom.go`: groom success events can trigger promotion through the current `BoardService`, so an old-board groom finish after a switch can target the wrong board when task IDs overlap.
- `docs/ARCHITECTURE.md` and `docs/FEATURES.md`: `interrupted` and `lost` are recovery-initiated stale-run states; normal shutdown currently records `cancelled`.

Related tasks: TB-53 (daemon lifecycle), TB-177/TB-179 (auto-implement), TB-208 (board-switch validation), TB-266 (daemon reconciliation preserves terminal states), TB-291 (auto-resume interrupted daemon-owned runs), TB-301 (startup delay before auto pickup), and TB-303 (future generic AgentStatus removal).

## Constraints

- Preserve the one-active-board GUI model; do not introduce background multi-board execution as part of this task.
- Do not write `interrupted` directly during a successful board switch. A controlled switch is lifecycle/user intent and should use the existing cancellation terminal path with a distinct reason such as `board switch`; `RecoverStale` remains responsible for `interrupted` or `lost` if the app dies before cancellation flushes.
- Terminal writes for old-board runs must use the old board's captured CLI client/board dir, not the newly active board service.
- Board-scoped follow-up actions, especially auto-groom promotion after a groom run succeeds, must either use the originating board context or be suppressed once that board is no longer active.
- Preserve existing `cancelled`, `needs-user`, `interrupted`, and `lost` invariants; do not make stale recovery, auto-resume, or daemon reconciliation retry states looser.
- Scope this to automation-started auto-groom/auto-implement runs and the shared AgentService/daemon lifecycle. If manual drawer runs share the same bug surface, either cover them in the same implementation path or file a follow-up with evidence.

## Acceptance Criteria

- [ ] Add backend coverage for switching from Board A to Board B while a Board A automation-started auto-groom or auto-implement run is running; Board A receives one terminal JSONL `finished` event with `status: cancelled` and reason `board switch`, and Board A task metadata is updated through the old board client.
- [ ] After the switch completes, Board B is the only active watcher/daemon/coordinator target, and old-board run completion cannot update Board B task metadata, enqueue state, promotion/review follow-up behavior, or stale board UI.
- [ ] Board-switch cancellation reuses the existing cancel/shutdown ordering: mark the active run cancelled before signalling, terminate the process group with the existing grace/escalation behavior, append JSONL before the task metadata edit, and keep the operation idempotent under races with natural process exit.
- [ ] Auto-groom success handling is board-scoped: a groom run started on Board A cannot promote or triage a same-ID task on Board B after the user switches boards.
- [ ] If the GUI process dies during the switch before the cancellation terminal write completes, the next activation of Board A still relies on existing recovery semantics: captured-session stale runs become `interrupted`, no-session stale runs become `lost`, and user-cancelled runs stay `cancelled`.
- [ ] Queued-but-not-started daemon work from the old board is handled explicitly and covered by tests, either by leaving durable `queued` work for the next Board A activation or by cancelling it with a documented JSONL/metadata outcome; no in-memory queue item may run after Board B is active.
- [ ] Existing board-switch validation behavior from TB-208, daemon reconciliation preservation from TB-266, and auto-resume behavior from TB-291 continue to pass.
- [ ] Manual test note: run the GUI with two boards, start a slow auto-groom or auto-implement run on Board A, switch to Board B, confirm Board B stays active and responsive, switch back to Board A, and confirm the old task shows a coherent cancelled run with reason `board switch` and no duplicate running row.
- [ ] Verification includes `cd gui && go test ./internal/daemon ./app` and `cd gui && go test ./...`; run frontend checks if any Wails event or visible state handling changes.

## Review Target

scope: GUI board switch cancellation for daemon, auto-groom, and auto-implement runs
files: gui/app/settings_service.go, gui/adapters.go, gui/app/agent_cancel.go, gui/app/agent_run.go, gui/app/agent_service.go, gui/app/auto_groom.go, gui/internal/daemon/daemon.go, gui/internal/daemon/daemon_test.go, gui/app/daemon_integration_test.go, gui/app/settings_service_test.go
verification: cd gui && go test ./...; subagent review Helmholtz returned SHIP with no CRITICAL or MAJOR issues.
follow-up: TB-311 tracks the remaining real desktop GUI smoke test.

## Review Findings

SHIP. No CRITICAL or MAJOR issues found in final review. Board switching now cancels old-board active runs through captured board clients/paths, uses a distinct `board switch` terminal reason, generation-scopes queued daemon work, preserves watcher failure atomicity, and suppresses stale-board auto-groom/reconciliation follow-up. Manual desktop smoke remains tracked separately in TB-311.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited priority=P1, type=bug, size=M, module=gui, tags=gui,board-switch,daemon,agent,automation,lifecycle, title=GUI board switch cancels old-board auto-runs cleanly
- 2026-05-20: Edited goal
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited goal
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited review-target
- 2026-05-20: Edited review-findings
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=codex, review-status=success, reviewref=local:Helmholtz-board-switch-review
- 2026-05-20: Submitted to code-review
- 2026-05-20: Done
