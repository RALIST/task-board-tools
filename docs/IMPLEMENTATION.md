# Implementation Plan

Living tracker. Update as work progresses. Each milestone has a target deliverable and acceptance set drawn from `FEATURES.md`.

Marker legend:
- ☐ todo · ⬚ in progress · ☑ done · ⊘ skipped/deferred

---

## M0 — Documentation foundation · ⬚

**Deliverable**: 4 docs + updated root README, sufficient for a new contributor to understand goals and architecture.

- ☑ `docs/PROJECT.md`
- ☑ `docs/ARCHITECTURE.md`
- ☑ `docs/FEATURES.md`
- ⬚ `docs/IMPLEMENTATION.md` (this file)
- ☐ Root `README.md`

**Estimate**: ~0.5 day.

---

## M1 — CLI extensions · ☑

**Deliverable**: `cli/` works as drop-in for `tb/`, adds `--json`, agent metadata fields, archive filter, regenerate consistency.

### Tasks
1. ☑ `tb/` → `cli/` (history bundled to `../task-board-tools-tb-history.bundle`)
2. ☑ Create root `go.work` with `use ./cli`
3. ☑ `cli/task.go`: add `Agent`, `AgentStatus` fields (incl. `cancelled` value); JSON tags on Task; extend `parseTaskFile`
4. ☑ `cli/edit.go`: add `-a`, `--agent-status` flags; extend `flagsWithValue`; call `regenerateBoard` at the end; use atomic write
5. ☑ `cli/create.go`: call `regenerateBoard` at the end of `cmdCreate`; use atomic write
6. ☑ `cli/board.go`: extend `resolveStatus` for `active`, `archive`, `all` (added `resolveStatusFilter`); archive write uses atomic helper; `findTask` now searches archive too
7. ☑ `cli/atomicfs.go` (new): `writeFileAtomic(path, data, perm)` helper (temp + fsync + rename) with cleanup on error; tests in `atomicfs_test.go`; callers in `create.go`, `edit.go`, `move.go`, `board.go (archiveTask)`, `scan.go` all migrated
8. ☑ `cli/list.go`: `--json` flag; honours new statuses via `resolveStatusFilter`
9. ☑ `cli/show.go`: `flag.NewFlagSet` + `reorderArgs`; `--json` flag emits `{metadata, body}`
10. ☑ `cli/regenerate.go`: `cmdBoard` `--json` mode emits structured `BoardSnapshot`
11. ☑ `cli/json_output.go`: new file with `marshalTask`, `marshalTasks`, `emitTasksJSON`, `emitShowJSON`, `buildBoardSnapshot`, `emitBoardJSON`
12. ☑ `cli/main.go`: usage text updated with new status filter values
13. ☑ Manual smoke tests (build, JSON valid, edit triggers regenerate, archive filter, no non-atomic `os.WriteFile` for `.md` paths)

**Estimate**: 1.5 days.

### Risks
- `tb/` → `cli/` rename may break someone's PATH symlink — call out in commit message.
- JSON serialization order shouldn't matter, but use struct tags consistently.
- Atomic write helper must respect symlinks and permissions of the destination (use `os.Chmod` after rename if needed). For the MVP we only mutate files we created ourselves, so default 0644 is fine.

---

## M2 — Wails3 read-only GUI · ☑

**Deliverable**: launch GUI on a board, see live kanban (read-only). All sub-tasks (TB-16..TB-24) closed; runtime acceptance verified via `/ui-test` at end of epic.

