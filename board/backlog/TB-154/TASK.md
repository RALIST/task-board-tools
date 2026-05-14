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

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
