# TB-250: Resolve GUI golangci-lint baseline findings

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Module:** tooling
**Tags:** lint,go,quality
**Branch:** —

## Goal

Clear the GUI-side golangci-lint findings discovered during TB-206 so the project can remove the temporary baseline suppressions.

## Acceptance Criteria

- [ ] `errcheck` in `gui/app/agent_run.go` handles the cancellation-path `killActiveRun` error, likely by recording/logging it without racing the normal finish path.
- [ ] `errcheck` in `gui/app/settings_service_test.go` checks the two `os.MkdirAll` setup errors before writing recents fixtures.
- [ ] `errorlint` in `gui/app/attachments.go` wraps the opener failure with `%w` while preserving command output context for the GUI.
- [ ] `errorlint` in `gui/internal/agent/groom_test.go` uses `errors.Is` or otherwise compares wrapped runner errors safely.
- [ ] `nilerr` in `gui/internal/agent/usage_codex.go` handles or documents `DirEntry.Info` failures while scanning Codex rollout files.
- [ ] `unused` in `gui/app/agent_run.go` removes or wires `AgentService.hasActiveRunID` so recovered-run tracking does not leave dead helper code behind.
- [ ] `unused` in `gui/app/agent_service_test.go` removes or reuses the unused `recordingEmitter.names` helper.
- [ ] The temporary TB-250 GUI exclusions in `.golangci.yml` are removed.
- [ ] `GOLANGCI_LINT=/Users/ralist/go/bin/golangci-lint make lint-go` runs the GUI module cleanly after the fixes.

## Related Tasks

- **TB-206** — Setup golangci-lint for project and initial run it (source: GUI first-pass findings)

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited acceptance
