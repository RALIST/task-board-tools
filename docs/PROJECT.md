# Task Board Tools — Project Overview

## What it is

A minimal task tracker for software projects focused on agentic development workflows: agents deal with board throught CLI (Claude Code, Codex), pick tasks and execute/review them autonomously. The whole board is a directory of plain Markdown files — agents and humans share the same source of truth.

Two interfaces, one data model:

- **`tb` CLI** — terminal-first, used by humans and by agents themselves
- **`tb-gui` desktop app** (Wails3 + Svelte) — kanban-style UI for humans

The board, the tasks, the agent assignments — everything lives in `.md` files on disk. No database, no server, no account.

## Who it's for

- **Solo developers and small teams** who want a lightweight tracker that lives in their repo and survives without external services.
- **Developers who use AI coding agents** and want a structured way to delegate work: "Claude, take WS-42 to done."
- **Heavy CLI users** who don't want to leave the terminal for task tracking but appreciate a visual kanban when working through priorities.

It's explicitly **not for**: large teams needing comments/permissions/SSO, projects needing reporting/dashboards, anyone who wants Jira-style customization.

## Key scenarios

### 1. Pick up a task and work on it manually
```
tb ls                    # see backlog sorted by priority
tb start WS-42           # move to in-progress
# ... code ...
tb done WS-42
```

### 2. Delegate a task to an agent
In GUI: open task → assign `claude` agent → click **Run**. The agent runs `claude -p <prompt>` in the project root, streams its output into the task drawer, and updates the task's `AgentStatus` field. Or set it from CLI: `tb edit WS-42 -a claude --agent-status queued` — the daemon picks it up.

### 3. Have an agent groom a vague task
A backlog item has only a one-line title. Click **Groom** → agent reads the existing description, improves Goal and Acceptance Criteria, writes them back via `tb edit`. Reviewer then accepts or refines.

### 4. Auto-capture TODOs from source code
`tb scan --apply` walks the repo, finds untagged `TODO`/`FIXME`/`HACK` comments, creates backlog tasks for each, and patches the source with the task ID (`// TODO(WS-42): refactor this`).

### 5. Track an epic
```
tb create "Search system" --epic -m editor
tb create "Index builder" --parent 1 -m editor
tb create "Search UI" --parent 1 -m editor
tb epic 1            # show progress: 0/2 children done
```
GUI shows epics in a dedicated section with a progress bar.

## Design principles

1. **Markdown is the source of truth.** Every task is a human-readable file. Anyone can `cat` a task, `git diff` it, edit in vim. No proprietary formats.
2. **Directories are status.** A task in `board/in-progress/` is in progress. Moving = renaming.
3. **CLI owns mutations.** `tb create`, `tb mv`, `tb edit`, … take an exclusive `.board.lock`. The GUI defers structured mutations to the CLI via `exec`. Direct writes from the GUI happen only for free-form body editing, under the same lock, with the same rules (preserve header/metadata, append log entry, atomic write, regenerate).
4. **Maximum simplicity in UI/UX.** No nested menus, no settings rabbit holes, no required configuration. Open a board, see a kanban, drag cards. Defaults work for one person; everything else is opt-in.
5. **Agents are a button.** Assigning an agent is one click. Running it is another. The user never authors prompts or fiddles with model parameters in the MVP.
6. **No external services.** No cloud, no auth, no sync server. The repo is the database.

## Glossary

| Term | Meaning |
|------|---------|
| **Board** | A directory containing `backlog/`, `in-progress/`, `done/`, `archive/` subdirs + `.tb.yaml` config in the parent. |
| **Task** | A markdown file `PREFIX-NNN.md` inside one of the status directories. The directory determines the status. |
| **Status** | One of `backlog`, `in-progress`, `done`, `archive`. Plus filter aliases: `active` (all but archive) and `all` (everything). |
| **Project root** | The directory containing `.tb.yaml`. Everything else is resolved from there. |
| **Prefix** | Project-specific task ID prefix (e.g., `WS`, `PR`). Tasks are `WS-1`, `WS-2`, … |
| **Epic** | A task with the `epic` tag. Other tasks can declare `**Parent:** WS-N` to belong to it. |
| **Agent** | One of `claude` (runs `claude -p`) or `codex` (runs `codex exec`). Assigned to a task via the `**Agent:**` metadata field. |
| **AgentStatus** | One of `queued`, `running`, `success`, `failed`, `cancelled`. The daemon watches for `queued` and picks them up. `cancelled` is user-initiated (Cancel button or `tb edit --agent-status cancelled`) and is never overwritten by stale-recovery. |
| **Run** | A single execution of an agent on a task. Recorded in `board/.agent-state/PREFIX-NNN.jsonl` (one event per line) and `board/.agent-logs/PREFIX-NNN/<run_id>.log`. |
| **Groom** | An agent run in "grooming" mode: instead of writing code, the agent refines the task's Goal and Acceptance Criteria. |
| **Daemon** | A goroutine inside the GUI process that scans for queued tasks and runs the assigned agent. MVP only — single-instance. |
| **BOARD.md** | A generated kanban-style summary of the board. Regenerated automatically after every mutation. Never edited by hand. |

## Out of scope (MVP)

- Multi-user, comments, mentions, notifications
- Web/mobile UI
- Multiple agents per task or branching workflows
- Agent prompt customization in the UI (you can edit `gui/internal/agent/prompts/*.md` but there's no UI for it)
- Reporting, time tracking, velocity charts
- Windows native build (POSIX-only `flock`; will work via WSL2)
- Multi-board view in one window
