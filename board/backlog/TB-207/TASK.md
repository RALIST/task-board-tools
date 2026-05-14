# TB-207: Allow tasl title edit from GUI

**Type:** improvement
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** ux,task-edit
**Branch:** —

## Goal

Allow users to rename a task title from the GUI without editing raw markdown or weakening the existing body-editor protections.

## Context

- Current title surfaces are `gui/frontend/src/lib/components/Card.svelte` and `gui/frontend/src/lib/components/TaskDrawer.svelte`; the drawer renders `detail.metadata.title` in its header.
- Existing structured metadata edits flow through `gui/frontend/src/lib/api.ts` -> `BoardService.EditTask` -> `gui/internal/cli.Client.Edit` -> `tb edit`.
- `tb edit` currently updates metadata and Goal/Acceptance sections, but it has no title flag. `BoardService.EditTaskBody` deliberately rejects title/header mutations.
- Add title rename support through the structured edit path, likely by extending `tb edit`, `cli.EditInput`, `BoardService.EditTask`/`EditTaskInput`, Wails bindings, and the frontend API wrapper.
- `docs/ARCHITECTURE.md` documents `EditTaskBody` as the body-only direct-write exception; title rename must preserve the same board lock, atomic write, and regenerate invariants as other structured task mutations.

## Constraints

- Do not make the CodeMirror/body editor responsible for title changes.
- Do not relax `EditTaskBody` header or metadata protection.
- Saving a rename must not change task ID, status, file/directory path, attachments, body text, or unrelated metadata.
- Keep card, drawer, search/filter text, and watcher-driven refresh behavior consistent after a successful rename.
- Provide mouse and keyboard-accessible controls: double-click can enter rename mode, but keyboard users must be able to start, save, and cancel the edit.
- Non-goals: bulk rename, task ID changes, file/directory renames, or moving the task between board columns.

## Acceptance Criteria

- [ ] Double-clicking a task title in the GUI opens an inline rename affordance with the current title prefilled or selected.
- [ ] Keyboard users can start the rename from the focused title control, save with Enter or an explicit Save action, and cancel with Escape or an explicit Cancel action.
- [ ] Empty titles are rejected before save; unchanged titles are treated as a no-op without writing the task file.
- [ ] Saving a changed title persists the header line as `# TB-N: New title`, appends an appropriate task log entry, regenerates `BOARD.md`, and preserves task ID/status/path, attachments, body, and unrelated metadata.
- [ ] On success, the card list, open drawer title, and client-side search/filter results reflect the new title without requiring an app restart.
- [ ] On failure, the GUI shows an error toast/message, keeps the rename draft available, and does not optimistically leave the old title hidden.
- [ ] Automated coverage includes the structured title mutation path plus frontend rename behavior; run relevant Go tests, `cd gui/frontend && npm run check`, and relevant Vitest tests.
- [ ] Manual test note: in `cd gui && wails3 dev` or `task dev`, open a real board, rename a throwaway task from the GUI, close/reopen the drawer, and confirm `tb show <ID>` shows the new title while the task remains in the same column.

## Related Tasks

- **TB-33** — BoardService: EditTaskBody direct-write under .board.lock (relationship: protected body-edit contract to preserve)
- **TB-37** — CodeMirror body editor in TaskDrawer (relationship: adjacent TaskDrawer editing surface)
- **TB-129** — Remove ”non-editable” section when edit task’s body (relationship: confirms title stays out of the body editor)

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited type=improvement, module=gui, tags=ux,task-edit
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success

