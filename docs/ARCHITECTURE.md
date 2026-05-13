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
    ├── .agent-state/                # one JSONL file per task with run history
    │   └── PR-2.jsonl
    └── .agent-logs/                 # full stdout/stderr per run
        └── PR-2/
            └── r_a1b2c3d4.log
```

The CLI manages `BOARD.md`, `.next-id`, all status dirs, and `archive/`. The GUI manages `.agent-state/` and `.agent-logs/` (the CLI doesn't touch them).

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

## Component responsibilities

### `cli/` — the CLI

- Loads `.tb.yaml`, resolves project root and board dir.
- All structured mutations acquire `.board.lock` (POSIX `flock`).
- Auto-regenerates `BOARD.md` after every mutation that changes status, task set, or metadata visible in the board summary.
- Adds `--json` mode to `ls`, `show`, `board` for machine consumption.
- Adds `--status active|archive|all` for filter clarity. `active` = backlog + in-progress + done. `all` = everything.

### `gui/app/` — Wails services (Go)

Exported to the frontend via Wails3 bindings.

- **`BoardService`** — Load board snapshot, get task detail, create/edit/move/close. All structured calls delegate to `exec tb …`. `EditTaskBody` is the one exception (direct write — see "Locking" below).
- **`AgentService`** — Assign agent, run agent (enqueue to in-process daemon), groom task (run with a different prompt), cancel, list runs.
- **`SettingsService`** — Project root selection, recent boards, agent timeout, CLI binary path.

### `gui/internal/` — non-exported helpers (Go)

- **`cli/`** — thin `exec` wrapper with consistent error handling.
- **`parser/`** — markdown reader (duplicates CLI parser; read-only, no lock).
- **`watcher/`** — fsnotify wrapper. Watches the status directories, ignores `BOARD.md`, `.next-id`, `.board.lock`, `.agent-state/`, `.agent-logs/`. Debounces 200ms. Emits Wails events `board:reloaded` (create/remove/rename) and `task:updated:<id>` (write).
- **`agent/`** — `Runner` interface + `ClaudeRunner`, `CodexRunner`, `GroomingDecorator`. Embedded prompt templates via `//go:embed`.
- **`daemon/`** — goroutine that owns the queue, scans for `AgentStatus: queued`, runs them through the worker pool, writes JSONL events.

### `gui/frontend/` — Svelte 5 + SvelteKit

- Single route (`+page.svelte`).
- Components: `Board`, `Column`, `Card`, `TaskDrawer`, `FilterBar`, `CreateTaskDialog`, `AgentRunLog`, `Toast`.
- Stores: `boardStore` (id→task map), `filterStore`, `runsStore`, `selectionStore`.
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

JSONL event shapes:

```jsonl
{"ts":"2026-05-13T10:00:00Z","run_id":"r_abc","event":"queued","agent":"claude","mode":"implement"}
{"ts":"2026-05-13T10:00:05Z","run_id":"r_abc","event":"started","pid":12345}
{"ts":"2026-05-13T10:00:10Z","run_id":"r_abc","event":"stdout","line":"Reading task..."}
{"ts":"2026-05-13T10:02:30Z","run_id":"r_abc","event":"finished","status":"success","exit_code":0}
```

A run is **complete** when a `finished` event exists. A run with no `finished` event after a process restart is **stale** and is recovered: if the PID from `started` is dead (verified via `os.FindProcess(pid).Signal(syscall.Signal(0))`), the daemon writes a synthetic `finished` event with `status: failed`, `reason: "stale after restart"`, and sets `AgentStatus: failed` via `tb edit`.

**Cancel carve-out**: if the task's current `AgentStatus` is already `cancelled` when the daemon checks, recovery does nothing — cancellation is user-intent and outranks crash inference. Cancel paths are responsible for writing both the JSONL `finished{status: cancelled}` event AND `AgentStatus: cancelled` (via `tb edit`) before the cancelling process exits.

## Daemon

A goroutine inside the GUI process. Starts on `App.OnStartup`, stops on `App.OnShutdown`.

1. **On start**: stale-recovery scan (above), then queue scan.
2. **Queue scan**: read all tasks with `AgentStatus: queued`, enqueue.
3. **Watcher integration**: subscribe to watcher events; on `task:updated:<id>`, re-parse and check if it newly became queued.
4. **Worker pool**: bounded by `semaphore` (default 1, configurable). Dedup by `task_id` — a task being run cannot be enqueued again.
5. **Per-run**:
   - Generate `run_id` (`r_<8 hex chars>`).
   - Set `AgentStatus: running` via `tb edit`.
   - Spawn agent process with `exec.CommandContext(ctx, …)`.
   - Tee stdout/stderr to `.agent-logs/PREFIX-NNN/<run_id>.log` AND emit Wails events.
   - On exit: write `finished` JSONL event; set `AgentStatus: success|failed` via `tb edit`.
6. **Shutdown**: cancel context, wait up to 5s for runners to flush JSONL, then return. Hard-kill leaves stale-running state that recovery handles on next start.

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
