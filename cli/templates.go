package main

import "fmt"

// conventionsTemplate returns a generic CONVENTIONS.md for the board.
func conventionsTemplate(prefix string) string {
	return fmt.Sprintf(`# Board Conventions

## Structure

`+"```"+`
board/
  BOARD.md              — Generated kanban view (DO NOT edit manually)
  CONVENTIONS.md        — This file
  SKILL.md              — AI agent instructions for using the board
  .next-id              — Counter for next %[1]s-NNN ID
  backlog/              — Prioritized, ready to pick up
  in-progress/          — Currently being worked on (max 2 tasks)
  done/                 — Completed (archive, clean periodically)
`+"```"+`

CLI tool `+"`tb`"+` manages board operations (create, move, edit, attach, assign, list, JSON views, regenerate).

**Directories are the source of truth.** `+"`BOARD.md`"+` is a generated view — never edit it manually.

Directory = status. Moving a task entry between directories = status change.

**CRITICAL: A task file must exist in exactly ONE directory.** When moving a task, always use `+"`tb mv`"+`/`+"`tb start`"+`/`+"`tb done`"+` which handle the move atomically. Never copy task files.

## Task File Format

Default path: `+"`<status>/%[1]s-NNN/TASK.md`"+` (e.g., `+"`backlog/%[1]s-001/TASK.md`"+`).

Legacy path: `+"`<status>/%[1]s-NNN.md`"+`. Create legacy files only when you intentionally pass `+"`tb create --legacy-file`"+`; they are kept for compatibility and do not support in-place attachments.

Folder-form tasks store new attachments directly under `+"`<status>/%[1]s-NNN/`"+`. Legacy `+"`attachments/<filename>`"+` entries remain supported for compatibility when older tasks are promoted.

**ID allocation:** Handled automatically by `+"`tb create`"+`. The `+"`"+`.next-id`+"`"+` file is the counter, protected by file locking for concurrent access.

`+"```"+`markdown
# %[1]s-NNN: Short title

**Type:** feature | bug | tech-debt | improvement | spike
**Priority:** P0 | P1 | P2
**Size:** S | M | L | XL
**Module:** module-name (optional)
**Tags:** comma-separated tags (optional)
**Branch:** feat/branch-name (set when work starts)
**Parent:** %[1]s-NNN (optional — links to parent epic)

## Goal

One-sentence objective.

## Context

Why this task exists. Link to the task or session where it was discovered.

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2

## Attachments

## Related Tasks

- **%[1]s-XXX** — Title (relationship: prerequisite | blocked by | shares infrastructure | complementary | depends on)

## Log

- YYYY-MM-DD: Created
- YYYY-MM-DD: Started — moved to in-progress
- YYYY-MM-DD: Done — [summary of what was done]
`+"```"+`

### Task types

| Type          | When to use                                 | Examples                                             |
| ------------- | ------------------------------------------- | ---------------------------------------------------- |
| `+"`feature`"+`     | New capability                              | Implement search, add export format                  |
| `+"`bug`"+`         | Broken behavior found during work           | Crash on empty input, wrong calculation              |
| `+"`tech-debt`"+`   | Shortcuts, workarounds, temporary solutions | Hardcoded limit, missing error handling, copied code |
| `+"`improvement`"+` | Enhancement to existing functionality       | Better UX, faster lookup                             |
| `+"`spike`"+`       | Research or investigation needed            | Evaluate approaches, benchmark alternatives          |

## Rules

### Before coding

1. Run `+"`tb ls`"+` for current state
2. Pick a task or create one with `+"`tb create \"Title\"`"+`
3. Start it with `+"`tb start %[1]s-NNN`"+`
4. Set the `+"`Branch`"+` field

### During work

- Add notes to the task's Log section as you make progress
- If blocked, note it in the Log

### After work

- Check all acceptance criteria boxes
- Run `+"`tb done %[1]s-NNN`"+`
- Add final Log entry with summary

### Backlog capture

Create backlog tasks when you encounter:

- Out-of-scope work or deferred features
- Bugs unrelated to current task
- Workarounds or temporary solutions
- `+"`TODO`"+`/`+"`FIXME`"+`/`+"`HACK`"+` in code — reference task ID: `+"`// TODO(%[1]s-NNN): description`"+`
- Performance concerns or improvement ideas

Quick capture: `+"`tb create \"Title\" -m module -d \"description\"`"+`

### Board hygiene

- P0 = drop everything. P1 = next up. P2 = when convenient
- Size guide: S = <1h, M = 1-4h, L = 4-8h, XL = multi-session
- Tags: comma-separated. Filter with `+"`tb ls -t tag`"+`

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

## BOARD.md

`+"`BOARD.md`"+` is **auto-generated** by `+"`tb regenerate`"+`. Do not edit it manually.

## Project Refresh

Existing boards can refresh generated project files without reinitializing tasks:

`+"```"+`
tb init
`+"```"+`

The command reads `+"`"+`.tb.yaml`+"`"+` for the current board path and prefix, rewrites generated files such as `+"`CONVENTIONS.md`"+` and `+"`SKILL.md`"+`, and saves previous copies as `+"`*.bak`"+` files for manual merge of local customizations. The old `+"`--refresh-docs`"+` flag is accepted for scripts, but plain `+"`tb init`"+` is the normal refresh path.

## CLI Reference

`+"```"+`
tb init [path] [--board-path=board] [--prefix=%[1]s] [--refresh-docs]
tb board [--json]
tb create "Title" [-m module] [-d desc] [-p P2] [-T bug] [-s M] [-t tags] [--parent ID] [--epic] [--legacy-file]
tb ls [-t tags] [-s size] [-m module] [-T type] [-p priority] [-n N] [--parent ID] [--status backlog|in-progress|done|archive|active|all] [--json]
tb mv <%[1]s-NNN> <status>                                                    — Move task between statuses
tb start <%[1]s-NNN>                                                          — Move to in-progress
tb done <%[1]s-NNN>                                                           — Move to done
tb edit <%[1]s-NNN> [-p P0] [-T type] [-s M] [-m module] [-t tags] [-a claude|codex] [--agent-status queued|running|success|failed|cancelled] [--title "New title"] [--goal file|-] [--acceptance file|-]
tb attach <%[1]s-NNN> <path>...                                               — Copy files into task attachments
tb attach --rm <%[1]s-NNN> <attachment-name>...                               — Remove task attachments
tb assign <%[1]s-NNN> <claude|codex>                                          — Assign a runnable agent and queue pickup
tb close <%[1]s-NNN>                                                          — Archive task
tb show <%[1]s-NNN> [--json]                                                  — Print task content or JSON
tb open <%[1]s-NNN>                                                           — Open in default editor
tb epic <%[1]s-NNN> [--status active|archive|all]                             — Show epic progress and children
tb triage [--json]                                                            — Find tasks needing grooming
tb grep <pattern> [--status backlog|in-progress|done|archive|active|all] [-s] [-l] — Search tasks by regex
tb scan [--apply] [--path dir]                                                — Find untagged TODOs, create tasks
tb regenerate                                                                 — Regenerate BOARD.md
`+"```"+`

**Defaults:** type=bug, priority=P2, size=M.

**Status aliases:** `+"`b`"+`=backlog, `+"`ip`"+`=in-progress, `+"`d`"+`=done

**Examples:**

`+"```"+`
tb create "Fix crash on empty input" -m core -p P1 -s S -t quick-win
tb create "Search system" --epic -m editor          # Create an epic
tb create "Search indexing" --parent 1 -m editor    # Create child of epic
tb create "Legacy integration probe" --legacy-file   # Explicit old <status>/<ID>.md layout
tb init                                   # Refresh generated project files with .bak backups
tb ls -T bug -p P1                       # P1 bugs
tb ls -t testing                         # All test-related tasks
tb ls --parent 1                         # Children of an epic
tb start 1                               # Prefix is optional — "1" = "%[1]s-1"
tb done 1
tb epic 1                                # View epic progress
tb grep "auth"                           # Search all tasks
tb scan --apply                          # Create tasks from TODOs
`+"```"+`
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
2. Review refreshed `+"`%[2]s/CONVENTIONS.md`"+` and `+"`%[2]s/SKILL.md`"+`
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

**`+"`start <%[1]s-NNN>`"+`**:

1. Run `+"`tb start %[1]s-NNN`"+`
   - Moves the task to `+"`in-progress/`"+`, auto-logs, auto-regenerates BOARD.md
2. Set Branch field in the task file to current git branch

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
2. Show summary: X backlog, Y in-progress, Z done

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
- **NEVER copy task files — always move** (tb handles this automatically)
- BOARD.md is auto-generated — `+"`tb`"+` regenerates it on every move/create
- Directories are the source of truth — `+"`BOARD.md`"+` is a derived view
- **Link related tasks** — when creating or grooming a task, use `+"`tb grep`"+` to find related tasks. Add `+"`## Related Tasks`"+` section with bidirectional links

**Before coding:**

1. Run `+"`tb ls`"+` to see the board
2. Pick a task or create one with `+"`tb create \"Title\"`"+`
3. Start it with `+"`tb start %[1]s-NNN`"+`

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
tb ls [-t tags] [-s size] [-m module] [-T type] [-p priority] [-n N] [--parent ID] [--status backlog|in-progress|done|archive|active|all] [--json]
tb mv <%[1]s-NNN> <status>                                               Move task
tb start <%[1]s-NNN>                                                     Start working
tb done <%[1]s-NNN>                                                      Mark done
tb edit <%[1]s-NNN> [--goal file|-] [--acceptance file|-]                Edit metadata/body sections
tb attach <%[1]s-NNN> <path>...                                          Copy files into task attachments
tb attach --rm <%[1]s-NNN> <attachment-name>...                          Remove task attachments
tb assign <%[1]s-NNN> <claude|codex>                                     Assign a runnable agent and queue pickup
tb close <%[1]s-NNN>                                                     Archive task
tb show <%[1]s-NNN> [--json]                                             Print task content or JSON
tb open <%[1]s-NNN>                                                      Open in default editor
tb epic <%[1]s-NNN> [--status active|archive|all]                        Show epic progress
tb triage [--json]                                                       Find tasks needing grooming
tb grep <pattern> [--status backlog|in-progress|done|archive|active|all] [-s] [-l]   Search tasks by regex
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
| `+"`start`"+` | | Move task to in-progress |
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

**Status aliases:** `+"`b`"+`=backlog, `+"`ip`"+`=in-progress, `+"`d`"+`=done

Task IDs use the configured prefix (default: %[1]s). The prefix is optional in commands — `+"`tb start 123`"+` and `+"`tb start %[1]s-123`"+` are equivalent.

**Configuration:** `+"`tb`"+` discovers `+"`"+`.tb.yaml`+"`"+` by walking up from the current directory. Fallback: `+"`TB_BOARD_DIR`"+` and `+"`TB_PREFIX`"+` environment variables.

**Examples:**

`+"```"+`
tb create "Fix crash on empty input" -m core -p P1 -s S -t quick-win
tb create "Quick bug note"                        # minimal — title only
tb create "Search system" --epic -m editor        # Create an epic
tb create "Search indexing" --parent 1 -m editor  # Create child of epic
tb create "Legacy integration probe" --legacy-file # Explicit old <status>/<ID>.md layout
tb init                                            # Refresh generated project files with .bak backups
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
