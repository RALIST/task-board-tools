# TB-287: Flaky race in TestDaemonPeriodicRecovery_ReconcilesStaleRunningWithoutRestart

**Type:** bug
**Priority:** P2
**Size:** M
**Module:** gui
**Branch:** —

## Goal

Run `cd gui && go test ./app/ -race -count=1 -run TestDaemonPeriodicRecovery` — fails ~50% of the time with a data race in goroutine created by Daemon.startPeriodicRecovery (gui/internal/daemon/daemon.go:328). Pre-existing race, manifests in isolation and under the full -race suite. Confirmed unrelated to TB-174 by running the suite with auto_groom.go + auto_groom_test.go temporarily removed (passed). Likely candidate: the periodic recovery ticker goroutine sharing mutable state with the test harness without a mutex.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-20: Created
