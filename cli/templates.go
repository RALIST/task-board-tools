package main

import "fmt"

// conventionsTemplate returns a generic CONVENTIONS.md for the configured board.
func conventionsTemplate(prefix, boardPath string) string {
	return fmt.Sprintf(`# Board Conventions

This file describes how to work with this board as a kanban system. It is intentionally a policy guide, not a command manual. Detailed command syntax belongs in CLI help and in the board skill file.

This board root is configured in `+"`"+`.tb.yaml`+"`"+` as `+"`%[2]s`"+`. Paths in this guide are relative to the project root unless stated otherwise. For this board, generated views such as `+"`%[2]s/BOARD.md`"+` live under that configured root. Another project may use a different board path.

## Kanban Flow

The board flows in one direction:

`+"```"+`
backlog → ready → in-progress → code-review → done → archive
`+"```"+`

Each column has a job:

- `+"`backlog`"+` is intake. Ideas can be rough here, but they should not be treated as committed work.
- `+"`ready`"+` is the commitment point. A task is ready only when it has a priority, a clear goal, and enough acceptance criteria for someone else to finish it.
- `+"`in-progress`"+` is active work. Keep it small; do not hoard tasks.
- `+"`code-review`"+` is work that claims to be done and needs reviewer signoff.
- `+"`done`"+` is accepted work. The task should explain what changed and how it was verified.
- `+"`archive`"+` is for obsolete, superseded, duplicate, or long-closed work that should leave the active board.

Tasks flow forward. A failed review returns to `+"`ready`"+` with a clear rework note because the task is still groomed; it just needs another implementation pass.

## Source Of Truth

Directories are the source of truth. A task entry exists in one status only, and its status is the directory it lives in. Moving work by copying files breaks the board; move the task entry instead.

`+"`BOARD.md`"+` is a generated board view. Do not edit it by hand. If it disagrees with task entries, trust the task entries and regenerate the view through the board tooling.

Use the managed board tools for structured changes such as creating, moving, editing metadata, assigning agents, managing attachments, closing tasks, and rebuilding generated views. Direct file edits are acceptable for human-readable task body improvements, but preserve the metadata block and the one-task-one-status rule.

Task IDs use the `+"`%[1]s-NNN`"+` shape. The numeric allocator is owned by the board tooling; do not invent IDs manually.

## Task Quality

A good task is small enough to finish, specific enough to review, and explicit about success. Before a task leaves `+"`backlog`"+`, it should have:

- A concise title.
- A type: `+"`feature`"+`, `+"`bug`"+`, `+"`tech-debt`"+`, `+"`improvement`"+`, or `+"`spike`"+`.
- A priority: `+"`P0`"+` for urgent work, `+"`P1`"+` for next-up work, `+"`P2`"+` for normal backlog.
- A size: `+"`S`"+`, `+"`M`"+`, `+"`L`"+`, or `+"`XL`"+`; split `+"`XL`"+` tasks before implementation when possible.
- A real goal that describes the outcome, not only the activity.
- Acceptance criteria that can be checked by a reviewer.
- Relevant module, tags, parent epic, and related-task links when they help routing.

Use `+"`spike`"+` for research whose output is a decision, summary, or follow-up task list. Do not let spikes quietly become implementation tasks without updating their goal and acceptance criteria.

## Working Agreements

Before starting work, pull from `+"`ready`"+` unless the user explicitly chooses a specific task. If `+"`ready`"+` is empty, groom intake first instead of treating raw backlog as committed work.

Respect WIP limits when they are configured. A WIP warning is a signal to finish, review, or unblock existing work before adding more.

Set or update the branch/reference fields when they help reviewers find the implementation. Work submitted to `+"`code-review`"+` should include enough review reference information to inspect the actual change.

Keep the `+"`Log`"+` useful. Record meaningful transitions, blockers, verification results, review outcomes, and final summaries. Avoid noisy diary entries that do not help the next reader.

Check acceptance criteria before marking work done. If a criterion no longer applies, edit it or explain why it changed rather than silently ignoring it.

## Backlog Capture

Create a new backlog task when you find work that is real but outside the current scope:

- Bugs unrelated to the task in hand.
- Follow-up improvements or polish.
- Temporary workarounds.
- Missing tests or coverage gaps.
- Dead code, cleanup, performance, or security concerns.
- Source comments that identify future work.

Keep capture lightweight: title, module if known, priority guess, and enough context for someone to understand why the task exists. Link it from the current task when the relationship matters.

## Related Tasks

Use `+"`Related Tasks`"+` to preserve context across split work. Good relationship labels include `+"`prerequisite`"+`, `+"`blocked by`"+`, `+"`shares infrastructure`"+`, `+"`complementary`"+`, and `+"`depends on`"+`.

When decomposing an epic, connect children to the parent and keep sibling ordering meaningful. If one child must happen before another, make that dependency explicit instead of relying on memory.

## Review Loop

`+"`code-review`"+` is a claim that implementation is complete enough to inspect. A review should focus on behavior, regressions, missing tests, data loss, security, and contract drift.

If review passes, use `+"`tb review --pass`"+` to record findings and move the task to `+"`done`"+`. If review fails, use `+"`tb review --fail`"+` to return it to `+"`ready`"+`, preserve the findings, and make the next required action obvious.

Every done task needs evidence. No task should move to `+"`done`"+` without proof of done in the task log, review reference, attachments, or related repository history. Implementation tasks should point to a commit or review artifact that includes the task ID. Spikes should link or attach the investigation result, decision record, notes file, or follow-up task list.

Do not use `+"`archive`"+` as a shortcut for unfinished work or as a substitute for evidence. Archive is only for closing work that should leave the active board: obsolete, superseded, duplicate, or intentionally dropped tasks.

## Agent Handoffs

`+"`AgentStatus`"+` metadata is a coordination aid, not proof of done. Use it to show whether a task is assigned, queued, running, blocked on a person, or carrying the result of the last agent run.

| Value | Meaning |
|-------|---------|
| _(empty)_ | No agent run in progress. |
| `+"`queued`"+` | Assigned, waiting for a worker. |
| `+"`running`"+` | Currently executing. |
| `+"`success`"+` | Last run finished with exit code 0. |
| `+"`failed`"+` | Last run finished with a non-zero exit code or runtime error from the agent runner. |
| `+"`cancelled`"+` | Stopped by a user or coordinating tool. |
| `+"`interrupted`"+` | Stopped before completion, with enough context for the tool to continue later. |
| `+"`lost`"+` | Stopped before completion, and the exact continuation state is unknown. |
| `+"`needs-user`"+` | Agent stopped because user input is required. Do not start more agent work until a human clears it. |

Agents that cannot continue safely use the `+"`needs-user`"+` handoff. The task should include a `+"`User Attention`"+` section with:

- Reason: short category such as unclear requirement, external blocker, conflict, failed verification, or stale task.
- Question/Action: the specific ask the user must answer or do.
- Attempted context: what the agent already tried, read, or ruled out.
- Unblock condition: exactly what answer or state lets the run resume.

After making a `+"`needs-user`"+` handoff, stop cleanly. Do not mark the task done, failed, or cancelled just to end the run.

### Tag taxonomy

**Cross-cutting concerns:**

| Tag | When to apply |
|-----|---------------|
| `+"`testing`"+` | Test coverage, test improvements |
| `+"`performance`"+` | Optimization, caching, memory |
| `+"`security`"+` | Vulnerabilities, input validation |
| `+"`dead-code`"+` | Dead code removal, unused exports |
| `+"`cleanup`"+` | Code style, naming, cosmetic fixes |
| `+"`refactor`"+` | Structural changes — extract, split, consolidate |

**Workflow hints:**

| Tag | When to apply |
|-----|---------------|
| `+"`quick-win`"+` | S-size tech-debt/improvement/bug |
| `+"`epic`"+` | Parent/umbrella tasks with sub-tasks |
| `+"`needs-split`"+` | XL tasks that should be broken down |
`, prefix, boardPath)
}

