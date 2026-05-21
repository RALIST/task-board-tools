# Board Conventions

This file describes how to work with this board as a kanban system. It is intentionally a policy guide, not a command manual. Detailed command syntax belongs in CLI help and in the board skill file.

This board root is configured in `.tb.yaml` as `board`. Paths in this guide are relative to the project root unless stated otherwise. For this board, generated views such as `board/BOARD.md` live under that configured root. Another project may use a different board path.

## Kanban Flow

The board flows in one direction:

```
backlog → ready → in-progress → code-review → done → archive
```

Each column has a job:

- `backlog` is intake. Ideas can be rough here, but they should not be treated as committed work.
- `ready` is the commitment point. A task is ready only when it has a priority, a clear goal, and enough acceptance criteria for someone else to finish it.
- `in-progress` is active work. Keep it small; do not hoard tasks.
- `code-review` is work that claims to be done and needs reviewer signoff.
- `done` is accepted work. The task should explain what changed and how it was verified.
- `archive` is for obsolete, superseded, duplicate, or long-closed work that should leave the active board.

Tasks flow forward. A failed review returns to `ready` with a clear rework note because the task is still groomed; it just needs another implementation pass.

## Source Of Truth

Directories are the source of truth. A task entry exists in one status only, and its status is the directory it lives in. Moving work by copying files breaks the board; move the task entry instead.

`BOARD.md` is a generated board view. Do not edit it by hand. If it disagrees with task entries, trust the task entries and regenerate the view through the board tooling.

Use the managed board tools for structured changes such as creating, moving, editing metadata, assigning agents, managing attachments, closing tasks, and rebuilding generated views. Direct file edits are acceptable for human-readable task body improvements, but preserve the metadata block and the one-task-one-status rule.

Task IDs use the `TB-NNN` shape. The numeric allocator is owned by the board tooling; do not invent IDs manually.

## Task Quality

A good task is small enough to finish, specific enough to review, and explicit about success. Before a task leaves `backlog`, it should have:

- A concise title.
- A type: `feature`, `bug`, `tech-debt`, `improvement`, or `spike`.
- A priority: `P0` for urgent work, `P1` for next-up work, `P2` for normal backlog.
- A size: `S`, `M`, `L`, or `XL`; split `XL` tasks before implementation when possible.
- A real goal that describes the outcome, not only the activity.
- Acceptance criteria that can be checked by a reviewer.
- Relevant module, tags, parent epic, and related-task links when they help routing.

Use `spike` for research whose output is a decision, summary, or follow-up task list. Do not let spikes quietly become implementation tasks without updating their goal and acceptance criteria.

## Working Agreements

Before starting work, pull from `ready` unless the user explicitly chooses a specific task. If `ready` is empty, groom intake first instead of treating raw backlog as committed work.

Respect WIP limits when they are configured. A WIP warning is a signal to finish, review, or unblock existing work before adding more.

Set or update the branch/reference fields when they help reviewers find the implementation. Work submitted to `code-review` should include enough review reference information to inspect the actual change.

Keep the `Log` useful. Record meaningful transitions, blockers, verification results, review outcomes, and final summaries. Avoid noisy diary entries that do not help the next reader.

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

Use `Related Tasks` to preserve context across split work. Good relationship labels include `prerequisite`, `blocked by`, `shares infrastructure`, `complementary`, and `depends on`.

When decomposing an epic, connect children to the parent and keep sibling ordering meaningful. If one child must happen before another, make that dependency explicit instead of relying on memory.

## Review Loop

`code-review` is a claim that implementation is complete enough to inspect. A review should focus on behavior, regressions, missing tests, data loss, security, and contract drift.

If review passes, use `tb review --pass` to record findings and move the task to `done`. If review fails, use `tb review --fail` to return it to `ready`, preserve the findings, and make the next required action obvious.

Every done task needs evidence. No task should move to `done` without proof of done in the task log, review reference, attachments, or related repository history. Implementation tasks should point to a commit or review artifact that includes the task ID. Spikes should link or attach the investigation result, decision record, notes file, or follow-up task list.

