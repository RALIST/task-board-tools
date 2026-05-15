# TB-164: TB-93/GUI: surface drag-and-drop in-flight state via attach:dropping/attach:dropped events

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,ux,frontend
**Branch:** —
**Parent:** TB-93

## Goal

TaskDrawer.svelte:255-263 - attachmentsBusy is shared between add and remove flows but when a *drop* happens (handled entirely by Wails in gui/main.go), attachmentsBusy is never set. The user can still click Add Files or Remove during the drop's tb-attach execution. Concurrent tb attach runs are serialized by .board.lock but the GUI gives no feedback. Also: TaskDrawer.svelte:255-263 onRemoveAttachment doesn't force a refresh - relies on watcher board:reloaded round-trip which is 'eventual' (on slow FS the row stays visible after the toast 'Removed X'). Fix: emit paired attach:dropping/attach:dropped from gui/main.go bracketing each drop; subscribe to it in TaskDrawer to toggle attachmentsBusy (or a separate dropInFlight flag). Consider optimistic local mutation on remove that the next watcher refresh corrects. Source: GUI frontend review findings #6 + #8.

## Acceptance Criteria

- [x] `gui/main.go` emits `attach:dropping` immediately before invoking `BoardService.AddAttachments` from the file-drop handler, and the existing `attach:dropped` continues to fire after the call. Both events carry `{taskId, count}` (and `attach:dropped` adds `ok`/`error`).
- [x] `TaskDrawer.svelte` subscribes to both events. The drawer disables Add Files and Remove buttons (`attachmentsBusy = true`) while a drop is in flight for the currently open task, and re-enables on dropped. Events for other task IDs are ignored.
- [x] Optimistic-local-mutation-on-remove out of scope: watcher already refreshes via board:reloaded after the CLI commits.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

