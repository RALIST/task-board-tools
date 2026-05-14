# Board Management

All operations use the `tb` CLI. Read `board/CONVENTIONS.md` for full rules.

### Operations

Based on the argument, perform one of:

**`view`** (default if no argument):

1. Run `tb ls --status all` to see all tasks
2. Display current board state to the user
3. Highlight any issues (empty in-progress, stale tasks)

**`create <title>`**:

1. Run `tb create "Title"` with optional flags:
   - `-m module` — module (optional)
   - `-d "description"` — goal
   - `-p P0|P1|P2|P3` — priority (default: P2)
   - `-s S|M|L|XL` — size (default: M)
   - `-T feature|bug|tech-debt|improvement|spike` — type (default: bug)
   - `-t tag1,tag2` — tags (see taxonomy in `board/CONVENTIONS.md`)
   - `--parent ID` — parent epic task ID (links child to parent)
   - `--epic` — create as epic (sets type=feature, adds epic tag)
2. After creation, edit the generated task file to add Acceptance Criteria and any extra detail
3. **Link related tasks:** Search the board with `tb grep` for tasks in the same module or with overlapping scope. Add a `## Related Tasks` section with bidirectional links

**`start <PR-NNN>`**:

1. Run `tb start PR-NNN`
   - Moves the task to `in-progress/`, auto-logs, auto-regenerates BOARD.md
2. Set Branch field in the task file to current git branch

**`done <PR-NNN>`**:

1. Check all acceptance criteria boxes in the task file
2. Add Log entry with completion summary
3. Run `tb done PR-NNN`

**`list`**:

1. Run `tb ls --status all`
2. Show summary: X backlog, Y in-progress, Z done

**`grep <pattern>`**:

1. Run `tb grep "<pattern>"` to search full task content
   - Default: case-insensitive regex, all statuses
   - `-l` for compact output (task IDs + match counts only)
   - `--status b` to limit to backlog
2. Display matching tasks with matched lines

**`epic <PR-NNN>`**:

1. Run `tb epic PR-NNN` to view epic progress and all children
2. Shows: epic title, status, progress (done/total), and sorted child list
3. Use before/after work to track epic completion

### Working with epics

- Before creating a task, check if it belongs to an existing epic (`tb grep` or `tb epic <ID>`)
- Use `--parent <ID>` when creating sub-tasks: `tb create "Sub-task" --parent 32`
- Use `tb epic <ID>` to review epic progress before/after work
- When grooming or decomposing an epic, always link children with `--parent`
- If a task was created without `--parent` but should belong to an epic, manually add `**Parent:** PR-NNN` to the task file
- All ID arguments accept bare numbers (`32`) or prefixed (`PR-32`) — both are equivalent

### Rules for agents

- ALWAYS check the board before starting work
- NEVER code without a task in `in-progress/`
- **NEVER copy task files — always move** (tb handles this automatically)
- BOARD.md is auto-generated — `tb` regenerates it on every move/create
- Directories are the source of truth — `BOARD.md` is a derived view
- **Link related tasks** — when creating or grooming a task, use `tb grep` to find related tasks. Add `## Related Tasks` section with bidirectional links

**Before coding:**

1. Run `tb ls` to see the board
2. Pick a task or create one with `tb create "Title"`
3. Start it with `tb start PR-NNN`

**During work — backlog capture:**
When you encounter any of these, IMMEDIATELY create a backlog task:

- Out-of-scope work, deferred features
- Bugs unrelated to current task
- Workarounds, temporary solutions, tech debt
- `TODO`/`FIXME`/`HACK` in code — must reference task ID: `// TODO(PR-NNN): description`

Quick capture: `tb create "Title" -m module -d "description"`
Or run `tb scan --apply` to auto-create tasks from untagged TODO/FIXME/HACK comments.

**After coding:**

1. Update the task file (check acceptance criteria, add log entry)
2. Move with `tb done PR-NNN`
3. Commit changes with task ID in message: `feat: PR-NNN: concise description`

### CLI Reference

```
tb init [path] [--board-path=board] [--prefix=PR]                     Initialize board
tb create "Title" -m module [-d desc] [-p P2] [-T feature] [-s M] [-t tags] [--parent ID] [--epic]
tb ls [-t tags] [-s size] [-m module] [-T type] [-p priority] [--parent ID]  List/filter tasks
tb mv <PR-NNN> <status>                                               Move task
tb start <PR-NNN>                                                     Start working
tb done <PR-NNN>                                                      Mark done
tb close <PR-NNN>                                                     Delete task
tb show <PR-NNN>                                                      Print task content
tb open <PR-NNN>                                                      Open in default editor
tb epic <PR-NNN>                                                      Show epic progress
tb triage                                                                Find tasks needing grooming
tb grep <pattern> [--status all] [-s] [-l]                               Search tasks by regex
tb scan [--apply] [--path dir]                                           Find untagged TODOs
tb regenerate                                                            Regenerate BOARD.md
```

**Commands:**

| Command | Aliases | Description |
|---------|---------|-------------|
| `init` | | Initialize board structure (creates `.tb.yaml` in project root) |
| `create` | `new` | Create a new task |
| `ls` | `list` | List and filter tasks |
| `mv` | `move` | Move task between statuses |
| `start` | | Move task to in-progress |
| `done` | | Move task to done |
| `close` | | Delete task from board |
| `show` | `cat` | Print task content to stdout |
| `open` | | Open task file in default editor/app |
| `epic` | | Show epic task with children and progress |
| `triage` | | Find tasks needing grooming (placeholder goals, no module, auto-created) |
| `grep` | `search` | Full-text regex search across all task files |
| `scan` | | Find untagged TODO/FIXME/HACK comments, create tasks, update source |
| `regenerate` | `regen` | Regenerate BOARD.md from directory contents |

**Defaults:** type=bug, priority=P2, size=M. Module and tags are optional.

**Status aliases:** `b`=backlog, `ip`=in-progress, `d`=done

Task IDs use the configured prefix (default: PR). The prefix is optional in commands — `tb start 123` and `tb start PR-123` are equivalent.

**Configuration:** `tb` discovers `.tb.yaml` by walking up from the current directory. Fallback: `TB_BOARD_DIR` and `TB_PREFIX` environment variables.

**Examples:**

```
tb create "Fix crash on empty input" -m core -p P1 -s S -t quick-win
tb create "Quick bug note"                        # minimal — title only
tb create "Search system" --epic -m editor        # Create an epic
tb create "Search indexing" --parent 1 -m editor  # Create child of epic
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
```

**Board architecture:** Directories are the source of truth. `BOARD.md` is auto-generated by `tb regenerate`. `.next-id` is managed by `tb create`. Never edit `BOARD.md` manually.
