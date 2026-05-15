# Architecture

## High-level

Two binaries built from one repo, sharing the same on-disk format.

```
                           ┌────────────────────────────────────┐
                           │             tb-gui                 │
                           │  ┌─────────────────────────────┐   │
                           │  │  Svelte frontend            │   │
                           │  │  - Kanban, drawer, filters  │   │
                           │  │  - Wails bindings + events  │   │
                           │  └────────────┬────────────────┘   │
                           │               │                    │
                           │  ┌────────────▼────────────────┐   │
                           │  │  Go services (board/agent/  │   │
                           │  │  settings)                  │   │
                           │  │  + watcher (fsnotify)       │   │
                           │  │  + daemon (worker pool)     │   │
                           │  │  + shell (menu/tray)        │   │
                           │  └─┬────────────┬──────────┬───┘   │
                           └────│────────────│──────────│───────┘
              exec("tb …")     │            │          │
                               ▼            ▼          ▼
                          ┌────────┐   ┌────────┐  ┌──────────┐
                          │  tb    │   │  .md   │  │  claude  │
                          │ (CLI)  │──▶│ files  │  │  codex   │
                          └────────┘   └────────┘  └──────────┘
                                            ▲
                                            │ also edits
                                            │ (during agent run)
```

- **`tb` (CLI)** lives in `cli/`. Single-binary Go program, only stdlib. Owns the `.board.lock` for every structured mutation.
- **`tb-gui`** lives in `gui/`. Wails3 (Go backend) + Svelte 5/SvelteKit (frontend). Talks to the filesystem read-only for snapshots; calls `tb` via `exec` for structured mutations; direct-writes only for free-form body content (under the same lock).
- **External agents**: `claude` and `codex` CLIs invoked by the daemon. They read the task content as input and may themselves run `tb edit` / `tb done` etc. to update the task — closing the loop.

## On-disk layout

```
<projectRoot>/
├── .tb.yaml                         # config: board path, prefix, wip_limit
└── <boardDir>/                      # default: ./board, configurable
    ├── BOARD.md                     # generated kanban view
    ├── CONVENTIONS.md               # human/agent-facing conventions
    ├── SKILL.md                     # agent skill instructions
    ├── .next-id                     # ID counter, locked
    ├── .board.lock                  # flock target, never read
    ├── backlog/
    │   ├── PR-1.md
    │   └── …
    ├── in-progress/
    │   └── PR-2.md
    ├── done/
    │   └── PR-3.md
    ├── archive/                     # tasks closed via `tb close`
    │   └── PR-4.md
    ├── .agent-state/                # one JSONL file per file-form task with run history
    │   └── PR-2.jsonl
    └── .agent-logs/                 # full stdout/stderr per run (file-form tasks only)
        └── PR-2/
            └── r_a1b2c3d4.log
```

