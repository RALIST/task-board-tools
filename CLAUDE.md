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
cd gui/frontend && npm run lint        # ESLint (typescript-eslint + Svelte 5)
cd gui/frontend && npm run deadcode    # knip — unused exports/deps
cd gui/frontend && npm test
cd gui && task dev      # or: wails3 dev -config ./build/config.yml
cd gui && task build    # or: wails3 build -config ./build/config.yml
```

## Architecture invariants (do not break)

- **Markdown is the source of truth.** Task `.md` files in status directories are canonical. `BOARD.md` is generated; never edit it.
- **Directory = status.** Moving a task = renaming the file between `backlog/`, `ready/`, `in-progress/`, `code-review/`, `done/`, `archive/`. The canonical kanban flow is `backlog → ready → in-progress → code-review → done → archive`.
- **`.board.lock`** (POSIX `flock`) serializes every structured mutation. The CLI takes it; the GUI delegates to the CLI; the one exception (free-form body editing in `EditTaskBody`) takes the same lock with the rules listed in `docs/ARCHITECTURE.md` → "Locking and atomic writes".
- **Atomic writes.** Every task-file mutation must use `writeFileAtomic` (temp + fsync + `os.Rename`). Direct `os.WriteFile(...".md")` is forbidden outside `cli/atomicfs.go`. This is what makes lock-free GUI reads safe.
- **Status filter semantics** (`cli/board.go:resolveStatusFilter`):
  - `backlog`, `ready`, `in-progress`, `code-review`, `done`, `archive` — concrete dirs (single)
  - `active` = backlog + ready + in-progress + code-review + done
  - `all` = active + archive
  - aliases: `b`=backlog, `r`=ready, `ip`/`wip`=in-progress, `cr`/`review`=code-review, `d`=done
- **Pull-based mechanics.** Tasks flow forward, never sideways:
  - `tb ready <ID>` is the commitment point from backlog → ready; it runs the triage gate (priority + non-placeholder goal) and rejects un-groomed tasks.
  - `tb pull` (no arg) pulls the highest-priority oldest ready task into in-progress. `tb pull <ID>` overrides selection.
  - `tb start <ID>` still works push-style for compatibility but warns when the source is backlog (which skips the canonical commitment column).
  - Failed code review returns the task to `ready` (not `backlog`) with the `review-failed` tag — already groomed, just needs rework.
- **WIP limits.** `.tb.yaml` may declare `wip_limit_ready`, `wip_limit_in_progress`, `wip_limit_code_review` (the legacy scalar `wip_limit` seeds in-progress for backwards compatibility). `wip_enforcement: warn` (default) emits a stderr warning when a move would exceed the limit; `strict` blocks the move with a non-zero exit. Enforcement runs for `tb ready`, `tb pull`, `tb start`, and `tb mv` against the destination column.
- **Agent state is hybrid**: `Agent` / `AgentStatus` fields in task `.md` (current state, visible to humans/CLI); file-form tasks use `board/.agent-state/<ID>.jsonl` and `board/.agent-logs/<ID>/<run_id>.log`; folder-form tasks use `<status>/<ID>/.agent-state.jsonl` and `<status>/<ID>/.agent-logs/<run_id>.log`.
- **`AgentStatus` values**: `queued | running | success | failed | cancelled | interrupted | needs-user`. `cancelled` is user-initiated and `interrupted` is recovery-initiated (TB-130) — stale-recovery never overwrites `cancelled`, and convention reserves writes of `interrupted` to `RecoverStale` even though the validator accepts it from any path. `needs-user` is the agent-attention handoff (TB-182): an autonomous agent stopped because user input is required; clear with `tb edit <ID> --agent-status none`.
- **Session resume (TB-130)**: each run captures the agent CLI's `session_id` as a `session` JSONL event written immediately AFTER `started` (PID is durable first). Recovery's dead-PID branch reads that id: `interrupted` if present (Resume button surfaces in the GUI), otherwise existing `failed`. Resume re-invokes the same agent CLI with its native flag (`claude -r <uuid>` / `codex exec --json resume <uuid> <prompt>`), in the parent run's persisted cwd, with the parent's `TB_`-prefixed env replayed. **Security**: only env keys prefixed `TB_` are persisted in JSONL `run_env`; credential vars (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.) never reach disk.
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
- use subagents for speed up work and parallelization where possible, but coordinate through the main agent to maintain a single source of truth
-  
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
