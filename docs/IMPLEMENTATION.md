# Implementation Plan

Living tracker. Update as work progresses. Each milestone has a target deliverable and acceptance set drawn from `FEATURES.md`.

Marker legend:
- ☐ todo · ⬚ in progress · ☑ done · ⊘ skipped/deferred

---

## M0 — Documentation foundation · ☑

**Deliverable**: 4 docs + updated root README, sufficient for a new contributor to understand goals and architecture.

- ☑ `docs/PROJECT.md`
- ☑ `docs/ARCHITECTURE.md`
- ☑ `docs/FEATURES.md`
- ☑ `docs/IMPLEMENTATION.md` (this file)
- ☑ Root `README.md`

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
1. ☑ Pre-check: `wails3 doctor` against the Go 1.26.x toolchain → SUCCESS on `wails3 v3.0.0-alpha.91`; toolchain pinned in `ARCHITECTURE.md` § Toolchain
2. ☑ `wails3 init -t sveltekit-ts` in `gui/` (module `tools/tb-gui`); demo `GreetService` + time-emitter stripped
3. ☑ Added `./gui` to root `go.work`
4. ☑ Enabled `application.SingleInstanceOptions` (uniqueID `com.taskboard.tbgui`); `OnSecondInstanceLaunch` restores+focuses the existing window
5. ☑ Backend `gui/internal/cli/cli.go` — `exec` wrapper with `Client.Run` / `Client.RunJSON`, ExitError mapping, ctx cancellation, 7 tests
6. ⊘ Backend `gui/internal/parser/parser.go` — deferred: M2 doesn't need a read-only Go-side markdown parser because the frontend renders the body via `marked`; the only fields we read from `.md` come from `tb show --json` already
7. ☑ Backend `gui/internal/watcher/watcher.go` — fsnotify with pump-goroutine swap design; ignore list (BOARD.md, .next-id, .board.lock, board-root and task-local agent artifacts) + 200ms debounce; 8 unit + 1 integration tests
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

## M6 — Groom flow · ☑

**Deliverable**: Groom button refines task descriptions.

### Tasks
1. ☑ `gui/internal/agent/prompts/groom.md` embedded as `agent.PromptGroom`
2. ☑ `gui/internal/agent/runner.go`: `GroomingDecorator` swaps the runner prompt for groom-mode runs
3. ☑ Service `gui/app/agent_service.go`: `GroomTask`
4. ☑ Frontend `TaskDrawer.svelte` + `Card.svelte`: Groom button, mode-labelled runs, and grooming-needed indicator
5. ☑ Backend triage helper: `BoardService.Triage()` wraps `tb triage --json`, caches the map, and invalidates on board events

**Estimate**: 1 day.

### Risks
- Groom prompt quality is iterative — may need 2–3 revisions after manual testing.

---

## M7 — Polish · ☑

**Deliverable**: daily-use polish for settings, shortcuts, native menu, and tray.

### Tasks
1. ☑ Preferences expanded in `gui/app/preferences.go`: `agent_timeout_minutes` (1-240, default 30), `default_agent` (`none|claude|codex`), `cli_path`, plus existing `max_workers`
2. ☑ `AgentService` timeout provider wired from `SettingsService.GetAgentTimeoutMinutes()` per run, so settings changes apply without restarting the daemon
3. ☑ `SettingsService.SetCLIPath` persists the path and rebuilds the active `BoardService` CLI client without reopening the board
4. ☑ Frontend `preferencesStore` + `SettingsPanel.svelte` expose timeout, max workers, default agent, and CLI path with optimistic writes and rollback toasts
5. ☑ Default-agent preference is visual-only in `TaskDrawer.svelte` for unassigned tasks; it does not auto-write `Agent`
6. ☑ Keyboard shortcuts: `N` opens create, `/` focuses search, `Esc` closes topmost panel/drawer, `Enter` opens DOM-focused cards; typing targets and modifiers are suppressed
7. ☑ `gui/internal/shell` installs a native File/View/Help menu with Open board, Open Recent, Settings, Reload board, About/docs, and Quit
8. ☑ System tray/menu-bar item toggles the main window, exposes Show/Settings/Quit, and swaps idle/running template icons from `agent:run-started` / terminal events

**Estimate**: shipped in one implementation pass.

---

## M10 — Canonical kanban (TB-239) · ☑

**Deliverable**: introduce the canonical kanban `ready` column between `backlog` and `in-progress`, generalise WIP limits to per-column (`wip_limit_ready`, `wip_limit_in_progress`, `wip_limit_code_review`) with a `wip_enforcement: warn|strict` mode, add `tb ready` (commitment with triage gate) and `tb pull` (highest-priority oldest from ready), warn when `tb start` skips ready, and route `tb review --fail` back to `ready` instead of `backlog`. CLI, GUI (board service, watcher, Svelte board/column/api/store), docs and agent prompts all updated in lockstep.