The CLI manages `BOARD.md`, `.next-id`, all status dirs, and `archive/`. The GUI manages `.agent-state/` and `.agent-logs/` (the CLI doesn't touch them).

Tasks can be stored either as a single `.md` file or as a directory; the layout above shows the file form. See "Folder-form tasks" below for the directory form and the rules that govern both.

## Task file format

```markdown
# PR-42: Fix crash on empty input

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** core
**Tags:** quick-win
**Branch:** fix/empty-input
**Parent:** PR-32
**Agent:** claude
**AgentStatus:** queued

## Goal

One-sentence objective.

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2

## Log

- 2026-05-10: Created
- 2026-05-11: Started — moved to in-progress
```

- Header `# PREFIX-NNN: Title` parsed by line scan.
- Metadata block: `**Field:** value` (bold key with colon inside) OR `**Field**: value`. Both forms supported; CLI writes the first form.
- Only the first 15 lines are scanned for metadata (performance).
- `Agent` and `AgentStatus` are new optional fields. Missing = unassigned.
- Valid `AgentStatus` values: `queued`, `running`, `success`, `failed`, `cancelled`. `cancelled` is reserved for user-initiated cancellation; the daemon never writes it from a crash or timeout.

## Folder-form tasks

A task is stored on disk in one of two forms. Both are first-class and coexist on the same board.

### Side-by-side layout

```
<boardDir>/                          # shared regardless of form
├── BOARD.md                         # generated; board-wide (see "Path-visible differences")
├── .next-id                         # board-wide ID allocator
├── .board.lock                      # board-wide flock target
├── backlog/
│   ├── TB-1.md                      # file form: task = a single markdown file
│   └── TB-2/                        # folder form: task = a directory
│       ├── TASK.md                  #   canonical markdown (same format as file form)
│       ├── attachments/             #   user attachments live here
│       │   ├── design.pdf
│       │   └── screenshot.png
│       ├── .agent-state.jsonl       #   task-local agent run history
│       └── .agent-logs/             #   task-local agent stdout/stderr
│           └── r_a1b2c3d4.log
├── in-progress/                     # folder-form layout repeats per status
├── done/
├── archive/
├── .agent-state/                    # board-root agent state (file-form tasks only)
│   └── TB-1.jsonl
└── .agent-logs/                     # board-root agent logs (file-form tasks only)
    └── TB-1/
        └── r_x.log
```

### File form vs folder form

| Aspect              | File form                                | Folder form                                  |
|---------------------|------------------------------------------|----------------------------------------------|
| Path                | `<status>/<ID>.md`                       | `<status>/<ID>/TASK.md`                      |
| Status = directory  | the `.md` lives in the status dir        | the task dir lives in the status dir         |
| Attachments         | not supported in-place                   | `<status>/<ID>/attachments/<filename>`       |
| Agent state JSONL   | `<boardDir>/.agent-state/<ID>.jsonl`     | `<status>/<ID>/.agent-state.jsonl`           |
| Agent run logs      | `<boardDir>/.agent-logs/<ID>/<rid>.log`  | `<status>/<ID>/.agent-logs/<rid>.log`        |
| Created by          | legacy / explicit opt-in flag            | `tb create` default                          |

The task ID, title, metadata block, body, log section, and `BOARD.md` rendering are identical across forms. A reader that has resolved the task content does not need to know which form it came from.

### Resolution (which form wins)

For a given `<ID>` in a status directory, both the CLI and the GUI parser resolve in this order:

1. If `<status>/<ID>/TASK.md` exists → folder form.
2. Else if `<status>/<ID>.md` exists → file form.
3. Else → the task is not in that status.

The status-directory scanner ignores entries whose name begins with `.` (e.g. the promotion staging dir below, any future dotfile). The two namespaces — `<ID>` (directory) and `<ID>.md` (file) — are disjoint, so a status dir cannot contain a collision.

If both `<ID>/TASK.md` and `<ID>.md` are present at the same time, that is a **crash-recovery transient**, not steady state — it can only occur if a process died mid-promotion (see below). The resolver picks folder form and logs a warning to stderr (deduped per `(taskID, status)` per process); the orphan `<ID>.md` is removed by the next structured mutation via `cleanupOrphanFileFormSibling`. There is no automatic startup sweep today — recovery is opportunistic on the next mutation. Dual-form is never created by design.

### Attachments section in `TASK.md`

A folder-form task may carry an optional `## Attachments` section in the body. It lists what is in the `attachments/` directory, one entry per line:

```markdown
## Attachments

- attachments/design.pdf
- attachments/screenshot.png
```

The section is maintained by `tb attach` / `tb attach --rm` — it is not hand-edited. File-form tasks do not have this section.

### Lock semantics

`.board.lock` (POSIX `flock` at board root) serializes every structured mutation regardless of form. The lock scope is unchanged from the file-form world:

- Every write to `TASK.md`, every attachment add or remove, every cross-status rename of a task directory, and the promotion procedure below all acquire `LOCK_EX` for the duration of the operation.
- Lock-free readers (the GUI parser and the fsnotify watcher) still do not take the lock; they rely on the atomic-write rule below and on the resolution order.

### Atomic-write rule for files inside a task folder

Every write that produces a task-content file inside a task folder goes through `writeFileAtomic` (temp file in the same directory + fsync + `os.Rename`):

- `<status>/<ID>/TASK.md` — exactly the same rule as `<status>/<ID>.md` in the file form. Direct `os.WriteFile` of a `.md` file is forbidden outside `cli/atomicfs.go`.
- `<status>/<ID>/attachments/<filename>` — each attachment is staged in the same directory under a `.tmp.*` name and renamed into place, so a reader either sees the previous bytes (if any) or the full new bytes, never a half-copied file.

Per-task agent state (`.agent-state.jsonl`) keeps the same write semantics as its board-root predecessor — append-only via the agent runtime. The atomic-write invariant is about task-content files (the markdown and attachments); JSONL append behavior is owned by the daemon and documented in "Agent state" below.

### File → folder promotion

Promotion runs when a file-form task acquires its first attachment, or by explicit request. It is atomic from the reader's perspective and never produces a state where the task appears to be missing:

1. Acquire `.board.lock` (`LOCK_EX`).
2. Re-read `<status>/<ID>.md` and confirm the task still exists in file form (defends against a race lost to a concurrent move).
3. Create a staging directory `<status>/.<ID>.promote.<pid>.<rand>/`. The leading `.` keeps the staging dir invisible to the status-directory scanner.
4. Copy the existing `.md` body into `<staging>/TASK.md` via `writeFileAtomic`.
5. If the operation that triggered promotion brings inbound attachments, stage them under `<staging>/attachments/` via `writeFileAtomic`. Update the staged `TASK.md`'s `## Attachments` section accordingly.
6. `os.Rename(<staging>, <status>/<ID>)`. This single rename publishes the folder. Because `<ID>` (directory) and `<ID>.md` (file) are disjoint names, the rename cannot collide; from this point on the resolver returns folder form.
7. `os.Remove(<status>/<ID>.md)`. The legacy file disappears.
8. Append a `## Log` entry to `<status>/<ID>/TASK.md` (`Promoted to folder form on <date>`) via `writeFileAtomic`.
9. `regenerateBoard` (still under lock).
10. Release the lock.

The ordering is load-bearing:

- The folder appears (step 6) before the file disappears (step 7), so any lock-free reader that interleaves between the two steps still resolves the task — to the folder form by the resolution order above. The task is never "missing" from a reader's point of view.
- If the process dies between step 6 and step 7, the next CLI invocation finds both forms; the resolver prefers folder form, and the next structured mutation removes the orphan `<ID>.md` via `cleanupOrphanFileFormSibling`. This is the only path to a dual-form state and it is self-healing on the next mutation.
- The staging name's `.<ID>.promote.` prefix means partially-built staging dirs left by a crash mid-build (before step 6) are ignored by all readers and by `BOARD.md` regeneration. They accumulate in the status directory until manually cleaned up; a future opportunistic sweep on `tb` invocation may garbage-collect them, but there is no startup recovery sweep today. Same applies to `.attach.<pid>.<rand>/` staging dirs left by a crash mid-attach. These leftovers are functionally invisible — they cost disk space but never affect correctness.

Demotion (folder → file) is **not supported**. Once promoted, a task stays in folder form even if its attachments are later removed. This keeps the resolution order total and avoids a second class of transient states.

### Move / archive of folder tasks

`tb mv`, `tb start`, `tb done`, `tb close`, and archive/restore move a folder-form task by a single `os.Rename` of `<status_from>/<ID>` to `<status_to>/<ID>`, under `.board.lock`. Attachments, the task-local `.agent-state.jsonl`, and the task-local `.agent-logs/` ride along inside the renamed directory — there is no separate cleanup of board-root agent paths for folder tasks. File-form tasks continue to move only their `.md` file; their board-root `.agent-state/<ID>.jsonl` and `.agent-logs/<ID>/` are unaffected by status moves, because status is encoded in the file's parent directory and not in the agent-state path.

### Path-visible differences between forms

Most of the contract is form-agnostic. The deliberate exceptions, each with its one-line rationale:

- **`BOARD.md` lives at `<boardDir>/BOARD.md` for both forms.** It is a single board-wide view, not task-owned content; placing it inside a task folder would break the "one kanban view per board" UX.
- **Board-root `.agent-state/` and `.agent-logs/` exist only for file-form tasks.** Folder-form tasks own their agent artifacts so they travel on rename; the daemon looks board-local for file tasks and task-local for folder tasks.
- **The `<status>/<ID>.md` filename is reserved for file form; the `<status>/<ID>/` directory is reserved for folder form.** The two namespaces are disjoint so the resolution order is total.

No other path differs between forms. `BOARD.md` content, `tb --json` output, watcher event shapes, and agent JSONL event shapes are identical across forms.

## Component responsibilities

### `cli/` — the CLI

- Loads `.tb.yaml`, resolves project root and board dir.
- All structured mutations acquire `.board.lock` (POSIX `flock`).
- Auto-regenerates `BOARD.md` after every mutation that changes status, task set, or metadata visible in the board summary.
- Adds `--json` mode to `ls`, `show`, `board` for machine consumption.
- Adds `--status active|archive|all` for filter clarity. `active` = backlog + in-progress + done. `all` = everything.

### `gui/app/` — Wails services (Go)

Exported to the frontend via Wails3 bindings.

- **`BoardService`** — Load board snapshot, get task detail, create/edit/move/close, and expose `Triage()` for grooming indicators. Structured calls delegate to `exec tb …`; `Triage()` consumes `tb triage --json`. `EditTaskBody` is the one exception (direct write — see "Locking" below).
- **`AgentService`** — Assign agent, run agent (enqueue to in-process daemon), groom task (run with a different prompt), cancel, list runs. The run timeout comes from a late-bound provider so settings changes apply to the next run.
- **`SettingsService`** — Project root selection, recent boards, max workers, agent timeout, default agent, CLI binary path.

### `gui/internal/` — non-exported helpers (Go)

- **`cli/`** — thin `exec` wrapper with consistent error handling.
- **`parser/`** — markdown reader (duplicates CLI parser; read-only, no lock).
- **`watcher/`** — fsnotify wrapper. Watches the status directories, ignores `BOARD.md`, `.next-id`, `.board.lock`, `.agent-state/`, `.agent-logs/`. Debounces 200ms. Emits Wails events `board:reloaded` (create/remove/rename) and `task:updated:<id>` (write).
- **`agent/`** — `Runner` interface + `ClaudeRunner`, `CodexRunner`, `GroomingDecorator`. Embedded prompt templates via `//go:embed`; `GroomingDecorator` is the only mode-aware runner layer and swaps the prompt for `mode=groom`.
- **`daemon/`** — goroutine that owns the queue, scans for `AgentStatus: queued`, runs them through the worker pool, writes JSONL events.
- **`shell/`** — native application menu and system tray controller. It calls the same Wails services as the frontend and emits `settings:open-panel` for the Svelte settings panel.

### `gui/frontend/` — Svelte 5 + SvelteKit

- Single route (`+page.svelte`).
- Components: `Board`, `Column`, `Card`, `TaskDrawer`, `FilterBar`, `CreateTaskDialog`, `SettingsPanel`, `AgentRunLog`, `Toast`.
- Stores: `boardStore` (id→task map), `filterStore`, `preferencesStore`, `runsStore`, `selectionStore`.
- Talks to Wails via auto-generated bindings; listens to events for live updates.

## Locking and atomic writes

Two invariants together make multi-writer + lock-free-reader safe:

### Invariant A — Exclusive lock for all task-file mutations

Every writer (CLI subcommand or GUI `EditTaskBody`) holds `.board.lock` via `syscall.Flock(LOCK_EX)` for the duration of read-modify-write. Released on return. This serializes mutations.

### Invariant B — All task-file writes are atomic (temp + rename)

A reader must never observe a half-written `.md` file. Therefore every write to a task file (or to `BOARD.md`, `.next-id`, generated outputs) follows this pattern:

```go
tmp := destPath + ".tmp." + strconv.Itoa(os.Getpid())
os.WriteFile(tmp, content, 0644)
os.Rename(tmp, destPath)  // atomic on POSIX within the same filesystem
```

This applies to **every** mutation site: `create`, `edit`, `mv`/`start`/`done`, `close`, `scan --apply`, `addTagToTaskFile`, `addChildToSubtasks`, plus GUI's `EditTaskBody`. The existing `regenerate.go` already does this; M1 extends it to the others (see `FEATURES.md` F1.6).

**Why it matters**: GUI readers (`parseTaskFile` over fsnotify events) don't take the lock. With atomic rename, a reader either sees the file as it was before the write or as it is after — never partially written. Without atomic rename, a reader could observe a truncated file (no header, no metadata) and either drop the task from its snapshot or render an empty card. The fsnotify event for the rename arrives once the new content is fully in place.

### GUI direct writes (`EditTaskBody`)

Used only for free-form body content (sections like `## Goal`, `## Acceptance Criteria`, `## Context`):
1. Open `.board.lock` with `LOCK_EX`.
2. Read the existing file.
3. Reject if the caller's new content modifies the header (`# PREFIX-NNN: …`) or the metadata block (first 15 lines).
4. Append a `## Log` entry: `- YYYY-MM-DD: Edited body via GUI`.
5. Write atomically (Invariant B).
6. Release the lock.
7. Run `exec tb regenerate` to refresh `BOARD.md`.

### Reader rules

GUI readers (parser, watcher) do **not** take the lock. They rely on Invariant B. The parser should still tolerate the edge case where a write is in progress on a system whose filesystem semantics are weaker than expected: if `parseTaskFile` returns a task with empty `ID` or empty `Title` (i.e., header line not found in the first 15 lines), the GUI **discards** that read and waits for the next fsnotify event rather than emitting a phantom delete. This keeps M2/M3 robust against filesystems where rename isn't perfectly atomic (e.g., some network mounts).

## Concurrency model

- **CLI ↔ CLI**: serialized via flock. Multiple `tb` processes wait their turn.
- **CLI ↔ GUI structured ops**: GUI invokes CLI, so it's the same flock. Safe.
- **CLI ↔ GUI direct body writes**: same flock. Safe.
- **CLI/GUI ↔ Agents**: agents are external processes; they run their own `tb edit` invocations which acquire flock normally. Safe.
- **GUI reads** (snapshot): no lock. Safety relies on Invariant B (atomic writes) plus the reader rule above (discard malformed parses).

## Agent state

Hybrid storage:

| Where | Lives in | Purpose |
|-------|----------|---------|
| `Agent`, `AgentStatus` metadata in task.md | the task file | Current assignment, visible to humans and to CLI |
| `.agent-state/PREFIX-NNN.jsonl` | append-only JSONL | Full run history: queued → started → stdout lines → finished |
| `.agent-logs/PREFIX-NNN/<run_id>.log` | one file per run | Full stdout/stderr text for inspection |

JSONL event shapes (every event carries `task_id` so a log-trawler needs no cross-file index; agent-run events also carry `mode`, currently `implement` or `groom`):

```jsonl
{"ts":"2026-05-13T10:00:00Z","run_id":"r_abc","task_id":"TB-1","event":"queued","agent":"claude","mode":"implement"}
{"ts":"2026-05-13T10:00:05Z","run_id":"r_abc","task_id":"TB-1","event":"started","pid":12345,"agent":"claude","mode":"implement"}
{"ts":"2026-05-13T10:00:10Z","run_id":"r_abc","task_id":"TB-1","event":"stdout","mode":"implement","line":"Reading task..."}
{"ts":"2026-05-13T10:02:30Z","run_id":"r_abc","task_id":"TB-1","event":"finished","agent":"claude","mode":"implement","status":"success","exit_code":0}
```

A run is **complete** when a `finished` event exists. A run with no `finished` event after a process restart is **stale** and is recovered: the daemon verifies the PID from `started` via `pidAlive(pid, expectedAgent)` — `os.FindProcess(pid).Signal(syscall.Signal(0))` (`ESRCH` → dead) plus a command-name cross-check (`ps -o comm=` / `ps -o args=`) that tolerates npm shebang wrappers (e.g. `node /usr/local/bin/claude`). If dead, the daemon writes a synthetic `finished` event with `status: failed`, `reason: "stale after restart"`, and sets `AgentStatus: failed` via `tb edit`. If alive, the daemon leaves the task alone — **M5 does not re-attach to live runs.**

**Cancel carve-out**: recovery honors cancellation intent expressed in *either* the task's `.md` or the JSONL trail. If `AgentStatus` is already `cancelled` *or* the latest JSONL event for the latest `run_id` is `finished{status: cancelled}`, recovery reconciles to `cancelled` (writing `AgentStatus=cancelled` if the `.md` is out of sync) and never appends a `failed` line. This defends the M4 5-step cancel ordering (kill → JSONL → Wails → `tb edit`) against a `kill -9` of the GUI between the JSONL write and the `tb edit`.

Groom runs use the same JSONL/storage lifecycle with `mode:"groom"`. The underlying agent runner still owns process execution; `GroomingDecorator` only replaces the prompt with `gui/internal/agent/prompts/groom.md`, so prompt selection stays in one layer rather than leaking mode checks into Claude/Codex runners or the daemon.

## Daemon

A goroutine inside the GUI process. Constructed in `main` before `app.Run()`; *activated* by the `SettingsService.OpenBoard` hook once a project root is selected. Stops on app shutdown.

1. **On board activation**: stale-recovery scan → watcher event sink registered → startup queue scan. The ordering is load-bearing: registering the sink before the scan closes the race where an edit lands between scan-read and subscription-attached.
2. **Queue scan**: read tasks with `AgentStatus: queued` via in-process `BoardService.LoadBoard("active")`, enqueue via `tryEnqueue` (dedup).
3. **Watcher integration**: a second `watcher.Emitter` implementation (tee/fan-out wired in `main.go`) forwards events to a daemon-side channel without changing the watcher's public API. The daemon handles both `task:updated:<id>` (in-place Write — `tb` direct writes) AND `board:reloaded` (atomic Rename — the CLI's mandated path). Atomic CLI edits trigger Rename → `board:reloaded`, so a daemon that only watched `task:updated:<id>` would miss them.
4. **Worker pool**: N workers (N = `MaxWorkers`, default 1, configurable 1–4 via `$XDG_CONFIG_HOME/tb-gui/preferences.json`) read a buffered task-ID channel. In-memory active-set keyed by `task_id` plus `AgentService.HasActiveRun` cross-check prevents duplicate enqueue.
5. **Per-run**: workers call an internal blocking executor `AgentService.RunQueuedAgentSync(ctx, id)` (distinct from public `RunAgent` which still serves the drawer Run button). The executor accepts an `AgentStatus=queued` task, writes `started`+pid+agent JSONL, sets `AgentStatus: running` via `tb edit`, spawns the runner with the caller-supplied ctx (so daemon shutdown cancellation propagates), tees stdout/stderr to log file + Wails events, writes `finished` + terminal `AgentStatus` on exit, returns the terminal status. Blocks until terminal.
6. **Shutdown**: cancel root context; workers' cancel-finish helper writes `finished{status: cancelled, reason: "shutdown"}` JSONL events. Wait up to 5s; whatever didn't drain is reconciled by next-start recovery.

