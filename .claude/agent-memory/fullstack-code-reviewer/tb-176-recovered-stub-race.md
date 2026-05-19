---
name: tb-176-recovered-stub-race
description: TB-176 cancel-vs-monitor race model for orphaned PIDs adopted as stub activeRuns on daemon restart
metadata:
  type: project
---

After TB-176, RecoveryService.recoverOne installs a stub activeRun (via `adoptRecoveredRun`) when an orphaned PID is observed live, and spawns a recovered-run monitor goroutine that polls `liveFn(pid)`. The stub's Done channel has TWO closers: `runGoroutine`'s defer (for normal runs) and the monitor's `reconcileOrphanExit` (for stubs). Both route through `ar.closeDone()` which is gated by `doneOnce`.

**Why:** `killActiveRun` ends with an unbounded `<-ar.Done`. For a stub there is no `cmd.Wait` goroutine to close Done — only the monitor's PID-death observation does.

**How to apply when reviewing this code:**
- The monitor's `pollRecoveredRunMonitor` MUST check `latest.LastFinished != nil` BEFORE the live-PID branch, otherwise a cancel that wrote the terminal first would cause the monitor to re-enter `reconcileOrphanExit` and write a duplicate finished line.
- `reconcileOrphanExit` is only reachable when `LastFinished == nil`. The `wasCancelled()` branch then defers terminal writing to CancelRun's pending `finishCancelled`. The else-branch calls `recordTerminal` directly.
- `recordTerminal` already calls `delete(s.active, taskID)` inside its `finishOnce.Do` body. The `removeActiveRun` belt-and-braces call in `reconcileOrphanExit` is a defensive no-op on the happy path; it only matters if a prior finishOnce consumer never reached that delete (currently no such path exists, but the guard is cheap).
- `syncFinishedStatus` (the `LastFinished != nil` branch) does NOT close Done or remove active. That's correct: if a terminal record exists, CancelRun has already finished and is no longer waiting on Done; `recordTerminal` already removed active.
- `syscall.Getpgid(pid)` works for reparented orphans because pgid is preserved across init reparenting. If the call fails the stub uses pgid=0 and `killActiveRun` falls back to `kill(pid, SIGKILL)` only — grandchildren may be missed but this matches the existing `runExternal` Setpgid-verify-failed fallback.
- The stub's existence in `s.active` blocks `RunAgent`/`RunQueuedAgentSync` via `ErrAlreadyRunning` — daemon dedup via `HasActiveRun(taskID)` is intentionally extended to cover orphans.
