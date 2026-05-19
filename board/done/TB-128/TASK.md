# TB-128: Keep ”Done” column sorted by timestamp, not priority

**Type:** improvement
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** gui,sorting
**ImplementedBy:** codex
**ImplementStatus:** success
**Branch:** —

## Goal

Make the GUI Done column show the most recently completed tasks first, regardless of task priority, while leaving Backlog and In Progress ordering unchanged.

## Context

`gui/app/board_service.go` builds the GUI board snapshot from `tb ls --json --status active` and preserves the order of tasks as it buckets them by status. `cli/list.go` currently sorts the whole result set by priority and then numeric ID, so older high-priority done tasks appear above newer lower-priority done tasks in the GUI Done column.

Use a completion-time sort key for tasks in `done/`: prefer the latest `## Log` entry that records the task entering Done (`Done` or `Moved to done`), and provide a deterministic fallback for older or malformed tasks that do not have a parseable completion entry. Keep archive visibility unchanged; archived tasks must not reappear in the Done column. This task is not a card-layout redesign and should not change Backlog or In Progress priority ordering.

## Acceptance Criteria

- [x] `tb ls --json --status done` returns done tasks in completion-time descending order before priority is considered; a newer P2 done task appears above an older P0 done task.
- [x] `tb ls --json --status active` preserves priority-first ordering for Backlog and In Progress while the `done` tasks in the result bucket sort by completion time descending.
- [x] `BoardService.LoadBoard`/`LoadBoardWithMode("active")` exposes `snapshot.done` in the same completion-time descending order; covered by `TestLoadBoard_PreservesDoneCompletionOrderFromCLI`.
- [x] Legacy or malformed done tasks without a parseable Done/Moved-to-done log entry use a deterministic fallback order; covered by `TestListDoneFallbackForTasksWithoutCompletionLog`.
- [x] Archive behavior is unchanged: archived tasks stay out of the default active board and do not appear in the GUI Done column.
- [ ] Manual test: not run in this headless session; CLI and backend tests cover the ordering observed by the GUI Done column.

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
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-19: Implementation complete — `tb ls` now sorts done slots by latest Done/Moved-to-done log date while preserving priority/ID order for active non-done columns and deterministic fallback for malformed legacy done tasks.
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=failed, implemented-by=codex, implement-status=failed
- 2026-05-19: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-19: Done