## Native shell

`gui/internal/shell.Controller` owns desktop-only affordances that do not belong in Svelte component state:

- **Application menu**: File → Open board / Open Recent / Settings / Quit, View → Reload board, Help → About / Open docs. Menu items call `SettingsService`, `BoardService`, or emit `settings:open-panel`; they do not duplicate board-opening logic.
- **Recent boards**: `SettingsService.OpenBoard` emits `recents:changed` after `rememberBoard`; the shell controller reloads `ListRecentBoards()` and rebuilds the native Open Recent submenu.
- **Settings entry points**: native menu and tray both emit `settings:open-panel`, while the frontend also exposes a topbar button. The Svelte `SettingsPanel` is the only settings form.
- **Tray state**: the tray maintains an in-memory active-run counter from Wails events. `agent:run-started` increments; `agent:run-finished` and `agent:run-cancelled` decrement with a zero floor. The icon is idle when the counter is zero and running otherwise. JSONL remains the durable source of truth; tray state is presentation only.
- **Window lifetime**: on tray-capable desktop platforms, `ApplicationShouldTerminateAfterLastWindowClosed=false` so closing the main window does not kill daemon work. Quit from the menu or tray still exits the app and runs daemon shutdown.

## Single instance

`tb-gui` uses the Wails3 single-instance plugin. A second invocation does not start a new process — it focuses the existing window. This prevents two daemons from racing on the same board.

