# tb — Task Board CLI

A lightweight, zero-dependency Go CLI for managing a markdown task board. Concurrency-safe, works from any directory inside a project.

## Install

```bash
cd cli && go build -o tb . && ln -sf "$(pwd)/tb" ~/.local/bin/tb
```

## Setup

Initialize a board in your project:

```bash
tb init                                             # creates ./board/ with PR- prefix
tb init . --board-path=.claude/board --prefix=WS    # custom board path and prefix
```

This creates a `.tb.yaml` file at the project root and the board directory structure.

### `.tb.yaml` format

```yaml
board: .claude/board
prefix: WS
wip_limit: 2
scan_extensions: .go,.ts,.tsx,.svelte
```

- `board` — relative path from `.tb.yaml` to the board directory (default: `board`)
- `prefix` — task ID prefix used in filenames and headers (default: `PR`)
- `wip_limit` — threshold for the `tb start` WIP warning (default: `2`; warning only, not enforced)
- `scan_extensions` — comma-separated file extensions for `tb scan` (default: `.go,.ts,.svelte,.js,.tsx,.jsx`)

### Discovery

`tb` finds the board by walking up from the current directory looking for `.tb.yaml`. Fallback: `TB_BOARD_DIR` environment variable (uses `PR` prefix).

### Re-initialization

Running `tb init` again in an existing project updates `.tb.yaml` without touching existing tasks. Explicitly provided flags override existing values; omitted flags keep their current values.

```bash
tb init . --prefix=NEW    # changes prefix, keeps existing board path
```

## Commands

| Command | Example | Description |
|---------|---------|-------------|
| `create` | `tb create "Fix crash" -m editor -d "Modal crashes on open"` | Create a folder-form task (defaults: type=bug, priority=P2, size=M) |
| `epic` | `tb epic 32` | Show epic with active children and progress |
| `ls` | `tb ls -T bug -p P1 -m editor` | List/filter tasks sorted by priority (shows all by default; use `-n N` to limit) |
| `start` | `tb start 123` | Move task to in-progress (warns above `wip_limit`) |
| `done` | `tb done 123` | Move task to done |
| `mv` | `tb mv 123 done` | Move task to any status |
| `edit` | `tb edit 123 -p P1 -s L -m ui` | Edit task metadata (priority, type, size, module, tags) |
| `attach` | `tb attach 123 screenshot.png notes.pdf` | Copy files into a folder-form task directory |
| `close` | `tb close 123` | Archive task (moves to `archive/`) |
| `show` | `tb show 123` | Print task content to stdout |
| `open` | `tb open 123` | Open task in default editor/app |
| `triage` | `tb triage` | Find tasks needing grooming |
| `grep` | `tb grep "lint"` | Full-text regex search across all tasks |
| `scan` | `tb scan --apply` | Find untagged TODOs, create tasks, update source |
| `regenerate` | `tb regenerate` | Regenerate BOARD.md from directories |
| `init` | `tb init` | Initialize a new board |

The prefix is optional in commands — `tb start 123` and `tb start WS-123` are equivalent (when prefix is `WS`).

## Create flags

```
-m module        Module name (optional)
-d "description" Goal/description (optional)
-p P0|P1|P2|P3   Priority (default: P2)
-T type          bug|feature|tech-debt|improvement|spike (default: bug)
-s S|M|L|XL      Size (default: M)
-t tag1,tag2     Tags, comma-separated (optional)
--parent ID      Parent epic task ID (links child to parent)
--epic           Create as epic (type=feature, tag=epic)
--legacy-file    Create legacy <status>/<ID>.md instead of <status>/<ID>/TASK.md
```

By default, `tb create` writes `board/<status>/<ID>/TASK.md`, prints that path, and includes empty `## Attachments` plus `## Log` sections. The `--legacy-file` escape hatch is for boards or scripts that intentionally still need the older single-file layout; legacy file tasks do not support in-place attachments.

Minimal create — just a title:

```bash
tb create "Quick bug note"
tb create "Old integration probe" --legacy-file
```

## List flags

```
-t tag           Filter by tag (substring match)
-s size          Filter by size (exact)
-m module        Filter by module (substring match)
-T type          Filter by type (exact)
-p priority      Filter by priority (exact)
--parent ID      Filter by parent epic ID
--status status  Which directory: b, ip, d, or "all" (default: backlog)
-n N             Limit results to N (default: no limit, shows all)
```

Combine filters freely:

```bash
tb ls -T bug -p P1 -m editor -s S
tb ls --status all                  # everything across all statuses
```

## Status aliases

| Alias | Status |
|-------|--------|
| `b` | backlog |
| `ip` | in-progress |
| `d` | done |

## Grep — full-text search across tasks

