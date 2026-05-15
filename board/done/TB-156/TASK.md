# TB-156: TB-93/CLI: tb attach add path should validate destination filename (parity with --rm)

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** cli
**Tags:** epic-tb93,review-tb93,attach
**Branch:** —
**Parent:** TB-93

## Goal

cli/attach.go:299-304 in prepareAttachmentSources - info.Mode().IsRegular() is the right check on Linux/macOS but a NUL-byte filename could still enter as filepath.Base(clean). validateAttachmentRemovalName rejects NULs for --rm but the attach add path has no equivalent guard on the destination name. Likely impossible to reach in practice (filesystems won't let you create such a file) but the add and remove sides should agree. Fix: after name := filepath.Base(clean) in prepareAttachmentSources, call validateAttachmentRemovalName(name) and reject if it fails. Source: CLI grand review finding #9.

## Acceptance Criteria

- [x] `prepareAttachmentSources` calls `validateAttachmentRemovalName(name)` after `filepath.Base(clean)`, so the add path rejects the same set of malformed destination names (NUL, separators, abs paths, `.`/`..`) that `--rm` rejects.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

