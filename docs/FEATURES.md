# Features

Features grouped by milestone. Each has acceptance criteria — implementation isn't done until those pass.

Status notation: ☐ planned · ⬚ partial · ☑ done.

> **Folder-form tasks (TB-93 epic):** the on-disk contract for tasks stored as `<status>/<ID>/TASK.md` directories — including attachments, per-task agent state/logs, lock semantics, atomic-write rules, and the file → folder promotion procedure — is specified in [`docs/ARCHITECTURE.md` → "Folder-form tasks"](ARCHITECTURE.md#folder-form-tasks). All child tasks of TB-93 must conform to that section.

---

## M0 — Documentation foundation

- ☑ **F0.1** `docs/PROJECT.md` describing product, users, scenarios, glossary
- ☑ **F0.2** `docs/ARCHITECTURE.md` with components, on-disk format, locking rules, agent state
- ☑ **F0.3** `docs/FEATURES.md` (this file)
- ☑ **F0.4** `docs/IMPLEMENTATION.md` — living tracker of milestones, risks
- ☑ **F0.5** Root `README.md` updated for two-binary layout

**Acceptance**: a developer who has never seen the project can read the 4 docs and understand goals, scope, and how to build/run.

---

## M1 — CLI extensions

### F1.1 — Repo restructure ☑
- `tb/` → `cli/`. Existing tests pass.
- Root `go.work` lets `go build ./cli` work from repo root.
- **Acceptance**: `cd cli && go build -o tb . && ./tb ls --status all` works on an existing board untouched.

### F1.2 — Agent metadata fields ☑
- New optional fields `**Agent:**` and `**AgentStatus:**` in task `.md`.
- Parsed by `parseTaskFile`. Empty if absent.
- Settable via `tb edit -a <agent> --agent-status <status>`.
- `AgentStatus` enum: `queued | running | success | failed | cancelled | interrupted | lost | needs-user`. `failed` is reserved for agent-reported failures; `interrupted` and `lost` are recovery-initiated daemon-loss states. Resume availability is derived from the latest run's captured `session_id` plus a terminal task status (`interrupted`, `lost`, `failed`, `cancelled`, or `success`), not from `interrupted` alone. `needs-user` (TB-182) is the autonomous-agent handoff — the task pauses with a `## User Attention` section, automation skips it, and the user clears with `tb edit <ID> --agent-status none`.
- **Acceptance**: `tb edit WS-1 -a claude --agent-status queued && tb show WS-1 | grep Agent` shows both fields.

### F1.3 — JSON output ☑
- `tb ls --json` → JSON array of task objects (camelCase keys).
- `tb show <ID> --json` and `tb show --json <ID>` both work; output is `{metadata, body}`.
- `tb board --json` → object with `epics`, `activeEpics`, `finishedEpics`, `inProgress`, `backlog`, `recentlyDone`.
- Empty result → `[]` or empty object, never prose like "No tasks found.".
- **Acceptance**: `tb ls --json | jq .` parses without errors for both empty and populated boards. All Task fields present in output.

### F1.4 — Status semantics ☑
- `--status active` = backlog + ready + in-progress + code-review + done.
- `--status archive` = archive directory only.
- `--status all` = active + archive (everything on disk).
- Default for `tb ls` remains `backlog` (backward-compatible).
- **Acceptance**: `tb ls --status archive --json` returns only archived tasks. `tb ls --status all` returns archive entries too.

### F1.7 — Canonical kanban: `ready` + pull mechanics ☑
- Adds a `ready` column between `backlog` and `in-progress` (canonical kanban commitment column).
- `tb ready <ID>` promotes a backlog task to `ready`, running the same gate as `tb triage` (priority + non-placeholder goal).
- `tb pull` (no arg) auto-picks the highest-priority oldest ready task and moves it to `in-progress`. `tb pull <ID>` overrides selection.
- `tb start <ID>` continues to work for backwards compatibility but emits a stderr warning when the source is `backlog` (skipping the commitment column).
- Per-column WIP limits in `.tb.yaml`: `wip_limit_ready`, `wip_limit_in_progress`, `wip_limit_code_review`. Legacy `wip_limit` scalar still seeds `in-progress`. `wip_enforcement: warn` (default) warns; `strict` blocks moves over the limit.
- `BOARD.md` renders a `## Ready` section between Code Review and Backlog with a `(n/m)` WIP header on limited columns (with `⚠` when at/over).
- `tb board --json` adds `ready` array plus `wipLimits`, `wipCounts`, and `wipEnforcement` fields.
- Failed code review (`tb review --fail`) now returns the task to `ready` (was `backlog`) with the `review-failed` tag.
- **Acceptance**: `tb create … && tb edit <ID> -p P1 && tb ready <ID> && tb pull` flows a single task from backlog through ready to in-progress with logged entries on each move. `tb ready <noPriorityID>` exits non-zero with a "needs grooming" message.

### F1.5 — Regenerate consistency ☑
- `tb create` and `tb edit` call `regenerateBoard` at the end of the operation.
- Documented behavior: `BOARD.md` is always up-to-date after any mutation.
- **Acceptance**: `tb edit WS-1 -p P0 && grep "P0" board/BOARD.md` — change is visible without a manual `tb regenerate`.

### F1.6 — Atomic task-file writes ☑
- Every task-file mutation writes via temp file + `os.Rename` (the pattern `regenerate.go` already uses).
- Sites to convert: `create.go:128` (new task), `create.go:229/252/257/267/273` (`addTagToTaskFile`, `addChildToSubtasks`), `edit.go:116`, `move.go:135` (move destination), `board.go:335` (`archiveTask`), `scan.go:122` (scan-created tasks).
- Single shared helper `writeFileAtomic(path string, data []byte, perm os.FileMode) error` in `cli/board.go` (or a new `cli/atomicfs.go`).
- **Why**: M2+ GUI readers do not hold the board lock; they rely on this invariant for safe snapshots (see `docs/ARCHITECTURE.md` → "Locking and atomic writes").
- **Acceptance**: `grep -nE 'os\.WriteFile\(.*\.md' cli/*.go` returns no hits outside `cli/atomicfs.go` (or wherever the helper lives). Concurrent `tb edit` in a tight loop while a Go reader parses every event produces zero parses with empty header. (Manual test: run the loop for 30s; expect zero "missing header" log entries from a tiny `parseTaskFile` harness.)

---

## M2 — Read-only GUI

### F2.1 — Wails3 + Svelte scaffold ☑
- `gui/` initialized via `wails3 init -t sveltekit-ts`.
- Builds and runs (`wails3 dev`).
- Single-instance lock enabled.
- **Acceptance**: `wails3 dev` opens a window. Starting a second instance focuses the first.

### F2.2 — Board picker / context ☑
- On first start: native folder picker → pick project root (containing `.tb.yaml`).
- Recent boards saved to `~/.config/tb-gui/recent.json`.
- Menu item "Open Board…" switches active board.
- **Acceptance**: pick a board, see its tasks; close and reopen — same board loads.

### F2.3 — Three-column kanban (read-only) ☑
- Columns (canonical kanban, in pull order): backlog / ready / in-progress / code-review / done (+ archive when toggled).
- Each card shows: ID, title, priority pill, type, tags, agent badge (if assigned), parent epic indicator.
- Cards sorted by priority then numeric ID.
- WIP-limited columns surface a `(n/m)` badge with a red over-limit highlight.
- **Acceptance**: open GUI on a populated board, all tasks rendered in correct columns matching `tb ls --status active`.

### F2.4 — Task drawer (read-only) ☑
- Click a card → slide-over drawer on the right with all fields and rendered markdown body.
- Drawer closes with Esc or click outside.
- **Acceptance**: click any task, see its full content; matches `tb show <ID>`.

### F2.5 — Live updates via fsnotify ☑
- Watcher emits Wails events on file changes (`board:reloaded`, `task:updated:<id>`).
- Frontend store patches reactively.
- Ignores `BOARD.md`, `.next-id`, `.board.lock`, board-root `.agent-state/*` / `.agent-logs/*`, task-local `.agent-state.jsonl` / `.agent-logs/*`, and hidden task-local temp/staging paths.
- **Acceptance**: run `tb mv WS-1 ip` in terminal; GUI moves the card within 1s without manual refresh.

---

## M3 — Mutations + DnD + editing

### F3.1 — Drag-and-drop ☑
- `svelte-dnd-action`. Cards draggable between columns.
- Optimistic UI; backend call via `BoardService.MoveTask` (→ `tb mv …`).
- On error: revert + toast.
- **Acceptance**: drag a card backlog→in-progress; file moves on disk; another CLI `tb mv` racing the drag results in a revert + toast.

### F3.2 — Create task dialog ☑
- "+" button opens `CreateTaskDialog` modal with: title, module, type, priority, size, tags, description, optional parent, "is epic" toggle.
- Submit → `tb create "Title" -m … …`.
- **Acceptance**: new task appears in backlog within 1s.

### F3.3 — Edit metadata from drawer ☑
- Drawer fields (priority, type, size, module, tags) editable inline.
- Autosave: a debounced background save (TB-190) fires through `tb edit <ID> …`; the Details section shows a per-section status chip (`Unsaved` / `Saving…` / `Saved` / `Save failed`) instead of an explicit Save button. Edits coalesce inside the debounce window; if a save is already in flight the next debounce queues a single resave once the inner call resolves.
- A pending autosave is flushed (fire-and-forget) when the drawer is closed or the user switches tasks; failures surface via toast and inline status without dropping the user's draft.
- The CLI clear-field limitation is preserved: blanking a non-empty Module/Tags field is surfaced as an info toast and the field snaps back to the on-disk value rather than silently no-op'ing.
- **Acceptance**: change priority in GUI, file on disk has the new field and `BOARD.md` reflects it; no Save button is rendered next to Details; the autosave status chip reads `Saved` only after the watcher refresh reconciles disk and form values.

### F3.4 — Edit body (Goal / Acceptance / Context) ☑
- Markdown editor (CodeMirror) for the body section (everything below metadata block).
- Autosave (TB-190): body edits debounce and persist through `BoardService.EditTaskBody` (direct write under `.board.lock`, rules in `ARCHITECTURE.md`). The `Save body` button is gone — only `Edit` and `Discard` remain to enter or abandon edit mode. Cmd/Ctrl+S flushes the pending debounce instead of acting as the primary save.
- Pending body saves flush on close and task switch; the close confirm "you have unsaved body edits" prompt is removed (autosave + flush make it redundant).
- **Acceptance**: edit Goal text in the editor; without pressing any save button the file on disk is updated, header+metadata intact, log entry appended, `BOARD.md` regenerated.

### F3.5 — Filters ☑
- `FilterBar` with: type, priority, module, tags (multi), parent epic, agent.
- Toggle "Show archived" adds an Archive column.
- Filters apply client-side over the loaded snapshot.
- **Acceptance**: filter to `type=bug priority=P1`; only matching cards visible.

### F3.6 — Close task ☑
- Drawer has "Archive" button → `tb close <ID>`.
- Card leaves the active board (unless "Show archived" is on).
- **Acceptance**: archive a task; verify it appears in `archive/` dir.

---

## M4 — Manual agent runs

### F4.1 — Agent assignment UI ☑
- Drawer has a dropdown: Agent = `none | claude | codex`.
- Selecting sets the `**Agent:**` field via `tb edit`.
- Card gets an agent badge.
- **Acceptance**: assign claude; reload — agent shown; `tb show <ID>` has `**Agent:** claude`.

### F4.2 — Run button ☑
- Drawer has **Run agent** button (enabled when an agent is assigned).
- Click → `AgentService.RunAgent(id)` → adds to in-process daemon's queue with mode=`implement`.
- Run produces a `run_id`. JSONL events are written to the task's agent-state path; full logs are written to the corresponding `.agent-logs` path (board-root for file-form tasks, task-local for folder-form tasks).
- **Acceptance**: assign claude to a task, click Run; `ps aux | grep claude` shows the process; JSONL file accumulates events; log file accumulates output.

### F4.3 — Live run log in drawer ☑
- `AgentRunLog` panel inside drawer streams stdout lines as they arrive (via `agent:run-log` events).
- Shows status pill: queued / running / success / failed / cancelled / interrupted / lost.
- **Acceptance**: log lines appear in UI within ~1s of agent emitting them.

### F4.4 — Cancel run ☑
- Cancel button on a running task → cancels context → process killed (SIGTERM, then SIGKILL after 5s grace).
- Cancel writes BOTH:
  - JSONL `finished` event with `status: cancelled`
  - Task `**AgentStatus:** cancelled` via `tb edit <ID> --agent-status cancelled`
- The order is: kill, write JSONL, write metadata. The metadata write is the last step so `AgentStatus: cancelled` is durable before the daemon's main loop revisits the task.
- M5 stale-recovery treats `AgentStatus: cancelled` as terminal and never overwrites it.
- **Acceptance**: start a long-running agent, click Cancel; process exits within ~6s; `tb show <ID>` shows `**AgentStatus:** cancelled`; the task's agent-state JSONL last event is `finished` with `status: cancelled`; restart GUI — the cancel sticks (recovery does not touch it).

### F4.5 — Run history ☑
- Drawer shows list of past runs (parsed from JSONL) with timestamps + status.
- Click a past run to view its log file.
- **Acceptance**: run an agent 3 times; all 3 runs listed; each opens its log.

---

## M5 — Daemon auto-pickup

### F5.1 — Queued-task daemon ☑
- Daemon goroutine starts on app launch, stops on shutdown.
- Picks up any task with `**AgentStatus:** queued` via two paths: (a) startup scan after `SettingsService.OpenBoard` activates the daemon, and (b) the watcher event sink, which sees the atomic-rename `board:reloaded` event from `tb edit --agent-status queued`.
- Default worker pool: 1.
- **Acceptance**: `tb edit WS-2 -a claude --agent-status queued` in terminal; daemon picks it up within 5s.

### F5.2 — Stale-running recovery ☑
- On daemon activation, scans for tasks with `AgentStatus: running`.
- Checks last run in JSONL; if no `finished` event and PID is dead → write a recovery terminal event. Captured session id becomes `finished{status: interrupted, reason: "interrupted by daemon restart"}` and `AgentStatus: interrupted`; no captured session becomes `finished{status: lost, reason: "stale after restart"}` and `AgentStatus: lost`. The pid-liveness probe accepts npm-shebang scripts (`node` argv containing `/path/to/claude`) so a Node-wrapped agent is recognised as alive.
- **Carve-outs**:
  - If `AgentStatus: cancelled` is seen, OR the latest JSONL event for the latest run is `finished{status: cancelled}`, reconcile to `cancelled` and never overwrite as `failed`. JSONL intent outranks `.md` state during recovery.
  - If PID is still alive, leave the task alone — **M5 does not re-attach to live runs**. Re-attach (resume streaming output of a still-running agent) is intentionally deferred beyond M5. Conservative: avoids killing someone else's process and avoids surprising re-stream of stale output.
- **Acceptance 1**: start an agent, `kill -9` the GUI process, restart GUI; the stale task is marked `interrupted` if a session id was captured, otherwise `lost`; the task's agent-state JSONL has the recovery event.
- **Acceptance 2**: cancel a task via F4.4, then `kill -9` the GUI mid-cancel; restart; task remains `cancelled` (recovery does not turn it into `failed`).

### F5.3 — Concurrency control ☑
- Worker goroutines read a buffered channel of task IDs, and each daemon-queued or direct AgentService run must acquire a live `max_workers` concurrency gate before launching an agent. The setting defaults to 1, is persisted at `$XDG_CONFIG_HOME/tb-gui/preferences.json`, and is clamped to `[1, runtime.NumCPU()]` with a minimum ceiling of 1 on read.
- Configurable via settings (1 through the host CPU count) with runtime reload: lowering the value does not cancel already-running work, but new work waits for capacity; raising it wakes queued automation scans.
- Dedup: an in-memory active-set keyed by `task_id`, cross-checked with `AgentService.HasActiveRun`, prevents the startup scan, watcher sink, and manual UI run paths from spawning the same task twice.
- **Acceptance**: queue 3 tasks at once; they run sequentially (default config).

### F5.4 — Graceful shutdown ☑
- App close cancels the daemon's root context (propagated as the parent of the runner ctx — see TB-54 ctx plumbing). Workers' in-flight runs observe ctx cancellation, the shared `finishCancelled(reason: "shutdown")` helper writes the JSONL `finished{cancelled}` line + Wails emit + `tb edit --agent-status cancelled`. `Daemon.Close()` waits up to 5s for workers to flush; whatever remains is reconciled by next-start recovery.
- **Acceptance**: close GUI during a run; the task's agent-state JSONL has a coherent `finished` event (status either success/failed/cancelled, not orphaned).

### F5.5 — Agent session resume (TB-130) ☑
- Every run captures the agent CLI's `session_id` as a `session` JSONL event written immediately after `started` (PID is durable first). Claude pre-allocates the UUID and passes `--session-id <uuid>`; Codex emits its id mid-stream and the `codex exec --json` translator parses it via an `OnSessionID` callback.
- Stale-recovery's dead-PID branch now splits: SessionID captured → `interrupted`; no SessionID → `lost` (no Resume, because there is no captured session to continue). The cancelled carve-out still wins — a user-cancelled run with a SessionID stays `cancelled`, never `interrupted`. Resume surfaces whenever the backend reports a captured latest-run SessionID and the task is terminal, with source-status copy such as "Resume failed run".
- Resume re-invokes the agent CLI with its native flag (`claude -r <uuid>` / `codex exec --json resume <uuid> <prompt>`), in the parent run's persisted cwd, with the parent's `TB_`-prefixed env replayed. The new run's `queued` event carries `resumed_from` + `resumed_from_run` so the UI shows a `↻ r_xxxx` chip linking back to the parent.
- **Security**: only env keys prefixed `TB_` are persisted in JSONL `run_env`; credential vars never reach disk.
- **Acceptance**: 12 sub-tasks (TB-131..TB-142). Fake-runner integration tests in `gui/app/agent_run_test.go` (TestResumeCycle_KillRecoverResume, TestResumeCycle_KillBeforeSessionStaysLost) drive the full kill → interrupted → resume cycle without real Claude/Codex binaries.

---

## M6 — Groom flow

### F6.1 — Groom button ☑
- Drawer has **Groom** button next to Run.
- Click → `AgentService.GroomTask` → run with `mode=groom` and `groom.md` prompt.
- Prompt instructs the agent to refine Goal and Acceptance Criteria via `tb edit`, not to write code.
- Implementation: `gui/internal/agent/prompts/groom.md`, `agent.PromptGroom`, `GroomingDecorator`, and `gui/app/agent_service.go` `GroomTask`.
- **Acceptance**: backlog task with placeholder Goal; click Groom; after agent finishes, Goal section is improved on disk; GUI reflects it.

### F6.2 — Triage highlighting ☑
- Tasks flagged by `tb triage` (no module / placeholder goal / auto-created) get a "needs grooming" indicator on the card.
- Implementation: `BoardService.Triage()` consumes `tb triage --json`; `triageStore` feeds `Card.svelte` and `TaskDrawer.svelte`.
- **Acceptance**: backlog with such tasks shows the indicator; clicking it suggests Groom in the drawer.

---

## M7 — Polish (optional)

### F7.1 — Settings UI ☑
- Settings panel: agent timeout, max workers, default agent, CLI binary path.
- `preferences.json` persists all four knobs; timeout is read per run, CLI path reloads the active board client, and default agent is shown as a visual dropdown default for unassigned tasks.
- **Acceptance**: change timeout in UI; next run respects it.

### F7.2 — Keyboard shortcuts ☑
- `N` — new task. `/` — focus search/filter. `Esc` — close drawer. `Enter` — open selected card.
- **Acceptance**: all shortcuts work without modifier conflicts.

### F7.3 — System tray ☑
- Tray icon shows agent activity (idle / running).
- Click to show/hide window.
- **Acceptance**: minimize to tray, agent runs in background, click tray to return.

### F7.4 — Native application menu ☑
- File menu: Open board, Open Recent, Settings, Quit.
- View menu: Reload board.
- Help menu: About and docs entry.
- **Acceptance**: recent boards rebuild after board open; menu entries call the same service paths as the in-window controls.

---

## M8 — Folder-form tasks + attachments (TB-93)

- ☑ **TB-93** ([epic](../board/done/TB-93.md)) — umbrella milestone for folder-backed tasks, task-local artifacts, and user attachments while legacy file tasks keep working.
- **Contract**: folder paths, attachment layout, task-local agent files, locking, atomic writes, and file → folder promotion are defined by [TB-94](../board/done/TB-94.md) and [`docs/ARCHITECTURE.md` → "Folder-form tasks"](ARCHITECTURE.md#folder-form-tasks); this feature block tracks delivery status only.
- **Acceptance**: file/folder read parity; default folder creation; whole-folder moves/archive; attachment add/remove with validation; GUI picker + drag-and-drop workflow; watcher refresh after attachment operations and folder moves; mixed-board smoke covering CLI, GUI, agent artifacts, archive/restore, regeneration, and orphan checks.

### Children
- ☑ **TB-94** ([task](../board/done/TB-94.md)) — Define the folder-task on-disk contract before implementation work begins.
- ☑ **TB-95** ([task](../board/done/TB-95.md)) — Publish the TB-93 milestone tracker in `docs/FEATURES.md` and `docs/IMPLEMENTATION.md`.
- ☑ **TB-96** ([task](../board/done/TB-96.md)) — Make CLI read and JSON paths treat folder-form and file-form tasks as the same logical task.
- ☑ **TB-97** ([task](../board/done/TB-97.md)) — Make `tb create` default to folder-form tasks with an empty `## Attachments` section.
- ☑ **TB-98** ([task](../board/done/TB-98.md)) — Move, close/archive, and restore folder-form tasks as whole directories without orphaning artifacts.
- ☑ **TB-99** ([task](../board/done/TB-99.md)) — Add `tb attach <ID> <path>...` and atomically promote legacy file tasks on first attachment.
- ☑ **TB-100** ([task](../board/done/TB-100.md)) — Remove attachments through `tb attach --rm` with path validation and markdown updates.
- ☑ **TB-101** ([task](../board/done/TB-101.md)) — Keep `BOARD.md` byte-identical for equivalent file-form, folder-form, and mixed boards.
- ☑ **TB-102** ([task](../board/done/TB-102.md)) — Resolve agent state/log paths by storage form, including folder-task stale recovery.
- ☑ **TB-103** ([task](../board/done/TB-103.md)) — List, open, add, and remove drawer attachments through `tb` commands.
- ☑ **TB-104** ([task](../board/done/TB-104.md)) — Add drag-and-drop attachment workflows for cards and the task drawer through `tb`.
- ☑ **TB-105** ([task](../board/done/TB-105.md)) — Emit one logical GUI refresh for attachment operations and folder-task moves.
- ☑ **TB-106** ([task](../board/done/TB-106.md)) — Run the final mixed-board smoke and record evidence on TB-93.

---

## M9 — Code-review column (TB-194)

- ☑ **TB-194** ([epic](../board/done/TB-194/TASK.md)) — code-review board status, managed review-section commands, GUI column + drawer affordances, review-mode agents, `review-failed` rework loop, and the docs sweep that ties it together.
- **Status / tag invariants**: `code-review` is a board status (directory between in-progress and done). `review-failed` is a tag on ready tasks (was backlog before M10 — see F1.7), not an `AgentStatus`. `needs-user` (TB-182) is unrelated — it parks autonomous runs that need human input.
- **CLI surface**: `tb review --submit | --target | --notes | --findings | --pass | --fail`. `--submit` accepts in-progress work and ready `review-failed` rework, clearing the marker on move; backlog `review-failed` submit remains accepted only for legacy boards that already carry the old marker. `--pass` writes no-blocking findings and moves task to done; `--fail` writes findings, moves task to ready, adds `review-failed`, regenerates `BOARD.md` — all atomically under `.board.lock`.
- **GUI surface**: BoardSnapshot gains a `codeReview` bucket; Board renders Backlog / Ready / In Progress / Code Review / Done by default. Cards show a `↩` marker on ready tasks tagged `review-failed` (also rendered on legacy backlog tasks still carrying the tag). TaskDrawer exposes "Submit for review" (in-progress) and "Review" (code-review, runs a review-mode agent).
- **Agent surface**: `ModeReview` + `ReviewDecorator` + embedded `prompts/review.md`. Review-mode runs inherit the implement/groom JSONL lifecycle, daemon pickup, cancellation, and recovery, and label their JSONL events `mode=review`. Reviewers do NOT edit implementation files; they write findings through `tb review --pass` for clean reviews or `tb review --fail` for blocking findings.
- **Acceptance**: managed review surface (CLI + Wails) operates atomically; GUI column visible by default; review-mode runs end-to-end (queue → run → findings on disk); failed-review marker visible in cards/drawer; resubmit clears the marker; docs (`board/CONVENTIONS.md`, `board/SKILL.md`, `CLAUDE.md`, `docs/ARCHITECTURE.md`, `cli/templates.go`) describe the workflow.

### Children
- ☑ **TB-195** — CLI: add code-review status and submit flow.
- ☑ **TB-196** — CLI: review target / reviewer notes / findings commands.
- ☑ **TB-197** — GUI: code-review column, card marker, drawer Submit/Review actions.
- ☑ **TB-198** — Agent: review mode + findings prompt.
- ☑ **TB-199** — Workflow: `review-failed` marker + retry priority.
- ☑ **TB-200** — Docs: board conventions / skill / architecture / CLAUDE updates.

---

## M11 — Staged autonomous workflow (TB-172 / TB-177 / TB-262)

- ☑ **Contract:** autonomous operation is three independently toggleable stages, not one global automation switch. `auto-groom` owns backlog grooming into `ready`; `auto-implement` owns committed ready work into `in-progress` and then `code-review`; `auto-review` owns review-mode pass/fail from `code-review`.
- ☑ **Auto-groom (TB-172):** Settings exposes `auto_groom_enabled` and `auto_groom_settle_minutes`, with a board-header quick toggle backed by the same preference. When enabled and a valid `default_agent` exists, backlog tasks reported by triage after the settle window are queued as `mode=groom` within the shared `max_workers` budget; successful clean grooming promotes through the managed ready gate. Disabled, no-default, and worker-capacity-full paths leave tasks untouched and keep manual Groom available.
- ☑ **Auto-implement (TB-177):** Settings exposes `auto_implement_enabled` and a saved query, mirrored by a compact board-header control. When enabled with a valid default agent and query, matching `ready` tasks are eligible when their generic `AgentStatus` is blank or otherwise nonblocking for a new implement run. The stage uses assigned-agent/default-agent fallback, reserves shared `max_workers` capacity, moves through the canonical pull path into `in-progress`, and the implementation agent submits with `ReviewRef` to `code-review`. Backlog tasks are never auto-implemented.
- ☑ **Auto-implement dependency gate (TB-267):** child tasks in the same epic are considered in numeric task-ID order. A later child is skipped while any lower-ID same-parent child is not closed/done, and diagnostics identify the blocker instead of silently guessing.
- ☑ **Review-failed retry state (TB-268):** a failed review returns to `ready` with `review-failed` and clears retry-blocking generic `AgentStatus` so auto-implement can pick up eligible rework. Per-mode review attribution and JSONL history remain the audit trail.
- ☑ **Auto-review (TB-262):** Settings exposes `auto_review_enabled`, defaulted off and validated against `default_agent`, with a compact board-header control. When enabled, eligible `code-review` tasks with concrete `ReviewRef` are queued as `mode=review` with `initiator=auto-review`; missing `ReviewRef` writes a `needs-user` handoff. Pass moves to `done` through `tb review --pass`; fail uses `tb review --fail` back to `ready` with `review-failed`. Dedupe/recovery use objective JSONL queued events for `initiator=auto-review`; no pass/fail is inferred from prose.
- ☑ **Daemon reconciliation (TB-266):** housekeeping repairs only objective missed transitions, uses managed CLI operations, never guesses from prose/logs, never overrides `needs-user`/`cancelled` or unrelated `interrupted`/`lost`, and records durable skip/backoff for WIP-blocked or partially applied repairs.
- ☐ **Prompt alignment (TB-270):** `groom.md` and `implement.md` must use the same ownership boundaries as this staged contract. Remaining prompt drift is tracked there rather than treated as daemon behavior.

**Acceptance**: a user can enable each stage independently; disabling one stage does not disable or imply another. The documented kanban transitions are `backlog -> ready` for auto-groom, `ready -> in-progress -> code-review` for auto-implement, and `code-review -> done|ready` for auto-review. Failed reviews return to `ready` with `review-failed` and a cleared generic scheduling cursor, and auto-implement must not pick a later epic child while an earlier same-parent child is unfinished.

---

## Explicit non-goals

- Multi-user / collaboration / comments
- Web UI, mobile UI
- Database backend (Markdown is the source of truth)
- Built-in prompt editor in the UI (edit `gui/internal/agent/prompts/*.md` directly)
- Multiple LLM providers beyond `claude` and `codex` CLIs
- Native Windows build (use WSL2)
- Multi-board kanban view in one window
- Notifications, email, Slack integration
