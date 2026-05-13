# task-board-tools

Markdown-based task tracker with optional AI-agent execution.

Two binaries built from this repo:

| Binary | Path | Purpose |
|--------|------|---------|
| **`tb`** | `cli/` | Terminal CLI. Manages the board, owns all structured mutations. Used by humans and by AI agents. Zero external Go dependencies. |
| **`tb-gui`** | `gui/` | Desktop app (Wails3 + Svelte 5). Kanban board with DnD, filters, markdown editor, agent assignment, live updates. |

Tasks are plain Markdown files in directories (`backlog/`, `in-progress/`, `done/`, `archive/`) — the directory is the status. No database. No server.

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

## Quick start (CLI today)

The CLI currently lives in `tb/` and has its own git history. It will be renamed to `cli/` and merged into this repo in milestone M1.

```bash
cd tb && go build -o tb . && ln -sf "$(pwd)/tb" ~/.local/bin/tb

cd /your/project
tb init                                # creates ./board and .tb.yaml
tb create "First task" -m core         # adds backlog/PR-1.md
tb ls                                  # see the backlog
tb start 1 && tb done 1                # workflow
tb board                               # print kanban summary
```

GUI is in development — see `docs/IMPLEMENTATION.md` for current milestone.

## Repo layout (current)

```
task-board-tools/
├── tb/                 # tb CLI — separate git repo, gitignored here until M1
├── docs/               # PROJECT, ARCHITECTURE, FEATURES, IMPLEMENTATION
└── README.md
```

After M1 lands the layout becomes:

```
task-board-tools/
├── cli/                # was tb/ — renamed, merged into this repo
├── gui/                # tb-gui Wails3 app (M2+)
├── docs/
├── go.work             # Go workspace tying cli + gui together
└── README.md
```

## Build

**CLI** (today, in `tb/`):
```bash
cd tb && go build -o tb .
```

**CLI** (after M1, from repo root):
```bash
go build -o tb ./cli
```

**GUI** (planned for M2):
```bash
cd gui && wails3 build
```

Requires Go 1.26+, Node/pnpm, and a C toolchain (for Wails GUI).

## Status

Early development. CLI is fully functional and used in production by the author. GUI is being built in milestones; see `docs/IMPLEMENTATION.md`.

## License

Not yet specified — treat as all-rights-reserved until the author decides.
