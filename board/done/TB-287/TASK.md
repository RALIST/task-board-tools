# TB-287: Flaky race in TestDaemonPeriodicRecovery_ReconcilesStaleRunningWithoutRestart

**Type:** bug
**Priority:** P2
**Size:** S
**Module:** gui
**Agent:** codex
**Tags:** daemon,recovery,testing,flaky
**GroomedBy:** codex
**GroomStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewedBy:** codex
**ReviewStatus:** success
**AgentStatus:** success
**Branch:** —

## Goal

Fix the data race in the periodic recovery integration test path so `TestDaemonPeriodicRecovery_ReconcilesStaleRunningWithoutRestart` can run reliably under Go's race detector while still proving that stale `running` tasks are reconciled without restarting the daemon.

## Context

- Repro command: `cd gui && go test ./app/ -race -count=1 -run TestDaemonPeriodicRecovery`; the current report is flaky (~50%) and points at the goroutine started by `Daemon.startPeriodicRecovery` in `gui/internal/daemon/daemon.go`.
- The behavior under test is in `gui/app/daemon_integration_test.go`: append queued/started JSONL for `TB-1`, set `AgentStatus: running`, wait for the periodic ticker to mark the task `lost`, and assert an `agent:run-finished` emit.
- `recordingEmitter` in `gui/app/agent_service_test.go` is mutex-protected and already exposes `snapshot()` / `names()` for race-safe assertions. The current test directly ranges over `svc.emitter.(*recordingEmitter).events`, which can race with a recovery emit from the ticker goroutine.
- Related implementation/docs: `gui/internal/daemon/daemon.go` periodic recovery loop and `docs/ARCHITECTURE.md` -> Daemon -> Periodic recovery.

## Constraints / Non-goals

- Preserve the periodic recovery behavior from TB-244: no daemon restart required, dead untracked `running` run becomes `lost`, finished JSONL is appended, and `agent:run-finished` is emitted.
- Do not disable the ticker, remove the emit assertion, replace the test with a weaker startup-recovery-only case, or paper over the race with arbitrary sleeps.
- Prefer fixing the unsafe test harness access if the race report matches the direct `recordingEmitter.events` read; only change production daemon/recovery code if the race detector proves production shared state is actually unsynchronized.

## Acceptance Criteria

- [x] The focused reproducer no longer reports a data race when run repeatedly: `cd gui && go test ./app/ -race -count=10 -run TestDaemonPeriodicRecovery`.
- [x] `TestDaemonPeriodicRecovery_ReconcilesStaleRunningWithoutRestart` still verifies the core behavior: stale `AgentStatus: running` is reconciled to `lost`, the appended `finished` event keeps `RunID=r_periodic`, and an `agent:run-finished` event is observed.
- [x] Any `recordingEmitter` assertions added or touched by this fix read through mutex-protected helpers such as `snapshot()` or `names()`; no test ranges over or inspects `recordingEmitter.events` without holding the emitter mutex.
- [x] The production periodic recovery lifecycle remains unchanged: `Activate` still starts the ticker when enabled, `Close`/`Deactivate` still stop it cleanly, and `SetPeriodicRecoveryEnabled` still toggles the live ticker.
- [x] Broader verification passes after the focused fix: `cd gui && go test ./... -race`.

## Related Tasks

- **TB-244** — Introduced/shipped periodic re-recovery for stale agent runs; this bug is test stability for that behavior.
- **TB-174** — Auto-groom work was checked as not being the cause of this race; avoid folding auto-groom scope into this fix.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited size=S, tags=daemon,recovery,testing,flaky, goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Done
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=codex, review-status=success
- 2026-05-20: Edited acceptance
- 2026-05-20: Summary — replaced unsafe `recordingEmitter.events` reads with mutex-backed snapshots; verified focused and full GUI race suites; subagent review found no CRITICAL/MAJOR issues.
- 2026-05-20: Edited agentstatus=none
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success
