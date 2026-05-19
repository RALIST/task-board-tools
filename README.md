# task-board-tools

Markdown-based task tracker with optional AI-agent execution.

Two binaries built from this repo:

| Binary | Path | Purpose |
|--------|------|---------|
| **`tb`** | `cli/` | Terminal CLI. Manages the board, owns all structured mutations. Used by humans, the GUI, and AI agents. |
| **`tb-gui`** | `gui/` | Desktop app (Wails3 + Svelte 5). Kanban board with DnD, filters, markdown editor, attachments, agent assignment, live updates, and settings. |

Tasks are plain Markdown files in directories (`backlog/`, `ready/`, `in-progress/`, `code-review/`, `done/`, `archive/`) — the directory is the status, and the canonical kanban flow is `backlog → ready → in-progress → code-review → done → archive`. `ready` is the commitment column: promote groomed backlog tasks into it with `tb ready <ID>`, then `tb pull` to pull the next one into in-progress. Current tasks default to folder form (`board/backlog/TB-123/TASK.md`) so attachments and task-local agent artifacts can live beside the task. Legacy single-file tasks (`TB-123.md`) remain readable. No database. No server.

## Why

You want a tracker that:
- Lives in your repo, survives without external services.
- Plays well with AI coding agents — assign Claude or Codex to a task and let it run.
- Is browseable in the terminal AND in a real kanban GUI.

## Docs

- [docs/PROJECT.md](docs/PROJECT.md) — what this is, who it's for, scenarios
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — components, on-disk format, locking, agent state
- [docs/FEATURES.md](docs/FEATURES.md) — feature list with acceptance criteria
- [docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md) — milestones, risks, status

## Quick Start

```bash
# Build the CLI.
cd cli
go build -o tb .
ln -sf "$(pwd)/tb" ~/.local/bin/tb

# Create a board in another project.
cd /your/project
tb init                                # creates ./board and .tb.yaml
tb create "First task" -m core -p P1   # adds board/backlog/PR-1/TASK.md
tb ls                                  # see the backlog
tb ready 1                             # commit it to the ready column (triage-gated)
tb pull                                # pull the highest-priority ready task → in-progress
tb done 1                              # mark done
tb regenerate                          # refresh generated board/BOARD.md
```

The GUI uses the same board format:

```bash
cd gui/frontend && npm install
cd ..
task dev                               # or: wails3 dev -config ./build/config.yml
```

## Repo layout (current)

```
task-board-tools/
├── cli/                # tb CLI module (tools/tb)
├── gui/                # tb-gui Wails3 desktop app module (tools/tb-gui)
│   ├── app/            # Wails services exposed to Svelte
│   ├── internal/       # CLI bridge, watcher, agent runner, daemon helpers
│   └── frontend/       # Svelte 5 frontend, tests, generated bindings
├── docs/               # PROJECT, ARCHITECTURE, FEATURES, IMPLEMENTATION
├── board/              # this repo's own task board; BOARD.md is generated
├── .codex/             # Codex config, hooks, and agents
├── .claude/            # Claude placeholder/skills kept in repo; local runtime files ignored
├── go.work             # Go workspace tying cli + gui together
└── README.md
```

## Build

Run Go commands from the module directories; the repo root has `go.work` but is not itself a Go module.

**CLI**:

```bash
cd cli && go build -o tb .
cd cli && go test ./...
```

**GUI backend**:

```bash
cd gui && go test ./...
```

**GUI frontend**:

```bash
cd gui/frontend && npm install
cd gui/frontend && npm run check
cd gui/frontend && npm test
```

**Desktop build/dev**:

```bash
cd gui && task dev
cd gui && task build
```

Equivalent Wails commands are `wails3 dev -config ./build/config.yml` and `wails3 build -config ./build/config.yml`.

Requires Go 1.26.1+, Wails3 `v3.0.0-alpha.91`, Node/npm, and a C toolchain for the desktop app.

## Status

The CLI, GUI, agent run/groom flow, daemon, settings polish, folder-form tasks, and attachments are implemented through M8. The board still tracks active backlog work for worktree isolation, session resume, auto-groom/auto-implement, code-review workflow, and UX/tooling polish. See `docs/IMPLEMENTATION.md` and this repo's `board/` for current state.

## License

Not yet specified — treat as all-rights-reserved until the author decides.
