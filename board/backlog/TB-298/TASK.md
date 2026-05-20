# TB-298: Allow DnD into the Archive column

**Type:** improvement
**Priority:** P2
**Size:** S
**Agent:** codex
**AgentStatus:** success
**Module:** gui-frontend
**Tags:** ui,dnd,archive
**GroomedBy:** codex
**GroomStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**Branch:** —

## Goal

Let users archive a task by dragging its card into the Archive column when Show archived is enabled, using the existing `BoardService.CloseTask` / `tb close` path so archive semantics, folder-form task moves, log entries, generated board state, and active/all board views stay consistent.

### Context

- The Archive column is already loaded and rendered only when Show archived switches the board store to `all` mode.
- Current frontend drag targets stop at `backlog | ready | in-progress | code-review | done`; `Board.svelte` renders Archive with `draggable={false}` and no drop handler, and `Column.svelte` explicitly returns before handling drops into `archive`.
- `boardDrop.ts` already centralizes drop routing for `readyTask`, `pullTask`, `moveTask`, optimistic updates, revert, and toast handling.
- `BoardService.CloseTask` and the `closeTask` API wrapper already archive tasks through `tb close`; reuse that instead of adding direct file writes or a new archive mutation path.
- Related shipped work: TB-34 (optimistic DnD), TB-36 (drawer Archive button), TB-38 (Show archived / Archive column), TB-40 (archive bucket in board snapshot).

### Constraints / Non-goals

- Do not make Archive visible by default; it remains available only when Show archived is on.
- Do not introduce drag-out restore behavior from Archive to active columns in this task. Archived cards may remain non-draggable as a source unless the implementation deliberately scopes and tests restore semantics.
- Preserve the special DnD routes for backlog -> ready (`readyTask`) and ready -> in-progress (`pullTask`). Other active-column moves should keep using `moveTask`.
- Keep the existing failure model: revert the optimistic snapshot and surface a readable toast when the archive operation fails.

## Acceptance Criteria

- [ ] Archive is a valid drop destination when the Archive column is visible, with the same drag-over affordance as other columns.
- [ ] Dropping an active task into Archive routes through `closeTask` / `BoardService.CloseTask` / `tb close`, not direct file writes and not the generic `moveTask` active-column path.
- [ ] On successful archive drop, the optimistic board state moves the card into Archive in `all` mode; when Show archived is off or after active-mode refresh, the archived card is hidden from the active board.
- [ ] On archive-drop failure, the previous board snapshot is restored and the existing readable "Move failed" toast path is used.
- [ ] Existing DnD command routing still works: backlog -> ready calls `readyTask`, ready -> in-progress calls `pullTask`, and active-column moves other than Archive call `moveTask`.
- [ ] Frontend tests cover archive-drop success, archive-drop failure/revert, and the existing special-case DnD routes.
- [ ] Verification: `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- --run boardDrop` pass, or the task log records why a broader/narrower equivalent frontend test command was used.
- [ ] Manual test: run the GUI, enable Show archived, drag a non-archived task into Archive, verify the card appears in Archive, verify `tb ls --status archive` includes the task, then disable Show archived and verify it is hidden from the active board.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited type=improvement, size=S, module=gui-frontend, tags=ui,dnd,archive, title=Allow DnD into the Archive column
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success
