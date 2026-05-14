# TB-127: Open ticket full screen

**Type:** improvement
**Priority:** P1
**Size:** M
**Agent:** claude
**AgentStatus:** success
**Module:** gui
**Tags:** gui,frontend,drawer,ui
**Branch:** —

## Goal

Open the selected task in a full-window detail surface instead of the current narrow right drawer, with the task content taking the primary space and task metadata plus agent controls/logs in a secondary rail.

## Context

- Current frontend entry point: `gui/frontend/src/routes/+page.svelte` renders `<TaskDrawer taskId={$selectedTaskId} onClose={closeTask} />` over the board.
- Current detail UI: `gui/frontend/src/lib/components/TaskDrawer.svelte` uses a fixed right-side overlay capped at `width: min(720px, 96vw)` and stacks metadata editing, agent controls, attachments, body editing, and archive controls vertically.
- Current agent log UI: `gui/frontend/src/lib/components/AgentRunLog.svelte` already supports live log streaming and past-run log display; the redesign should relocate this surface, not replace the run/log contract.
- Feature docs that should keep working: `docs/FEATURES.md` F2.4 task drawer, F3.3 metadata editing, F3.4 body editing, F4.1-F4.5 agent assignment/run/log/history, and F6.1 groom flow.

### Constraints and Non-goals

- Keep the existing selected-task store and open/close behavior unless a smaller local rename makes the code clearer; Escape and close button must still close the detail surface.
- Preserve all current task actions: metadata save, body edit/save/discard, attachments add/remove/open, archive, assign agent, run, groom, cancel, run-history selection, and log display.
- Prefer frontend-only changes in `gui/frontend/src/lib/components/TaskDrawer.svelte` and adjacent tests/helpers; do not require a backend or CLI contract change for this layout task.
- The target is Jira-like information architecture, not a pixel clone: about two thirds primary task content and about one third metadata/actions/logs on desktop.
- On narrow windows, stack the secondary rail below or above the primary content so controls remain reachable without horizontal overflow.

## Acceptance Criteria

- [ ] Clicking a task opens a full-window task detail surface instead of the current half-screen/right-drawer layout; the board behind it is blocked or visually de-emphasized while the task is open.
- [ ] On desktop-width windows, the detail surface uses a stable two-column layout: roughly two thirds for title and main task content, and roughly one third for metadata, task actions, agent controls, run history, and run log.
- [ ] Main content keeps the task title, rendered task body, Goal/Context/Acceptance Criteria content, attachments, and body editor readable without crowding or unnecessary nested card styling.
- [ ] Secondary rail exposes status, priority, type, size, module, tags, parent, branch, agent assignment/status, Run, Groom, Cancel, run history, run log, Archive, and metadata save controls in predictable groups.
- [ ] Existing behaviors still work through the same frontend stores/API wrappers: metadata edit, body edit/save/discard, attachments add/remove/open, archive, agent assign/run/groom/cancel, run-history selection, and live/static run-log display.
- [ ] The layout is responsive: on narrow windows the columns stack cleanly, text and buttons do not overlap or overflow, and all controls remain reachable by scrolling.
- [ ] Existing close affordances still work: close button, Escape shortcut, and the current outside-click/backdrop behavior if the backdrop remains part of the design.
- [ ] Verification passes with `cd gui/frontend && npm run check` and `cd gui/frontend && npm test`.
- [ ] Manual test: run the GUI, open tasks with a long body, many tags, attachments, no agent, an assigned agent, and at least one completed or running agent run; confirm the full-screen layout, close behavior, edits, attachment actions, agent actions, and log viewing all work visually.

## Related Tasks

- **TB-24** - Frontend TaskDrawer: read-only markdown body (relationship: baseline surface)
- **TB-36** - TaskDrawer: inline metadata editing and Archive button (relationship: shares surface)
- **TB-37** - CodeMirror body editor in TaskDrawer (relationship: shares surface)
- **TB-50** - TaskDrawer agent dropdown + Run/Cancel buttons + Card agent badge (relationship: shares surface)
- **TB-51** - AgentRunLog.svelte: live streaming and past-run log rendering (relationship: shares surface)

## Attachments

## Log

- 2026-05-14: Created
- 2026-05-14: Edited body via GUI
- 2026-05-14: Edited agent=codex
- 2026-05-14: Edited agentstatus=queued
- 2026-05-14: Edited agentstatus=running
- 2026-05-14: Edited priority=P1, type=feature
- 2026-05-14: Edited module=gui, tags=gui,frontend,drawer,ui
- 2026-05-14: Edited goal
- 2026-05-14: Edited acceptance
- 2026-05-14: Edited acceptance
- 2026-05-14: Edited type=improvement
- 2026-05-14: Edited agentstatus=success
- 2026-05-14: Edited agent=claude
- 2026-05-14: Edited agentstatus=queued
- 2026-05-14: Edited agentstatus=running
- 2026-05-14: Reworked TaskDrawer into a centered full-window detail surface with a two-column desktop layout (main 2fr / rail 1fr): description + attachments on the left; status/priority/type/size/module/tags, agent assign + Run/Groom/Cancel, run history, run log, and Archive in the rail. Collapses to a single stacked column under 960px. Close button, Esc, and backdrop click preserved; metadata save, body edit, attachments, and agent flows unchanged. `npm run check` (0 errors) and `npm test` (68 tests) pass.
- 2026-05-14: Done
- 2026-05-14: Edited agentstatus=success

