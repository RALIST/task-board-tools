# TB-166: TB-93/GUI: folder_tasks_test.go uses temp/staging names that don't match the CLI's real pattern

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,testing
**Branch:** —
**Parent:** TB-93

## Goal

gui/internal/watcher/folder_tasks_test.go:69 - taskTmp := taskFile + '.tmp.X' produces TASK.md.tmp.X. isIgnored accepts this (matches .tmp. substring) but the test does not validate that the CLI's *actual* temp pattern is similarly ignored. The CLI uses .<base>.tmp.<pid>.<token> (leading dot, see cli/attach.go:503 and cli/atomicfs.go). That pattern is also caught by isIgnored via the leading-dot check. Same issue with the test using .TB-1.staging - actual CLI staging dir is .attach<token> per makeHiddenWorkDir(taskDir, '.attach'). Fix: add tests using the CLI's actual temp naming pattern (.TASK.md.tmp.1234.deadbeef) and actual staging pattern (.attach<token>) to prevent a future watcher change that loosens dot-prefix handling. Source: GUI backend review finding #7.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
