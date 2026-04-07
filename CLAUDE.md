# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build

```bash
go build -o tb .
```

There are no tests, no linter configuration, and no external dependencies (only stdlib).

## Architecture

Single-package (`main`) Go CLI. All source files are in the root directory — no sub-packages.

**Command routing:** `main.go` dispatches `os.Args[1]` to `cmd*` functions. Commands that don't need config (`init`, `help`) run before `loadProjectConfig()`.

**Config & infrastructure (`board.go`):** Global `cfg` (`tbConfig`) holds `RootDir`, `BoardDir`, and `Prefix`. Discovered by walking up from CWD looking for `.tb.yaml`, falling back to `TB_BOARD_DIR` env var. Custom minimal YAML parser (no external deps). File-based exclusive locking via `syscall.Flock` on `.board.lock` serializes all mutations.

**Task model (`task.go`):** Tasks are markdown files (`PREFIX-NNN.md`) in status directories. `parseTaskFile()` extracts metadata from the first 15 lines only. Metadata format is `**Field:** value` (bold key with colon). The `Task` struct has no JSON/YAML tags — it's populated by line-by-line parsing.

**Key patterns:**
- **Flag reordering:** Go's `flag` package stops at the first non-flag arg. `reorderArgs()` in `create.go` separates flags from positional args so `tb create "Title" -m mod` works. Used by `create`, `grep`, and `init`.
- **ID allocation:** `allocateID()` reads `.next-id`, returns current value, writes incremented value. Must hold board lock.
- **Board regeneration:** `regenerateBoard()` writes `BOARD.md` atomically (temp file + rename). Called automatically after every mutation (create, mv, start, done, close, scan).
- **Status = directory:** Moving a task means writing to the destination directory and removing from source. `moveTask()` in `move.go` handles this with log entry appending.

**File responsibilities:**
- `create.go` — task creation, parent/epic linking, subtask section management
- `move.go` — mv/start/done/close commands, `normalizeTaskID()` (accepts bare numbers or prefixed IDs)
- `list.go` — filtering/sorting (priority rank then numeric ID), tabwriter output
- `grep.go` — regex search across task file contents, normalizes BRE `\|` to ERE `|`
- `scan.go` — walks source tree for untagged TODO/FIXME/HACK/WORKAROUND, creates tasks, patches source with `PREFIX-NNN` references
- `regenerate.go` — builds BOARD.md with epics, in-progress, backlog, and done sections
- `templates.go` — generates CONVENTIONS.md and SKILL.md during `tb init`
- `epic.go` — finds children by matching `Parent` field, shows progress
- `triage.go` — surfaces tasks with placeholder goals, missing modules, or auto-created by scan
