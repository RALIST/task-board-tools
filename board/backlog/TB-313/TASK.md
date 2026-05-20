# TB-313: GUI: virtualize large kanban columns

**Type:** improvement
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** gui,performance,follow-up
**Branch:** —

## Goal

The TB-312 batch render cap prevents thousands of cards from mounting at once, but very large boards should eventually use true virtualized/lazy column rendering so users can browse large backlog/done/archive columns without manual paging or high renderer CPU.

## Acceptance Criteria

- [ ] Large columns use virtualized or lazy rendering without mounting every card.
- [ ] Keyboard selection and task opening still work for tasks outside the initial viewport.
- [ ] Drag/drop behavior remains correct for visible cards and disabled/guarded for unsupported virtualized cases.
- [ ] Add a regression/performance check using a board with thousands of done/archive tasks.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited acceptance

