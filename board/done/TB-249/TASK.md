# TB-249: Resolve CLI golangci-lint baseline findings

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Module:** tooling
**Tags:** lint,go,quality
**Agent:** codex
**ImplementedBy:** codex
**ImplementStatus:** success
**AgentStatus:** success
**ReviewedBy:** codex
**ReviewStatus:** success
**ReviewRef:** main
**Branch:** —

## Goal

Clear the CLI-side golangci-lint findings discovered during TB-206 so the project can remove the temporary baseline suppressions.

## Acceptance Criteria

- [x] `errcheck` in `cli/board.go` handles or deliberately documents unlock/close failures around `syscall.Flock` instead of ignoring them.
- [x] `errcheck`/`nilerr` in `cli/scan.go` preserves or reports `filepath.Walk` errors instead of silently returning nil from the walk callback and outer call.
- [x] `nilerr` in `cli/board.go` distinguishes missing folder-task markdown from unexpected `os.Stat` failures in `cleanupOrphanFileFormSibling`.
- [x] `nilerr` in `cli/ready.go` reports `collectTasks` failures from WIP-limit enforcement instead of ignoring them.
- [x] `unused` in `cli/board.go` is resolved by removing or wiring `writeSimpleYAML` and `statusFromTaskPath`.
- [x] The temporary TB-249 CLI exclusions in `.golangci.yml` are removed.
- [x] `GOLANGCI_LINT=/Users/ralist/go/bin/golangci-lint make lint-go` still runs the CLI module cleanly after the fixes.

## Review Findings

Subagent code review completed for the scoped TB-249 lint-baseline diff.

Result: no Critical, Important, or Minor findings.

Residual test gaps noted by reviewer: `cmdScan` fatal wrapping is covered through `scanForTodos` rather than a direct CLI-exit test, and the `syscall.Flock`/`Close` failure paths are documented/warned but not unit-forced because they are difficult to trigger deterministically.

## Related Tasks

- **TB-206** — Setup golangci-lint for project and initial run it (source: CLI first-pass findings)

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited acceptance
- 2026-05-19: Committed — moved to ready
- 2026-05-20: Edited agent=codex
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-20: Moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=none
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited review-findings
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=codex, review-status=success, reviewref=main
- 2026-05-20: Done
