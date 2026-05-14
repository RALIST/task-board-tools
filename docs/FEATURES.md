# Features

Features grouped by milestone. Each has acceptance criteria — implementation isn't done until those pass.

Status notation: ☐ planned · ⬚ partial · ☑ done.

---

## M0 — Documentation foundation

- ☐ **F0.1** `docs/PROJECT.md` describing product, users, scenarios, glossary
- ☐ **F0.2** `docs/ARCHITECTURE.md` with components, on-disk format, locking rules, agent state
- ☐ **F0.3** `docs/FEATURES.md` (this file)
- ☐ **F0.4** `docs/IMPLEMENTATION.md` — living tracker of milestones, risks
- ☐ **F0.5** Root `README.md` updated for two-binary layout

**Acceptance**: a developer who has never seen the project can read the 4 docs and understand goals, scope, and how to build/run.

---

## M1 — CLI extensions

### F1.1 — Repo restructure ☐
- `tb/` → `cli/`. Existing tests pass.
- Root `go.work` lets `go build ./cli` work from repo root.
- **Acceptance**: `cd cli && go build -o tb . && ./tb ls --status all` works on an existing board untouched.

### F1.2 — Agent metadata fields ☐
- New optional fields `**Agent:**` and `**AgentStatus:**` in task `.md`.
- Parsed by `parseTaskFile`. Empty if absent.
- Settable via `tb edit -a <agent> --agent-status <status>`.
- **Acceptance**: `tb edit WS-1 -a claude --agent-status queued && tb show WS-1 | grep Agent` shows both fields.

### F1.3 — JSON output ☐
- `tb ls --json` → JSON array of task objects (camelCase keys).
- `tb show <ID> --json` and `tb show --json <ID>` both work; output is `{metadata, body}`.
- `tb board --json` → object with `epics`, `activeEpics`, `finishedEpics`, `inProgress`, `backlog`, `recentlyDone`.
- Empty result → `[]` or empty object, never prose like "No tasks found.".
- **Acceptance**: `tb ls --json | jq .` parses without errors for both empty and populated boards. All Task fields present in output.

### F1.4 — Status semantics ☐
- `--status active` = backlog + in-progress + done.
- `--status archive` = archive directory only.
- `--status all` = active + archive (everything on disk).
- Default for `tb ls` remains `backlog` (backward-compatible).
- **Acceptance**: `tb ls --status archive --json` returns only archived tasks. `tb ls --status all` returns archive entries too.

### F1.5 — Regenerate consistency ☐
- `tb create` and `tb edit` call `regenerateBoard` at the end of the operation.
- Documented behavior: `BOARD.md` is always up-to-date after any mutation.
- **Acceptance**: `tb edit WS-1 -p P0 && grep "P0" board/BOARD.md` — change is visible without a manual `tb regenerate`.

### F1.6 — Atomic task-file writes ☐
- Every task-file mutation writes via temp file + `os.Rename` (the pattern `regenerate.go` already uses).
- Sites to convert: `create.go:128` (new task), `create.go:229/252/257/267/273` (`addTagToTaskFile`, `addChildToSubtasks`), `edit.go:116`, `move.go:135` (move destination), `board.go:335` (`archiveTask`), `scan.go:122` (scan-created tasks).
- Single shared helper `writeFileAtomic(path string, data []byte, perm os.FileMode) error` in `cli/board.go` (or a new `cli/atomicfs.go`).
- **Why**: M2+ GUI readers do not hold the board lock; they rely on this invariant for safe snapshots (see `docs/ARCHITECTURE.md` → "Locking and atomic writes").
- **Acceptance**: `grep -nE 'os\.WriteFile\(.*\.md' cli/*.go` returns no hits outside `cli/atomicfs.go` (or wherever the helper lives). Concurrent `tb edit` in a tight loop while a Go reader parses every event produces zero parses with empty header. (Manual test: run the loop for 30s; expect zero "missing header" log entries from a tiny `parseTaskFile` harness.)

---

## M2 — Read-only GUI

### F2.1 — Wails3 + Svelte scaffold ☐
- `gui/` initialized via `wails3 init -t sveltekit-ts`.
- Builds and runs (`wails3 dev`).
- Single-instance lock enabled.
- **Acceptance**: `wails3 dev` opens a window. Starting a second instance focuses the first.

