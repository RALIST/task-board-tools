package main

import "fmt"

// conventionsTemplate returns a generic CONVENTIONS.md for the board.
func conventionsTemplate(prefix string) string {
	return fmt.Sprintf(`# Board Conventions

This file describes how to work with this board as a kanban system. It is intentionally a policy guide, not a command manual. Detailed command syntax belongs in CLI help and in the board skill file.

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

If review passes, move the task to `+"`done`"+` with a concise completion note. If review fails, return it to `+"`ready`"+`, preserve the findings, and make the next required action obvious.

Every done task needs evidence. No task should move to `+"`done`"+` without proof of done in the task log, review reference, attachments, or related repository history. Implementation tasks should point to a commit or review artifact that includes the task ID. Spikes should link or attach the investigation result, decision record, notes file, or follow-up task list.

Do not use `+"`archive`"+` as a shortcut for unfinished work or as a substitute for evidence. Archive is only for closing work that should leave the active board: obsolete, superseded, duplicate, or intentionally dropped tasks.

### Agent lifecycle (AgentStatus)

| Value | Meaning |
|-------|---------|
| _(empty)_ | No agent run in progress. |
| `+"`queued`"+` | Assigned, waiting for a worker. |
| `+"`running`"+` | Currently executing. |
| `+"`success`"+` | Last run finished with exit code 0. |
| `+"`failed`"+` | Last run finished with a non-zero exit code or runtime error. |
| `+"`cancelled`"+` | User-initiated cancel. |
| `+"`interrupted`"+` | Recovery-initiated; daemon crashed mid-run with a captured session id. |
| `+"`needs-user`"+` | Agent stopped because user input is required. Automation should skip the task until a human clears it. |

Autonomous agents that cannot continue safely use the `+"`needs-user`"+` handoff. The task should include a `+"`User Attention`"+` section with:

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
`, prefix)
}

