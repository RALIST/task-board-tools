# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

`task-board-tools` is being grown from a single Go CLI (in `cli/`) into a two-binary product:

- **`tb` CLI** — terminal-first task tracker; markdown files in status directories; owns all structured mutations via `.board.lock`.
- **`tb-gui`** — Wails3 alpha + Svelte 5 desktop app; kanban UI; runs an embedded agent daemon that can hand tasks to `claude` or `codex` CLIs.

The transformation is in progress. Treat `docs/` as the authoritative spec.

## Task board

@board/CONVENTIONS.md

## Where to start

Read in this order:

1. `docs/PROJECT.md` — product, audience, scenarios, glossary.
2. `docs/ARCHITECTURE.md` — components, on-disk format, locking rules, agent state, daemon.
3. `docs/FEATURES.md` — feature roadmap M0–M7 with acceptance criteria.
4. `docs/IMPLEMENTATION.md` — current milestone status + risk register. **Update this file as work progresses.**
5. `cli/CLAUDE.md` — authoritative guide for the existing CLI's internals.

Don't rely on this file for details — those docs are the source of truth.

## Current state

- M0 (documentation foundation) — committed.
- M1 (CLI extensions: `cli/` rename, `--json` output, agent metadata fields, `--status active|archive|all`, regenerate consistency, root `go.work`) — done.
- M2+ — not started.

The CLI now lives at `cli/` and is tracked directly by this repo. The original `tb/` git history was preserved separately as `../task-board-tools-tb-history.bundle`.

## Build & run

```bash
cd cli && go build -o tb .
# or from repo root via go.work:
go build ./cli   # produces ./tb at repo root (module is tools/tb)
```

There is no linter config. The only tests are `cli/board_test.go` — run with `cd cli && go test ./...`.

GUI does not exist yet; planned to live in `gui/` with `wails3 build`.

## Architecture invariants (do not break)

- **Markdown is the source of truth.** Task `.md` files in status directories are canonical. `BOARD.md` is generated; never edit it.
- **Directory = status.** Moving a task = renaming the file between `backlog/`, `in-progress/`, `done/`, `archive/`.
- **`.board.lock`** (POSIX `flock`) serializes every structured mutation. The CLI takes it; the GUI delegates to the CLI; the one exception (free-form body editing in `EditTaskBody`) takes the same lock with the rules listed in `docs/ARCHITECTURE.md` → "Locking and atomic writes".
- **Atomic writes.** Every task-file mutation must use `writeFileAtomic` (temp + fsync + `os.Rename`). Direct `os.WriteFile(...".md")` is forbidden outside `cli/atomicfs.go`. This is what makes lock-free GUI reads safe.
- **Status filter semantics** (`cli/board.go:resolveStatusFilter`):
  - `backlog`, `in-progress`, `done`, `archive` — concrete dirs (single)
  - `active` = backlog + in-progress + done
  - `all` = active + archive
  - aliases: `b`=backlog, `ip`/`wip`=in-progress, `d`=done
- **Agent state is hybrid**: `Agent` / `AgentStatus` fields in task `.md` (current state, visible to humans/CLI); `board/.agent-state/<ID>.jsonl` (append-only run history); `board/.agent-logs/<ID>/<run_id>.log` (full stdout/stderr).
- **`AgentStatus` values**: `queued | running | success | failed | cancelled`. `cancelled` is user-initiated; stale-recovery never overwrites it.
- **`.next-id` allocator** detects collisions on every allocation — don't bypass it.
- **Folder-form tasks.** Tasks may be stored as `<status>/<ID>.md` (file form) or `<status>/<ID>/TASK.md` (folder form, with `attachments/` and task-local `.agent-state.jsonl` / `.agent-logs/`). The contract — resolution order, lock semantics, atomic-write rules for files inside a task folder, the file → folder promotion procedure, and which paths deliberately differ between forms — is specified in [`docs/ARCHITECTURE.md` → "Folder-form tasks"](docs/ARCHITECTURE.md#folder-form-tasks). Implementations of the TB-93 epic must conform to that section.

## Working conventions

- All structured task mutations call `regenerateBoard` at the end so `BOARD.md` never lags the directory state and the GUI watcher (M2+) gets a single fsnotify signal per mutation.
- JSON output (M1): empty result → `[]` or `{}`, never prose like "No tasks found.". Stdout = data; stderr = errors/warnings.
- Single-instance lock for `tb-gui` (M2) — only one GUI process per user; second invocation focuses the existing window.
- Daemon stale-recovery on startup (M5): tasks left in `AgentStatus: running` after a crash get reconciled by checking PID liveness + replaying JSONL.
- use `frontend-design` skill for GUI work
- always run code review session after each meaningful unit of work through /codex:adversarial-review or  `fullstack-code-reviewer`

## Critical files (CLI today)

- `cli/main.go` — command dispatch
- `cli/board.go` — config loading, `.board.lock`, ID allocation, archive logic
- `cli/task.go` — `parseTaskFile` (first 15 lines only) + `Task` struct
- `cli/move.go` — `moveTask` + `appendLogEntry` + status transition logic
- `cli/regenerate.go` — `BOARD.md` generator + task collection helpers
- `cli/create.go`, `cli/edit.go` — task creation / metadata edits
- `cli/atomicfs.go` — `writeFileAtomic`; the only sanctioned write path for task `.md` files
- `cli/json_output.go` — `--json` serializers for Task and BoardSnapshot

The CLI is in `package main` with no sub-packages — all `.go` files share one namespace.

## When to update docs

- After completing tasks in `docs/IMPLEMENTATION.md`, flip the marker (`☐` → `☑`) and add to "Completed work log".
- If you change an architecture invariant, update `docs/ARCHITECTURE.md` in the same change.
- If a feature's acceptance criteria shift, update `docs/FEATURES.md` first.
- Treat `docs/` as code: review it before shipping a milestone.
