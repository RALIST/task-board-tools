# TB-325: GUI: task edits refresh board cards live

**Type:** bug
**Priority:** P1
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** gui,frontend,watcher,live-updates,ux
**GroomedBy:** codex
**GroomStatus:** success
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** ee9e9a1
**ReviewedBy:** codex
**ReviewStatus:** success
**Branch:** —

## Goal

When a task changes outside the current card render path, the open GUI should update that task's kanban card from the live board data within about one second, so card-visible fields such as title, tags, priority, assignment, user-attention state, and mode status never stay stale until manual refresh.

UPDATE: agents statuses also do not updated properly in real time: inside task - failed, on card - running. Need general solution for informing frontend about any changes in the task

## Context

- `docs/FEATURES.md` F2.5 defines the live-update contract: watcher emits `board:reloaded` and `task:updated:<id>`, and frontend stores patch reactively.
- `gui/internal/watcher/watcher.go` emits `task:updated:<id>` for task markdown rewrites and `board:reloaded` for create/remove/rename membership changes.
- `gui/frontend/src/lib/stores/board.ts` already has `patchTask(id)`, which refetches one task and splices it into the current board snapshot.
- `gui/frontend/src/routes/+page.svelte` owns board-level event subscriptions; verify task-specific update events reach `patchTask` for cards, not only the drawer.
- `gui/frontend/src/lib/components/Card.svelte` renders the stale surfaces reported here: title, type, priority, module, size, tags, agent/user-attention, review-failed, resume, and per-action status chips.
- `gui/frontend/src/lib/components/TaskDrawer.svelte` already listens for `task:updated:<id>` and `board:reloaded` for detail refresh; use it as a comparison point.

## Review Target

branch: main
commit: ee9e9a1

summary:
- frontend subscribes to exact `task:updated:<id>` events for visible board tasks and calls the board-store task patch path.
- board store replaces stale card metadata in-place, preserves counters, and ignores stale async patch results after newer patches, refreshes, or board switches.
- Column card cache now refreshes when same-ID task objects change.
- watcher emits task-specific updates for file-form and folder-form atomic task markdown rewrites; create/move/remove/attachment membership changes still reconcile through `board:reloaded`.

manual smoke:
- ran `cd gui && task dev`, opened populated board, and edited visible TB-325 from terminal.
- `tb edit TB-325 --title "GUI: LIVE card smoke title"` updated visible card title without refresh.
- `tb edit TB-325 -t smoke-live,gui,frontend,watcher,live-updates,ux` updated visible tags without refresh.
- `tb edit TB-325 --agent-status needs-user` updated visible agent/user-attention status without refresh.
- restored title, tags, and agent status after smoke.

verification:
- `cd gui/frontend && npm run check` passed.
- `cd gui/frontend && npm run lint` passed.
- `cd gui/frontend && npm run deadcode` passed.
- `cd gui/frontend && npm test -- --run` passed: 28 files, 296 tests.
- `cd cli && go test ./...` passed.
- `cd gui && go test ./app/... ./internal/watcher/... -count=1` passed.
- code review pass: no CRITICAL/MAJOR findings after final review.

known unrelated failures/follow-ups:
- `cd gui && go test ./...` still fails in `internal/agent` because PromptGroom is missing `{{TASK_TITLE}}` and `{{TASK_BODY}}`; tracked by TB-338.
- `make lint-go` fails in CLI `cli/init.go:392` on `errorlint`; tracked by TB-339.

## Review Findings

- No blocking findings.

## Related Tasks

- **TB-105** — prior watcher coalescing and folder-task event coverage.
- **TB-303 / TB-309** — sibling AgentStatus removal and column-specific card status work; this bug must not reintroduce generic `AgentStatus` as card truth.

## Constraints

