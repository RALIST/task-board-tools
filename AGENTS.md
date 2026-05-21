# Repository Guidelines

## Project Structure & Module Organization

This repo builds two tools over the same markdown board format. `cli/` contains the `tb` Go CLI (`module tools/tb`) as a single `package main`; tests live beside the command files. `gui/` contains the Wails3 desktop app (`module tools/tb-gui`): exported services are in `gui/app/`, non-exported helpers in `gui/internal/`, and the Svelte 5 frontend in `gui/frontend/src/`. Wails build config and platform assets live under `gui/build/`; frontend static assets live in `gui/frontend/static/`. `docs/` is the product and architecture source of truth. `board/` is this repo's own task board; do not hand-edit generated `board/BOARD.md`.

## Where to start

Read in this order:
 **Update this files as work progresses.**

1. `docs/PROJECT.md` — product, audience, scenarios, glossary.
2. `docs/ARCHITECTURE.md` — components, on-disk format, locking rules, agent state, daemon.
3. `docs/FEATURES.md` — feature roadmap with acceptance criteria.
4. `docs/IMPLEMENTATION.md` — current milestone status + risk register.
5. `README.md`

Don't rely on this file for details — those docs are the source of truth. We have to keep them up-to-date.


## Rules

- Always read `@board/CONVENTIONS.md` for the task board workflow and follow the conventions and guidelines.
- Create new tasks for any bug/follow-up work that you identify while working on an existing task. This helps keep track of all the work that needs to be done and ensures that nothing falls through the cracks.
- Rebuild and relink cli binary after changes in /cli/; the `tb` binary is not tracked by git and must be built locally.

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
- **`AgentStatus` values**: `queued | running | success | failed | cancelled | interrupted | lost | needs-user`. `failed` means the agent runner reported a failed terminal outcome. `cancelled` is user-initiated. `interrupted` is recovery-initiated and resumable because a session id was captured (TB-130). `lost` is recovery-initiated daemon/run-state loss without a resumable session (TB-251). Stale-recovery never overwrites `cancelled`, and convention reserves writes of `interrupted`/`lost` to `RecoverStale` even though the validator accepts them from any path. Resume is offered for terminal statuses with a captured latest-run session (`interrupted`, `lost`, `failed`, `cancelled`, `success`); `queued`, `running`, and `needs-user` are blocked. `needs-user` is the agent-attention handoff (TB-182): an autonomous agent stopped because user input is required; clear with `tb edit <ID> --agent-status none`.
- **Session resume**: each run captures the agent CLI's `session_id` as a `session` JSONL event written immediately AFTER `started` (PID is durable first). Recovery's dead-PID branch reads that id: `interrupted` if present, otherwise `lost`; the GUI's Resume button is driven by the backend's captured-session flag plus terminal status, not by `interrupted` alone. Resume re-invokes the same agent CLI with its native flag (`claude -r <uuid>` / `codex exec --json resume <uuid> <prompt>`), in the parent run's persisted cwd, with the parent's `TB_`-prefixed env replayed. **Security**: only env keys prefixed `TB_` are persisted in JSONL `run_env`; credential vars (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.) never reach disk.
- **`.next-id` allocator** detects collisions on every allocation — don't bypass it.
- **Folder-form tasks.** Tasks may be stored as `<status>/<ID>.md` (file form) or `<status>/<ID>/TASK.md` (folder form, with new attachments directly under `<status>/<ID>/`, legacy `attachments/` compatibility files, and task-local `.agent-state.jsonl` / `.agent-logs/`). The contract — resolution order, lock semantics, atomic-write rules for files inside a task folder, the file → folder promotion procedure, and which paths deliberately differ between forms — is specified in [`docs/ARCHITECTURE.md` → "Folder-form tasks"](docs/ARCHITECTURE.md#folder-form-tasks). Follow-up work touching storage forms must conform to that section.

## Build, Test, and Development Commands

- `cd cli && go build -o tb .` builds the CLI.
- `cd cli && go test ./...` runs CLI tests.
- `cd gui && go test ./...` runs GUI backend tests.
- `make lint-go` runs golangci-lint for both Go modules by entering `cli/` and `gui/` explicitly; set `GOLANGCI_LINT=/path/to/golangci-lint` when the binary is not on `PATH`.
- `cd gui/frontend && npm install` installs frontend dependencies.
- `cd gui/frontend && npm run check` runs Svelte/TypeScript checks.
- `cd gui/frontend && npm run lint` runs ESLint (typescript-eslint + eslint-plugin-svelte).
- `cd gui/frontend && npm run deadcode` runs knip to surface unused exports/dependencies.
- `cd gui/frontend && npm test` runs Vitest.
- `cd gui && task dev` or `wails3 dev` starts the desktop app in development mode.
- `cd gui && task build` or `wails3 build` creates a production GUI build.

Run Go commands from `cli/` or `gui/`; the workspace root has `go.work` but is not itself a module.

## Coding Style & Naming Conventions

Format Go with `gofmt`; keep CLI code in the existing single-package style and use lower-case command/helper names such as `cmdCreate`, `moveTask`, and `writeFileAtomic`. Preserve the board invariants from `docs/ARCHITECTURE.md`: structured mutations take `.board.lock`, task writes are atomic, and `BOARD.md` is regenerated. In Svelte/TypeScript, keep components PascalCase, stores lower-case (`runsStore`, `boardStore`), and prefer typed wrappers in `gui/frontend/src/lib/api.ts` over direct binding calls.

## Testing Guidelines

Add table-driven Go tests next to the changed package (`*_test.go`). For GUI agent, watcher, daemon, and filesystem behavior, include integration-style tests when the bug depends on real processes, locks, or file events. Frontend logic tests use Vitest (`*.test.ts`); before treating UI code as complete, run `npm run check`, `npm run lint`, and (for changes touching exports or `package.json`) `npm run deadcode`. `npm run deadcode` exits non-zero when knip reports any findings; the current baseline has 13 findings tracked by TB-247, so compare your finding list against that baseline rather than treating exit 1 as a regression.

## Commit & Pull Request Guidelines

Recent commits use short, imperative subjects with a scope when useful: `cli: ship M1 ...`, `gui: ship M5 ...`, `board: groom TB-5 ...`, or `TB-26 atomic next-id writes`. Keep commits focused and avoid staging unrelated board churn. PRs should summarize behavior changes, link task IDs (`TB-26`), list verification commands, and include screenshots or smoke-test notes for GUI changes.
