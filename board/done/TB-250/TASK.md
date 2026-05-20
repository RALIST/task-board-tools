# TB-250: Resolve GUI golangci-lint baseline findings

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
**ReviewRef:** 06f59a8
**Branch:** —

## Goal

Clear the GUI-side golangci-lint findings discovered during TB-206 so the project can remove the temporary baseline suppressions.

## Acceptance Criteria

- [x] `errcheck` in `gui/app/agent_run.go` handles the cancellation-path `killActiveRun` error, likely by recording/logging it without racing the normal finish path.
- [x] `errcheck` in `gui/app/settings_service_test.go` checks the two `os.MkdirAll` setup errors before writing recents fixtures.
- [x] `errorlint` in `gui/app/attachments.go` wraps the opener failure with `%w` while preserving command output context for the GUI.
- [x] `errorlint` in `gui/internal/agent/groom_test.go` uses `errors.Is` or otherwise compares wrapped runner errors safely.
- [x] `nilerr` in `gui/internal/agent/usage_codex.go` handles or documents `DirEntry.Info` failures while scanning Codex rollout files.
- [x] `unused` in `gui/app/agent_run.go` removes or wires `AgentService.hasActiveRunID` so recovered-run tracking does not leave dead helper code behind.
- [x] `unused` in `gui/app/agent_service_test.go` removes or reuses the unused `recordingEmitter.names` helper.
- [x] The temporary TB-250 GUI exclusions in `.golangci.yml` are removed.
- [x] `GOLANGCI_LINT=/Users/ralist/go/bin/golangci-lint make lint-go` runs the GUI module cleanly after the fixes.

## Related Tasks

- **TB-206** — Setup golangci-lint for project and initial run it (source: GUI first-pass findings)

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
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=codex, review-status=success, reviewref=06f59a8
- 2026-05-20: Done
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success
