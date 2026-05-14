# TB-181: Persist draft task/prevent close unsaved form

**Type:** bug
**Priority:** P1
**Size:** S
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** frontend,create-dialog,ux,quick-win
**Branch:** —

## Goal

Prevent accidental loss of entered `CreateTaskDialog` fields by requiring confirmation before a dirty new-task form can close.

## Context

Relevant surfaces:
- `gui/frontend/src/lib/components/CreateTaskDialog.svelte` currently resets all local form state whenever `open` becomes true and calls `onClose()` directly from Escape, backdrop click, the header close button, and Cancel.
- `gui/frontend/src/routes/+page.svelte` owns the `createOpen` state and opens the created task via `onCreated`.
- `gui/frontend/src/lib/components/TaskDrawer.svelte` already has a `window.confirm` dirty-close guard for unsaved body edits; reuse that behavior pattern or an equivalent confirmation UI.
- `board/done/TB-35.md` originally accepted Esc/click-outside dismissal, so this task intentionally tightens that behavior for dirty forms.

Constraints:
- Keep the change scoped to the GUI create-task dialog; do not alter `BoardService.CreateTask`, CLI task creation, board file formats, or generated board files.
- Treat the form as dirty when any user-editable field differs from the default create-task values: title, module, tags, type, priority, size, parent epic, epic toggle, or description.
- Empty/default forms may still close immediately.
- Successful submit behavior stays the same: create the task, show the success toast, call `onCreated`, then close/reset without an extra discard prompt.
- Draft persistence across dialog reopen or app restart is out of scope for this task; the required behavior is no silent data loss.

## Acceptance Criteria

- [ ] Closing a default/empty create-task dialog through Cancel, the header close button, Escape, or backdrop click closes immediately with no prompt.
- [ ] After any create-task field is changed from its default value, every close path (Cancel, header close, Escape, backdrop click, and any parent/global Esc close path) shows a discard confirmation before closing.
- [ ] If the user rejects the discard confirmation, the dialog stays open and all entered values remain intact.
- [ ] If the user confirms discard, the dialog closes; reopening the create dialog starts from the default empty form.
- [ ] Submitting a valid task still calls `createTask`, shows the existing success/error toast behavior, invokes `onCreated` on success, closes the dialog, and does not show the discard confirmation.
- [ ] Add frontend test coverage for dirty detection and at least one guarded close path; cover the default/empty close path so accidental prompts are not introduced.
- [ ] Manual test note: in `wails3 dev` or the GUI dev build, open `+ New`, type into title/tags/description, then try backdrop click, Escape, the header close button, and Cancel; verify dismissing the confirmation preserves the form and confirming it discards the form.

## Related Tasks

- **TB-35** — Original `CreateTaskDialog` implementation whose outside/Esc dismissal behavior is being tightened.
- **TB-84** — Global shortcut behavior; ensure Esc routes through the same dirty-form guard when the create dialog is open.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited type=bug, size=S, module=gui, tags=frontend,create-dialog,ux,quick-win, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success