**Acceptance**: `tb create … && tb edit -p P1 && tb ready <ID> && tb pull` flows a single task through the new column with logged moves; un-groomed `tb ready` exits non-zero with a "needs grooming" message; `tb board --json` returns the new `ready` array plus `wipLimits`/`wipCounts`/`wipEnforcement`; the GUI renders the column with `(n/m)` headers and a red badge over the limit; failed review lands in `ready/` (not `backlog/`) with `review-failed`. Type-checks pass, all Go + Svelte tests pass.

---

## M9 — Code-review column (TB-194) · ☑

**Deliverable**: `code-review` is a first-class board status with managed CLI commands, a GUI column + drawer affordances, a `review` agent mode, and a `review-failed` rework loop. Detailed contract in [`board/CONVENTIONS.md`](../board/CONVENTIONS.md) → "Code review workflow"; agent prompt locked at [`gui/internal/agent/prompts/review.md`](../gui/internal/agent/prompts/review.md).

**Acceptance**: `tb review --submit / --target / --notes / --findings / --fail` operate atomically under `.board.lock` and regenerate `BOARD.md`; the GUI renders Backlog / In Progress / Code Review / Done with drag/drop and optimistic move; review-mode runs reuse the existing JSONL/daemon/cancellation pipeline; failed reviews land in backlog tagged `review-failed` with visible findings; resubmit clears the tag.

### Tasks
1. ☑ [TB-195](../board/done/TB-195/TASK.md) — CLI: add `code-review` status and submit flow.
2. ☑ [TB-196](../board/done/TB-196/TASK.md) — CLI: review target / reviewer notes / findings commands.
3. ☑ [TB-197](../board/done/TB-197/TASK.md) — GUI: code-review column + review fields.
4. ☑ [TB-198](../board/done/TB-198/TASK.md) — Agent: review mode + findings.
5. ☑ [TB-199](../board/done/TB-199/TASK.md) — Workflow: `review-failed` marker + retry priority.
6. ☑ [TB-200](../board/done/TB-200/TASK.md) — Docs: code-review workflow.

**Estimate**: shipped in one implementation pass.

---

## M8 — Folder-form tasks + attachments (TB-93) · ☑

