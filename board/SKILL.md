## Board Management

All operations use the `tb` CLI. Read `board/CONVENTIONS.md` for full rules.

### Operations

Based on the argument, perform one of:

**`view`** (default if no argument):

1. Run `tb ls --status all` to see all tasks
2. Display current board state to the user
3. Highlight any issues (empty in-progress, stale tasks)

**`refresh`**:

1. Run `tb init` from an existing project root
2. Review refreshed `.tb.yaml`, `board/CONVENTIONS.md`, and `board/SKILL.md`
3. Merge any local customizations from the generated `*.bak` files when needed

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
   - `--legacy-file` — intentionally create old `<status>/<ID>.md` layout instead of folder form
2. By default, creation writes `<status>/<ID>/TASK.md` with an empty `## Attachments` section
3. After creation, edit the generated task file to add Acceptance Criteria and any extra detail
4. **Link related tasks:** Search the board with `tb grep` for tasks in the same module or with overlapping scope. Add a `## Related Tasks` section with bidirectional links

**`ready <TB-NNN>`**:

1. Run `tb ready TB-NNN` to promote a backlog task to the ready column (canonical kanban commitment point).
2. The triage gate runs first — if priority is missing, the goal is a placeholder, or `tb scan` auto-created the task, the command rejects. Fix with `tb edit` and retry.
3. WIP limits on `ready` are honoured: warn mode emits a stderr note, strict mode refuses the move.

**`pull [<TB-NNN>]`**:

1. Run `tb pull` (no argument) to pull the highest-priority oldest ready task into in-progress.
2. Run `tb pull TB-NNN` to override selection — fails if the task isn't currently in ready.
3. Respects `wip_limit_in_progress` like `tb ready` respects `wip_limit_ready`.

**`start <TB-NNN>`**:

1. Run `tb start TB-NNN` to push a task into in-progress (works from any source column).
2. When the source is `backlog`, the command warns that the canonical commitment column was skipped — prefer `tb ready` + `tb pull` unless you specifically need the push.
3. Moves the task to `in-progress/`, auto-logs, auto-regenerates BOARD.md.
4. Set Branch field in the task file to current git branch.

**`done <TB-NNN>`**:

1. Check all acceptance criteria boxes in the task file
2. Add Log entry with completion summary
3. Run `tb done TB-NNN`

**`show <TB-NNN>`**:

1. Run `tb show TB-NNN` for markdown output
2. Use `tb show TB-NNN --json` when another tool needs structured metadata plus the raw task body

**`attach <TB-NNN> <path>...`**:

1. Run `tb attach TB-NNN <path>...` to copy files into a task folder
2. Use `tb attach --rm TB-NNN <attachment-name>...` to remove task attachments
3. New attachments are stored in the task directory; legacy `attachments/<filename>` entries remain supported for compatibility

**`assign <TB-NNN> <claude|codex>`**:

1. Run `tb assign TB-NNN claude` or `tb assign TB-NNN codex`
2. Confirm the task metadata shows the intended `Agent` and `AgentStatus: queued`

**`list`**:

1. Run `tb ls --status all`
2. Show summary: W backlog, X ready, Y in-progress, Z done (call out columns over their WIP limit if `tb board --json` reports them)

**`grep <pattern>`**:

1. Run `tb grep "<pattern>"` to search full task content
   - Default: case-insensitive regex, all statuses
   - `-l` for compact output (task IDs + match counts only)
   - `--status b` to limit to backlog
2. Display matching tasks with matched lines

**`epic <TB-NNN>`**:

1. Run `tb epic TB-NNN` to view epic progress and all children
2. Shows: epic title, status, progress (done/total), and sorted child list
3. Use before/after work to track epic completion

### Working with epics

- Before creating a task, check if it belongs to an existing epic (`tb grep` or `tb epic <ID>`)
- Use `--parent <ID>` when creating sub-tasks: `tb create "Sub-task" --parent 32`
- Use `tb epic <ID>` to review epic progress before/after work
- When grooming or decomposing an epic, always link children with `--parent`
- If a task was created without `--parent` but should belong to an epic, manually add `**Parent:** TB-NNN` to the task file
- All ID arguments accept bare numbers (`32`) or prefixed (`TB-32`) — both are equivalent

### Rules for agents

- ALWAYS check the board before starting work
- NEVER code without a task in `in-progress/`
- **NEVER push backlog → in-progress directly** — promote with `tb ready <ID>` first, then `tb pull` (or accept that `tb start <ID>` will warn when it skips the ready column)
- **NEVER copy task files — always move** (tb handles this automatically)
- BOARD.md is auto-generated — `tb` regenerates it on every move/create
- Directories are the source of truth — `BOARD.md` is a derived view
- **Link related tasks** — when creating or grooming a task, use `tb grep` to find related tasks. Add `## Related Tasks` section with bidirectional links
- Respect WIP limits surfaced in `BOARD.md` column headers (`(n/m)` markers). If a column shows `⚠`, finish or move a task out before pushing more in.

### Stopping for user attention (`needs-user`)

If you cannot continue safely — unclear requirements, conflicting
instructions, an external/manual blocker, a verification failure that
needs a human call, or a stale task — stop and hand off via the managed
`tb` flow. Do NOT guess, do NOT silently retry, do NOT mark the task
done or failed.

```
tb edit <TB-NNN> --user-attention - <<'EOF'
Reason: <unclear requirement | external blocker | conflict | verification failed | stale task>

Question/Action: <the specific ask the user must answer or do>

Attempted context: <what you tried, read, ruled out — be concrete>

Unblock condition: <what answer/state lets the run resume>
EOF
tb edit <TB-NNN> --agent-status needs-user
```