### Tasks
1. ☑ Pre-check: `wails3 doctor` against Go 1.26.2 → SUCCESS on `wails3 v3.0.0-alpha.91`; toolchain pinned in `ARCHITECTURE.md` § Toolchain
2. ☑ `wails3 init -t sveltekit-ts` in `gui/` (module `tools/tb-gui`); demo `GreetService` + time-emitter stripped
3. ☑ Added `./gui` to root `go.work`
4. ☑ Enabled `application.SingleInstanceOptions` (uniqueID `com.taskboard.tbgui`); `OnSecondInstanceLaunch` restores+focuses the existing window
5. ☑ Backend `gui/internal/cli/cli.go` — `exec` wrapper with `Client.Run` / `Client.RunJSON`, ExitError mapping, ctx cancellation, 7 tests
6. ⊘ Backend `gui/internal/parser/parser.go` — deferred: M2 doesn't need a read-only Go-side markdown parser because the frontend renders the body via `marked`; the only fields we read from `.md` come from `tb show --json` already
7. ☑ Backend `gui/internal/watcher/watcher.go` — fsnotify with pump-goroutine swap design; ignore list (BOARD.md, .next-id, .board.lock, .agent-state/, .agent-logs/) + 200ms debounce; 8 unit + 1 integration tests
8. ☑ Service `gui/app/settings_service.go` — OpenBoard / GetBoardInfo / GetProjectRoot / PickBoardDialog / ListRecentBoards; recents at `$XDG_CONFIG_HOME/tb-gui/recent.json`; 8 tests
9. ☑ Service `gui/app/board_service.go` — `LoadBoard` (status-bucketed) + `GetTask` + `ErrNoBoard`/`ErrNotFound`; 7 tests
10. ⊘ Frontend deps: `svelte-dnd-action` (M3), `svelte-codemirror-editor` (M3). M2 added only `marked` for read-only markdown rendering
11. ☑ Frontend `src/lib/api.ts` — typed wrappers + error-branch helpers (`isNoTbYamlError`, `isCancelledError`, `isNoBoardError`)
12. ☑ Frontend stores: `board.ts` (snapshot + `refresh` + `patchTask`), `selection.ts`, `filter.ts` (M3 placeholder), `toast.ts`
13. ☑ Frontend `Board.svelte` / `Column.svelte` / `Card.svelte` (type glyphs, priority pills, tag overflow, epic accent)
14. ☑ Frontend `TaskDrawer.svelte` — right-side slide-over, Esc + click-outside dismiss, metadata grid + `marked`-rendered body, subscribes to `task:updated:<id>`
15. ☑ Frontend `+page.svelte` — orchestrator: empty-state with recent-board list, picker integration, Wails event wiring
16. ☑ Acceptance: backend integration test (`TestIntegration_TBMvFiresOneBoardReloaded`) drives real `tb` end-to-end; end-of-epic `/ui-test` covers the interactive window flow (live update, single-instance focus, drawer Esc, picker round-trip)

**Estimate**: 2 days.

### Risks
- **Wails3 alpha API surface** may differ from v2 docs. Build a `hello world` binding first as a probe.
- CodeMirror import may need SvelteKit SSR fixup (`+page.svelte` is static, but components may try SSR — use `<script context="module">` or `onMount`).
- macOS code signing for unsigned dev builds — Wails docs cover this.

---

## M3 — Mutations + DnD + editor · ☑

**Deliverable**: full CRUD via GUI; DnD reflects status changes.

### Tasks
1. ☑ Service `board_service.go`: `CreateTask`, `EditTask`, `MoveTask`, `CloseTask`, `Regenerate` (all via `exec tb`)
2. ☑ Service `board_service.go`: `EditTaskBody` — direct write under `.board.lock` with rules (see ARCHITECTURE.md "Locking")
3. ☑ Frontend `Column.svelte`: integrate `svelte-dnd-action`; optimistic moves; revert on error
4. ☑ Frontend `CreateTaskDialog.svelte`
5. ☑ Frontend `TaskDrawer.svelte`: editable metadata fields + body editor (CodeMirror 6)
6. ☑ Frontend `FilterBar.svelte`: client-side filtering over `boardStore`
7. ☑ Frontend `Toast.svelte` for errors
8. ☑ Filter: "Show archived" toggle adds Archive column (BoardSnapshot.archive bucket + LoadBoardWithMode)
9. ☑ Manual acceptance tests — `wails3 dev` runtime smoke: created TB-42 via dialog → toast; edited priority P2→P1 inline → toast; body edit via CodeMirror writes through `EditTaskBody` (.board.lock held, atomic rename, log entry appended, BOARD.md regenerated); two-click Archive sent TB-42 to archive; Show-archived toggle materialized the Archive column; DnD moved TB-5 backlog→in-progress→backlog with both `tb mv` log entries persisted

**Estimate**: 2 days.

### Risks
- `svelte-dnd-action` Svelte-5 compatibility — verify with a small spike first.
- Body editor write contract — must reject changes that touch metadata block. Add a Go-side validator in `EditTaskBody`.

---

## M4 — Manual agent runs · ☑

**Deliverable**: assign agent in GUI, click Run, see live log.

