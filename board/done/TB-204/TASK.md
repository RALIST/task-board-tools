# TB-204: Show epic progress

**Type:** improvement
**Priority:** P2
**Size:** M
**Agent:** claude
**AgentStatus:** running
**Module:** gui/frontend
**Tags:** ux,epic,frontend
**Branch:** —

## Goal

Show epic completion progress in the GUI wherever an epic task is summarized: on the kanban card and in the task drawer Details rail.

## Context

The CLI already calculates epic progress from child tasks whose `Parent` metadata matches the epic ID: `tb epic` prints `Progress: done/total`, and generated `board/BOARD.md` has an Epics progress column. The GUI board currently receives enough data in `BoardSnapshot` to compute the same value client-side because each task exposes `id`, `tags`, `parent`, and `status`.

Relevant implementation surfaces:
- `gui/frontend/src/lib/components/Card.svelte` renders kanban task cards and already detects epic cards via the `epic` tag.
- `gui/frontend/src/lib/components/TaskDrawer.svelte` renders task metadata in the Details rail.
- `gui/frontend/src/lib/components/Board.svelte` and `gui/frontend/src/routes/+page.svelte` pass the current filtered and unfiltered board snapshots into card/drawer surfaces.
- `gui/frontend/src/lib/filtering.ts` is the closest existing home for pure snapshot-derived helpers, including observed epic helpers.

Constraints / non-goals:
- Match the current CLI semantics: count children by `task.parent === epic.id`; count only `status === "done"` as complete.
- Use the active board snapshot by default. Archived children should only affect progress when the GUI is showing/loading archive data, consistent with existing archive-mode behavior.
- Do not change the markdown task format, parent/subtask mutation behavior, `tb epic`, or generated `BOARD.md` behavior.
- Do not show progress UI for non-epic tasks.

## Acceptance Criteria

- [x] A pure frontend helper computes epic progress from a `BoardSnapshot` as `{ done, total, percent }`, using child tasks whose `parent` matches the epic ID and treating only `done` children as complete.
- [x] `Card.svelte` shows epic progress on epic kanban cards as a compact `done/total` label with a stable visual progress indicator; `0/0` epics render cleanly without divide-by-zero, layout shift, or misleading completion styling.
- [x] `TaskDrawer.svelte` shows the same progress in the Details rail when the open task is tagged `epic`, and hides the progress row for non-epic tasks.
- [x] Progress updates from the same board data used by the visible board, including after `board:reloaded` or task status changes, without adding a new backend call per card.
- [x] Frontend coverage exercises partial progress, all-done progress, no-child epics, and non-epic tasks at the nearest practical level, such as the progress helper plus Card/TaskDrawer component tests.
- [ ] Manual test: run the GUI on a board with one epic and children in backlog, in-progress, and done; confirm the epic card and drawer show the same `done/total`, move a child to Done, and confirm both surfaces update after refresh. *(deferred — agent cannot run the desktop GUI; the user should verify in a `task dev` session.)*

## Related Tasks

- **TB-28** - Defines active/archive inclusion semantics for `tb epic`; this GUI display should match those semantics.
- **TB-187** - Quick add task to epic, a sibling epic UX task that creates the child relationships this progress display summarizes.
- **TB-188** - Quick jump to child ticket, a sibling epic navigation task touching the same task drawer experience.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited type=improvement, size=M, module=gui/frontend, tags=ux,epic,frontend, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Implemented epicProgress helper in filtering.ts, wired it into Card.svelte (compact `done/total` label + fixed-height progress bar, gated on `epic` tag) and TaskDrawer.svelte (Progress row in the Details rail). Both surfaces subscribe to the live `board` store so child status changes reflow without new backend calls. Added 5 helper unit tests + 4 Card component tests + 3 TaskDrawer component tests; full frontend suite (161 tests) + `npm run check` clean; CLI and GUI Go tests pass. Manual GUI test deferred — needs `task dev`.
- 2026-05-19: Done

