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
- Cancel button on a running task → cancels context → process killed.
- JSONL gets a `finished` event with `status: cancelled`.
- **Acceptance**: start a long-running agent, click Cancel; process exits; status shows `cancelled`.

### F4.5 — Run history ☐
- Drawer shows list of past runs (parsed from JSONL) with timestamps + status.
- Click a past run to view its log file.
- **Acceptance**: run an agent 3 times; all 3 runs listed; each opens its log.

---

## M5 — Daemon auto-pickup

### F5.1 — Queued-task daemon ☐
- Daemon goroutine starts on app launch, stops on shutdown.
- Picks up any task with `**AgentStatus:** queued`.
- Default worker pool: 1.
- **Acceptance**: `tb edit WS-2 -a claude --agent-status queued` in terminal; daemon picks it up within 5s.

### F5.2 — Stale-running recovery ☐
- On daemon start, scans for tasks with `AgentStatus: running`.
- Checks last run in JSONL; if no `finished` event and PID is dead → write synthetic `finished{status: failed, reason: "stale after restart"}` event, set `AgentStatus: failed`.
- **Acceptance**: start an agent, `kill -9` the GUI process, restart GUI; the stale task is marked failed; `.agent-state/<id>.jsonl` has the recovery event.

### F5.3 — Concurrency control ☐
- Worker pool semaphore default = 1.
- Configurable via settings (1–4).
- Dedup: a task already running cannot be re-enqueued.
- **Acceptance**: queue 3 tasks at once; they run sequentially (default config).

### F5.4 — Graceful shutdown ☐
- App close cancels context, waits 5s for runners to flush, then exits.
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

### F7.1 — Settings UI ☐
- Settings panel: agent timeout, max workers, default agent, CLI binary path.
- **Acceptance**: change timeout in UI; next run respects it.

### F7.2 — Keyboard shortcuts ☐
- `N` — new task. `/` — focus search/filter. `Esc` — close drawer. `Enter` — open selected card.
- **Acceptance**: all shortcuts work without modifier conflicts.

### F7.3 — System tray ☐
- Tray icon shows agent activity (idle / running).
- Click to show/hide window.
- **Acceptance**: minimize to tray, agent runs in background, click tray to return.

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
