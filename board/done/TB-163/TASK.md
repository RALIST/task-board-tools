# TB-163: TB-93/GUI: add error-path tests for removeAttachments and openAttachment in api.test.ts

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,testing,frontend
**Branch:** —
**Parent:** TB-93

## Goal

gui/frontend/src/lib/api.test.ts - only addAttachments (line 148-152) covers the rejection path. openAttachment is the most likely to fail at runtime (OS open command missing, file moved, permission denied) yet has no error test. Add parallel 'propagates binding errors' cases for removeAttachments and openAttachment. Source: GUI frontend review finding #10.

## Acceptance Criteria

- [x] `api.test.ts` adds `removeAttachments propagates binding errors` and `openAttachment propagates binding errors (missing dir, missing file, OS failure)` cases, mirroring the existing `addAttachments` rejection coverage.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

