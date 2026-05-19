# TB-249: Resolve CLI golangci-lint baseline findings

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Module:** tooling
**Tags:** lint,go,quality
**Branch:** —

## Goal

Clear the CLI-side golangci-lint findings discovered during TB-206 so the project can remove the temporary baseline suppressions.

## Acceptance Criteria

- [ ] `errcheck` in `cli/board.go` handles or deliberately documents unlock/close failures around `syscall.Flock` instead of ignoring them.
- [ ] `errcheck`/`nilerr` in `cli/scan.go` preserves or reports `filepath.Walk` errors instead of silently returning nil from the walk callback and outer call.
- [ ] `nilerr` in `cli/board.go` distinguishes missing folder-task markdown from unexpected `os.Stat` failures in `cleanupOrphanFileFormSibling`.
- [ ] `nilerr` in `cli/ready.go` reports `collectTasks` failures from WIP-limit enforcement instead of ignoring them.
- [ ] `unused` in `cli/board.go` is resolved by removing or wiring `writeSimpleYAML` and `statusFromTaskPath`.
- [ ] The temporary TB-249 CLI exclusions in `.golangci.yml` are removed.
- [ ] `GOLANGCI_LINT=/Users/ralist/go/bin/golangci-lint make lint-go` still runs the CLI module cleanly after the fixes.

## Related Tasks

- **TB-206** — Setup golangci-lint for project and initial run it (source: CLI first-pass findings)

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited acceptance
