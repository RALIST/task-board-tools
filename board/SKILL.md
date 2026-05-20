---
name: task-board
description: "Use when working with a markdown task board through the tb CLI: inspecting board state, creating or grooming tasks, moving work through kanban, capturing follow-ups, or keeping agent handoffs safe."
---

# Task Board Workflow

Compatible with Claude Code and Codex. This file is designed to be copied as a single `SKILL.md` into either agent's skill location.

Read `board/CONVENTIONS.md` before changing board state. It is the policy source for kanban flow, done evidence, archive semantics, task quality, WIP limits, and agent handoffs.

## Core Rules

- Use the `tb` CLI for structured board mutations.
- Directories are status: `backlog`, `ready`, `in-progress`, `code-review`, `done`, `archive`.
- Never copy task files between statuses. Move through the board tooling.
- `BOARD.md` is generated; never edit `BOARD.md` by hand.
- Do not invent task IDs. Use the board allocator.
- Link related tasks when work is split, blocked, or discovered from another task.
- Respect WIP limits; finish, review, or unblock existing work before adding more.

## Normal Workflow

1. Inspect the board before starting. Prefer the committed `ready` queue over raw backlog.
2. Pull from `ready` before coding. Do not move backlog directly to `in-progress` unless the user explicitly asks for a push-style override.
3. Keep the task log useful: record meaningful transitions, blockers, verification, review outcomes, and final evidence.
4. Submit implementation work to `code-review` when it is ready for inspection.
5. Move to `done` only after acceptance criteria are satisfied and evidence is recorded.
6. Use `archive` only to close obsolete, duplicate, superseded, or intentionally dropped tasks.

Every `done` task needs evidence. Implementation tasks should cite a commit or review artifact that includes `TB-NNN`. Spike tasks should link or attach the investigation result, decision record, notes file, or follow-up task list.

## Autonomous Stages

Treat autonomous work as three separate opt-in stages:

- `auto-groom`: backlog intake is groomed and may be promoted to `ready` only after it is no longer triage-reported.
- `auto-implement`: committed `ready` work is pulled into `in-progress`, implemented, and submitted to `code-review` with review target metadata.
- `auto-review`: `code-review` work is reviewed; pass moves to `done`, fail returns to `ready` with `review-failed`.

Do not auto-implement backlog tasks. Do not auto-review ready `review-failed` rework. Failed review handoff should clear retry-blocking generic `AgentStatus` while preserving review history.

For epic children, auto-implement must not pick a later numeric child while an earlier same-parent child is still active outside `done`; missing or unreadable earlier siblings block with a diagnostic. Daemon housekeeping is deterministic repair only: use objective board/run markers, managed board operations, and durable backoff for WIP-blocked repairs; never guess from prose or override `needs-user`, `cancelled`, `interrupted`, or `lost`.

## Backlog Capture

Create or update backlog tasks for real work outside the current scope:

- unrelated bugs
- deferred features or polish
- missing tests
- temporary workarounds or tech debt
- cleanup, performance, security, or dead-code concerns
- source comments that identify future work

Keep captures small: title, module if known, priority guess, context, and related-task links. Do not let follow-up work live only in chat.

## User Attention

Use `needs-user` when you cannot continue safely because requirements are unclear, instructions conflict, a manual/external blocker exists, verification needs a human decision, or the task is stale.

Add a `User Attention` section with:

- Reason: short category.
- Question/Action: the exact user decision or action needed.
- Attempted context: what you tried, read, or ruled out.
- Unblock condition: what answer or state lets work resume.

Then set `AgentStatus` to `needs-user` and stop cleanly. Do not mark the task done, failed, or cancelled just to end the run.

## Minimal Commands

- Inspect board: `tb ls --status ready`, `tb ls --status all`, `tb show TB-NNN`, `tb board --json`.
- Create/capture: `tb create "Title" -m module -d "context"`.
- Groom/commit intake: edit the task until it has a real goal and acceptance criteria, then `tb ready TB-NNN`.
- Start work: `tb pull` or `tb pull TB-NNN`.
- Move/review/finish: `tb review --submit TB-NNN`, `tb review --fail TB-NNN file|-`, `tb done TB-NNN`, `tb close TB-NNN`.
- Search/link: `tb grep "pattern"`, `tb epic TB-NNN`.
- Ask for user input: write the `User Attention` section, then set `--agent-status needs-user`.

Task IDs may be passed with or without the `TB-` prefix when the CLI accepts an ID.