### F2.2 — Board picker / context ☐
- On first start: native folder picker → pick project root (containing `.tb.yaml`).
- Recent boards saved to `~/.config/tb-gui/recent.json`.
- Menu item "Open Board…" switches active board.
- **Acceptance**: pick a board, see its tasks; close and reopen — same board loads.

### F2.3 — Three-column kanban (read-only) ☐
- Columns: backlog / in-progress / done.
- Each card shows: ID, title, priority pill, type, tags, agent badge (if assigned), parent epic indicator.
- Cards sorted by priority then numeric ID.
- **Acceptance**: open GUI on a populated board, all tasks rendered in correct columns matching `tb ls --status active`.

### F2.4 — Task drawer (read-only) ☐
- Click a card → slide-over drawer on the right with all fields and rendered markdown body.
- Drawer closes with Esc or click outside.
- **Acceptance**: click any task, see its full content; matches `tb show <ID>`.

### F2.5 — Live updates via fsnotify ☐
- Watcher emits Wails events on file changes (`board:reloaded`, `task:updated:<id>`).
- Frontend store patches reactively.
- Ignores `BOARD.md`, `.next-id`, `.board.lock`, `.agent-state/*`, `.agent-logs/*`.
- **Acceptance**: run `tb mv WS-1 ip` in terminal; GUI moves the card within 1s without manual refresh.

---

## M3 — Mutations + DnD + editing

### F3.1 — Drag-and-drop ☐
- `svelte-dnd-action`. Cards draggable between columns.
- Optimistic UI; backend call via `BoardService.MoveTask` (→ `tb mv …`).
- On error: revert + toast.
- **Acceptance**: drag a card backlog→in-progress; file moves on disk; another CLI `tb mv` racing the drag results in a revert + toast.

### F3.2 — Create task dialog ☐
- "+" button opens `CreateTaskDialog` modal with: title, module, type, priority, size, tags, description, optional parent, "is epic" toggle.
- Submit → `tb create "Title" -m … …`.
- **Acceptance**: new task appears in backlog within 1s.

### F3.3 — Edit metadata from drawer ☐
- Drawer fields (priority, type, size, module, tags) editable inline.
- Save button → `tb edit <ID> …`.
- **Acceptance**: change priority in GUI, file on disk has the new field and `BOARD.md` reflects it.

### F3.4 — Edit body (Goal / Acceptance / Context) ☐
- Markdown editor (CodeMirror) for the body section (everything below metadata block).
- Save → `BoardService.EditTaskBody` (direct write under `.board.lock`, rules in `ARCHITECTURE.md`).
- **Acceptance**: edit Goal text, save; file on disk updated, header+metadata intact, log entry appended, `BOARD.md` regenerated.

### F3.5 — Filters ☐
- `FilterBar` with: type, priority, module, tags (multi), parent epic, agent.
- Toggle "Show archived" adds an Archive column.
- Filters apply client-side over the loaded snapshot.
- **Acceptance**: filter to `type=bug priority=P1`; only matching cards visible.

### F3.6 — Close task ☐
- Drawer has "Archive" button → `tb close <ID>`.
- Card leaves the active board (unless "Show archived" is on).
- **Acceptance**: archive a task; verify it appears in `archive/` dir.

---

## M4 — Manual agent runs

### F4.1 — Agent assignment UI ☐
- Drawer has a dropdown: Agent = `none | claude | codex`.
- Selecting sets the `**Agent:**` field via `tb edit`.
- Card gets an agent badge.
- **Acceptance**: assign claude; reload — agent shown; `tb show <ID>` has `**Agent:** claude`.

### F4.2 — Run button ☐
- Drawer has **Run agent** button (enabled when an agent is assigned).
- Click → `AgentService.RunAgent(id)` → adds to in-process daemon's queue with mode=`implement`.
- Run produces a `run_id`. JSONL events written; full log piped to `.agent-logs/<id>/<run_id>.log`.
- **Acceptance**: assign claude to a task, click Run; `ps aux | grep claude` shows the process; JSONL file accumulates events; log file accumulates output.

### F4.3 — Live run log in drawer ☐
- `AgentRunLog` panel inside drawer streams stdout lines as they arrive (via `agent:run-log` events).
- Shows status pill: queued / running / success / failed.
- **Acceptance**: log lines appear in UI within ~1s of agent emitting them.

