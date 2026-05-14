# TB-128: Keep ”Done” column sorted by timestamp, not priority

**Type:** improvement
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** gui,sorting
**Branch:** —

## Goal

Make the GUI Done column show the most recently completed tasks first, regardless of task priority, while leaving Backlog and In Progress ordering unchanged.

## Context

`gui/app/board_service.go` builds the GUI board snapshot from `tb ls --json --status active` and preserves the order of tasks as it buckets them by status. `cli/list.go` currently sorts the whole result set by priority and then numeric ID, so older high-priority done tasks appear above newer lower-priority done tasks in the GUI Done column.

Use a completion-time sort key for tasks in `done/`: prefer the latest `## Log` entry that records the task entering Done (`Done` or `Moved to done`), and provide a deterministic fallback for older or malformed tasks that do not have a parseable completion entry. Keep archive visibility unchanged; archived tasks must not reappear in the Done column. This task is not a card-layout redesign and should not change Backlog or In Progress priority ordering.

## Acceptance Criteria

- [ ] `tb ls --json --status done` returns done tasks in completion-time descending order before priority is considered; a newer P2 done task appears above an older P0 done task.
- [ ] `tb ls --json --status active` preserves priority-first ordering for Backlog and In Progress while the `done` tasks in the result bucket sort by completion time descending.
- [ ] `BoardService.LoadBoard`/`LoadBoardWithMode("active")` exposes `snapshot.done` in the same completion-time descending order; cover this with a Go backend test using mixed priorities and completion dates.
- [ ] Legacy or malformed done tasks without a parseable Done/Moved-to-done log entry use a documented deterministic fallback order, covered by a focused CLI test.
- [ ] Archive behavior is unchanged: archived tasks stay out of the default active board and do not appear in the GUI Done column.
- [ ] Manual test: complete or move two tasks into Done with different priorities, reload the GUI board, and confirm the later-completed task is the first card in Done.

## Related Tasks

- **TB-28** — collectAllTasks / findChildren: archive inclusion semantics (relationship: preserves the existing rule that Recently Done/Done views exclude archived tasks unless archive is requested explicitly)

## Attachments

## Log

- 2026-05-14: Created
- 2026-05-14: Edited agent=codex
- 2026-05-14: Edited agentstatus=queued
- 2026-05-14: Edited agentstatus=running
- 2026-05-14: Edited module=gui, tags=gui,sorting, goal
- 2026-05-14: Edited acceptance
- 2026-05-14: Edited agentstatus=success

