# TB-316: Profile GUI idle CPU usage

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** perf,follow-up
**Branch:** —

## Goal

Activity Monitor showed Task Board Tools itself using noticeable CPU while the app was open during/after autonomous agent activity. TB-315 fixes agent fan-out by enforcing max_workers, but plain GUI idle/startup CPU still needs profiling. Check UsageService refresh/session-file scanning, watcher churn, and Wails dev/runtime loops; add a focused regression or manual profiling note.

## Acceptance Criteria

- [ ] Capture a baseline CPU profile or sampling output for Task Board Tools while no agents are running.
- [ ] Identify whether idle CPU comes from UsageService refresh/session scanning, watcher churn, Wails runtime/dev-mode behavior, or another loop.
- [ ] Add a regression test where feasible, or document a reproducible manual profiling check when the behavior is runtime-only.
- [ ] Update docs or settings text if an intentional background refresh cadence is changed.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited acceptance