Do not use `archive` as a shortcut for unfinished work or as a substitute for evidence. Archive is only for closing work that should leave the active board: obsolete, superseded, duplicate, or intentionally dropped tasks.

## Autonomous Stages

Autonomous board work is split into three independent stages. Enabling one stage does not enable or imply the others:

- `auto-groom` works on `backlog` intake. It may refine task content and, after the task no longer needs triage, promote through the managed ready gate into `ready`.
- `auto-implement` works only on committed `ready` tasks. The coordinator owns the `ready` to `in-progress` transition; the implementation agent owns the change and submits completed work to `code-review` with review target metadata.
- `auto-review` works only on `code-review` tasks. It is off by default through `auto_review_enabled`, requires a valid default agent, and reviews only tasks with a top-level `ReviewRef`. A pass moves to `done` through `tb review --pass`; a failure returns to `ready` through `tb review --fail` with `review-failed` and a clear rework note. Missing `ReviewRef` uses the `needs-user` handoff.

Backlog tasks are not auto-implemented. A ready task tagged `review-failed` is rework for auto-implement, not a review candidate. Failed review handoff should clear retry-blocking generic `AgentStatus` while keeping review history in the task log, review fields, and agent artifacts.

Auto-implement obeys epic child order. For tasks with the same parent epic, a later numeric child must not be selected while an earlier child is still active outside `done`; `archive` is treated as closed work because archive is reserved for obsolete, superseded, duplicate, or intentionally dropped tasks. Missing or unreadable earlier siblings should block with a diagnostic instead of being ignored.

Daemon housekeeping for autonomous stages is soft and deterministic. It may repair missed transitions only from objective board/run markers, must use managed board operations, and must not guess from arbitrary prose, logs, or comments. Auto-review recovery applies only to JSONL runs queued with `initiator=auto-review`. It must preserve `needs-user`, `cancelled`, and unrelated `interrupted`/`lost` states. WIP-blocked repairs should back off durably so watcher reloads do not loop on the same blocked transition.

### Agent lifecycle (AgentStatus)

| Value | Meaning |
|-------|---------|
| _(empty)_ | No agent run in progress. |
| `queued` | Assigned, waiting for a worker. |
| `running` | Currently executing. |
| `success` | Last run finished with exit code 0. |
| `failed` | Last run finished with a non-zero exit code or runtime error from the agent runner. |
| `cancelled` | User-initiated cancel. |
| `interrupted` | Recovery-initiated; daemon crashed mid-run with a captured session id, so Resume is available. |
| `lost` | Recovery-initiated; daemon lost the terminal run result and no resumable session was captured. |
| `needs-user` | Agent stopped because user input is required. Automation should skip the task until a human clears it. |

Resume is offered when the backend reports that the latest run has a captured session id and the task is in a terminal status (`interrupted`, `lost`, `failed`, `cancelled`, or `success`). `queued`, `running`, and `needs-user` remain blocked; the UI labels the source status so the user intentionally resumes failed, cancelled, or successful runs.

Autonomous agents that cannot continue safely use the `needs-user` handoff. The task should include a `User Attention` section with:

- Reason: short category such as unclear requirement, external blocker, conflict, failed verification, or stale task.
- Question/Action: the specific ask the user must answer or do.
- Attempted context: what the agent already tried, read, or ruled out.
- Unblock condition: exactly what answer or state lets the run resume.

After making a `needs-user` handoff, stop cleanly. Do not mark the task done, failed, or cancelled just to end the run.

### Tag taxonomy

**Cross-cutting concerns:**

| Tag | When to apply |
|-----|---------------|
| `testing` | Test coverage, test improvements |
| `performance` | Optimization, caching, memory |
| `security` | Vulnerabilities, input validation |
| `dead-code` | Dead code removal, unused exports |
| `cleanup` | Code style, naming, cosmetic fixes |
| `refactor` | Structural changes — extract, split, consolidate |

**Workflow hints:**

| Tag | When to apply |
|-----|---------------|
| `quick-win` | S-size tech-debt/improvement/bug |
| `epic` | Parent/umbrella tasks with sub-tasks |
| `needs-split` | XL tasks that should be broken down |
