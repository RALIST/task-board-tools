# TB-187: Quick add task to epic

**Type:** improvement
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui/frontend
**Tags:** ux,frontend
**Branch:** —

## Goal

Add an epic-scoped create affordance in the GUI task drawer so a user viewing an epic can create a new child task with that epic preselected as the parent.

## Context

- `gui/frontend/src/lib/components/TaskDrawer.svelte` renders task details and body; an epic can be detected from `detail.metadata.tags` containing `epic`.
- `gui/frontend/src/lib/components/CreateTaskDialog.svelte` already has a Parent epic select and submits `parent` through `$lib/api.createTask`.
- `gui/frontend/src/routes/+page.svelte` owns the global create dialog, `observedEpics($board)`, and the existing `onCreated={(id) => openTask(id)}` flow that opens a newly created task.
- `gui/app/board_service.go` and `gui/internal/cli/mutations.go` already pass `CreateTaskInput.Parent` to `tb create --parent`; the CLI then writes the child `Parent` field and updates the parent's `## Subtasks` section.

### Constraints / Non-Goals

- Do not create child tasks by editing markdown directly; creation must go through the existing `createTask` / `BoardService.CreateTask` / `tb create --parent` path.
- Show the quick-add affordance only when the selected task is an epic; non-epic task drawers should not show child-task creation controls.
- Keep the topbar `+ New` flow unparented by default unless the user explicitly chooses a parent.
- Keep board status semantics unchanged; the new child should use the CLI's normal create destination and then open through the existing task-selection flow.
- Do not implement parent reassignment or child/parent navigation in this task; those are tracked separately.

## Acceptance Criteria

- [ ] In the GUI task drawer, epic tasks show a clear quick-add child control near the task body/Subtasks area or other existing drawer actions; non-epic tasks do not show that control.
- [ ] Activating the control opens the existing create task dialog with the current epic preselected in the Parent epic field while leaving the user able to edit title, module, type, priority, size, tags, and description before submit.
- [ ] Submitting the dialog creates the task through the existing `createTask` API with `parent` set to the current epic ID; the created task has `**Parent:** <epic-id>` and the parent epic's generated `## Subtasks` section gains the new child entry after the board reloads.
- [ ] After a successful quick-add create, the new child task opens in the drawer using the existing selection flow; Cancel, Escape, and failed creates leave the current epic open and do not create a task.
- [ ] The topbar `+ New` flow still opens with no parent preselected by default and retains the current create-task behavior.
- [ ] Add or update frontend coverage for the preselected-parent create path and the non-epic/no-default-parent behavior at the nearest practical level.
- [ ] Verification passes with `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- --run`; if Go bindings or backend services change, also run `cd gui && go test ./...`.
- [ ] Manual test: run the GUI on a board with an epic, open the epic drawer, click the quick-add child control, confirm the Parent epic field is prefilled with that epic, create a child, confirm the child drawer opens, then reopen the epic and verify the child appears under `## Subtasks`. Also open topbar `+ New` and confirm Parent epic is blank.

## Related Tasks

- **TB-186** - Change parent task (sibling parent-assignment workflow; out of scope here).
- **TB-188** - Quick jump to child ticket (sibling epic-drawer workflow).
- **TB-189** - Quick jump to parent task (sibling reverse-navigation workflow).

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited type=improvement, size=M, module=gui/frontend, tags=ux,frontend, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success

