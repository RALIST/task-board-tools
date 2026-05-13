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

## M2 — Wails3 read-only GUI · ☐

**Deliverable**: launch GUI on a board, see live kanban (read-only).

### Tasks
1. ☐ Pre-check: `wails3 doctor` against Go 1.26.1; if incompatible, pin a Wails3 tag or document Go downgrade
2. ☐ `wails3 init -t sveltekit-ts` in `gui/`
3. ☐ Add `./gui` to root `go.work`
4. ☐ Enable Wails3 single-instance plugin
5. ☐ Backend `gui/internal/cli/cli.go` — `exec` wrapper for `tb`
6. ☐ Backend `gui/internal/parser/parser.go` — read-only markdown parser
7. ☐ Backend `gui/internal/watcher/watcher.go` — fsnotify with ignore list + debounce
8. ☐ Service `gui/app/settings_service.go` — project root, recent boards, board picker
9. ☐ Service `gui/app/board_service.go` — `LoadBoard`, `GetTask` (read-only methods)
10. ☐ Frontend deps: `svelte-dnd-action`, `svelte-markdown`, `svelte-codemirror-editor`
11. ☐ Frontend `src/lib/api.ts` — typed Wails bindings re-export
12. ☐ Frontend `src/lib/stores/board.ts`, `selection.ts`
13. ☐ Frontend `src/lib/components/Board.svelte`, `Column.svelte`, `Card.svelte`
14. ☐ Frontend `src/lib/components/TaskDrawer.svelte` (read-only)
15. ☐ Frontend `src/routes/+page.svelte` — assembly
16. ☐ Acceptance tests (manual): live update via `tb mv`, second-instance lock

**Estimate**: 2 days.

### Risks
- **Wails3 alpha API surface** may differ from v2 docs. Build a `hello world` binding first as a probe.
- CodeMirror import may need SvelteKit SSR fixup (`+page.svelte` is static, but components may try SSR — use `<script context="module">` or `onMount`).
- macOS code signing for unsigned dev builds — Wails docs cover this.

---

## M3 — Mutations + DnD + editor · ☐

**Deliverable**: full CRUD via GUI; DnD reflects status changes.

### Tasks
1. ☐ Service `board_service.go`: `CreateTask`, `EditTask`, `MoveTask`, `CloseTask`, `Regenerate` (all via `exec tb`)
2. ☐ Service `board_service.go`: `EditTaskBody` — direct write under `.board.lock` with rules (see ARCHITECTURE.md "Locking")
3. ☐ Frontend `Column.svelte`: integrate `svelte-dnd-action`; optimistic moves; revert on error
4. ☐ Frontend `CreateTaskDialog.svelte`
5. ☐ Frontend `TaskDrawer.svelte`: editable metadata fields + body editor
6. ☐ Frontend `FilterBar.svelte`: client-side filtering over `boardStore`
7. ☐ Frontend `Toast.svelte` for errors
8. ☐ Filter: "Show archived" toggle adds Archive column
9. ☐ Manual acceptance tests

**Estimate**: 2 days.

### Risks
- `svelte-dnd-action` Svelte-5 compatibility — verify with a small spike first.
- Body editor write contract — must reject changes that touch metadata block. Add a Go-side validator in `EditTaskBody`.

---

## M4 — Manual agent runs · ☐

**Deliverable**: assign agent in GUI, click Run, see live log.

### Tasks
1. ☐ `gui/internal/agent/runner.go` — `Runner` interface, `Mode` type, `RunResult`
2. ☐ `gui/internal/agent/claude.go`, `codex.go` — implementations
3. ☐ `gui/internal/agent/prompts/implement.md` (embedded)
4. ☐ `gui/internal/agent/state.go` — JSONL writer, log file rotation per run
5. ☐ Service `gui/app/agent_service.go` — `AssignAgent`, `RunAgent`, `CancelRun`, `ListRuns`
6. ☐ Wails events: `agent:run-started`, `agent:run-log`, `agent:run-finished`
7. ☐ Frontend `Card.svelte`: agent badge
8. ☐ Frontend `TaskDrawer.svelte`: agent dropdown + Run + Cancel buttons
9. ☐ Frontend `AgentRunLog.svelte` — streaming logs
10. ☐ Frontend `runsStore.ts` — keyed by `run_id`

**Estimate**: 2 days.

### Risks
- `claude -p` and `codex exec` argument shapes — confirm flags by running them once. Adjust prompts.
- Stdout buffering: ensure agents flush often; use `cmd.StdoutPipe` + `bufio.Scanner`.
- Process group: spawn agents in their own process group so kill cascades to children.

---

## M5 — Daemon auto-pickup + durability · ☐

**Deliverable**: queued tasks auto-run; crashes recover.

### Tasks
1. ☐ `gui/internal/daemon/daemon.go` — main goroutine, queue, worker pool
2. ☐ Stale-running recovery on startup (PID check, JSONL replay)
3. ☐ Scan on start + fsnotify subscription for queued-trigger
4. ☐ Dedup map; semaphore default 1
5. ☐ Graceful shutdown (5s grace)
6. ☐ Hook into Wails `OnStartup` / `OnShutdown`

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