### F4.4 — Cancel run ☐
- Cancel button on a running task → cancels context → process killed (SIGTERM, then SIGKILL after 5s grace).
- Cancel writes BOTH:
  - JSONL `finished` event with `status: cancelled`
  - Task `**AgentStatus:** cancelled` via `tb edit <ID> --agent-status cancelled`
- The order is: kill, write JSONL, write metadata. The metadata write is the last step so `AgentStatus: cancelled` is durable before the daemon's main loop revisits the task.
- M5 stale-recovery treats `AgentStatus: cancelled` as terminal and never overwrites it.
- **Acceptance**: start a long-running agent, click Cancel; process exits within ~6s; `tb show <ID>` shows `**AgentStatus:** cancelled`; `.agent-state/<ID>.jsonl` last event is `finished` with `status: cancelled`; restart GUI — the cancel sticks (recovery does not touch it).

### F4.5 — Run history ☐
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
- Checks last run in JSONL; if no `finished` event and PID is dead → write synthetic `finished{status: failed, reason: "stale after restart"}` event, set `AgentStatus: failed`. The pid-liveness probe accepts npm-shebang scripts (`node` argv containing `/path/to/claude`) so a Node-wrapped agent is recognised as alive.
- **Carve-outs**:
  - If `AgentStatus: cancelled` is seen, OR the latest JSONL event for the latest run is `finished{status: cancelled}`, reconcile to `cancelled` and never overwrite as `failed`. JSONL intent outranks `.md` state during recovery.
  - If PID is still alive, leave the task alone — **M5 does not re-attach to live runs**. Re-attach (resume streaming output of a still-running agent) is intentionally deferred beyond M5. Conservative: avoids killing someone else's process and avoids surprising re-stream of stale output.
- **Acceptance 1**: start an agent, `kill -9` the GUI process, restart GUI; the stale task is marked failed; `.agent-state/<id>.jsonl` has the recovery event.
- **Acceptance 2**: cancel a task via F4.4, then `kill -9` the GUI mid-cancel; restart; task remains `cancelled` (recovery does not turn it into `failed`).

### F5.3 — Concurrency control ☑
- N worker goroutines read a buffered channel of task IDs (N = `max_workers`, default 1, persisted at `$XDG_CONFIG_HOME/tb-gui/preferences.json`, clamped to `[1, 4]` on read).
- Configurable via settings (1–4); no hot-reload — the value is read at daemon construction.
- Dedup: an in-memory active-set keyed by `task_id`, cross-checked with `AgentService.HasActiveRun`, prevents the startup scan, watcher sink, and manual UI run paths from spawning the same task twice.
- **Acceptance**: queue 3 tasks at once; they run sequentially (default config).

### F5.4 — Graceful shutdown ☑
- App close cancels the daemon's root context (propagated as the parent of the runner ctx — see TB-54 ctx plumbing). Workers' in-flight runs observe ctx cancellation, the shared `finishCancelled(reason: "shutdown")` helper writes the JSONL `finished{cancelled}` line + Wails emit + `tb edit --agent-status cancelled`. `Daemon.Close()` waits up to 5s for workers to flush; whatever remains is reconciled by next-start recovery.
- **Acceptance**: close GUI during a run; `.agent-state` file has a coherent `finished` event (status either success/failed/cancelled, not orphaned).

---

## M6 — Groom flow

### F6.1 — Groom button ☐
- Drawer has **Groom** button next to Run.
- Click → `AgentService.GroomTask` → run with `mode=groom` and `groom.md` prompt.
- Prompt instructs the agent to refine Goal and Acceptance Criteria via `tb edit`, not to write code.
- **Acceptance**: backlog task with placeholder Goal; click Groom; after agent finishes, Goal section is improved on disk; GUI reflects it.

### F6.2 — Triage highlighting ☐
- Tasks flagged by `tb triage` (no module / placeholder goal / auto-created) get a "needs grooming" indicator on the card.
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

## Explicit non-goals

- Multi-user / collaboration / comments
- Web UI, mobile UI
- Database backend (Markdown is the source of truth)
- Built-in prompt editor in the UI (edit `gui/internal/agent/prompts/*.md` directly)
- Multiple LLM providers beyond `claude` and `codex` CLIs
- Native Windows build (use WSL2)
- Multi-board kanban view in one window
- Notifications, email, Slack integration
