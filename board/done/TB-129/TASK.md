# TB-129: Remove ”non-editable” section when edit task’s body

**Type:** bug
**Priority:** P2
**Size:** S
**Agent:** claude
**AgentStatus:** failed
**Module:** gui
**Tags:** quick-win
**Branch:** —

## Goal

Remove the separate read-only header/metadata preview from the task body edit UI so edit mode shows only the editable markdown body while keeping the existing protected-prefix save contract.

## Context

- Current UI surface: `gui/frontend/src/lib/components/TaskDrawer.svelte` renders `<BodyEditor>` when the Description section enters edit mode.
- Current implementation: `gui/frontend/src/lib/components/BodyEditor.svelte` computes the protected prefix (`headerStrip`) from `originalBody`, renders it in `<pre class="header-strip" aria-label="Read-only header">`, and shows the hint text `header above is read-only`.
- Backend contract: `gui/app/edit_body.go` / `BoardService.EditTaskBody` still receives the full file body and rejects header or metadata mutations; `docs/ARCHITECTURE.md` documents this as the only GUI direct-write path.

### Constraints / Non-goals

- Do not make the title or metadata editable in the CodeMirror body editor; metadata stays owned by the existing TaskDrawer fields and `EditTask`.
- Do not relax `EditTaskBody` validation, locking, atomic write, log append, or `tb regenerate` behavior.
- Keep `BodyEditor` emitting the full markdown (`protected prefix + edited body`) expected by `editTaskBody`.
- Prefer a frontend-only fix unless tests reveal the save contract is broken.

## Acceptance Criteria

- [ ] In Description edit mode, the separate read-only header/metadata preview is not rendered or visible above CodeMirror.
- [ ] The editor still starts at the first editable `## ...` body section; header/title/metadata text is not inserted into the editable CodeMirror document.
- [ ] Saving a body edit still calls `editTaskBody` / `BoardService.EditTaskBody` with the protected prefix preserved so existing backend header/metadata rejection behavior remains valid.
- [ ] Any UI hint, label, ARIA label, or CSS that only supported the removed read-only header strip is removed or rewritten so it does not reference hidden UI.
- [ ] Focused frontend checks cover the body editor behavior, and `cd gui/frontend && npm run check` passes.
- [ ] Manual test: run the GUI, open a task, click Description -> Edit, confirm only the body editor appears, edit Goal/Acceptance text, save, reload/reopen the task, and confirm title/metadata are unchanged.

## Related Tasks

- **TB-37** - CodeMirror body editor in TaskDrawer (relationship: original body editor behavior to revise)
- **TB-33** - BoardService: EditTaskBody direct-write under .board.lock (relationship: protected save contract to preserve)

## Attachments

## Log

- 2026-05-14: Created
- 2026-05-14: Edited body via GUI
- 2026-05-14: Edited agent=codex
- 2026-05-14: Edited agentstatus=queued
- 2026-05-14: Edited agentstatus=running
- 2026-05-14: Edited size=S, module=gui, tags=quick-win, goal
- 2026-05-14: Edited acceptance
- 2026-05-14: Edited agentstatus=success
- 2026-05-15: Edited agent=claude
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited agentstatus=failed
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Done
- 2026-05-15: Edited agentstatus=failed

