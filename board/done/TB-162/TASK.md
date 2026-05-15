# TB-162: TB-93/GUI: api.ts listAttachments re-mapping strips the Attachment binding type

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,frontend
**Branch:** —
**Parent:** TB-93

## Goal

gui/frontend/src/lib/api.ts:111-114 - listAttachments re-maps the rows to { name, size }, discarding the Attachment class identity from the Wails binding. Functionally identical (only those two fields exist per bindings/.../models.ts:16-39) but if Wails ever adds a field (e.g., modTime) the mapping silently strips it. The test in api.test.ts:138-147 passes plain objects which masks this. Fix: return list as Attachment[]; (already typed) or destructure only when you need the POJO. Source: GUI frontend review finding #9.

## Acceptance Criteria

- [x] `listAttachments` returns the binding rows verbatim instead of re-mapping to `{ name, size }`. Any field Wails adds in the future survives the boundary.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

