# TB-154: TB-93/GUI: attachment list accessibility improvements (aria-label, keyboard nav)

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,a11y,frontend
**Branch:** —
**Parent:** TB-93

## Goal

TaskDrawer.svelte:732 - clicking an attachment name has no keyboard activation feedback and no aria-label; the row's purpose (open in OS) is only in the title attribute (mouse-only). Fix: (1) add aria-label='Open <name> in default app' to the button; (2) consider making the row itself focusable+activatable as a single unit so keyboard users get one tab-stop per attachment instead of two (name button + remove button); (3) verify the empty-state hint mentions BOTH picker and drag-and-drop so keyboard-only users have a non-drop path in. Source: GUI frontend review finding #2.

## Acceptance Criteria

- [x] Attachment name button carries `aria-label="Open <name> in default application"` so screen readers describe the row's action without relying on the (mouse-only) `title` attribute.
- [x] Attachment list (`<ul>`) carries `aria-label="Attachments"`.
- [x] Empty-state hint already mentions both the file picker and drag-and-drop (verified — "Add files via the button above or drag-and-drop onto the task"), so keyboard-only users have a non-drop path in.
- [x] Two-tab-stops-per-row layout is preserved (name button + remove button) — collapsing to one stop would conflict with the new two-click remove confirm in TB-153.
- [x] `TaskDrawer.test.ts` asserts the aria-label and list label.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