### Tasks
1. ☑ `gui/internal/agent/runner.go` — `Runner` interface, `Mode` type, `RunResult`
2. ☑ `gui/internal/agent/claude.go`, `codex.go` — implementations
3. ☑ `gui/internal/agent/prompts/implement.md` (embedded)
4. ☑ `gui/internal/agent/state.go` — JSONL writer, log file rotation per run
5. ☑ Service `gui/app/agent_service.go` — `AssignAgent`, `RunAgent`, `CancelRun`, `ListRuns`, `GetRunLog`
6. ☑ Wails events: `agent:run-queued`, `agent:run-started`, `agent:run-log`, `agent:run-finished`
7. ☑ Frontend `Card.svelte`: agent badge
8. ☑ Frontend `TaskDrawer.svelte`: agent dropdown + Run + Cancel buttons + past-runs list
9. ☑ Frontend `AgentRunLog.svelte` — streaming logs
10. ☑ Frontend `runsStore.ts` — keyed by `run_id`

**Estimate**: 2 days.

### Risks
- `claude -p` and `codex exec` argument shapes — confirm flags by running them once. Adjust prompts.
- Stdout buffering: ensure agents flush often; use `cmd.StdoutPipe` + `bufio.Scanner`.
- Process group: spawn agents in their own process group so kill cascades to children.

---

## M5 — Daemon auto-pickup + durability · ☑

**Deliverable**: queued tasks auto-run; crashes recover.

### Tasks
1. ☑ `gui/internal/daemon/daemon.go` — main goroutine, queue, worker pool, active-set dedup
2. ☑ Stale-running recovery on activation (`gui/app/agent_recovery.go`; PID check via `gui/internal/daemon/pid.go`; JSONL replay; cancelled carve-out)
3. ☑ Scan on Activate + watcher event sink (`gui/internal/daemon/watcher_sink.go`) that re-enqueues on `task:updated:<id>` and `board:reloaded`
4. ☑ Active-set dedup (in-memory) cross-checked against `AgentService.HasActiveRun`; `max_workers` setting (1–4) at `preferences.json`
5. ☑ Graceful shutdown via `Daemon.Close()` + 5s WaitGroup grace; `finishCancelled(reason)` helper shared by `CancelRun` ("user cancelled") and daemon shutdown ("shutdown")
6. ☑ Daemon constructed in `gui/main.go` before `app.Run()`; activated via `SettingsService.OpenBoard` `BoardActivator` hook (TB-54 also splits a synchronous `RunQueuedAgentSync` executor from the public `RunAgent` so daemon ctx cancellation reaches the runner)

**Estimate**: 1.5 days.

### Risks
- PID re-use after crash is theoretically possible — mitigation: also store start time, verify `os.FindProcess(pid).Signal(0)` returns nil AND check `/proc` or `ps` for command name match (Linux/macOS).
- Two GUIs on different boards: separate single-instance lock keys per board, OR a single global lock (prefer global for simplicity).

---

## M6 — Groom flow · ☐

**Deliverable**: Groom button refines task descriptions.

### Tasks
1. ☐ `gui/internal/agent/prompts/groom.md`
2. ☐ `gui/internal/agent/runner.go`: `GroomingDecorator` swaps prompt
3. ☐ Service `agent_service.go`: `GroomTask`
4. ☐ Frontend `TaskDrawer.svelte`: Groom button + grooming-needed indicator on cards from triage rules
5. ☐ Backend triage helper (calls `tb triage --json` once that exists, or runs the same rules locally)

**Estimate**: 1 day.

### Risks
- Groom prompt quality is iterative — may need 2–3 revisions after manual testing.

---

## M7 — Polish · ⊘ (optional)

Settings UI, keyboard shortcuts, system tray. Deferred unless explicitly prioritized.

---

## Risk register