// skillTemplate returns a generic SKILL.md for AI agents.
// boardPath is the board directory relative to the project root (e.g., "board" or ".claude/board").
func skillTemplate(prefix, boardPath string) string {
	return fmt.Sprintf(`## Board Management

All operations use the `+"`tb`"+` CLI. Read `+"`%[2]s/CONVENTIONS.md`"+` for full rules.

### Operations

Based on the argument, perform one of:

**`+"`view`"+`** (default if no argument):

1. Run `+"`tb ls --status all`"+` to see all tasks
2. Display current board state to the user
3. Highlight any issues (empty in-progress, stale tasks)

**`+"`refresh`"+`**:

1. Run `+"`tb init`"+` from an existing project root
2. Review refreshed `+"`"+`.tb.yaml`+"`"+`, `+"`%[2]s/CONVENTIONS.md`"+`, and `+"`%[2]s/SKILL.md`"+`
3. Merge any local customizations from the generated `+"`*.bak`"+` files when needed

**`+"`create <title>`"+`**:

1. Run `+"`tb create \"Title\"`"+` with optional flags:
   - `+"`-m module`"+` — module (optional)
   - `+"`-d \"description\"`"+` — goal
   - `+"`-p P0|P1|P2|P3`"+` — priority (default: P2)
   - `+"`-s S|M|L|XL`"+` — size (default: M)
   - `+"`-T feature|bug|tech-debt|improvement|spike`"+` — type (default: bug)
   - `+"`-t tag1,tag2`"+` — tags (see taxonomy in `+"`%[2]s/CONVENTIONS.md`"+`)
   - `+"`--parent ID`"+` — parent epic task ID (links child to parent)
   - `+"`--epic`"+` — create as epic (sets type=feature, adds epic tag)
   - `+"`--legacy-file`"+` — intentionally create old `+"`<status>/<ID>.md`"+` layout instead of folder form
2. By default, creation writes `+"`<status>/<ID>/TASK.md`"+` with an empty `+"`## Attachments`"+` section
3. After creation, edit the generated task file to add Acceptance Criteria and any extra detail
4. **Link related tasks:** Search the board with `+"`tb grep`"+` for tasks in the same module or with overlapping scope. Add a `+"`## Related Tasks`"+` section with bidirectional links

**`+"`ready <%[1]s-NNN>`"+`**:

1. Run `+"`tb ready %[1]s-NNN`"+` to promote a backlog task to the ready column (canonical kanban commitment point).
2. The triage gate runs first — if priority is missing, the goal is a placeholder, or `+"`tb scan`"+` auto-created the task, the command rejects. Fix with `+"`tb edit`"+` and retry.
3. WIP limits on `+"`ready`"+` are honoured: warn mode emits a stderr note, strict mode refuses the move.

**`+"`pull [<%[1]s-NNN>]`"+`**:

1. Run `+"`tb pull`"+` (no argument) to pull the highest-priority oldest ready task into in-progress.
2. Run `+"`tb pull %[1]s-NNN`"+` to override selection — fails if the task isn't currently in ready.
3. Respects `+"`wip_limit_in_progress`"+` like `+"`tb ready`"+` respects `+"`wip_limit_ready`"+`.

**`+"`start <%[1]s-NNN>`"+`**:

1. Run `+"`tb start %[1]s-NNN`"+` to push a task into in-progress (works from any source column).
2. When the source is `+"`backlog`"+`, the command warns that the canonical commitment column was skipped — prefer `+"`tb ready`"+` + `+"`tb pull`"+` unless you specifically need the push.
3. Moves the task to `+"`in-progress/`"+`, auto-logs, auto-regenerates BOARD.md.
4. Set Branch field in the task file to current git branch.

**`+"`done <%[1]s-NNN>`"+`**:

1. Check all acceptance criteria boxes in the task file
2. Add Log entry with completion summary
3. Run `+"`tb done %[1]s-NNN`"+`

**`+"`show <%[1]s-NNN>`"+`**:

1. Run `+"`tb show %[1]s-NNN`"+` for markdown output
2. Use `+"`tb show %[1]s-NNN --json`"+` when another tool needs structured metadata plus the raw task body

**`+"`attach <%[1]s-NNN> <path>...`"+`**:

1. Run `+"`tb attach %[1]s-NNN <path>...`"+` to copy files into a task folder
2. Use `+"`tb attach --rm %[1]s-NNN <attachment-name>...`"+` to remove task attachments
3. New attachments are stored in the task directory; legacy `+"`attachments/<filename>`"+` entries remain supported for compatibility

**`+"`assign <%[1]s-NNN> <claude|codex>`"+`**:

1. Run `+"`tb assign %[1]s-NNN claude`"+` or `+"`tb assign %[1]s-NNN codex`"+`
2. Confirm the task metadata shows the intended `+"`Agent`"+` and `+"`AgentStatus: queued`"+`

**`+"`list`"+`**:

1. Run `+"`tb ls --status all`"+`
2. Show summary: W backlog, X ready, Y in-progress, Z done (call out columns over their WIP limit if `+"`tb board --json`"+` reports them)

**`+"`grep <pattern>`"+`**:

1. Run `+"`tb grep \"<pattern>\"`"+` to search full task content
   - Default: case-insensitive regex, all statuses
   - `+"`-l`"+` for compact output (task IDs + match counts only)
   - `+"`--status b`"+` to limit to backlog
2. Display matching tasks with matched lines

**`+"`epic <%[1]s-NNN>`"+`**:

1. Run `+"`tb epic %[1]s-NNN`"+` to view epic progress and all children
2. Shows: epic title, status, progress (done/total), and sorted child list
3. Use before/after work to track epic completion

### Working with epics

- Before creating a task, check if it belongs to an existing epic (`+"`tb grep`"+` or `+"`tb epic <ID>`"+`)
- Use `+"`--parent <ID>`"+` when creating sub-tasks: `+"`tb create \"Sub-task\" --parent 32`"+`
- Use `+"`tb epic <ID>`"+` to review epic progress before/after work
- When grooming or decomposing an epic, always link children with `+"`--parent`"+`
- If a task was created without `+"`--parent`"+` but should belong to an epic, manually add `+"`**Parent:** %[1]s-NNN`"+` to the task file
- All ID arguments accept bare numbers (`+"`32`"+`) or prefixed (`+"`%[1]s-32`"+`) — both are equivalent

### Rules for agents

- ALWAYS check the board before starting work
- NEVER code without a task in `+"`in-progress/`"+`
- **NEVER push backlog → in-progress directly** — promote with `+"`tb ready <ID>`"+` first, then `+"`tb pull`"+` (or accept that `+"`tb start <ID>`"+` will warn when it skips the ready column)
- **NEVER copy task files — always move** (tb handles this automatically)
- BOARD.md is auto-generated — `+"`tb`"+` regenerates it on every move/create
- Directories are the source of truth — `+"`BOARD.md`"+` is a derived view
- **Link related tasks** — when creating or grooming a task, use `+"`tb grep`"+` to find related tasks. Add `+"`## Related Tasks`"+` section with bidirectional links
- Respect WIP limits surfaced in `+"`BOARD.md`"+` column headers (`+"`(n/m)`"+` markers). If a column shows `+"`⚠`"+`, finish or move a task out before pushing more in.

### Stopping for user attention (`+"`needs-user`"+`)

If you cannot continue safely — unclear requirements, conflicting
instructions, an external/manual blocker, a verification failure that
needs a human call, or a stale task — stop and hand off via the managed
`+"`tb`"+` flow. Do NOT guess, do NOT silently retry, do NOT mark the task
done or failed.

`+"```"+`
tb edit <%[1]s-NNN> --user-attention - <<'EOF'
Reason: <unclear requirement | external blocker | conflict | verification failed | stale task>

Question/Action: <the specific ask the user must answer or do>

Attempted context: <what you tried, read, ruled out — be concrete>

Unblock condition: <what answer/state lets the run resume>
EOF
tb edit <%[1]s-NNN> --agent-status needs-user
`+"```"+`

The user clears the status with `+"`tb edit <%[1]s-NNN> --agent-status none`"+`
once they've responded. Auto-implement and auto-groom skip `+"`needs-user`"+`
tasks; manual Run/Groom are blocked in the GUI with an explanatory
tooltip.

**Before coding:**

1. Run `+"`tb ls --status ready`"+` (and fall back to `+"`--status all`"+` if you need the full board).
2. If `+"`ready`"+` is empty, groom a backlog task and commit it: `+"`tb edit`"+` to fix any gaps, then `+"`tb ready %[1]s-NNN`"+`.
3. Pull into in-progress with `+"`tb pull`"+` (auto-picks highest-priority oldest) or `+"`tb pull %[1]s-NNN`"+` to override.
4. If you specifically need to push directly, `+"`tb start %[1]s-NNN`"+` still works — it just warns when the source is backlog.

**During work — backlog capture:**
When you encounter any of these, IMMEDIATELY create a backlog task:

- Out-of-scope work, deferred features
- Bugs unrelated to current task
- Workarounds, temporary solutions, tech debt
- `+"`TODO`"+`/`+"`FIXME`"+`/`+"`HACK`"+` in code — must reference task ID: `+"`// TODO(%[1]s-NNN): description`"+`

Quick capture: `+"`tb create \"Title\" -m module -d \"description\"`"+`
Or run `+"`tb scan --apply`"+` to auto-create tasks from untagged TODO/FIXME/HACK comments.

**After coding:**

1. Update the task file (check acceptance criteria, add log entry)
2. Move with `+"`tb done %[1]s-NNN`"+`
3. Commit changes with task ID in message: `+"`feat: %[1]s-NNN: concise description`"+`

### CLI Reference

`+"```"+`
tb init [path] [--board-path=board] [--prefix=%[1]s] [--refresh-docs]     Initialize or reconcile a board
tb board [--json]                                                      Print board status or JSON snapshot
tb create "Title" -m module [-d desc] [-p P2] [-T feature] [-s M] [-t tags] [--parent ID] [--epic] [--legacy-file]
tb ls [-t tags] [-s size] [-m module] [-T type] [-p priority] [-n N] [--parent ID] [--status backlog|ready|in-progress|code-review|done|archive|active|all] [--json]
tb mv <%[1]s-NNN> <status>                                               Move task
tb ready <%[1]s-NNN>                                                     Promote backlog → ready (canonical kanban commitment)
tb pull [<%[1]s-NNN>]                                                    Pull next ready task into in-progress
tb start <%[1]s-NNN>                                                     Start working (push-style; warns when source is backlog)
tb done <%[1]s-NNN>                                                      Mark done
tb edit <%[1]s-NNN> [--goal file|-] [--acceptance file|-] [--user-attention file|-] [--agent-status queued|running|success|failed|cancelled|interrupted|needs-user|none] [--review-ref value|none]   Edit metadata/body sections
tb attach <%[1]s-NNN> <path>...                                          Copy files into task attachments
tb attach --rm <%[1]s-NNN> <attachment-name>...                          Remove task attachments
tb assign <%[1]s-NNN> <claude|codex>                                     Assign a runnable agent and queue pickup
tb close <%[1]s-NNN>                                                     Archive task
tb show <%[1]s-NNN> [--json]                                             Print task content or JSON
tb open <%[1]s-NNN>                                                      Open in default editor
tb epic <%[1]s-NNN> [--status active|archive|all]                        Show epic progress
tb triage [--json]                                                       Find tasks needing grooming
tb grep <pattern> [--status backlog|ready|in-progress|code-review|done|archive|active|all] [-s] [-l]   Search tasks by regex
tb scan [--apply] [--path dir]                                           Find untagged TODOs
tb regenerate                                                            Regenerate BOARD.md
`+"```"+`

**Commands:**

| Command | Aliases | Description |
|---------|---------|-------------|
| `+"`init`"+` | | Initialize board structure (creates `+"`"+`.tb.yaml`+"`"+` in project root) |
| `+"`board`"+` | | Print board status or JSON snapshot |
| `+"`create`"+` | `+"`new`"+` | Create a new folder-form task |
| `+"`ls`"+` | `+"`list`"+` | List and filter tasks |
| `+"`mv`"+` | `+"`move`"+` | Move task between statuses |
| `+"`ready`"+` | | Promote a backlog task to ready (canonical kanban commitment column) |
| `+"`pull`"+` | | Pull the highest-priority oldest ready task into in-progress |
| `+"`start`"+` | | Move task to in-progress (push-style; warns when source is backlog) |
| `+"`done`"+` | | Move task to done |
| `+"`edit`"+` | | Edit task metadata and Goal/Acceptance Criteria sections |
| `+"`attach`"+` | | Copy or remove task attachments |
| `+"`assign`"+` | | Assign claude or codex and queue daemon pickup |
| `+"`close`"+` | | Archive task |
| `+"`show`"+` | `+"`cat`"+` | Print task content or JSON |
| `+"`open`"+` | | Open task file in default editor/app |
| `+"`epic`"+` | | Show epic task with children and progress |
| `+"`triage`"+` | | Find tasks needing grooming (placeholder goals, no module, auto-created) |
| `+"`grep`"+` | `+"`search`"+` | Full-text regex search across all task files |
| `+"`scan`"+` | | Find untagged TODO/FIXME/HACK comments, create tasks, update source |
| `+"`regenerate`"+` | `+"`regen`"+` | Regenerate BOARD.md from directory contents |

**Defaults:** type=bug, priority=P2, size=M. Module and tags are optional.

**Status aliases:** `+"`b`"+`=backlog, `+"`r`"+`=ready, `+"`ip`"+`/`+"`wip`"+`=in-progress, `+"`cr`"+`/`+"`review`"+`=code-review, `+"`d`"+`=done

Task IDs use the configured prefix (default: %[1]s). The prefix is optional in commands — `+"`tb start 123`"+` and `+"`tb start %[1]s-123`"+` are equivalent.

**Configuration:** `+"`tb`"+` discovers `+"`"+`.tb.yaml`+"`"+` by walking up from the current directory. Fallback: `+"`TB_BOARD_DIR`"+` and `+"`TB_PREFIX`"+` environment variables.

**Examples:**

`+"```"+`
tb create "Fix crash on empty input" -m core -p P1 -s S -t quick-win
tb create "Quick bug note"                        # minimal — title only
tb create "Search system" --epic -m editor        # Create an epic
tb create "Search indexing" --parent 1 -m editor  # Create child of epic
tb create "Legacy integration probe" --legacy-file # Explicit old <status>/<ID>.md layout
tb init                                            # Refresh generated docs/config with .bak backups
tb ls -T bug -p P1                                # P1 bugs
tb ls -t testing                                  # All test-related tasks
tb ls -t quick-win -T tech-debt                   # Easy tech-debt wins
tb ls --parent 1                                  # Children of an epic
tb start 1                                        # Prefix optional
tb done 1
tb epic 1                                         # View epic progress
tb grep "auth"                                    # Search all tasks (case-insensitive regex)
tb grep "cache" -l --status b                     # Compact search in backlog only
tb scan                                           # Dry-run: preview untagged TODOs
tb scan --apply                                   # Create tasks + update source comments
tb scan --path internal/core --apply              # Scoped scan
`+"```"+`

**Board architecture:** Directories are the source of truth. `+"`BOARD.md`"+` is auto-generated by `+"`tb regenerate`"+`. `+"`"+`.next-id`+"`"+` is managed by `+"`tb create`"+`. Never edit `+"`BOARD.md`"+` manually.
`, prefix, boardPath)
}
