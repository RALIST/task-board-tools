# TB-193: TaskDrawer: edit parent epic from task page

**Type:** improvement
**Priority:** P2
**Size:** M
**Module:** gui/frontend
**Tags:** parent-task,ux,frontend
**Branch:** —
**Parent:** TB-186

## Goal

Let users change or clear a task's parent epic from the GUI task drawer without leaving the task page.

## Context

- `gui/frontend/src/lib/components/TaskDrawer.svelte` already owns inline metadata editing for priority, type, size, module, and tags.
- `gui/frontend/src/lib/components/CreateTaskDialog.svelte` already has a parent-epic select, and `gui/frontend/src/lib/filtering.ts` exposes observed epics from the loaded board snapshot.
- **TB-189** makes the existing Parent display navigable; this task adds editing and should coexist with that parent-jump affordance.
- **TB-192** should provide a typed `editTask` parent field backed by the CLI mutation from **TB-191**.

### Constraints

- Use the structured `editTask` API for parent changes; do not rewrite task body markdown from the frontend.
- Exclude the current task from the parent options and rely on the backend/CLI for final validation.
- Keep the drawer open after save and refresh through the existing watcher or task reload path.
- Do not change task status, archive visibility, or the CLI markdown format.

## Acceptance Criteria

- [ ] The task drawer shows the current parent epic as an editable control in metadata/details mode, with an explicit empty state for tasks that have no parent.
- [ ] The control lists available epic tasks from the current board snapshot, excludes the open task itself, and provides a clear-parent option that sends the backend sentinel `none`.
- [ ] Saving a parent change calls the existing typed `editTask` flow with only the parent diff when no other metadata changed, and can also save parent plus other dirty metadata in one user action when supported by the backend.
- [ ] On success, the drawer remains open, the task detail refreshes to the new parent value, and the board refresh shows the child removed from the old epic and listed under the new epic.
- [ ] Validation failures from the CLI/backend, such as missing parent or self-parent, are shown through the existing toast/error path and leave the form state recoverable.
- [ ] Add or update frontend coverage for selecting a new parent, clearing a parent, excluding the current task from options, and showing a validation failure.
- [ ] Verification passes with `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- --run`; if backend bindings changed during the same implementation slice, also run `cd gui && go test ./...`.
- [ ] Manual test: run the GUI on a board with two epics and one child task, open the child drawer, move it from the old epic to the new epic, verify both epic `## Subtasks` displays update after refresh, then clear the parent and verify the child has no Parent row and no stale subtask bullet remains on either epic.

## Related Tasks

- **TB-186** - Parent epic for changing a task's parent from the task page.
- **TB-191** - CLI mutation prerequisite.
- **TB-192** - GUI backend/API prerequisite.
- **TB-187** - Sibling epic-drawer workflow for quick child creation.
- **TB-188** - Sibling child-navigation workflow from an epic.
- **TB-189** - Sibling parent-navigation workflow from a child.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