## Security

Agents run with the user's privileges in the project root. There is no sandbox, no container, no review-before-apply step. This is a conscious tradeoff for MVP simplicity. Users should:

- Not assign agents to boards they don't trust.
- Use git: agents are expected to make file changes; the safety net is `git diff` / `git reset`.
- Set a reasonable agent timeout (default 30 minutes).

If isolation is needed later, the `Runner` interface is the seam — a `SandboxedRunner` can wrap the existing implementations with cwd in a tempdir + a git worktree.

## Build & ship

- Repo uses a single Go module per binary; a root `go.work` ties them together for development.
- `cli/`: `go build -o tb .` → static binary, no CGo.
- `gui/`: `wails3 build` → app bundle. Requires CGo, Node/pnpm. Mac: `.app`, Linux: AppImage / static binary, Windows: not in MVP.
- CI: build both with workspace.

## Toolchain (M2+)

Pinned versions confirmed by `wails3 doctor` (see `board/in-progress/TB-16.md` log for full output):

| Component | Version | Notes |
|-----------|---------|-------|
| Wails CLI | `v3.0.0-alpha.91` | Alpha. Pin in `gui/go.mod` until v3 stable. |
| Go | `1.26.2+` (darwin/arm64 verified) | `1.26.x` series — newer minors should work; revisit if doctor fails after a Go bump. |
| Node | `v20+` with `npm` `11.x` (or `pnpm`) | SvelteKit frontend toolchain. |
| CGo | `gcc` (Xcode CLI tools) or `clang` | Required for Wails3 native windowing — `cli/` itself stays CGo-free. |
| Xcode CLI tools | `2416+` | macOS only; provides system frameworks Wails3 links against. |

**OS support (MVP):** macOS 13+ and Linux (GTK/WebKit2 — distro packages cover this). Windows is out of MVP scope (risk #3 in `IMPLEMENTATION.md`: `syscall.Flock` is POSIX-only); we ship `tb` (CLI) on Windows but not `tb-gui`.

If `wails3 doctor` ever fails on a newer Go release, pin Wails3 to `v3.0.0-alpha.91` in `gui/go.mod` and re-run; do not silently bump the alpha tag without re-running doctor.
