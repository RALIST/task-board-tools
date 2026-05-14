# TB-197: GUI: show code-review column and review fields

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb194,code-review,frontend,ux
**Branch:** —
**Parent:** TB-194

## Goal

Render the new Code Review lane in the desktop app and expose review metadata clearly on cards and in the task drawer.

## Context

- Parent epic: TB-194.
- Backend board bucketing currently lives in `gui/app/board_service.go` (`BoardSnapshot` has backlog, inProgress, done, archive).
- Frontend board state currently lives in `gui/frontend/src/lib/stores/board.ts`, with drag targets restricted to backlog/in-progress/done.
- The visible kanban UI lives under `gui/frontend/src/lib/components/` and `gui/frontend/src/routes/+page.svelte`.

## Constraints / Non-goals

- The Code Review column should be an active column shown by default between In Progress and Done.
- Drag/drop and move APIs should allow moving tasks into and out of `code-review` using the same optimistic/revert behavior as other active columns.
- Cards in Code Review should surface enough review context to scan: review target present/missing, reviewer notes present, findings present, and `review-failed` when applicable.
- The drawer should render `## Review Target`, `## Reviewer Notes`, and `## Review Findings` as first-class task body sections; editing those sections from the GUI can stay body-editor based unless TB-196 exposes typed bindings in time.
- Do not design the review-agent run button here; TB-198 owns review mode.

## Related Tasks

- **TB-194** - Parent epic.
- **TB-195** - CLI/status data plane consumed by the GUI.
- **TB-196** - Review sections rendered by the GUI.
- **TB-198** - Adds review-mode run affordances.
- **TB-199** - Defines `review-failed` visual treatment.

## Acceptance Criteria

- [ ] `BoardSnapshot` and Wails bindings include a `codeReview` bucket populated from tasks with `status == "code-review"`.
- [ ] The frontend renders columns in order: Backlog, In Progress, Code Review, Done, with Archive still controlled by the existing archive toggle.
- [ ] Dragging a card to Code Review calls the existing move path with `code-review`; failure reverts the optimistic move and shows the existing error feedback.
- [ ] `task:updated:<id>` and `board:reloaded` refresh paths correctly add, remove, and patch Code Review cards.
- [ ] Cards visually distinguish missing review target, present review target, present findings, and `review-failed` without overflowing compact cards.
- [ ] TaskDrawer read mode clearly renders Review Target, Reviewer Notes, and Review Findings when present; edit mode preserves those sections.
- [ ] Vitest or component tests cover board bucketing, optimistic move to/from Code Review, and card/drawer review indicators.
- [ ] Manual test note: start the GUI, move a sample task In Progress -> Code Review by drag/drop and by CLI, open the drawer, confirm review sections/markers render, then move it Code Review -> Done and Code Review -> Backlog.
- [ ] Verification includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

