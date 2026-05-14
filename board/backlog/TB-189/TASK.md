# TB-189: Quick jump to parent task

**Type:** improvement
**Priority:** P2
**Size:** S
**Agent:** codex
**AgentStatus:** success
**Module:** gui/frontend
**Tags:** ui,quick-win
**Branch:** —

## Goal

Make the GUI task drawer render a task's `Parent` metadata as an in-app control that opens the parent/epic task in the same drawer.

## Context

- `gui/frontend/src/lib/components/TaskDrawer.svelte` currently shows `detail.metadata.parent` as plain text in the Details rail.
- Task selection is already centralized through `gui/frontend/src/lib/stores/selection.ts` (`openTask(id)`), and `gui/frontend/src/routes/+page.svelte` renders `<TaskDrawer taskId={$selectedTaskId} ...>`.
- Backend task metadata already exposes `parent`; keep this frontend-focused unless implementation discovers the existing data is insufficient.
- Constraints / non-goals: do not add a new route, external browser navigation, or a new backend command just to jump between tasks; keep archive/status semantics unchanged.

## Acceptance Criteria

- [ ] In the task drawer Details rail, tasks with `metadata.parent` render `Parent` as an accessible in-app button/link instead of inert plain text.
- [ ] Activating the parent control opens that parent/epic task in the same drawer using the existing task-selection flow; the drawer stays open and refreshes from the child to the parent task.
- [ ] Tasks without parent metadata keep the current Details layout and do not show an empty Parent row or disabled control.
- [ ] Add or update frontend coverage for the parent-jump behavior at the nearest practical level, such as a TaskDrawer component test or selection-flow test.
- [ ] Manual test: run the GUI on a board with a child task that has `**Parent:** <ID>`, open the child, click the Parent control, and verify the parent/epic task opens in the drawer without a full app reload.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited type=improvement, size=S, module=gui/frontend, tags=ui,quick-win, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success

