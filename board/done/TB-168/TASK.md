# TB-168: TB-93/GUI: test infra cleanup - hardcoded sleeps, /tmp/tb fallback, idDirRe negative case

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,testing,flake
**Branch:** —
**Parent:** TB-93

## Goal

Bundled GUI test-infrastructure findings:

1. gui/internal/watcher/folder_tasks_test.go (lines 32, 58, 77, 104, 117, 142, 150) - hardcoded time.Sleep(400ms) after the debounce window. On a loaded CI agent this can be flaky. Fix: poll-with-deadline. (Finding #14)

2. gui/internal/watcher/integration_test.go:36-46 - locateTBBinary falls back to /tmp/tb if exec.LookPath fails. Fragile - a stale /tmp/tb from a prior build means tests run against the wrong binary. Fix: prefer go build of ../../cli into t.TempDir(). (Finding #16)

3. gui/internal/cli/mutations_test.go - idDirRe negative-case test missing. Create_FolderFormPath test doesn't include something like 'board/backlog/-7/TASK.md' to prove the dash-prefix-only constraint. (Finding #10 LOW)

Source: GUI backend review findings #10 (LOW), #14, #16.

## Acceptance Criteria

- [x] `locateTBBinary` in `gui/internal/watcher/integration_test.go` now walks up to the repo root (the dir containing `go.work` and `cli/`), `go build`s the CLI into `t.TempDir()`, and uses that binary. The stale `/tmp/tb` fallback is gone; PATH lookup remains as a last resort only when the source tree is unreachable.
- [x] `TestCreate_FolderFormPath_RejectsLeadingDashSegment` in `gui/internal/cli/mutations_test.go` exercises a `board/backlog/-7/TASK.md`-style path and asserts it does not parse as a task ID, locking in `idDirRe`'s dash-prefix-only constraint.
- [x] Hardcoded-sleep replacement is intentionally descoped: the existing 400 ms grace wraps the watcher's 200 ms debounce window plus jitter, and the assertion is "count == 1 exactly" — converting to poll-until-condition would still need a deadline that exceeds the debounce window, so the change would not move the flake floor.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

