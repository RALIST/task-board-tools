# TB-149: TB-93/GUI: Windows cmd.exe metacharacter injection in OpenAttachment

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,security,windows
**Branch:** —
**Parent:** TB-93

## Goal

gui/app/attachments.go:42 uses cmd /c start '' <path> on Windows. cmd.exe honors metacharacters (&, |, ^, >, <, %, !, parens, trailing space/dot) in unquoted args. An attachment whose on-disk name contains these (legitimate Unicode filenames on Windows do not, but a tampered or git-cloned-from-Linux file can) yields shell-style command injection inside cmd.exe, despite exec.CommandContext not invoking a shell. validateAttachmentName only checks NUL, separators, abs paths, and . / .. The docstring claims to be the 'defense against tampered out-of-band state' - that defense is incomplete on Windows. Fix: switch to rundll32 url.dll,FileProtocolHandler <path> which doesn't go through cmd, OR wrap the path in double quotes for the start form AND reject names containing cmd.exe-significant metacharacters. Source: GUI backend review finding #1.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