```bash
tb grep "lint"                           # case-insensitive regex search across all tasks
tb grep "autocomplete|proofreading"      # regex alternation
tb grep "lint" -l                        # compact: task IDs + match counts only
tb grep "lint" --status backlog          # search only backlog tasks
tb grep "lint" -s                        # case-sensitive search
```

Searches full file content (title, goal, context, acceptance criteria, log — everything). Default: case-insensitive, all statuses. Alias: `tb search`.

### Grep flags

```
--status status  Which directory: b, ip, d, or "all" (default: all)
-l               Compact output — show only task IDs and match counts
-s               Case-sensitive search (default: case-insensitive)
```

## Attachments

```bash
tb attach 123 screenshot.png notes.pdf
```

`tb attach` copies regular files directly into the task directory (`<status>/<ID>/<filename>`) while holding `.board.lock`. If the task is still in legacy file form (`<status>/<ID>.md`), the command promotes it to folder form (`<status>/<ID>/TASK.md`) and preserves the existing markdown body and log history.

Collision policy: attachment names are the source file basenames, and `tb attach` refuses to overwrite task internals or existing attachments. Existing legacy files under `<status>/<ID>/attachments/<filename>` remain readable and removable during the compatibility period; when both locations contain the same basename, use `attachments/<filename>` to target the legacy file explicitly.

Reserved attachment names are excluded from attachment behavior: `TASK.md`, `attachments`, any dotfile or dotdir (including `.agent-state.jsonl`, `.agent-logs/`, `.attach.*` staging directories, and `.*.tmp.*` temp files), path traversal, and path separators except the legacy `attachments/<filename>` reference accepted by remove and GUI open paths.

## Scan — auto-create tasks from TODOs

```bash
tb scan                              # dry-run: preview what would be created
tb scan --apply                      # create tasks + update source comments
tb scan --path internal/ai --apply   # scoped to a directory
```

Finds untagged `TODO`/`FIXME`/`HACK`/`WORKAROUND` comments in files matching `scan_extensions` (default: `.go`, `.ts`, `.tsx`, `.jsx`, `.js`, `.svelte`). Creates backlog tasks with type and module inferred from the comment and file path. Updates source comments in-place using the configured prefix:

```
// TODO: refactor this  →  // TODO(WS-1200): refactor this
```

Idempotent — already-tagged comments are skipped.

## Triage — find tasks needing grooming

```bash
tb triage
```

Surfaces backlog tasks that have:
- No priority
- No module
- Placeholder goal ("to be filled")
- Placeholder acceptance criteria
- Auto-created by `tb scan`

## Epics — hierarchical task grouping

Epics are parent tasks that group related sub-tasks. A task becomes an epic when it has the `epic` tag.

```bash
# Create an epic
tb create "Search system" --epic -m editor -d "Full-text search"

# Create children (--parent auto-tags parent as epic if needed)
tb create "Search indexing" --parent 1 -m editor -s M
tb create "Search UI" --parent 1 -m editor -s S

# View epic progress
tb epic 1

# List only children of an epic
tb ls --parent 1

# Done warning: completing an epic with open children shows a warning
tb done 1

# Include archived epic/children only when you ask for that scope
tb epic 1 --status all
```

Children have a `**Parent:** PREFIX-NNN` field in their metadata. The parent's `## Subtasks` section is updated automatically when children are created with `--parent`.

BOARD.md includes an **Epics** section for active epics and a separate **Finished Epics** section for epics with status `done`. Both show progress (done/total) for each epic. Archived tasks are closed/hidden: they are available through explicit status filters such as `tb ls --status archive`, `tb ls --status all`, or `tb epic --status all`, but they are not treated as done for default board or epic progress.

## Concurrency

All board writers (`create`, `mv`, `start`, `done`, `close`, `attach`, `scan --apply`, `regenerate`) acquire an exclusive file lock on `.board.lock`. Safe for multiple agents running in parallel.

## Board structure

```
project/
  .tb.yaml          Configuration (board path, prefix, wip_limit, scan_extensions)
  board/             (or custom path)
    backlog/         Tasks ready to pick up
    in-progress/     Currently being worked on (warns above wip_limit)
    done/            Completed
    archive/         Closed tasks (via `tb close`)
    .next-id         Auto-incrementing ID counter
    .board.lock      Concurrency lock file
    BOARD.md         Auto-generated kanban view
```

New tasks are folder-form by default: `status/PREFIX-NNN/TASK.md`, with new attachments stored directly in `status/PREFIX-NNN/`. Legacy tasks may still exist as `status/PREFIX-NNN.md`, and `tb create --legacy-file` can create one intentionally. `BOARD.md` is regenerated automatically by `tb create`/`mv`/`start`/`done`/`close`/`attach`/`scan --apply`.
