# TB-157: TB-93/CLI: log warnings to stderr from best-effort rollback removal failures

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** cli
**Tags:** epic-tb93,review-tb93,attach
**Branch:** —
**Parent:** TB-93

## Goal

cli/attach.go:436-439 in attachToFolderTask - if writeFileAtomic of TASK.md fails, the rollback calls removeFiles(published) which uses _ = os.Remove(path). Those failures are silently swallowed - if one fails (permissions, antivirus on macOS) the user sees only the TASK.md write error while an attachment file remains in attachments/ with no Attachments section entry. The next tb attach self-heals via readAttachmentNames so this is cosmetic. Fix: rename removeFiles to bestEffortRemoveFiles and fmt.Fprintf(os.Stderr, 'warning: failed to remove %s: %v', path, err) on remove failure. Source: CLI grand review finding #6.

## Acceptance Criteria

- [x] `removeFiles` renamed to `bestEffortRemoveFiles`; rollback now logs a stderr warning per failed removal instead of silently swallowing errors.
- [x] Legacy-agent-artifact cleanup at the end of promotion (TB-146 follow-up) is also degraded from a hard error to a warning so a publish that succeeded is not retroactively aborted by a janitorial step.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

