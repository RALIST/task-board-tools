# TB-125: GUI: whole TaskDrawer accepts attachment file drops

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

When a task drawer is open, dropping files anywhere inside the drawer should attach them to that task. Today only the small `Attachments` section is a valid drop target, which is easy to miss.

## Context

- TB-104 wired native file-drop via Wails `EnableFileDrop` + `data-file-drop-target`. The drawer's drop target is currently only the `.attachments-section` element (see `gui/frontend/src/lib/components/TaskDrawer.svelte` — the section that has `data-file-drop-target` and `data-task-id={detail.metadata.id}`).
- The Wails runtime resolves the drop target by `document.elementFromPoint(x, y).closest('[data-file-drop-target]')`. Moving the attribute pair to the drawer's outermost element (the `.drawer` aside) makes the whole drawer body a valid target without changing the Go-side handler.

## Acceptance Criteria

- [x] Dropping files anywhere inside the open task drawer attaches them to the drawer's task; the Attachments section retains its existing visual treatment but is no longer the only valid target.
- [x] Dropping outside the drawer (on the backdrop) does not attach to the drawer's task — only elements with their own `data-file-drop-target` (cards, future surfaces) participate.
- [x] The drawer shows the same drop-active highlight the runtime already applies (`:global(.file-drop-target-active)`), but visually consistent with the drawer chrome (e.g. an inset accent border) so the affordance is obvious.
- [x] Existing card-to-column DnD inside columns is unaffected.

## Related Tasks

- **TB-93** — parent epic.
- **TB-104** — initial drag-and-drop wiring this task extends.
- **TB-103** — drawer attachment list this task feeds into.
- **TB-126** — sibling follow-up for card-level drop, shares the runtime/Go path.

## Attachments

## Log

- 2026-05-14: Created
- 2026-05-14: Groomed as follow-up to TB-104 after user feedback that the attachments-only target is too small to discover.
- 2026-05-14: Edited agent=claude
- 2026-05-14: Edited agentstatus=queued
- 2026-05-14: Edited agentstatus=running
- 2026-05-14: Edited agentstatus=success
- 2026-05-14: Edited agentstatus=queued
- 2026-05-14: Edited agentstatus=running
- 2026-05-14: Started — moved to in-progress
- 2026-05-14: Edited acceptance
- 2026-05-14: Done
- 2026-05-14: Edited agentstatus=success
