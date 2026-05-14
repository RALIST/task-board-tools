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

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
