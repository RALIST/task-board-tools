# TB-148: TB-93/CLI: confirm TB-96 hard-error reverted to warn+self-heal is intentional

**Type:** tech-debt
**Priority:** P1
**Size:** S
**Module:** cli
**Tags:** epic-tb93,review-tb93,docs
**Branch:** —
**Parent:** TB-93

## Goal

Commit 374dd4d ('TB-96 cli: fail on duplicate folder task forms') added duplicateTaskFormError so any reader seeing both <ID>.md and <ID>/TASK.md would fail loudly. The uncommitted cli/board.go diff replaces that with warnDualForm + cleanupOrphanFileFormSibling, downgrading to a warning + self-heal. This is consistent with the new docs/ARCHITECTURE.md 'crash-recovery transient' framing and with TestFolderTaskDuplicateFormSelfHeals, but invalidates TB-96's explicit AC ('fail on duplicate folder task forms'). Either the TB-96 ACs need updating (likely the right call given criterion-1 promotion-atomicity contract) or this is a regression. Also: verify the dual-form warning only emits once per resolver call (currently discoverTaskRefsInStatus may print one warning per status with a dual-form ID, and across resolveTaskRefInStatus the same ID may double-emit when both code paths fire). Source: CLI grand review finding #4.

## Decision

Warn+self-heal is intentional and matches `docs/ARCHITECTURE.md`. TB-96's AC #1 ("duplicate file/folder forms for the same task ID return a clear non-zero error") was superseded by the docs spec when dual-form was reframed as a crash-recovery transient. The behavior is correct; the only outstanding bug was the warning emission count.

## Acceptance Criteria

- [x] Dual-form warning is deduped per process so a single dual-form task no longer produces 6+ identical stderr lines during one command (one mutation followed by `regenerateBoard` walks each status twice via `collectActiveTasks` + direct `collectTasks`).
- [x] First encounter of each `(taskID, status)` still emits exactly one warning, so a real crash-recovery transient is not silenced.
- [x] `TestFolderTaskDuplicateFormSelfHeals` still asserts the warning surface and self-heal-on-next-mutation behavior unchanged; a new `TestDualFormWarningDedupedAcrossDiscoveryCalls` covers the dedupe.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

