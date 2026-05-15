# TB-165: TB-93/GUI: empty-state hint should say 'drag onto this drawer' not 'onto the task'

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,ux,frontend
**Branch:** —
**Parent:** TB-93

## Goal

TaskDrawer.svelte:727 - empty-state hint says 'drag-and-drop onto the task' but the task drawer is open as a modal over the cards. Users won't know whether to drop on the card behind the modal or on the modal itself. Both work (per Card.svelte:70 and TaskDrawer.svelte:666) but the wording is ambiguous. Fix: 'drag-and-drop files onto this drawer' - it's the obvious target when the drawer is open. Source: GUI frontend review finding #11.

## Acceptance Criteria

- [x] Empty-state hint reworded to "No attachments. Add files via the button above or drag-and-drop files onto this drawer." The drawer is the obvious target when it is open.
- [x] `TaskDrawer.test.ts` empty-state assertion updated to match the new phrase precisely so future copy drift breaks the test loudly.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

