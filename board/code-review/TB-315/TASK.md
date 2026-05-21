# TB-315: Auto-groom and auto-resume must respect worker budget

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** auto-groom,daemon,resume,settings
**ReviewRef:** main
**Branch:** —

## Goal

Auto-groom and coordinator resume/restart paths currently start agent runs directly through AgentService, bypassing the daemon worker pool. Limit fresh grooming and auto-resume/restart fan-out to the configured worker budget, counting daemon-active and AgentService-active runs.

## Acceptance Criteria

- [x] Auto-groom scan computes remaining worker capacity before queueing backlog triage candidates; max_workers=2 queues at most two fresh groom runs in one scan and records worker-capacity skips for the rest.
- [x] Auto-groom resume/restart sweep uses the same worker capacity so interrupted/lost coordinator-owned backlog tasks cannot all resume at once.
- [x] Auto-implement resume/restart sweep also uses worker capacity; fresh auto-implement behavior from TB-300 stays intact.
- [x] Capacity counts both daemon active task IDs and AgentService active task IDs, and falls back to SettingsService max_workers when no daemon budget is available.
- [x] Direct AgentService starts, including manual runs and auto-groom/auto-implement starts, reserve a shared daemon worker slot before launching so racing starts cannot overrun max_workers.
- [x] max_workers changes apply to the running daemon without app restart; lowering the value makes new work wait and raising it wakes automation scans.
- [x] Regression tests cover fresh auto-groom fan-out, auto-groom resume fan-out, auto-implement resume fan-out, manual/direct worker-budget rejection, runtime worker-budget changes, and shared automation reservations.
- [x] Verification passes with `cd gui && go test ./...`.

## Review Target

Implement shared max_workers enforcement for auto-groom, auto-implement resume paths, direct AgentService runs, and daemon queued work. Verify runtime max_workers changes apply without app restart.

## Reviewer Notes

Verification: cd gui && go test ./internal/daemon -run 'TestDaemon_DispatcherWaitsForAgentServiceActiveRun|TestDaemon_AutomationReservationConsumesWorkerSlot|TestDaemon_AutomationReservationCountsAgentServiceActiveRuns|TestDaemon_DeactivateDropsQueuedOldBoardWork|TestDaemon_SetMaxWorkers_ReducesRuntimeConcurrency|TestDaemon_SetMaxWorkers_IncreasesRuntimeConcurrency' -count=1; cd gui && go test ./app -run 'TestRunAgent_RespectsWorkerBudget|TestRunAgent_RejectsAlreadyRunning|TestAutoGroomCoordinator_OnAgentRunFinishedPromotesWhenClean|TestAutoGroomCoordinator_LimitsStartsToWorkerBudget|TestAutoImplementCoordinator_LimitsStartsToWorkerBudget' -count=1; cd gui && go test ./...; git diff --check. Code review subagent found no CRITICAL/MAJOR issues.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited acceptance
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited review-target
- 2026-05-21: Edited reviewer-notes
- 2026-05-21: Edited reviewref=main
- 2026-05-21: Submitted to code-review
