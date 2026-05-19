# TB-185: GUI: surface user-attention state and automation guard

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** user-attention,agent,automation,ux
**Branch:** —
**Parent:** TB-182

## Goal

Surface tasks that need user attention in the GUI and make manual, auto-groom, and auto-implement run paths treat `AgentStatus: needs-user` as an unresolved state that requires user action before retry.

## Context

- GUI task metadata flows through `gui/app/board_service.go`, `gui/internal/cli/mutations.go`, and `gui/frontend/src/lib/api.ts`.
- Cards and drawer UI live in `gui/frontend/src/lib/components/Card.svelte` and `gui/frontend/src/lib/components/TaskDrawer.svelte`.
- Run state and history already come from `runsStore`, task-local JSONL, and `AgentStatus` pills.
- Auto-groom queueing is scoped in TB-174; auto-implement queueing is scoped in TB-179.

## Constraints / Non-goals

- Display the attention request near the existing agent/status area; do not create a separate notification system for this task.
- Do not hide manual Run or Groom permanently. The UI should explain why a `needs-user` task will not run until the user resolves the request and clears/resets the status.
- Auto-groom and auto-implement must skip `needs-user` tasks without mutating task metadata or creating duplicate JSONL/log artifacts.
- Keep status rendering compact enough for cards and drawer headers; avoid raw backend errors in user-facing copy.

## Acceptance Criteria

- [x] Frontend and backend API types accept `AgentStatus: needs-user` without treating it as an unknown status.
- [x] Card and TaskDrawer surfaces show a clear user-attention badge/pill near the agent/status UI when a task has `needs-user`.
- [x] TaskDrawer renders the `## User Attention` content from the task body so the user can see the exact question/action and unblock condition.
- [x] Manual Run and Groom attempts on a `needs-user` task show an actionable explanation and do not start a run until the user resolves the request and clears/resets the status through the supported flow.
- [x] Auto-groom and auto-implement candidate selection skip `needs-user` tasks, record/emit enough diagnostic state for the UI, and do not create duplicate queued events, JSONL, or logs.
- [x] Tests cover status rendering, attention-section display, manual run/groom disabled or guarded state, and auto-groom/auto-implement skip behavior.
- [x] Manual test note: create a task, set `AgentStatus: needs-user` with a sample `## User Attention` request, confirm the card and drawer show the ask, confirm Run/Groom do not start, enable auto-groom/auto-implement and confirm the task is skipped without retry churn, then clear/reset the status and confirm normal controls work again.
- [x] Verification includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Related Tasks

- **TB-182** — Parent epic for the shared user-attention protocol.
- **TB-183** — Defines the CLI status and attention-section mutation path consumed here.
- **TB-184** — Documents the behavior and prompt expectations.
- **TB-174** — Auto-groom queueing must skip unresolved user-attention tasks.
- **TB-179** — Auto-implement queueing must skip unresolved user-attention tasks.
- **TB-175** — Adjacent auto-groom feedback UX.
- **TB-180** — Adjacent auto-implement feedback UX.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done