| # | Risk | Impact | Mitigation | Status |
|---|------|--------|------------|--------|
| R1 | Wails3 alpha + Go 1.26.1 incompatible | Blocks M2+ | Probe in M2 first task; pin tag or downgrade Go | open |
| R2 | fsnotify event loop from CLI's BOARD.md writes | UI flicker / wasted work | Ignore BOARD.md, `.next-id`, `.board.lock`, `.agent-state`, `.agent-logs` | mitigated by design |
| R3 | `syscall.Flock` POSIX-only | No Windows | Documented; use `gofrs/flock` if needed later | accepted |
| R4 | Agent runs with no sandbox | Untrusted board could harm system | Document, rely on git, encourage trusted boards | accepted |
| R5 | Stale `AgentStatus: running` after crash | Confusing state | M5 stale-recovery on startup | planned |
| R6 | Two GUI instances racing daemon | Duplicate runs / lock contention | Single-instance Wails plugin | planned (M2) |
| R7 | `exec tb ls --json` cost with hundreds of tasks | Slow load | Cache in GUI; invalidate on watcher events | deferred until measured |
| R8 | `tb` not in PATH from GUI | Service calls fail | Settings panel with explicit path; resolve via `exec.LookPath` at startup with friendly error | planned (M2) |
| R9 | CodeMirror SSR issues in SvelteKit | M3 blocker | Use `onMount` import; static adapter | planned (M3) |
| R10 | PID re-use on crash | False positive recovery | Cross-check command name; ok for MVP | accepted |
| R11 | Non-atomic CLI writes break unlocked GUI reads | Phantom card deletes, malformed cards | M1 F1.6 mandates atomic temp+rename; reader rule discards malformed parses | planned (M1) |
| R12 | `cancelled` AgentStatus undefined across enum sites | Stale-recovery overwrites cancellation as `failed` | Add `cancelled` to enum everywhere; M5 recovery skips it | planned (M1+M5) |

---

## Completed work log