**Deliverable**: folder-backed tasks and attachments shipped without breaking legacy file-backed boards. The detailed storage contract lives in [TB-94](../board/done/TB-94.md) and [`docs/ARCHITECTURE.md` → "Folder-form tasks"](ARCHITECTURE.md#folder-form-tasks).

**Acceptance**: file/folder read parity; default folder creation; whole-folder moves/archive; attachment add/remove with validation; GUI picker + drag-and-drop workflow; watcher refresh after attachment operations and folder moves; mixed-board smoke covering CLI, GUI, agent artifacts, archive/restore, regeneration, and orphan checks.

### Tasks
1. ☑ [TB-94](../board/done/TB-94.md) — Define the folder-task on-disk contract before implementation work begins.
2. ☑ [TB-95](../board/done/TB-95.md) — Publish the TB-93 milestone tracker in `docs/FEATURES.md` and `docs/IMPLEMENTATION.md`.
3. ☑ [TB-96](../board/done/TB-96.md) — CLI read and JSON paths treat folder-form and file-form tasks as the same logical task.
4. ☑ [TB-97](../board/done/TB-97.md) — `tb create` defaults to folder-form tasks with an empty `## Attachments` section.
5. ☑ [TB-98](../board/done/TB-98.md) — Move, close/archive, and restore folder-form tasks as whole directories without orphaning artifacts.
6. ☑ [TB-99](../board/done/TB-99.md) — `tb attach <ID> <path>...` atomically promotes legacy file tasks on first attachment.
7. ☑ [TB-100](../board/done/TB-100.md) — `tb attach --rm` removes attachments with path validation and markdown updates.
8. ☑ [TB-101](../board/done/TB-101.md) — `BOARD.md` byte-identical for equivalent file-form, folder-form, and mixed boards.
9. ☑ [TB-102](../board/done/TB-102.md) — Resolve agent state/log paths by storage form, including folder-task stale recovery.
10. ☑ [TB-103](../board/done/TB-103.md) — List, open, add, and remove drawer attachments through `tb` commands.
11. ☑ [TB-104](../board/done/TB-104.md) — Drag-and-drop attachment workflows for cards and the task drawer through `tb`.
12. ☑ [TB-105](../board/done/TB-105.md) — One logical GUI refresh for attachment operations and folder-task moves.
13. ☑ [TB-106](../board/done/TB-106.md) — Mixed-board smoke run with evidence recorded on TB-93.

### Review follow-ups (TB-146..TB-171)
A grand review produced 26 follow-up findings (TB-146..TB-171) covering legacy agent-state migration on promotion, doc/code reconciliation, Windows shell-injection hardening, watcher concurrency, drawer attachment UX (two-click remove, a11y, IEC unit labels, in-flight DnD events), API error-path coverage, and test-infra cleanup. All shipped; a code-review pass during burndown surfaced an additional cross-task remove-confirm data-loss path that was fixed before close.

**Estimate**: tracked across child tasks TB-94 through TB-106 plus TB-146..TB-171 — all done.

---

## Risk register

| # | Risk | Impact | Mitigation | Status |
|---|------|--------|------------|--------|
| R1 | Wails3 alpha + Go 1.26.1 incompatible | Blocks M2+ | Probe in M2 first task; pin tag or downgrade Go | open |
| R2 | fsnotify event loop from CLI's BOARD.md writes | UI flicker / wasted work | Ignore BOARD.md, `.next-id`, `.board.lock`, and board-root/task-local agent artifacts | mitigated by design |
| R3 | `syscall.Flock` POSIX-only | No Windows | Documented; use `gofrs/flock` if needed later | accepted |
| R4 | Agent runs with no sandbox | Untrusted board could harm system | Document, rely on git, encourage trusted boards | accepted |
| R5 | Stale `AgentStatus: running` after crash | Confusing state | M5 stale-recovery on startup | planned |
| R6 | Two GUI instances racing daemon | Duplicate runs / lock contention | Single-instance Wails plugin | planned (M2) |
| R7 | `exec tb ls --json` cost with hundreds of tasks | Slow load | Cache in GUI; invalidate on watcher events | deferred until measured |
| R8 | `tb` not in PATH from GUI | Service calls fail | Settings panel with explicit path; resolve via `exec.LookPath` at startup with friendly error | mitigated by M7 |
| R9 | CodeMirror SSR issues in SvelteKit | M3 blocker | Use `onMount` import; static adapter | planned (M3) |
| R10 | PID re-use on crash | False positive recovery | Cross-check command name; ok for MVP | accepted |
| R11 | Non-atomic CLI writes break unlocked GUI reads | Phantom card deletes, malformed cards | M1 F1.6 mandates atomic temp+rename; reader rule discards malformed parses | planned (M1) |
| R12 | `cancelled` AgentStatus undefined across enum sites | Stale-recovery overwrites cancellation as `failed` | Add `cancelled` to enum everywhere; M5 recovery skips it | planned (M1+M5) |

---

## Completed work log

- 2026-05-19: TB-237 shipped — per-mode agent attribution. The task `Task` struct (`cli/task.go`) grew six new optional metadata pairs — `GroomedBy/GroomStatus`, `ImplementedBy/ImplementStatus`, `ReviewedBy/ReviewStatus` — parsed via the existing `extractFieldAny` path; `maxMetadataLines` bumped 20 → 28 to keep room for tasks that also carry `Parent` / `ReviewRef`. `tb edit` gained `--groomed-by` / `--groom-status` / `--implemented-by` / `--implement-status` / `--reviewed-by` / `--review-status` flags using the same enums + `none` clear sentinel as `-a` / `--agent-status`; new flag names are also registered in `flagsWithValue` so the `reorderArgs` reshuffler keeps `tb create … --flag value Title` style working. JSON wire shape (`cli/json_output.go`) emits the six new camelCase keys on `tb show --json`, `tb ls --json`, and the board snapshot. GUI side: `gui/app/board_service.go` `Task` + `EditTaskInput` mirror the new fields; `gui/internal/cli/mutations.go` forwards the new flags to the CLI; the agent run terminal write in `gui/app/agent_run.go`'s `recordTerminal` now also writes the per-mode pair matching the run's mode via the new `applyPerModeAttribution` helper, under the same `shouldWriteStatus` carve-out so `needs-user` stays a single-cursor status. `ModeResume` updates the originating action's pair via a new `ParentMode` field on `activeRun`, resolved from `resumableSessionID` (extended to surface the parent's mode) for the drawer Resume path and via a new `runModeFor` helper for the daemon's `RunQueuedAgentSync` replay. The frontend `Card.svelte` shows compact G/I/R chips with status-coloured palettes; `TaskDrawer.svelte` adds a "Per action" sub-list above the "Run history" pane. The legacy `Agent`/`AgentStatus` fields continue to reflect the most recent run, preserving back-compat for `tb triage`, auto-implement (TB-177/TB-179), auto-groom (TB-172), daemon pickup (TB-5), and stale recovery (TB-130). Verification: `cd cli && go test ./...`, `cd gui && go test ./app/... ./internal/...`, `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run` all pass; new tests: `TestEditPerModeAttribution`, `TestEditPerModeAttributionInvalidValues` (subprocess re-exec for the validator's `os.Exit(1)` path), `TestRecordTerminalPerModeAttribution` (table-driven across groom/implement/review), `TestRecordTerminalResumeUsesParentMode`. Wails bindings regenerated automatically on save.
- 2026-05-19: TB-194 shipped — code-review column workflow (epic with TB-195/196/197/198/199/200). `code-review/` is a first-class active board status; `tb review --submit / --target / --notes / --findings / --fail` are the managed surface. **TB-195/196/199 (CLI)**: `cli/board.go` adds `code-review` to `statusDirs` / `allStatusDirs` / `resolveStatus` / `resolveStatusFilter` with `cr` / `review` aliases; `cli/regenerate.go` renders a `## Code Review` section in `BOARD.md`; `cli/json_output.go` adds a `codeReview` bucket to `boardSnapshotJSON`; `cli/review.go` owns submit/section/fail (the file consolidates the surfaces from TB-195, TB-196, and TB-199). Submit accepts in-progress (happy path) and backlog-with-`review-failed` (resubmit-after-rework, clears the tag on move); --fail moves a code-review task back to backlog with findings + `review-failed` tag; section placement rules in `cli/edit.go`'s `upsertTaskSection` slot Review Target/Notes/Findings between Acceptance Criteria and Related Tasks/Attachments/Log. **TB-197 (GUI)**: `gui/app/board_service.go` widens `BoardSnapshot` with `CodeReview []Task` and exposes `SubmitReview`/`SetReviewTarget`/`SetReviewerNotes`/`SetReviewFindings`/`FailReview` Wails methods; `gui/internal/cli/cli.go` adds `RunWithStdin` for stdin-piped section writes; the frontend `Board`/`Column` render the new column between In Progress and Done; `Card.svelte` adds a `↩` review-failed marker (red badge) for backlog tasks tagged `review-failed`; `TaskDrawer.svelte` exposes "Submit for review" on in-progress tasks and "Review" (review-mode agent) on code-review tasks. **TB-198 (Agent)**: new `gui/internal/agent/prompts/review.md` + `PromptReview` + `ReviewDecorator` + `ModeReview`; `parseRunMode`, `runMethodName`, `runnerForMode` learn review; `AgentService.ReviewTask` is the entry point. Run history labels review runs `mode=review`. Review-mode agents are read-only against implementation files — they only mutate the task via `tb review` commands. **TB-199 (workflow)**: `review-failed` is a tag, not an AgentStatus. It is cleared automatically on `tb review --submit` when the task moves from backlog → code-review. The GUI marker is shown by Card.svelte; the CLI exposes the tag through the existing `tags` JSON field so TB-177/TB-179 auto-implement can prioritize `review-failed` tasks once that epic lands. **TB-200 (docs)**: `board/CONVENTIONS.md` + `board/SKILL.md` describe the happy and failure paths plus the AgentStatus / status / tag distinctions; `cli/templates.go` keeps generated docs in sync for new boards (`tb init` now creates `board/code-review/`); `docs/ARCHITECTURE.md` updated for the status taxonomy + agent modes; `CLAUDE.md` gains the code-review invariant. Verification: `cd cli && go test ./...`, `cd gui && go test ./app/... ./internal/...`, `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run`; new tests `TestReviewSubmit*`, `TestReviewWriteTargetCreatesSection`, `TestReviewWriteNotesReplacesExisting`, `TestReviewWriteFindingsRejectsEmptyStdin`, `TestReviewFailMovesToBacklogWithMarker`, `TestReviewFailRejectsNonCodeReview`, `TestStatusAliasesCodeReview`, `TestActiveStatusFilterIncludesCodeReview`, `TestBoardJSONIncludesCodeReviewBucket`, `TestReviewTask_HappyPath_Success`. Wails bindings regenerated via `wails3 generate bindings -ts`.
- 2026-05-19: TB-182 shipped — user-attention handoff protocol (epic with TB-183 / TB-184 / TB-185). `AgentStatus: needs-user` is a new closed-set value alongside the existing `queued | running | success | failed | cancelled | interrupted` enum. Autonomous agents that cannot continue safely stop with two managed CLI calls — `tb edit <ID> --user-attention -` (heredoc body with reason / question / attempted context / unblock condition) and `tb edit <ID> --agent-status needs-user`. The user resolves by reading the ask and clearing the status with `tb edit <ID> --agent-status none`. **TB-183**: CLI updated — `validAgentStatuses` widened, parser/validator/JSON round-trip the new value, `--user-attention file|-` flag added as a peer of `--goal` / `--acceptance` using the existing `upsertTaskSection` + atomic-write path; new tests `TestEditAgentStatusNeedsUser` and `TestEditUserAttentionSection`. **TB-184**: docs sweep — `board/CONVENTIONS.md` and `board/SKILL.md` describe the handoff and the AgentStatus enum table; `cli/templates.go` keeps generated docs in sync; `gui/internal/agent/prompts/implement.md` + `groom.md` carry the protocol, locked by prompt-text assertions in `TestPromptImplement_NonEmptyAndContainsPlaceholders` / `TestPromptGroom_StatesGroomingMutationContract`; `docs/ARCHITECTURE.md` + `docs/FEATURES.md` updated. **TB-185**: GUI guard + surfacing — `ErrNeedsUserAttention` short-circuits `RunAgent` / `GroomTask` / `ResumeAgent` so manual run paths cannot retry an unresolved task; `recordTerminal` has a scoped carve-out — when the running agent set `needs-user` mid-run, the exit-mapped `success` / `failed` status is dropped on the floor while the JSONL `finished` line still records the true exit so run history is intact (cancel and recovery's `interrupted` write still go through — explicit human/system intent wins). Daemon is filtered by construction (`isReadyForDaemon` already requires `AgentStatus == "queued"`); a new `IsAutomationEligible` helper is reserved for TB-174 / TB-179 to consume. Frontend: `Card.svelte` shows a `?` indicator on needs-user cards; `TaskDrawer.svelte` extracts the `## User Attention` section, renders it in a purple panel near the agent controls, surfaces a `needs-user` pill, disables Run / Groom with explanatory tooltips, and offers a one-click "Clear status" button that calls `editTask(id, { agentStatus: 'none' })`. Verification: `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run` all pass; new tests: `TestStartAgentRunRejectsNeedsUser`, `TestPostRunPreservesNeedsUser` (table-driven, success and failed branches), `TestCancelOverridesNeedsUser`, plus drawer/card UI tests covering the pill, panel, fallback copy, and Clear button.
- 2026-05-19: TB-130 shipped — agent session resume + interrupted-run recovery. 12 sub-tasks landed in build order:
  - **TB-131** Closed-set sweep: `interrupted` AgentStatus + `resume` Mode added to `validAgentStatuses` (cli/task.go), `--agent-status` flag help text (cli/edit.go + cli/main.go), `StatusInterrupted` (gui/internal/agent/state.go), `ModeResume` (gui/internal/agent/runner.go), `parseRunMode` (gui/app/agent_finish.go), frontend `Run` interface unions (runs.ts), and Card.svelte CSS stub.
  - **TB-132** JSONL schema additions: `EvSession` constant + `Event.SessionID/ResumedFrom/ResumedFromRun/Cwd/RunEnv` fields, centralized `FilterTBEnv` allowlist (only `TB_`-prefixed keys land in `run_env`), `runRecoveryView` extension, new `ResumeCandidate` + `resumableSessionID` helper (latest-run-only — never walks backward), `Run` struct camelCase tags surfaced to the Wails-generated frontend types.
  - **TB-133** Post-`started` session-write hook in `runGoroutine`'s `OnStarted` callback — fires only when `ar.SessionID != ""` so TB-133 is a no-op until TB-135/TB-136 light up. Failure path uses `slog.Warn` like the surrounding diagnostics.
  - **TB-134** Codex `--json` switch + `codexJsonTranslator` (mirrors `claudeTranslator`): tolerant render of known event shapes (session_meta, agent_message, function_call, exit, etc.), `[type] {json}` breadcrumb for unknowns, passthrough for non-JSON. The translator's `OnSessionID` callback fires exactly once on the first UUIDv4-shaped session id at top level or one level into `payload` — non-UUID strings (e.g. `call_abc123`) are ignored.
  - **TB-135** Claude pre-allocation via `GenerateSessionID` (canonical UUIDv4 from `crypto/rand`) + `--session-id <uuid>` flag. Distinct from `GenerateRunID` (32-bit hex) because Claude rejects non-UUID input.
  - **TB-136** Codex `OnSessionID` wired into `runGoroutine` — translator → callback → JSONL session event with the captured id + the live PID under `ar.mu`.
  - **TB-137/TB-251** Recovery split: dead PID + SessionID → `interrupted` (resumable), no SessionID → `lost` (daemon/run-state loss without a captured session). Cancelled carve-out short-circuits first — user-cancelled with SessionID stays `cancelled`, never `interrupted`.
  - **TB-138** Claude resume backend: `ResumeDecorator` (mirror of `GroomingDecorator`) + embedded `prompts/resume.md` + `runnerForMode` `ModeResume` branch + `ResumeAgent` service method. Claude args switch to `-r <uuid>` (mutually exclusive with `--session-id`). New `activeRun.Cwd` / `activeRun.Env` consumed by `runGoroutine` for resume's persisted execution context.
  - **TB-139** Codex resume args: `exec --json resume <uuid> <prompt>`. New id codex emits flows through the existing TB-136 callback as a fresh session event — the chain stays traceable via each queued event's `resumed_from`.
  - **TB-140** Frontend Resume button (drawer-only), interrupted pill, `↻ r_xxxx` chip. `taskAgentStatus === 'interrupted'` is the gate (not `liveStatus`); `agent:run-queued` handler extracts `resumed_from` / `resumed_from_run` from Wails payload.
  - **TB-141** Fake-runner integration suite: `TestResumeCycle_KillRecoverResume` drives queued+started+session → RecoverStale → ResumeAgent through the shared `stubRunner`. `TestResumeCycle_KillBeforeSessionStaysLost` locks the negative gate. Plus `TestResumeAgent_CodexHappyPath` mirror of the Claude happy path.
  - **TB-142** Docs sweep — this entry plus AgentStatus enum widening across `CLAUDE.md`, `cli/CLAUDE.md`, `docs/ARCHITECTURE.md`, `docs/FEATURES.md` (new F5.5).

  Verification: `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check && npm test`. Wails bindings regenerated via `wails3 generate bindings -ts` (frontend/bindings is gitignored — bindings rebuild on every `task dev` / `task build`).
- 2026-05-15: M8 shipped — folder-form tasks + attachments (TB-93). All 13 child milestones (TB-94..TB-106 / TB-108) shipped: docs contract, CLI read/create/move/attach parity, BOARD.md byte-identical, task-local agent state with stale recovery, GUI drawer attachments list + picker + DnD, watcher single-event-per-mutation, mixed-board smoke. A grand review produced 26 follow-up findings (TB-146..TB-171) covering legacy agent-state migration on promotion (TB-146), doc/code reconciliation (TB-147/148/167), Windows shell-injection hardening (TB-149) and CLI argv `--` terminator (TB-158), watcher concurrency + promotion-rename synthesis (TB-150/151), drawer attachment UX (two-click remove, a11y aria-labels, IEC unit labels, in-flight DnD events) (TB-152..155/162..165/169), API error-path test coverage (TB-163), test-infra cleanup (TB-160/166/168), and one descoped performance task (TB-170, recorded in its Decision section). A code-review pass during burndown surfaced a cross-task remove-confirm data-loss path that was fixed before close. Verification: `cd cli && go test ./`, `cd gui && go test ./...`, `cd gui/frontend && npm test && npm run check`.
- 2026-05-14: M6 shipped — groom mode is a first-class agent run mode. `gui/internal/agent/prompts/groom.md` is embedded as `PromptGroom`; `GroomingDecorator` is the only mode-aware runner layer and swaps the normal implementation prompt for the groom prompt while preserving the underlying Claude/Codex process behavior. `AgentService.GroomTask` reuses the RunAgent lifecycle with `mode=groom`, so JSONL events carry `mode` and the drawer can label groom runs separately. `BoardService.Triage()` shells out to `tb triage --json`, caches a task-ID to reasons map, and invalidates on watcher events; the frontend `triageStore`, `Card.svelte`, and `TaskDrawer.svelte` surface "needs grooming" indicators and offer the Groom action. Verification: `cd gui && go test ./...`, `cd gui/frontend && npm test`, `cd gui/frontend && npm run check`.
- 2026-05-13: docs PROJECT/ARCHITECTURE/FEATURES drafted; plan synced with feedback (direct body writes allowed under flock; archive as first-class filter; daemon stale-recovery in M5; root `go.work`)
- 2026-05-13: Codex adversarial review applied — README path corrected to the then-current CLI path; atomic-write invariant documented and added to M1 (F1.6); `cancelled` added as a first-class `AgentStatus` value with carve-out from stale-recovery
- 2026-05-13: M1 shipped — `tb/` → `cli/` rename (history preserved as bundle outside repo); root `go.work` added; `cli/atomicfs.go` introduced with `writeFileAtomic` + tests; all task `.md` writers migrated; `Agent`/`AgentStatus` fields on `Task` with `tb edit -a` / `--agent-status` + enum validation; `cmdCreate` and `cmdEdit` now regenerate `BOARD.md`; new `resolveStatusFilter` implements `backlog|in-progress|done|archive|active|all` semantics; `findTask` extended to archive so archived tasks can be moved back; `cli/json_output.go` adds `--json` output for `tb ls`, `tb show`, `tb board` (empty results render as `[]` / `{}`)
- 2026-05-13: M2 shipped — `gui/` scaffolded with Wails3 alpha.91 + SvelteKit-TS; backend modules `gui/internal/cli`, `gui/internal/watcher` (pump-goroutine + 200ms debounce), `gui/app/board_service.go` (LoadBoard/GetTask, status bucketing, ErrNoBoard/ErrNotFound), `gui/app/settings_service.go` (OpenBoard/PickBoardDialog/recents at `$XDG_CONFIG_HOME/tb-gui/recent.json`); frontend `Board`/`Column`/`Card`/`TaskDrawer` Svelte components with `marked` for read-only markdown; `+page.svelte` orchestrator with empty-state, recent-board list, and Wails event wiring (`board:reloaded`, `board:opened`, `task:updated:*`). 30 Go tests pass; `wails3 generate bindings` emits 2 services / 7 methods / 6 models. Runtime acceptance via `/ui-test` at end of epic.
- 2026-05-13: M4 review fixes — moved `AgentStatus: running` write from runGoroutine into OnStarted (now guarded by wasCancelled() under ar.mu) so cancel-before-OnStarted can't lose the race against a stale `running` write; new `TestCancelRun_BeforeOnStarted` reproduces the race deterministically with a slow-start runner. exec.go now verifies `syscall.Getpgid(pid) == pid` after `cmd.Start` and zeroes pgid otherwise; killActiveRun and the timeout escalation fall back to SIGKILL-on-pid when pgid==0 rather than risk SIGKILL'ing an unrelated process group. state.go's AppendEvent/NewLogWriter swap their stat+mkdir for a stricter `requireBoardDir(Open+Stat+IsDir)` so a missing boardDir between checks no longer lets MkdirAll auto-create it. AgentRunLog.svelte takes `taskId` as a separate prop (no longer derives it from the runsStore Run record) so GetRunLog never races store hydration.
- 2026-05-14: M5 shipped — agent daemon with autopickup + crash recovery. New `gui/internal/daemon` package: `Daemon` with `New`/`Activate`/`Deactivate`/`Close` lifecycle; N-worker pool over a buffered task-ID channel (N = `max_workers` ∈ [1,4], persisted at `preferences.json`); in-memory active-set keyed by `task_id`, cross-checked against `AgentService.HasActiveRun` (new public accessor); `pidAlive(pid, expectedAgent)` with two-step `ps -o comm=` + `ps -o args=` fallback for npm-shebang `claude`/`codex` wrappers (TB-59); `EventSink` implements `watcher.Emitter` and forwards `task:updated:<id>` + `board:reloaded` to the daemon via a `TeeEmitter` chained alongside the Wails app bus (TB-58); strict Activate ordering — `recovery.RecoverStale` → watcher sink already registered (via `main.go` construction order) → startup queue scan. TB-54 split public `RunAgent` from internal `RunQueuedAgentSync`: the public method writes the queued JSONL + AgentStatus + activeRun placeholder outside `s.mu` (rollback on I/O failure), narrowed `s.mu` to the active-map insert/delete only; the daemon-only synchronous executor accepts `AgentStatus=queued`, uses the caller-supplied ctx so `Daemon.Close()` propagates to `exec.CommandContext`, and shares `runGoroutine` with the M4 manual path. JSONL `started` event now carries `agent` (TB-54 schema change) so TB-60's pidAlive cross-check has an unambiguous source; the recovery reader still falls back to the `queued` event's `agent` for pre-M5 JSONL files. `gui/app/agent_recovery.go` implements `daemon.Recovery`: walks `AgentStatus=running` tasks, syncs `.md` when JSONL has a finished record (cancelled→cancelled per TB-61 carve-out, success/failed→that status), writes synthetic `finished{interrupted}` when the dead run captured a session id, writes synthetic `finished{lost}` when recovery lost the run result without a resumable session, and leaves live PIDs alone (no re-attach in M5). `finishCancelled(c, ar, boardDir, reason)` helper factored from `CancelRun` and used by both the M4 user-cancel path (`reason="user cancelled"`) and the daemon shutdown path (`reason="shutdown"`); idempotent via `ar.finishOnce` so a CancelRun racing shutdown does not double-write. `SettingsService.OpenBoard` gained a `BoardActivator` hook — Deactivate prior board before Activate new one. `gui/main.go` wires daemon construction before `app.Run`, starts the sink reader goroutine, and defers `daemon.Close()` so a window-close triggers the 5s grace + JSONL flush. Integration test (`TestDaemonShutdown_FlushesCancelledJSONL`) drives a real `tb` board + the full daemon stack: Enqueue → runner.Run blocks on ctx → `Daemon.Close()` → JSONL ends with `finished{cancelled, reason:"shutdown"}` and `tb show` reports `AgentStatus: cancelled`. All Go tests pass with `-race`. Manual `kill -9` mid-flight harness (multi-process; can't be expressed inside a Go test binary) is documented as a smoke step in `gui/internal/daemon/README.md`.
- 2026-05-14: M7 shipped — preferences grew `agent_timeout_minutes`, `default_agent`, and `cli_path` next to `max_workers`, with clamp/default tests and live CLI-client reload on `SetCLIPath`. `AgentService` now reads the timeout through a late-bound provider per run. Frontend `preferencesStore` and `SettingsPanel.svelte` expose all four settings; the TaskDrawer shows the default agent as a visual fallback for unassigned tasks without writing metadata. `+page.svelte` owns global shortcuts (`N`, `/`, `Esc`, `Enter`) through a tested `shortcuts.ts` helper that suppresses typing targets and modifier chords. `gui/internal/shell` installs the native File/View/Help application menu, rebuilds Open Recent after board opens, emits `settings:open-panel` for the Svelte panel, and registers a system tray item with idle/running template icons driven by `agent:run-started` and terminal events. Verification: `cd gui && go test ./...`, `cd gui/frontend && npm test`, `cd gui/frontend && npm run check`.
- 2026-05-13: M4 shipped — agent assignment + manual runs from the GUI. `gui/internal/agent/` adds `Runner` interface, `ClaudeRunner`/`CodexRunner` with own process group (Setpgid + env whitelist + bufio.Scanner line streaming + OnStarted callback before output), embedded `prompts/implement.md` with locked `{{TASK_ID}}/{{TASK_TITLE}}/{{TASK_BODY}}` placeholders + `RenderPrompt`, and `state.go` (closed event vocabulary `queued|started|stdout|stderr|finished`, per-task mutex for concurrent JSONL appends, per-run log file). `gui/app/agent_service.go` exposes `AssignAgent` (with `none` clear sentinel; the CLI gained matching `tb edit -a none` / `--agent-status none` support that deletes the metadata line), `RunAgent` (sync queued + Wails + AgentStatus + activeRun register; goroutine spawns Runner, streams stdout/stderr to JSONL + log file + Wails `agent:run-log` events, post-run handler writes `finished` unless TB-48 marked it cancelled, error→status map handles binary-not-found/timeout/non-zero-exit), `CancelRun` (5-step ordering: mark → SIGTERM → 5s grace → SIGKILL on pgid → JSONL `cancelled` → Wails emit → `tb edit --agent-status cancelled` last so a crash between 4 and 5 still leaves the durable JSONL for M5 to reconcile), `ListRuns` (rolls per-task JSONL into `Run` records sorted by StartedAt desc, tolerates trailing partial line), `GetRunLog`. Frontend: `runsStore.ts` keyed by run_id with Wails handlers for run-queued/started/finished and `runsByTask` selector; `AgentRunLog.svelte` subscribes to `agent:run-log` for live runs and falls back to `GetRunLog` for terminal runs, ANSI strip + sticky-bottom scroll; `TaskDrawer.svelte` adds Agent dropdown + Run/Cancel buttons (two-click confirm on Cancel) + status pill + past-runs list; `Card.svelte` shows agent badge with single-letter glyph (C/X). Tests: 42 Go tests pass (incl. real-`tb` AssignAgent persistence proof per F4.1; live RunAgent lifecycle proves AgentStatus durability for success/failed/binary-not-found paths; CancelRun integration test spawns a real /bin/sh script that ignores SIGTERM and spawns a child sleep, then verifies both processes die within ~6s via `syscall.Kill(pid, 0)` liveness probe AND that exactly one `finished{cancelled}` JSONL line exists for that run_id AND that `tb show` reports `AgentStatus: cancelled` AND that a fresh `AgentService` instance reading the same task still sees `cancelled`); 8 Vitest tests cover runsStore hydration, sort order, queued-tiebreaker, and the three Wails event handlers; `svelte-check` clean (380 files, 0 errors, 0 warnings); production build green. `agent:run-queued` was added as a fourth lifecycle event so the frontend can render a queued pill before the runner actually spawns.
- 2026-05-13: M3 shipped (closed) — TB-3 closed after interactive `wails3 dev` smoke (created TB-42 via dialog, edited priority P2→P1 inline, body edit through CodeMirror writes via `EditTaskBody` under `.board.lock`, two-click Archive sent the task to archive, Show-archived toggle materialized the archive column with both archived tasks, DnD moved TB-5 backlog↔in-progress through `tb mv` and persisted log entries on disk). Two real bugs caught during smoke and fixed: (a) TaskDrawer never refreshed `detail` after a mutation because atomic temp+rename triggers `board:reloaded` not `task:updated:<id>` — drawer now subscribes to both events; (b) `svelte-dnd-action` crashed with `originalDragTarget.parentElement undefined` because a `$derived` was swapping the items array mid-drag — Column now keeps a `$state`-backed `items` array re-seeded by `$effect` only when `!dragging`. `gui/internal/cli/mutations.go` adds typed wrappers (`Create`, `Edit`, `Move`, `Close`, `Regenerate`) with `MutationError` classification (binary-not-found / board-not-found / validation / task-not-found / unknown). `gui/app/edit_body.go` implements the only direct-write path: acquires `.board.lock` via `syscall.Flock LOCK_EX`, rejects header/metadata changes via `protectedPrefix`, appends `- YYYY-MM-DD: Edited body via GUI`, writes via temp+fsync+rename, releases the lock BEFORE invoking `tb regenerate` (CLI takes the same flock — would deadlock). `BoardService.LoadBoardWithMode("all")` adds the `archive` bucket to `BoardSnapshot`. Frontend: `Column.svelte` integrates `svelte-dnd-action` with a `dragging` flag that freezes `dndItems` for the duration of a gesture so a `board:reloaded` mid-drag doesn't blow the library's state; `+page.svelte` calls `optimisticMove`/`revert` and pushes a toast on failure. `CreateTaskDialog.svelte` (+ button in topbar). `TaskDrawer.svelte` rewritten: inline metadata edit (priority/type/size/module/tags) → `tb edit`, two-click Archive button → `tb close`, body editor toggle. `BodyEditor.svelte` wraps CodeMirror 6 (markdown lang, line wrapping, history) with `internalChange` flag to avoid keystroke-echo loops; Cmd/Ctrl+S saves. `FilterBar.svelte` filters client-side over the loaded snapshot (types, priorities, modules, tags, agents, parent epic, search) with a "Show archived" toggle that switches the store to `all` mode. `Toast.svelte` is the reusable component (info / success / error). Untrusted markdown is sanitized via `DOMPurify` before `{@html}`. 32 Go tests pass (incl. a real-`tb` integration test that proves flock is held and the protected prefix survives an EditTaskBody round-trip byte-for-byte). `svelte-check` clean (333 files, 0 errors, 0 warnings); production build green.