The user clears the status with `tb edit <TB-NNN> --agent-status none`
once they've responded. Auto-implement and auto-groom skip `needs-user`
tasks; manual Run/Groom are blocked in the GUI with an explanatory
tooltip.

**Before coding:**

1. Run `tb ls --status ready` (and fall back to `--status all` if you need the full board).
2. If `ready` is empty, groom a backlog task and commit it: `tb edit` to fix any gaps, then `tb ready TB-NNN`.
3. Pull into in-progress with `tb pull` (auto-picks highest-priority oldest) or `tb pull TB-NNN` to override.
4. If you specifically need to push directly, `tb start TB-NNN` still works — it just warns when the source is backlog.

**During work — backlog capture:**
When you encounter any of these, IMMEDIATELY create a backlog task:

- Out-of-scope work, deferred features
- Bugs unrelated to current task
- Workarounds, temporary solutions, tech debt
- `TODO`/`FIXME`/`HACK` in code — must reference task ID: `// TODO(TB-NNN): description`

Quick capture: `tb create "Title" -m module -d "description"`
Or run `tb scan --apply` to auto-create tasks from untagged TODO/FIXME/HACK comments.

**After coding:**

1. Update the task file (check acceptance criteria, add log entry)
2. Move with `tb done TB-NNN`
3. Commit changes with task ID in message: `feat: TB-NNN: concise description`

### CLI Reference

```
tb init [path] [--board-path=board] [--prefix=TB] [--refresh-docs]     Initialize or reconcile a board
tb board [--json]                                                      Print board status or JSON snapshot
tb create "Title" -m module [-d desc] [-p P2] [-T feature] [-s M] [-t tags] [--parent ID] [--epic] [--legacy-file]
tb ls [-t tags] [-s size] [-m module] [-T type] [-p priority] [-n N] [--parent ID] [--status backlog|ready|in-progress|code-review|done|archive|active|all] [--json]
tb mv <TB-NNN> <status>                                               Move task
tb ready <TB-NNN>                                                     Promote backlog → ready (canonical kanban commitment)
tb pull [<TB-NNN>]                                                    Pull next ready task into in-progress
tb start <TB-NNN>                                                     Start working (push-style; warns when source is backlog)
tb done <TB-NNN>                                                      Mark done
tb edit <TB-NNN> [--goal file|-] [--acceptance file|-] [--user-attention file|-] [--agent-status queued|running|success|failed|cancelled|interrupted|needs-user|none] [--review-ref value|none]   Edit metadata/body sections
tb attach <TB-NNN> <path>...                                          Copy files into task attachments
tb attach --rm <TB-NNN> <attachment-name>...                          Remove task attachments
tb assign <TB-NNN> <claude|codex>                                     Assign a runnable agent and queue pickup
tb close <TB-NNN>                                                     Archive task
tb show <TB-NNN> [--json]                                             Print task content or JSON
tb open <TB-NNN>                                                      Open in default editor
tb epic <TB-NNN> [--status active|archive|all]                        Show epic progress
tb triage [--json]                                                       Find tasks needing grooming
tb grep <pattern> [--status backlog|ready|in-progress|code-review|done|archive|active|all] [-s] [-l]   Search tasks by regex
tb scan [--apply] [--path dir]                                           Find untagged TODOs
tb regenerate                                                            Regenerate BOARD.md
```

**Commands:**

| Command | Aliases | Description |
|---------|---------|-------------|
| `init` | | Initialize board structure (creates `.tb.yaml` in project root) |
| `board` | | Print board status or JSON snapshot |
| `create` | `new` | Create a new folder-form task |
| `ls` | `list` | List and filter tasks |
| `mv` | `move` | Move task between statuses |
| `ready` | | Promote a backlog task to ready (canonical kanban commitment column) |
| `pull` | | Pull the highest-priority oldest ready task into in-progress |
| `start` | | Move task to in-progress (push-style; warns when source is backlog) |
| `done` | | Move task to done |
| `edit` | | Edit task metadata and Goal/Acceptance Criteria sections |
| `attach` | | Copy or remove task attachments |
| `assign` | | Assign claude or codex and queue daemon pickup |
| `close` | | Archive task |
| `show` | `cat` | Print task content or JSON |
| `open` | | Open task file in default editor/app |
| `epic` | | Show epic task with children and progress |
| `triage` | | Find tasks needing grooming (placeholder goals, no module, auto-created) |
| `grep` | `search` | Full-text regex search across all task files |
| `scan` | | Find untagged TODO/FIXME/HACK comments, create tasks, update source |
| `regenerate` | `regen` | Regenerate BOARD.md from directory contents |

**Defaults:** type=bug, priority=P2, size=M. Module and tags are optional.

**Status aliases:** `b`=backlog, `r`=ready, `ip`/`wip`=in-progress, `cr`/`review`=code-review, `d`=done

Task IDs use the configured prefix (default: TB). The prefix is optional in commands — `tb start 123` and `tb start TB-123` are equivalent.

**Configuration:** `tb` discovers `.tb.yaml` by walking up from the current directory. Fallback: `TB_BOARD_DIR` and `TB_PREFIX` environment variables.

**Examples:**

```
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
```

**Board architecture:** Directories are the source of truth. `BOARD.md` is auto-generated by `tb regenerate`. `.next-id` is managed by `tb create`. Never edit `BOARD.md` manually.
