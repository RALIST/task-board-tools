# TB-153: TB-93/GUI: attachment remove is destructive single-click without confirmation

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,ux,a11y
**Branch:** —
**Parent:** TB-93

## Goal

TaskDrawer.svelte:736-742 - single click on the X button invokes tb attach --remove which moves the file to <task>/attachments/.trash/ (per backend), but to the user that looks like a delete. The button has aria-label and title='Remove attachment' but a misclick is irrecoverable from the UI. Fix: use the existing two-click pattern matching archivePrompt/cancelPrompt in the same file (lines 104-105) - first click switches to 'Click again to remove' state with a short timeout. Source: GUI frontend review finding #3.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
