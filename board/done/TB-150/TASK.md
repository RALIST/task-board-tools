# TB-150: TB-93/GUI: watcher race during file->folder promotion misses first TASK.md edit

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb93,review-tb93,watcher
**Branch:** —
**Parent:** TB-93

## Goal

gui/internal/watcher/watcher.go:283 - when a file-form task is promoted to folder form, the CLI's Mkdir(<status>/<ID>/) + Rename(<status>/<ID>.md, <status>/<ID>/TASK.md) sequence fires (a) Create on the dir within the status-dir watch, then (b) Create on TASK.md inside the new dir. watchCreatedDir runs on (a) and calls fsw.Add on the new dir, but the Rename of TASK.md into that dir may have already completed before fsw.Add returns - subsequent edits to TASK.md won't produce task:updated:<ID>. The board:reloaded debounce still fires (parent Create triggered it) so a full-board reload masks this, but the inline drawer auto-refresh path is silently broken for the first edit after promotion. Fix: after addWatchDir succeeds, os.Stat the expected TASK.md and synthesize a task:updated:<ID> emission if present; or accept and document the limitation. The existing folder-task tests stage the entire folder before the rename - promotion is a different sequence and needs its own test. Source: GUI backend review finding #2.

## Acceptance Criteria

- [x] `watchCreatedDir` synthesises a `task:updated:<ID>` event after a successful `addWatchDir` on a task dir whose `TASK.md` is already present. This covers the file→folder promotion publish-rename, where the Create event for the new dir fires only after `TASK.md` is already in place — any same-window atomic write would otherwise land before fsnotify is subscribed.
- [x] `addWatchDir` now returns a bool indicating whether a new subscription was added; the synthesised emission only fires on first-add to avoid double-emit on retries.
- [x] New `TestFolderTaskPromotion_EmitsTaskUpdatedAfterRename` watcher test reproduces the promotion sequence and asserts at least one `task:updated:<ID>` after the rename. Existing folder-task tests unchanged.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

