# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

`task-board-tools` is being grown from a single Go CLI (in `cli/`) into a two-binary product:

- **`tb` CLI** — terminal-first task tracker; markdown files in status directories; owns all structured mutations via `.board.lock`.
- **`tb-gui`** — Wails3 alpha + Svelte 5 desktop app; kanban UI; runs an embedded agent daemon that can hand tasks to `claude` or `codex` CLIs.

Treat `docs/` as the authoritative spec, and keep it current when behavior or architecture changes.

## Task board

@board/CONVENTIONS.md
@board/SKILL.md

## Where to start

Read in this order:

1. `docs/PROJECT.md` — product, audience, scenarios, glossary.
2. `docs/ARCHITECTURE.md` — components, on-disk format, locking rules, agent state, daemon.
3. `docs/FEATURES.md` — feature roadmap with acceptance criteria.
4. `docs/IMPLEMENTATION.md` — current milestone status + risk register. **Update this file as work progresses.**
5. `cli/CLAUDE.md` — authoritative guide for the existing CLI's internals.

Don't rely on this file for details — those docs are the source of truth.

## Current state

- M0-M8 are implemented: CLI extensions, Wails/Svelte GUI, mutations, agent runs, daemon, groom flow, polish, folder-form tasks, and attachments.
- Current active work is tracked in `board/`; `docs/IMPLEMENTATION.md` is the status log.

The CLI now lives at `cli/` and is tracked directly by this repo. The original `tb/` git history was preserved separately as `../task-board-tools-tb-history.bundle`.

## Build & run

```bash
cd cli && go build -o tb .
# or from repo root via go.work:
go build ./cli   # produces ./tb at repo root (module is tools/tb)
```

Run checks from the module directories:

```bash
cd cli && go test ./...
cd gui && go test ./...
cd gui/frontend && npm install
cd gui/frontend && npm run check
cd gui/frontend && npm test
cd gui && task dev      # or: wails3 dev -config ./build/config.yml
cd gui && task build    # or: wails3 build -config ./build/config.yml
```

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
- **Agent state is hybrid**: `Agent` / `AgentStatus` fields in task `.md` (current state, visible to humans/CLI); file-form tasks use `board/.agent-state/<ID>.jsonl` and `board/.agent-logs/<ID>/<run_id>.log`; folder-form tasks use `<status>/<ID>/.agent-state.jsonl` and `<status>/<ID>/.agent-logs/<run_id>.log`.
- **`AgentStatus` values**: `queued | running | success | failed | cancelled`. `cancelled` is user-initiated; stale-recovery never overwrites it.
- **`.next-id` allocator** detects collisions on every allocation — don't bypass it.
- **Folder-form tasks.** Tasks may be stored as `<status>/<ID>.md` (file form) or `<status>/<ID>/TASK.md` (folder form, with new attachments directly under `<status>/<ID>/`, legacy `attachments/` compatibility files, and task-local `.agent-state.jsonl` / `.agent-logs/`). The contract — resolution order, lock semantics, atomic-write rules for files inside a task folder, the file → folder promotion procedure, and which paths deliberately differ between forms — is specified in [`docs/ARCHITECTURE.md` → "Folder-form tasks"](docs/ARCHITECTURE.md#folder-form-tasks). Follow-up work touching storage forms must conform to that section.

## Working conventions

- All structured task mutations call `regenerateBoard` at the end so `BOARD.md` never lags the directory state and the GUI watcher (M2+) gets a single fsnotify signal per mutation.
- JSON output (M1): empty result → `[]` or `{}`, never prose like "No tasks found.". Stdout = data; stderr = errors/warnings.
- `tb-gui` is single-instance; a second invocation focuses the existing window.
- Daemon stale-recovery on startup: tasks left in `AgentStatus: running` after a crash get reconciled by checking PID liveness + replaying JSONL.
- use `frontend-design` skill for GUI work
- always run code review session after each meaningful unit of work through /codex:adversarial-review or  `fullstack-code-reviewer`
- rebuild and install cli binary after any changes in master branch to keep local bin up to date with latest changes.

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
