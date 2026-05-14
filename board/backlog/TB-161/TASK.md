# TB-161: TB-93/GUI: OpenAttachment surfaces opaque error when attachments/ dir is missing

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,ux
**Branch:** —
**Parent:** TB-93

## Goal

gui/app/attachments.go:170-172 - filepath.EvalSymlinks(attachmentsDir) returns an error if attachmentsDir does not exist; OpenAttachment then surfaces 'resolve attachments dir: ...' which is opaque to the user. The earlier resolveTaskDir succeeded (task dir exists) so the absence of attachments/ is benign. Fix: before EvalSymlinks, os.Stat(attachmentsDir) and return a typed ErrNotFound-style error if it does not exist. Otherwise users opening an attachment for a task whose attachments subdir was removed out-of-band get a stack-trace-flavored error. Source: GUI backend review finding #8.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
