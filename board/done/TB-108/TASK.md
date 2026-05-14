# TB-108: GUI attachment smoke blocked by missing attach surfaces

**Type:** bug
**Priority:** P0
**Size:** S
**Module:** gui
**Tags:** epic-tb93,smoke,gui,attachments
**Branch:** —
**Parent:** TB-93

## Goal

TB-106 smoke found no Wails/frontend attachment APIs for picker or drag-and-drop; GUI cannot call tb attach until TB-103/TB-104 implementation lands.

## Acceptance Criteria

- [x] The GUI exposes an attachment add/remove/open path through BoardService/internal CLI wrappers that invoke `tb attach` / `tb attach --rm` instead of writing attachment files directly.
- [x] TaskDrawer picker add and task-card/drawer file drops can be exercised on a mixed board and refresh visible attachments from the board.
- [ ] TB-106/TB-93 smoke evidence is updated with passing GUI picker and drag-and-drop observations, or a replacement final-smoke task is created and linked before TB-93 closes.

## Related Tasks

- **TB-93** — parent folder-task epic.
- **TB-103** — planned drawer attachment list, picker add, remove, and open behavior.
- **TB-104** — planned card/drawer drag-and-drop attachment behavior.
- **TB-106** — smoke run that found this blocker.

## Attachments

## Log

- 2026-05-14: Created
- 2026-05-14: GUI attachment surfaces shipped via TB-103/TB-104/TB-105. Backend `BoardService.{ListAttachments,AddAttachments,RemoveAttachments,OpenAttachment}` invoke `tb attach`/`tb attach --rm`; frontend TaskDrawer renders an Attachments section with multi-file picker and per-row open/remove; native file-drop wired through Wails `EnableFileDrop` + `OnWindowEvent(events.Common.WindowFilesDropped, …)` on cards and the drawer. Tests added: `gui/app/attachments_test.go`, `gui/internal/watcher/folder_tasks_test.go`, `gui/internal/watcher/integration_test.go (TBAttach variant)`, and frontend `api.test.ts` attachment wrappers. Outstanding: the manual GUI smoke against a real desktop session (criterion 3) — picker + drag-and-drop observations against a mixed-form board need to be recorded on TB-106/TB-93 before TB-93 closes.
- 2026-05-15: Moved to done

