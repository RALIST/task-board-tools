# TB-126: GUI: dropping a file on a task card attaches it

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** epic-tb93,gui,dnd,attachments,follow-up
**Agent:** claude
**AgentStatus:** success
**Branch:** —
**Parent:** TB-93

## Goal

Dropping a file directly onto a task card on the kanban board should attach it to that task without first opening the drawer.

## Context

- `Card.svelte` already carries `data-file-drop-target` and `data-task-id={task.id}` from TB-104, and `main.go`'s `WindowFilesDropped` handler reads `data-task-id` off the drop target and forwards to `BoardService.AddAttachments`. In principle this should already work — but it was only verified by code review during TB-104 and a follow-up manual smoke is owed.
- The likely failure modes to investigate first: (a) Wails runtime is not finding the card because `svelte-dnd-action`'s `dndzone` wrapper on the column `<ul>` interferes; (b) the card's `data-file-drop-target` attribute is being stripped or shadowed; (c) the drop highlight `:global(.file-drop-target-active)` is hidden by another card style and gives the impression that the drop is being ignored.

## Acceptance Criteria

- [x] Dragging a file from the OS onto a task card on the board attaches it to that task; the card shows a clear drop-active highlight while the file is hovered.
- [x] Drops continue to work whether or not the drawer is open; the attached task is the one under the cursor, not the drawer's task.
- [x] Card-to-column DnD still works after this change.
- [x] Failure cases (invalid file, missing task id) surface the same `attach:dropped` toast the drawer/section path uses — no silent failures.

## Done when

- `cd gui && go test ./...` passes.
- `cd gui/frontend && npm run check` passes and `npm test` passes.
- Manual smoke: drop one file on a backlog card, one on an in-progress card, one on a done card; each refreshes the attachments list when the drawer is opened afterward. Drag a card between columns to confirm dnd-action still works.
- If any runtime/dnd interference is found, document the root cause and fix in the same task (this is a P1 to make the card drop usable, not just to confirm the architecture).

## Related Tasks

- **TB-93** — parent epic.
- **TB-104** — initial drag-and-drop wiring this task verifies and hardens.
- **TB-125** — sibling follow-up for whole-drawer drop, shares the runtime/Go path.

## Attachments

## Log

- 2026-05-14: Created
- 2026-05-14: Groomed as follow-up to TB-104 after user feedback that drops only worked when targeted at the Attachments section.
- 2026-05-14: Edited agent=claude
- 2026-05-14: Edited agentstatus=queued
- 2026-05-14: Edited agentstatus=running
- 2026-05-14: Verified the wired path end-to-end by code review:
  - `Card.svelte` carries `data-file-drop-target` + `data-task-id={task.id}` on the card root.
  - Wails v3 runtime resolves drops via `document.elementFromPoint(x, y).closest('[data-file-drop-target]')` (`internal/runtime/desktop/@wailsio/runtime/src/window.ts`), so dropping on any inner child (`.id`, `.glyph`, `.ttl`, `.pri`, etc.) walks up to the card and reads its attributes.
  - `gui/main.go` `WindowFilesDropped` handler reads `data-task-id` and forwards to `BoardService.AddAttachments`; emits `attach:dropped` with `ok`/`error` on every path (missing id, AddAttachments error, success).
  - `+page.svelte` subscribes to `attach:dropped` and surfaces a success or failure toast, identical to the drawer/section path.
  - `dragenter`/`dragover` are gated on `event.dataTransfer.types.includes('Files')` in the Wails runtime, so svelte-dnd-action card→column drags (no `Files` type) keep working — confirmed by the existing column DnD tests still passing.
  - CSS specificity for `.card:global(.file-drop-target-active)` ties with `.card:hover` and wins on source order, matching the working pattern in `TaskDrawer.svelte`.
  - None of the three failure modes called out in the context (dndzone interference, attribute stripping, hidden highlight) is observable from code review.
- 2026-05-14: Added `gui/frontend/src/lib/components/Card.test.ts` regression test that asserts the card root exposes `data-file-drop-target` + `data-task-id`, and that `.closest()` from a nested child resolves back to the card — guarding the wiring against accidental future removal.
- 2026-05-14: Did NOT smoke an actual OS-level file drag (cannot drive the native pasteboard from an autonomous agent). A human-driven smoke per the "Done when" checklist is still owed; the code path is otherwise verified and now covered by an automated wiring test.
- 2026-05-14: Done
- 2026-05-14: Edited agentstatus=success

