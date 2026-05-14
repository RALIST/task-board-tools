# TB-188: Quick jump to child ticket

**Type:** improvement
**Priority:** P2
**Size:** S
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** ux,frontend,quick-win
**Branch:** —

## Goal

Make child task references in an epic's displayed task body clickable so selecting a child from the epic drawer opens that child task in the GUI drawer without changing board status or rewriting markdown.

## Context

- Epic child entries are currently written into the parent task body by `tb create --parent` as `## Subtasks` bullets like `- **TB-123** (S) — Child title`.
- The GUI drawer renders task markdown in `gui/frontend/src/lib/components/TaskDrawer.svelte` using `marked` and `DOMPurify`; task selection already flows through `openTask` / `selectedTaskId` in `gui/frontend/src/lib/stores/selection.ts`.
- `BoardService.GetTask` and `tb show --json` already expose the raw task body and metadata, so this should stay frontend-only unless implementation proves a typed child list is necessary.
- This is sibling UX to **TB-187** (quick add child from an epic) and **TB-189** (quick jump to parent task).

### Constraints

- Do not change the CLI markdown format, task parent/child relationships, or task status semantics for this task.
- Preserve the existing body editor behavior: linkification is display-only and must not mutate `detail.body`, the body editor draft, or the underlying markdown file.
- Keep markdown rendering safe; do not bypass the existing sanitization or introduce executable task-body HTML/JS.
- Missing or otherwise unresolved child IDs must not crash the drawer; the original text should remain readable and any failed open should use the existing error/toast path.

## Acceptance Criteria

- [ ] In the GUI task drawer, an epic body with generated `## Subtasks` entries such as `- **TB-123** (S) — Child title` renders each resolvable child task reference with a clear clickable affordance while preserving the surrounding markdown layout.
- [ ] Clicking a child reference opens that child task through the existing drawer selection flow; kanban columns, task statuses, and the browser/window route do not change.
- [ ] Keyboard users can tab to the child reference and activate it with Enter or Space; the accessible name includes the child task ID and enough title text to distinguish it.
- [ ] Missing or unresolved child IDs do not crash the drawer; the original child text remains visible and a failed open is surfaced through the existing error/toast path.
- [ ] Display-mode linkification does not alter `detail.body`, the body editor contents, or the markdown saved on disk.
- [ ] Add or update frontend coverage for detecting generated `## Subtasks` child entries and for click/key activation opening the expected task ID.
- [ ] Verification passes with `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- --run`; if Go bindings or services change, also run `cd gui && go test ./...`.
- [ ] Manual test: create or open an epic with at least one child, click the child from the epic drawer, confirm the child drawer opens, then reopen the epic and verify the displayed markdown was not rewritten.

## Related Tasks

- **TB-187** - Quick add task to epic (sibling epic-drawer workflow).
- **TB-189** - Quick jump to parent task (sibling reverse-navigation workflow).

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited type=improvement, size=S
- 2026-05-15: Edited module=gui, tags=ux,frontend,quick-win, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Groomed - clarified GUI child-link behavior, constraints, tests, manual test, and related tasks.
- 2026-05-15: Edited agentstatus=success