- Do not add polling, per-card CLI calls during render, or a manual refresh requirement; use the existing watcher/Wails event stream and board store update path.
- Prefer task-specific patching for task-file edits. Keep full board refreshes for membership changes such as moves, create/remove, archive, and folder promotion.
- Preserve markdown as the source of truth, existing `.board.lock` / atomic-write assumptions, file-form and folder-form task support, filters, WIP counts, DnD behavior, and drawer autosave behavior.
- If TB-303/TB-309 lands first, use the replacement per-mode/needs-user state source and do not reintroduce generic `AgentStatus` as card truth.
- Keep UI scope narrow: card data freshness and the event path that feeds it, not a visual redesign.

## Acceptance Criteria

- [ ] With the GUI open on a board, running `tb edit <ID> --title ...`, `tb edit <ID> -t ...`, and `tb edit <ID> --agent-status needs-user` updates the visible card for that task within about one second without manual refresh.
- [ ] The same live card refresh works for both file-form tasks (`<status>/<ID>.md`) and folder-form tasks (`<status>/<ID>/TASK.md`) when their task markdown is rewritten.
- [ ] Card-visible fields refresh from the current task snapshot: title, type, priority, module, size, tags, agent/needs-user or replacement mode state, review-failed marker, resume indicator, and per-action status chips.
- [ ] Column moves, archive/restore visibility, create/remove, and attachment/folder membership changes still reconcile through `board:reloaded`; filters, WIP counts, DnD, and drawer refresh behavior do not regress.
- [ ] Automated coverage proves the board/card event path invokes the task patch path for `task:updated:<ID>` or an equivalent task-specific event, and proves patched task data replaces the stale card data in the board store.
- [ ] Manual test note: run `cd gui && task dev`, open a populated board, mutate a visible card from a terminal with the commands above, and record that the card updates without using browser/app refresh.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited agent=codex
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited priority=P1, type=bug, size=M, module=gui, tags=gui,frontend,watcher,live-updates,ux, title=GUI: task edits refresh board cards live
- 2026-05-21: Edited goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited context
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited title=GUI: live card smoke title
- 2026-05-21: Edited tags=gui,frontend,watcher,live-updates,ux,smoke-live
- 2026-05-21: Edited agentstatus=needs-user
- 2026-05-21: Edited tags=gui,frontend,watcher,live-updates,ux, agentstatus=running, title=GUI: task edits refresh board cards live
- 2026-05-21: Edited title=GUI: LIVE card smoke title
- 2026-05-21: Edited tags=gui,frontend,watcher,live-updates,ux,smoke-live
- 2026-05-21: Edited agentstatus=needs-user
- 2026-05-21: Edited tags=gui,frontend,watcher,live-updates,ux, agentstatus=running, title=GUI: task edits refresh board cards live
- 2026-05-21: Edited title=GUI: LIVE card smoke title
- 2026-05-21: Edited title=GUI: task edits refresh board cards live
- 2026-05-21: Edited title=GUI: LIVE card smoke title
- 2026-05-21: Edited tags=gui,frontend,watcher,live-updates,ux,smoke-live
- 2026-05-21: Edited agentstatus=needs-user
- 2026-05-21: Edited tags=gui,frontend,watcher,live-updates,ux, agentstatus=running, title=GUI: task edits refresh board cards live
- 2026-05-21: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-21: Edited body via GUI
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited title=GUI: LIVE card smoke title
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited tags=gui,frontend,watcher,live-updates,ux, agentstatus=running, title=GUI: task edits refresh board cards live
- 2026-05-21: Edited title=GUI: LIVE card smoke title
- 2026-05-21: Edited tags=smoke-live,gui,frontend,watcher,live-updates,ux
- 2026-05-21: Edited agentstatus=needs-user
- 2026-05-21: Edited tags=gui,frontend,watcher,live-updates,ux, agentstatus=running, title=GUI: task edits refresh board cards live
- 2026-05-21: Edited review-target
- 2026-05-21: Edited reviewref=ee9e9a1
- 2026-05-21: Submitted to code-review
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Passed code review
- 2026-05-21: Edited agentstatus=success, reviewed-by=codex, review-status=success

