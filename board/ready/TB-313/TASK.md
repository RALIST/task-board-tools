# TB-313: GUI: virtualize large kanban columns

**Type:** improvement
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** gui,performance,large-board,virtualization,follow-up
**Agent:** codex
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Replace the TB-312 done/archive batch cap with true column virtualization or equivalent lazy viewport rendering so large GUI boards can browse thousands of tasks without mounting every card or requiring manual paging.

## Context

- TB-312 fixed board switching and added a temporary render cap for very large completed-history columns; its review notes say the cap is intentionally limited to `done` / `archive` so active workflow columns stay fully visible.
- Current frontend surface is `gui/frontend/src/lib/components/Column.svelte`: it slices `visibleTasks`, renders `Show more`, and feeds `svelte-dnd-action` with a stable `items` array during drag.
- Batch-size helpers and tests live in `gui/frontend/src/lib/columnVisibility.ts` and `gui/frontend/src/lib/columnVisibility.test.ts`.
- Board-level rendering passes all tasks through `gui/frontend/src/lib/components/Board.svelte`; selection and global Enter-to-open behavior are wired from `gui/frontend/src/routes/+page.svelte` and task card focus via `gui/frontend/src/lib/components/Card.svelte`.
- Large-board repro source from TB-312: Writer Studio had about 2,871 active tasks / 2,313 done tasks during smoke, enough to expose mount and CPU costs.

## Constraints

- Keep markdown board format, `BoardSnapshot` shape, CLI commands, watcher behavior, and backend board loading unchanged unless a failing test proves a narrow frontend contract needs adjustment.
- Reuse the existing board/filter/sort/card/drawer paths; do not introduce a separate large-board view or manual paging replacement.
- Virtualization must preserve visible task order, filters, WIP counts, card badges, focusability, and opening a task drawer by ID.
- Treat drag/drop carefully: only keep DnD enabled for virtualized columns when the implementation proves visible-card moves are correct; otherwise guard or disable unsupported virtualized DnD with clear UI feedback and tests.
- Avoid mounting hidden cards for measurement. Performance check should assert rendered card count stays bounded for thousands of tasks.

## Acceptance Criteria

- [ ] Large `done` and `archive` columns render through virtualization or lazy viewport rendering, with DOM card count bounded near the viewport/buffer size instead of total task count.
- [ ] Large `backlog` columns are either virtualized with correct selection/open behavior or explicitly guarded with a documented reason and no regression from the TB-312 completed-column cap.
- [ ] Filtering, sorting, WIP counts, card badges, and task drawer opening still use the same task IDs and visible order as the unvirtualized board snapshot.
- [ ] Keyboard opening works for virtualized cards that become visible after scrolling; focus/Enter opens the intended task and does not depend on cards outside the rendered range being mounted.
- [ ] Drag/drop remains correct for visible cards in supported virtualized columns; unsupported virtualized drag/drop states are disabled or guarded so no silent wrong-column move can occur.
- [ ] Regression/performance coverage builds a board snapshot or fixture with thousands of `done` / `archive` tasks and proves the rendered card count stays bounded while scrolling/opening still works.
- [ ] Frontend verification passes: `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run`, and `cd gui/frontend && npm run lint`.
- [ ] Manual test note: run the desktop GUI against a large board such as Writer Studio, scroll large `done` / `archive` columns, open tasks near the top and far down the list, confirm renderer CPU settles after scrolling, and confirm any guarded DnD case is visibly disabled.

## Related Tasks

- **TB-312** — introduced the temporary done/archive batch render cap and recorded the large-board smoke context this task replaces.
- **TB-318** — sibling board-switch loading UX task; keep this task focused on large-column rendering, not switch feedback.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited acceptance
- 2026-05-21: Edited agent=codex
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited priority=P2, type=improvement, size=M, module=gui, tags=gui,performance,large-board,virtualization,follow-up, goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Committed — moved to ready

