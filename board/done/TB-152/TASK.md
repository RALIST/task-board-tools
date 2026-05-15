# TB-152: TB-93/GUI: TaskDrawer attachments UI has no component-level tests

**Type:** tech-debt
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb93,review-tb93,testing,frontend
**Branch:** —
**Parent:** TB-93

## Goal

There is no gui/frontend/src/lib/components/TaskDrawer.test.ts. The new attachment add/remove/open/drop-target attributes/formatSize logic in TaskDrawer.svelte is entirely uncovered at component level. Card.test.ts exists and tests the 6-line drop-target attribute change but the drawer carries the bulk of the attachment UX. Fix: add TaskDrawer.test.ts that mocks /api and asserts (1) the attachment list renders with name + human-readable size, (2) the Add Files button calls pickAttachmentFiles + addAttachments, (3) the Remove button calls removeAttachments with the right name, (4) the row click calls openAttachment, (5) data-file-drop-target and data-task-id are on .surface (the load-bearing contract with gui/main.go:154-180). Source: GUI frontend review finding #4.

## Acceptance Criteria

- [x] New `gui/frontend/src/lib/components/TaskDrawer.test.ts` mocks `$lib/api` and the relevant stores, mounts the drawer, and covers: attachment list rendering with `formatSize` output, Add Files → `pickAttachmentFiles` + `addAttachments`, row click → `openAttachment`, remove button → two-click confirm → `removeAttachments`, and `data-file-drop-target` + `data-task-id` on `.surface`.
- [x] `npm test -- --run src/lib/components/TaskDrawer.test.ts` passes (9 tests).
- [x] `npm run check` clean.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