- 2026-05-13: docs PROJECT/ARCHITECTURE/FEATURES drafted; plan synced with feedback (direct body writes allowed under flock; archive as first-class filter; daemon stale-recovery in M5; root `go.work`)
- 2026-05-13: Codex adversarial review applied — README path corrected to current `tb/`; atomic-write invariant documented and added to M1 (F1.6); `cancelled` added as a first-class `AgentStatus` value with carve-out from stale-recovery
- 2026-05-13: M1 shipped — `tb/` → `cli/` rename (history preserved as bundle outside repo); root `go.work` added; `cli/atomicfs.go` introduced with `writeFileAtomic` + tests; all task `.md` writers migrated; `Agent`/`AgentStatus` fields on `Task` with `tb edit -a` / `--agent-status` + enum validation; `cmdCreate` and `cmdEdit` now regenerate `BOARD.md`; new `resolveStatusFilter` implements `backlog|in-progress|done|archive|active|all` semantics; `findTask` extended to archive so archived tasks can be moved back; `cli/json_output.go` adds `--json` output for `tb ls`, `tb show`, `tb board` (empty results render as `[]` / `{}`)
- 2026-05-13: M2 shipped — `gui/` scaffolded with Wails3 alpha.91 + SvelteKit-TS; backend modules `gui/internal/cli`, `gui/internal/watcher` (pump-goroutine + 200ms debounce), `gui/app/board_service.go` (LoadBoard/GetTask, status bucketing, ErrNoBoard/ErrNotFound), `gui/app/settings_service.go` (OpenBoard/PickBoardDialog/recents at `$XDG_CONFIG_HOME/tb-gui/recent.json`); frontend `Board`/`Column`/`Card`/`TaskDrawer` Svelte components with `marked` for read-only markdown; `+page.svelte` orchestrator with empty-state, recent-board list, and Wails event wiring (`board:reloaded`, `board:opened`, `task:updated:*`). 30 Go tests pass; `wails3 generate bindings` emits 2 services / 7 methods / 6 models. Runtime acceptance via `/ui-test` at end of epic.
- 2026-05-13: M4 review fixes — moved `AgentStatus: running` write from runGoroutine into OnStarted (now guarded by wasCancelled() under ar.mu) so cancel-before-OnStarted can't lose the race against a stale `running` write; new `TestCancelRun_BeforeOnStarted` reproduces the race deterministically with a slow-start runner. exec.go now verifies `syscall.Getpgid(pid) == pid` after `cmd.Start` and zeroes pgid otherwise; killActiveRun and the timeout escalation fall back to SIGKILL-on-pid when pgid==0 rather than risk SIGKILL'ing an unrelated process group. state.go's AppendEvent/NewLogWriter swap their stat+mkdir for a stricter `requireBoardDir(Open+Stat+IsDir)` so a missing boardDir between checks no longer lets MkdirAll auto-create it. AgentRunLog.svelte takes `taskId` as a separate prop (no longer derives it from the runsStore Run record) so GetRunLog never races store hydration.
- 2026-05-14: M5 shipped — agent daemon with autopickup + crash recovery. New `gui/internal/daemon` package: `Daemon` with `New`/`Activate`/`Deactivate`/`Close` lifecycle; N-worker pool over a buffered task-ID channel (N = `max_workers` ∈ [1,4], persisted at `preferences.json`); in-memory active-set keyed by `task_id`, cross-checked against `AgentService.HasActiveRun` (new public accessor); `pidAlive(pid, expectedAgent)` with two-step `ps -o comm=` + `ps -o args=` fallback for npm-shebang `claude`/`codex` wrappers (TB-59); `EventSink` implements `watcher.Emitter` and forwards `task:updated:<id>` + `board:reloaded` to the daemon via a `TeeEmitter` chained alongside the Wails app bus (TB-58); strict Activate ordering — `recovery.RecoverStale` → watcher sink already registered (via `main.go` construction order) → startup queue scan. TB-54 split public `RunAgent` from internal `RunQueuedAgentSync`: the public method writes the queued JSONL + AgentStatus + activeRun placeholder outside `s.mu` (rollback on I/O failure), narrowed `s.mu` to the active-map insert/delete only; the daemon-only synchronous executor accepts `AgentStatus=queued`, uses the caller-supplied ctx so `Daemon.Close()` propagates to `exec.CommandContext`, and shares `runGoroutine` with the M4 manual path. JSONL `started` event now carries `agent` (TB-54 schema change) so TB-60's pidAlive cross-check has an unambiguous source; the recovery reader still falls back to the `queued` event's `agent` for pre-M5 JSONL files. `gui/app/agent_recovery.go` implements `daemon.Recovery`: walks `AgentStatus=running` tasks, syncs `.md` when JSONL has a finished record (cancelled→cancelled per TB-61 carve-out, success/failed→that status), writes synthetic `finished{failed, "stale after restart"}` when JSONL has no finished and `pidAlive==false`, leaves alone when alive (no re-attach in M5). `finishCancelled(c, ar, boardDir, reason)` helper factored from `CancelRun` and used by both the M4 user-cancel path (`reason="user cancelled"`) and the daemon shutdown path (`reason="shutdown"`); idempotent via `ar.finishOnce` so a CancelRun racing shutdown does not double-write. `SettingsService.OpenBoard` gained a `BoardActivator` hook — Deactivate prior board before Activate new one. `gui/main.go` wires daemon construction before `app.Run`, starts the sink reader goroutine, and defers `daemon.Close()` so a window-close triggers the 5s grace + JSONL flush. Integration test (`TestDaemonShutdown_FlushesCancelledJSONL`) drives a real `tb` board + the full daemon stack: Enqueue → runner.Run blocks on ctx → `Daemon.Close()` → JSONL ends with `finished{cancelled, reason:"shutdown"}` and `tb show` reports `AgentStatus: cancelled`. All Go tests pass with `-race`. Manual `kill -9` mid-flight harness (multi-process; can't be expressed inside a Go test binary) is documented as a smoke step in `gui/internal/daemon/README.md`.
- 2026-05-13: M4 shipped — agent assignment + manual runs from the GUI. `gui/internal/agent/` adds `Runner` interface, `ClaudeRunner`/`CodexRunner` with own process group (Setpgid + env whitelist + bufio.Scanner line streaming + OnStarted callback before output), embedded `prompts/implement.md` with locked `{{TASK_ID}}/{{TASK_TITLE}}/{{TASK_BODY}}` placeholders + `RenderPrompt`, and `state.go` (closed event vocabulary `queued|started|stdout|stderr|finished`, per-task mutex for concurrent JSONL appends, per-run log file). `gui/app/agent_service.go` exposes `AssignAgent` (with `none` clear sentinel; the CLI gained matching `tb edit -a none` / `--agent-status none` support that deletes the metadata line), `RunAgent` (sync queued + Wails + AgentStatus + activeRun register; goroutine spawns Runner, streams stdout/stderr to JSONL + log file + Wails `agent:run-log` events, post-run handler writes `finished` unless TB-48 marked it cancelled, error→status map handles binary-not-found/timeout/non-zero-exit), `CancelRun` (5-step ordering: mark → SIGTERM → 5s grace → SIGKILL on pgid → JSONL `cancelled` → Wails emit → `tb edit --agent-status cancelled` last so a crash between 4 and 5 still leaves the durable JSONL for M5 to reconcile), `ListRuns` (rolls per-task JSONL into `Run` records sorted by StartedAt desc, tolerates trailing partial line), `GetRunLog`. Frontend: `runsStore.ts` keyed by run_id with Wails handlers for run-queued/started/finished and `runsByTask` selector; `AgentRunLog.svelte` subscribes to `agent:run-log` for live runs and falls back to `GetRunLog` for terminal runs, ANSI strip + sticky-bottom scroll; `TaskDrawer.svelte` adds Agent dropdown + Run/Cancel buttons (two-click confirm on Cancel) + status pill + past-runs list; `Card.svelte` shows agent badge with single-letter glyph (C/X). Tests: 42 Go tests pass (incl. real-`tb` AssignAgent persistence proof per F4.1; live RunAgent lifecycle proves AgentStatus durability for success/failed/binary-not-found paths; CancelRun integration test spawns a real /bin/sh script that ignores SIGTERM and spawns a child sleep, then verifies both processes die within ~6s via `syscall.Kill(pid, 0)` liveness probe AND that exactly one `finished{cancelled}` JSONL line exists for that run_id AND that `tb show` reports `AgentStatus: cancelled` AND that a fresh `AgentService` instance reading the same task still sees `cancelled`); 8 Vitest tests cover runsStore hydration, sort order, queued-tiebreaker, and the three Wails event handlers; `svelte-check` clean (380 files, 0 errors, 0 warnings); production build green. `agent:run-queued` was added as a fourth lifecycle event so the frontend can render a queued pill before the runner actually spawns.
- 2026-05-13: M3 shipped (closed) — TB-3 closed after interactive `wails3 dev` smoke (created TB-42 via dialog, edited priority P2→P1 inline, body edit through CodeMirror writes via `EditTaskBody` under `.board.lock`, two-click Archive sent the task to archive, Show-archived toggle materialized the archive column with both archived tasks, DnD moved TB-5 backlog↔in-progress through `tb mv` and persisted log entries on disk). Two real bugs caught during smoke and fixed: (a) TaskDrawer never refreshed `detail` after a mutation because atomic temp+rename triggers `board:reloaded` not `task:updated:<id>` — drawer now subscribes to both events; (b) `svelte-dnd-action` crashed with `originalDragTarget.parentElement undefined` because a `$derived` was swapping the items array mid-drag — Column now keeps a `$state`-backed `items` array re-seeded by `$effect` only when `!dragging`. `gui/internal/cli/mutations.go` adds typed wrappers (`Create`, `Edit`, `Move`, `Close`, `Regenerate`) with `MutationError` classification (binary-not-found / board-not-found / validation / task-not-found / unknown). `gui/app/edit_body.go` implements the only direct-write path: acquires `.board.lock` via `syscall.Flock LOCK_EX`, rejects header/metadata changes via `protectedPrefix`, appends `- YYYY-MM-DD: Edited body via GUI`, writes via temp+fsync+rename, releases the lock BEFORE invoking `tb regenerate` (CLI takes the same flock — would deadlock). `BoardService.LoadBoardWithMode("all")` adds the `archive` bucket to `BoardSnapshot`. Frontend: `Column.svelte` integrates `svelte-dnd-action` with a `dragging` flag that freezes `dndItems` for the duration of a gesture so a `board:reloaded` mid-drag doesn't blow the library's state; `+page.svelte` calls `optimisticMove`/`revert` and pushes a toast on failure. `CreateTaskDialog.svelte` (+ button in topbar). `TaskDrawer.svelte` rewritten: inline metadata edit (priority/type/size/module/tags) → `tb edit`, two-click Archive button → `tb close`, body editor toggle. `BodyEditor.svelte` wraps CodeMirror 6 (markdown lang, line wrapping, history) with `internalChange` flag to avoid keystroke-echo loops; Cmd/Ctrl+S saves. `FilterBar.svelte` filters client-side over the loaded snapshot (types, priorities, modules, tags, agents, parent epic, search) with a "Show archived" toggle that switches the store to `all` mode. `Toast.svelte` is the reusable component (info / success / error). Untrusted markdown is sanitized via `DOMPurify` before `{@html}`. 32 Go tests pass (incl. a real-`tb` integration test that proves flock is held and the protected prefix survives an EditTaskBody round-trip byte-for-byte). `svelte-check` clean (333 files, 0 errors, 0 warnings); production build green.