// skillTemplate returns a generic SKILL.md for AI agents.
// boardPath is the board directory relative to the project root (e.g., "board" or ".claude/board").
func skillTemplate(prefix, boardPath string) string {
	return fmt.Sprintf(`---
name: task-board
description: "Use when working with a markdown task board: inspecting board state, creating or grooming tasks, moving work through kanban, capturing follow-ups, or keeping agent handoffs safe."
---

# Task Board Workflow

Compatible with Claude Code and Codex. This file is designed to be copied as a single `+"`SKILL.md`"+` into either agent's skill location.

Read `+"`%[2]s/CONVENTIONS.md`"+` before changing board state. It is the policy source for kanban flow, done evidence, archive semantics, task quality, WIP limits, and agent handoffs.

## Core Rules

- Use the `+"`tb`"+` CLI for structured board mutations.
- Directories are status: `+"`backlog`"+`, `+"`ready`"+`, `+"`in-progress`"+`, `+"`code-review`"+`, `+"`done`"+`, `+"`archive`"+`.
- Never copy task files between statuses. Move through the board tooling.
- `+"`BOARD.md`"+` is generated; never edit `+"`BOARD.md`"+` by hand.
- Do not invent task IDs. Use the board allocator.
- Link related tasks when work is split, blocked, or discovered from another task.
- Respect WIP limits; finish, review, or unblock existing work before adding more.

## Normal Workflow

1. Inspect the board before starting. Prefer the committed `+"`ready`"+` queue over raw backlog.
2. Pull from `+"`ready`"+` before coding. Do not move backlog directly to `+"`in-progress`"+` unless the user explicitly asks for a push-style override.
3. Keep the task log useful: record meaningful transitions, blockers, verification, review outcomes, and final evidence.
4. Submit implementation work to `+"`code-review`"+` when it is ready for inspection.
5. Move to `+"`done`"+` only after acceptance criteria are satisfied and evidence is recorded.
6. Use `+"`archive`"+` only to close obsolete, duplicate, superseded, or intentionally dropped tasks.

Every `+"`done`"+` task needs evidence. Implementation tasks should cite a commit or review artifact that includes `+"`%[1]s-NNN`"+`. Spike tasks should link or attach the investigation result, decision record, notes file, or follow-up task list.

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

Use `+"`needs-user`"+` when you cannot continue safely because requirements are unclear, instructions conflict, a manual/external blocker exists, verification needs a human decision, or the task is stale.

Add a `+"`User Attention`"+` section with:

- Reason: short category.
- Question/Action: the exact user decision or action needed.
- Attempted context: what you tried, read, or ruled out.
- Unblock condition: what answer or state lets work resume.

Then set `+"`AgentStatus`"+` to `+"`needs-user`"+` and stop cleanly. Do not mark the task done, failed, or cancelled just to end the run.

## Minimal Commands

- Inspect board: `+"`tb ls --status ready`"+`, `+"`tb ls --status all`"+`, `+"`tb show %[1]s-NNN`"+`, `+"`tb board --json`"+`.
- Create/capture: `+"`tb create \"Title\" -m module -d \"context\"`"+`.
- Groom/commit intake: edit the task until it has a real goal and acceptance criteria, then `+"`tb ready %[1]s-NNN`"+`.
- Start work: `+"`tb pull`"+` or `+"`tb pull %[1]s-NNN`"+`.
- Move/review/finish: `+"`tb review --submit %[1]s-NNN`"+`, `+"`tb review --pass %[1]s-NNN file|-`"+`, `+"`tb review --fail %[1]s-NNN file|-`"+`, `+"`tb done %[1]s-NNN`"+`, `+"`tb close %[1]s-NNN`"+`.
- Search/link: `+"`tb grep \"pattern\"`"+`, `+"`tb epic %[1]s-NNN`"+`.
- Ask for user input: write the `+"`User Attention`"+` section, then set `+"`--agent-status needs-user`"+`.

Task IDs may be passed with or without the `+"`%[1]s-`"+` prefix when the CLI accepts an ID.
`, prefix, boardPath)
}
