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

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
