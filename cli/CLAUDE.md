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

**Config & infrastructure (`board.go`):** Global `cfg` (`tbConfig`) holds `RootDir`, `BoardDir`, `Prefix`, `WipLimit`, and `ScanExtensions`. Discovered by walking up from CWD looking for `.tb.yaml`, falling back to `TB_BOARD_DIR` env var. Custom minimal YAML parser (no external deps). File-based exclusive locking via `syscall.Flock` on `.board.lock` serializes all mutations. `archiveTask()` lives here (used by `tb close`) — it moves the task file into `archive/` with a log entry instead of deleting it.

**Task model (`task.go`):** Tasks are markdown files (`PREFIX-NNN.md`) in status directories. `parseTaskFile()` extracts metadata from the first 20 lines only (bumped from 15 in M1 to fit `Agent`/`AgentStatus`). Metadata format is `**Field:** value` (bold key with colon). The `Task` struct carries camelCase JSON tags consumed by `cli/json_output.go` (see `--json` flag on `ls`, `show`, `board`).

**Key patterns:**
- **Flag reordering:** Go's `flag` package stops at the first non-flag arg. `reorderArgs()` in `create.go` separates flags from positional args so `tb create "Title" -m mod` works. Used by `create`, `grep`, and `init`.
- **ID allocation:** `allocateID()` reads `.next-id`, returns current value, writes incremented value. Must hold board lock.
- **Board regeneration:** `regenerateBoard()` writes `BOARD.md` atomically (temp file + rename). Called automatically after every structured mutation (create, edit, mv, start, done, close, scan). After M1, `tb create` and `tb edit` also regenerate, so `BOARD.md` never lags the directory state.
- **Atomic writes:** Every task `.md` mutation goes through `writeFileAtomic` in `atomicfs.go` (temp + fsync + rename). Direct `os.WriteFile` on a task `.md` file is forbidden outside `atomicfs.go` — verifiable via `grep -nE 'os\.WriteFile\([^)]*\.md' cli/`.
- **Status = directory:** Moving a task means writing to the destination directory and removing from source. `moveTask()` in `move.go` handles this with log entry appending.
- **Status filter (`resolveStatusFilter`):** `backlog|in-progress|done|archive` map to single dirs; `active` = backlog+in-progress+done; `all` adds archive. Aliases: `b`, `ip`/`wip`, `d`. `findTask` searches all four dirs so archived tasks can be moved back.

**File responsibilities:**
- `create.go` — task creation, parent/epic linking, subtask section management
- `move.go` — mv/start/done commands, `normalizeTaskID()` (accepts bare numbers or prefixed IDs), `appendLogEntry()`. `start` warns when in-progress count ≥ `cfg.WipLimit`. `close` delegates to `archiveTask()` in `board.go`.
- `edit.go` — edits metadata fields (priority, type, size, module, tags, agent, agent-status) in place, appends a log entry, regenerates BOARD.md. `--agent-status` validates against the enum (`queued|running|success|failed|cancelled`).
- `list.go` — filtering/sorting (priority rank then numeric ID), tabwriter output
- `grep.go` — regex search across task file contents, normalizes BRE `\|` to ERE `|`
- `scan.go` — walks source tree for untagged TODO/FIXME/HACK/WORKAROUND, creates tasks, patches source with `PREFIX-NNN` references. File extensions come from `cfg.ScanExtensions` (configured via `.tb.yaml`).
- `regenerate.go` — builds BOARD.md with active epics, finished epics (epics whose own status is `done`), in-progress, backlog, and recently-done sections. `cmdBoard` also supports `--json` (delegates to `emitBoardJSON`).
- `json_output.go` — `marshalTask`, `emitTasksJSON`, `emitShowJSON`, `buildBoardSnapshot`, `emitBoardJSON`. Wire shape uses camelCase keys; empty selections render as `[]` not prose.
- `atomicfs.go` — `writeFileAtomic`: the only sanctioned write path for task `.md` files.
- `templates.go` — generates CONVENTIONS.md and SKILL.md during `tb init`
- `epic.go` — finds children by matching `Parent` field, shows progress
- `triage.go` — surfaces tasks with placeholder goals, missing modules, or auto-created by scan
