# Board Conventions

## Structure

```
board/
  BOARD.md              — Generated kanban view (DO NOT edit manually)
  CONVENTIONS.md        — This file
  SKILL.md              — AI agent instructions for using the board
  .next-id              — Counter for next TB-NNN ID
  backlog/              — Prioritized, ready to pick up
  in-progress/          — Currently being worked on (max 2 tasks)
  done/                 — Completed (archive, clean periodically)
```

CLI tool `tb` manages board operations (create, move, edit, attach, assign, list, JSON views, regenerate).

**Directories are the source of truth.** `BOARD.md` is a generated view — never edit it manually.

Directory = status. Moving a task entry between directories = status change.

**CRITICAL: A task file must exist in exactly ONE directory.** When moving a task, always use `tb mv`/`tb start`/`tb done` which handle the move atomically. Never copy task files.

## Task File Format

Default path: `<status>/TB-NNN/TASK.md` (e.g., `backlog/TB-001/TASK.md`).

Legacy path: `<status>/TB-NNN.md`. Create legacy files only when you intentionally pass `tb create --legacy-file`; they are kept for compatibility and do not support in-place attachments.

Folder-form tasks store new attachments directly under `<status>/TB-NNN/`. Legacy `attachments/<filename>` entries remain supported for compatibility when older tasks are promoted.

**ID allocation:** Handled automatically by `tb create`. The `.next-id` file is the counter, protected by file locking for concurrent access.

```markdown
# TB-NNN: Short title

**Type:** feature | bug | tech-debt | improvement | spike
**Priority:** P0 | P1 | P2
**Size:** S | M | L | XL
**Module:** module-name (optional)
**Tags:** comma-separated tags (optional)
**Branch:** feat/branch-name (set when work starts)
**Parent:** TB-NNN (optional — links to parent epic)

## Goal

One-sentence objective.

## Context

Why this task exists. Link to the task or session where it was discovered.

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2

## Attachments

## Related Tasks

- **TB-XXX** — Title (relationship: prerequisite | blocked by | shares infrastructure | complementary | depends on)

## Log

- YYYY-MM-DD: Created
- YYYY-MM-DD: Started — moved to in-progress
- YYYY-MM-DD: Done — [summary of what was done]
```

### Task types

| Type          | When to use                                 | Examples                                             |
| ------------- | ------------------------------------------- | ---------------------------------------------------- |
| `feature`     | New capability                              | Implement search, add export format                  |
| `bug`         | Broken behavior found during work           | Crash on empty input, wrong calculation              |
| `tech-debt`   | Shortcuts, workarounds, temporary solutions | Hardcoded limit, missing error handling, copied code |
| `improvement` | Enhancement to existing functionality       | Better UX, faster lookup                             |
| `spike`       | Research or investigation needed            | Evaluate approaches, benchmark alternatives          |

## Rules

### Before coding

1. Run `tb ls` for current state
2. Pick a task or create one with `tb create "Title"`
3. Start it with `tb start TB-NNN`
4. Set the `Branch` field

### During work

- Add notes to the task's Log section as you make progress
- If blocked, note it in the Log

### After work

- Check all acceptance criteria boxes
- Run `tb done TB-NNN`
- Add final Log entry with summary

### Backlog capture

Create backlog tasks when you encounter:

- Out-of-scope work or deferred features
- Bugs unrelated to current task
- Workarounds or temporary solutions
- `TODO`/`FIXME`/`HACK` in code — reference task ID: `// TODO(TB-NNN): description`
- Performance concerns or improvement ideas

Quick capture: `tb create "Title" -m module -d "description"`

### Board hygiene

- P0 = drop everything. P1 = next up. P2 = when convenient
- Size guide: S = <1h, M = 1-4h, L = 4-8h, XL = multi-session
- Tags: comma-separated. Filter with `tb ls -t tag`

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

## BOARD.md

`BOARD.md` is **auto-generated** by `tb regenerate`. Do not edit it manually.

## Project Refresh

Existing boards can refresh generated project files without reinitializing tasks:

```
tb init
```

The command reads `.tb.yaml` for the current board path and prefix, rewrites generated files such as `CONVENTIONS.md` and `SKILL.md`, and saves previous copies as `*.bak` files for manual merge of local customizations. The old `--refresh-docs` flag is accepted for scripts, but plain `tb init` is the normal refresh path.

## CLI Reference

```
tb init [path] [--board-path=board] [--prefix=TB] [--refresh-docs]
tb board [--json]
tb create "Title" [-m module] [-d desc] [-p P2] [-T bug] [-s M] [-t tags] [--parent ID] [--epic] [--legacy-file]
tb ls [-t tags] [-s size] [-m module] [-T type] [-p priority] [-n N] [--parent ID] [--status backlog|in-progress|done|archive|active|all] [--json]
tb mv <TB-NNN> <status>                                                    — Move task between statuses
tb start <TB-NNN>                                                          — Move to in-progress
tb done <TB-NNN>                                                           — Move to done
tb edit <TB-NNN> [-p P0] [-T type] [-s M] [-m module] [-t tags] [-a claude|codex] [--agent-status queued|running|success|failed|cancelled] [--title "New title"] [--goal file|-] [--acceptance file|-]
tb attach <TB-NNN> <path>...                                               — Copy files into task attachments
tb attach --rm <TB-NNN> <attachment-name>...                               — Remove task attachments
tb assign <TB-NNN> <claude|codex>                                          — Assign a runnable agent and queue pickup
tb close <TB-NNN>                                                          — Archive task
tb show <TB-NNN> [--json]                                                  — Print task content or JSON
tb open <TB-NNN>                                                           — Open in default editor
tb epic <TB-NNN> [--status active|archive|all]                             — Show epic progress and children
tb triage [--json]                                                            — Find tasks needing grooming
tb grep <pattern> [--status backlog|in-progress|done|archive|active|all] [-s] [-l] — Search tasks by regex
tb scan [--apply] [--path dir]                                                — Find untagged TODOs, create tasks
tb regenerate                                                                 — Regenerate BOARD.md
```

**Defaults:** type=bug, priority=P2, size=M.

**Status aliases:** `b`=backlog, `ip`=in-progress, `d`=done

**Examples:**

```
tb create "Fix crash on empty input" -m core -p P1 -s S -t quick-win
tb create "Search system" --epic -m editor          # Create an epic
tb create "Search indexing" --parent 1 -m editor    # Create child of epic
tb create "Legacy integration probe" --legacy-file   # Explicit old <status>/<ID>.md layout
tb init                                   # Refresh generated project files with .bak backups
tb ls -T bug -p P1                       # P1 bugs
tb ls -t testing                         # All test-related tasks
tb ls --parent 1                         # Children of an epic
tb start 1                               # Prefix is optional — "1" = "TB-1"
tb done 1
tb epic 1                                # View epic progress
tb grep "auth"                           # Search all tasks
tb scan --apply                          # Create tasks from TODOs
```
