# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

`task-board-tools` is being grown from a single Go CLI (in `tb/`) into a two-binary product:

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
5. `tb/CLAUDE.md` — authoritative guide for the existing CLI's internals.

Don't rely on this file for details — those docs are the source of truth.

## Current state

- M0 (documentation foundation) — committed.
- M1 (CLI extensions: `git mv tb cli`, `--json` output, agent metadata fields, `--status active|archive|all`, regenerate consistency, root `go.work`) — not started.
- M2+ — not started.

`tb/` is still its own git repo and is excluded from this repo via `.gitignore` until M1 merges it in as `cli/`.

## Build & run

CLI (current location):
```bash
cd tb && go build -o tb .
```

After M1 rename:
```bash
cd cli && go build -o tb .
# or from repo root once go.work exists:
go build ./cli
```

There is no linter config. The only tests are `tb/board_test.go` — run with `cd tb && go test ./...`.

GUI does not exist yet; planned to live in `gui/` with `wails3 build`.

## Architecture invariants (do not break)

- **Markdown is the source of truth.** Task `.md` files in status directories are canonical. `BOARD.md` is generated; never edit it.
- **Directory = status.** Moving a task = renaming the file between `backlog/`, `in-progress/`, `done/`, `archive/`.
- **`.board.lock`** (POSIX `flock`) serializes every structured mutation. The CLI takes it; the GUI delegates to the CLI; the one exception (free-form body editing in `EditTaskBody`) takes the same lock with the rules listed in `docs/ARCHITECTURE.md` → "Locking and atomic writes".
- **Atomic writes (M1).** Every task-file mutation must use temp + `os.Rename`. Direct `os.WriteFile(...".md")` is forbidden outside `cli/atomicfs.go`. This is what makes lock-free GUI reads safe.
- **Status filter semantics** (becoming canonical in M1):
  - `backlog`, `in-progress`, `done`, `archive` — concrete dirs
  - `active` = backlog + in-progress + done
  - `all` = active + archive
- **Agent state is hybrid**: `Agent` / `AgentStatus` fields in task `.md` (current state, visible to humans/CLI); `board/.agent-state/<ID>.jsonl` (append-only run history); `board/.agent-logs/<ID>/<run_id>.log` (full stdout/stderr).
- **`AgentStatus` values**: `queued | running | success | failed | cancelled`. `cancelled` is user-initiated; stale-recovery never overwrites it.
- **`.next-id` allocator** detects collisions on every allocation — don't bypass it.

## Working conventions

- All structured task mutations must call `regenerateBoard` at the end. M1 fixes the gap where `tb create` and `tb edit` skip it.
- JSON output (M1): empty result → `[]` or `{}`, never prose like "No tasks found.". Stdout = data; stderr = errors/warnings.
- Single-instance lock for `tb-gui` (M2) — only one GUI process per user; second invocation focuses the existing window.
- Daemon stale-recovery on startup (M5): tasks left in `AgentStatus: running` after a crash get reconciled by checking PID liveness + replaying JSONL.

## Critical files (CLI today)

- `tb/main.go` — command dispatch
- `tb/board.go` — config loading, `.board.lock`, ID allocation, archive logic
- `tb/task.go` — `parseTaskFile` (first 15 lines only) + `Task` struct
- `tb/move.go` — `moveTask` + `appendLogEntry` + status transition logic
- `tb/regenerate.go` — `BOARD.md` generator + task collection helpers
- `tb/create.go`, `tb/edit.go` — task creation / metadata edits (both will gain a `regenerateBoard` call in M1)

The CLI is in `package main` with no sub-packages — all `.go` files share one namespace.

## When to update docs

- After completing tasks in `docs/IMPLEMENTATION.md`, flip the marker (`☐` → `☑`) and add to "Completed work log".
- If you change an architecture invariant, update `docs/ARCHITECTURE.md` in the same change.
- If a feature's acceptance criteria shift, update `docs/FEATURES.md` first.
- Treat `docs/` as code: review it before shipping a milestone.
