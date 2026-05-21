# TB-313: GUI: virtualize large kanban columns

**Type:** improvement
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** gui,performance,large-board,virtualization,follow-up
**Agent:** codex
**GroomedBy:** codex
**GroomStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** main
**ReviewedBy:** codex
**ReviewStatus:** success
**AgentStatus:** success
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

## User Attention

No active user attention.

Prior TB-318 verification blocker is resolved in current main: `npm run check` and full Vitest no longer fail on missing board-switch exports.

Manual desktop smoke against Writer Studio was not run in this pass because prior TB-312 smoke showed opening Writer Studio can launch real autonomous work, and an existing local `tb-gui`/`wails3 dev` instance was already running during continuation. Automated large-column regression coverage now uses 3000-task done/archive fixtures and scroll/order/open checks.

## Review Target

branch: main
review ref: main
code commit: 49063fa

Summary:
- Completed the missing TB-313 manual desktop smoke with a clean HEAD Wails dev app copy and isolated synthetic large board.
- Verified large done/archive scrolling, top and far-down drawer opening, bounded visible card rendering, badge preservation, DnD guard visibility, and idle CPU settling.
- Reran frontend gates; no source changes in this rework pass.

Scope note:
- Core virtualization remains the existing TB-313 implementation in `Column.svelte` / `columnVisibility.ts`.
- This pass updates review evidence only; no backend board loading, CLI format, watcher behavior, or board-switch UX changes.
- Writer Studio was not opened because prior TB-312 smoke showed it can launch real autonomous work; the synthetic board avoids that side effect while matching the large done/archive shape.

## Reviewer Notes

Verification:
- `cd gui/frontend && npm run check` — 0 errors, 0 warnings.
- `cd gui/frontend && npm test -- --run` — 28 files passed, 304 tests passed.
- `cd gui/frontend && npm run lint` — passed.

Manual desktop smoke:
- Ran Wails dev from a clean `git archive HEAD` copy under `/tmp/tb313-smoke/app`, with isolated `XDG_CONFIG_HOME=/tmp/tb313-smoke/xdg`, unique smoke single-instance id, automation prefs off, and PATH pointed at `/Users/ralist/go/bin/tb`.
- Used a synthetic large board at `/tmp/tb313-smoke/large-board` with 60 backlog, 1 ready, 1 in-progress, 1 code-review, 3000 done, and 3000 archive tasks; no queued/running agent tasks.
- Startup attached watcher to `/tmp/tb313-smoke/large-board/board`; daemon startup scan completed with `enqueued=0`.
- Done column rendered count `3000` with bounded visible cards, preserved badges (`done-badge` on TB-1001), showed `Drag disabled while this large column uses lazy rendering.`, opened top task TB-1001, scrolled far down to TB-3751..TB-3772 range, and opened TB-3763.
- Show archived loaded archive count `3000`; archive column rendered bounded visible cards, preserved badge on TB-4001, scrolled far down to TB-6774..TB-6795 range, and opened TB-6784.
- After scrolling/opening, idle process sample settled at `tb-gui` 0.0% CPU / 64 MB RSS, Vite 0.0% CPU / 52 MB RSS, and Wails wrapper 0.0% CPU / 7 MB RSS.
- Stopped only the temp smoke app/processes after the run.

Scope note:
- Writer Studio was intentionally not opened because TB-312 proved that board can launch real autonomous work on open. The smoke board reproduced the large completed-history shape safely without agent side effects.
- Existing unrelated TB-323/TB-325 working-tree WIP remains dirty and is not part of this TB-313 rework.

## Review Findings

- No blocking findings.
- Reviewed top-level ReviewRef `main` at clean HEAD `d08419b`; relevant virtualization code and tests cover bounded done/archive rendering, order/badges, keyboard open after scroll, backlog non-virtualized guard, and DnD guard behavior.
- Fresh verification in clean `/tmp/tb313-review.LNGixf` copy: `npm run check` found 0 errors/0 warnings, `npm test -- --run` passed 28 files/299 tests, and `npm run lint` exited 0.

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
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-21: Moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited user-attention
- 2026-05-21: Edited agentstatus=needs-user
- 2026-05-21: Moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=none
- 2026-05-21: Moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited user-attention
- 2026-05-21: Edited review-target
- 2026-05-21: Edited reviewer-notes
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewref=49063fa
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Edited agentstatus=none, reviewed-by=codex, review-status=success
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited reviewer-notes
- 2026-05-21: Edited review-target
- 2026-05-21: Edited review-findings
- 2026-05-21: Edited agentstatus=success, reviewref=main
- 2026-05-21: Submitted to code-review
- 2026-05-21: Failed code review — moved to ready with review-failed marker
- 2026-05-21: Cleared review-failed marker on resubmit
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Passed code review
- 2026-05-21: Edited agentstatus=success, reviewed-by=codex, review-status=success

