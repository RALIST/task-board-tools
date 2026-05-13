# TB-5: M5: Agent daemon with autopickup and crash recovery

**Type:** feature
**Priority:** P1
**Size:** XL
**Module:** gui
**Tags:** milestone-m5,agent,daemon,epic
**Branch:** —

## Goal

Embed a background daemon in the GUI process that auto-picks up tasks with Agent set and AgentStatus=queued. Worker pool with semaphore, dedup by task_id, stale-running recovery on startup via PID liveness + JSONL replay, graceful shutdown.

## Context

Embed a background daemon in the GUI process that auto-picks up any task with `Agent` set and `AgentStatus=queued`. Worker pool with semaphore (default 1, configurable). Dedup by task_id (in-memory active set). On startup, scan tasks with `AgentStatus=running`: read last run from `board/.agent-state/PREFIX-NNN.jsonl`; if there is no `finished` event and the PID from the `started` event is dead, mark the run as failed (and the task `AgentStatus` as failed). `AgentStatus=cancelled` is user-initiated and stale-recovery must never overwrite it. Graceful shutdown via `context.Cancel` + 5s grace then kill. See plan M5 and `docs/ARCHITECTURE.md` → "Daemon".

## Subtasks

- **TB-53** (S) — Daemon skeleton + Wails OnStartup/OnShutdown lifecycle wiring
- **TB-54** (M) — Worker pool with bounded semaphore (default 1) feeding AgentService.RunAgent
- **TB-55** (S) — Daemon active-set dedup keyed by task_id
- **TB-56** (S) — Settings field max_workers (1-4) wired to daemon semaphore
- **TB-57** (S) — Daemon initial queue scan on startup (AgentStatus=queued → enqueue)
- **TB-58** (M) — Watcher subscription: enqueue on task:updated:<id> newly-queued
- **TB-59** (S) — pidAlive(pid, name) probe with command-name cross-check (R10 mitigation)
- **TB-60** (M) — Stale-running recovery: scan AgentStatus=running, JSONL replay, synthetic finished+failed
- **TB-61** (S) — Stale-recovery cancelled carve-out: never overwrite AgentStatus=cancelled
- **TB-62** (M) — Graceful shutdown: ctx cancel + 5s grace + JSONL flush

## Subtask → Feature ownership matrix

| F5 / invariant                              | Owner(s)                              |
|---------------------------------------------|---------------------------------------|
| F5.1 daemon start/stop lifecycle            | TB-53 (skeleton), TB-62 (shutdown)    |
| F5.1 startup queue pickup                   | TB-57 (scan after recovery + sink)    |
| F5.1 live queued pickup                     | TB-58 (emitter fan-out, board:reloaded + task:updated paths) |
| F5.2 PID check                              | TB-59 (`pidAlive` w/ node-wrapper)    |
| F5.2 stale recovery                         | TB-60 (replay + synthetic finished)   |
| F5.2 cancelled carve-out                    | TB-61 (JSONL intent over `.md`)       |
| F5.2 live-PID re-attach                     | **Out of M5 scope** (deferred)        |
| F5.3 worker pool + ctx plumb                | TB-54 (internal blocking executor)    |
| F5.3 `max_workers` setting                  | TB-56 (`preferences.json`)            |
| F5.3 dedup                                  | TB-55 (active-set + `HasActiveRun`)   |
| F5.4 graceful shutdown                      | TB-62 (ctx cancel + 5s grace + helper)|
| Structural: split public `RunAgent` from internal executor | TB-54 |
| Structural: narrow `AgentService.mu`        | TB-54                                 |
| Structural: `agent` field on `started` JSONL| TB-54 (schema change), TB-60 (consumer)|

## Acceptance Criteria

- [ ] **F5.1** Daemon goroutine started in `main` and *activated* via `SettingsService.OpenBoard` hook (TB-53). Tasks with `AgentStatus=queued` and an `Agent` are picked up via (a) startup scan (TB-57) for the at-open-board case and (b) watcher emitter sink (TB-58) for live edits — including atomic CLI renames that route through `board:reloaded`, not `task:updated:<id>`. End-to-end smoke: `tb edit X -a claude --agent-status queued` from a terminal triggers a `running` transition within 5s.
- [ ] **F5.2** Stale-running recovery: tasks with `AgentStatus=running` whose latest JSONL run has no `finished` event AND whose `started.pid` is dead (verified by `pidAlive(pid, expectedAgent)` TB-59, with node-wrapper fallback for npm-installed `claude`/`codex`) get a synthetic `finished{status: failed, reason: "stale after restart"}` appended and `AgentStatus` flipped to `failed` via `tb edit`. If `pidAlive==true`, the daemon leaves the task alone. **No re-attach in MVP** — F5.2's "re-attach" wording in `docs/FEATURES.md:179` is updated to match.
- [ ] **F5.2 carve-out** A task whose latest JSONL event for the latest `run_id` is `finished{status: cancelled}` is reconciled to `AgentStatus=cancelled`, never `failed`. Defends the M4 5-step cancel ordering against a kill-9-mid-cancel race.
- [ ] **F5.3** Worker pool: N workers reading a buffered task-ID channel, N = `MaxWorkers` (default 1, persisted at `preferences.json`, clamped to `[1, 4]`). In-memory active-set keyed by `task_id` plus `AgentService.HasActiveRun` cross-check prevents duplicate enqueue from racing `(startup scan, watcher event, manual UI run)`. Queue 3 tasks at once with default config → strictly sequential (asserted on JSONL timestamps).
- [ ] **F5.4** Graceful shutdown: `Daemon.Close()` cancels the root ctx (which is now the parent of the runner ctx — see TB-54 ctx-plumbing prereq), waits up to 5s for workers to run the shared `finishCancelled(reason: "shutdown")` helper, then returns. Anything left behind is reconciled by next-start recovery.
- [ ] **Structural** `AgentService.RunAgent` keeps its M4 contract for the drawer Run button (write queued + return run_id); daemon workers call a new internal `RunQueuedAgentSync(ctx, id)` that accepts queued state, plumbs ctx into the runner, and blocks until terminal. JSONL append / Wails emit / `tb edit` move outside `s.mu`; the mutex guards only `active` map insert/delete.
- [ ] Single-instance lock (inherited from M2 / TB-2) keeps exactly one daemon per user — the second GUI invocation focuses the existing window without starting a second daemon.
- [ ] `pid` and `run_id` are present in every `started` JSONL event (already done in M4 / TB-47); TB-54 also lands `agent` on `started` so TB-60's `pidAlive` source field is unambiguous.
- [ ] All M5 sub-tasks (TB-53..TB-62) closed.
- [ ] `docs/IMPLEMENTATION.md` M5 markers flipped to ☑; `docs/FEATURES.md` F5.2 wording updated (no re-attach); `docs/ARCHITECTURE.md` "Cancel carve-out" and "Daemon" sections updated to reference the M5 implementation (cf. TB-61 doc edits).

## Related Tasks

- **TB-4** — Prerequisite (Runner interface, JSONL writer with `pid`+`run_id` on `started`, AgentService.RunAgent/CancelRun, M4 5-step cancel ordering this defends)
- **TB-2** — Prerequisite (single-instance lock — keeps the daemon unique per user)
- **TB-20** — Prerequisite (fsnotify watcher TB-58 subscribes to)
- **TB-11** — Prerequisite (`Agent`/`AgentStatus` fields + `cancelled` enum value)
- **TB-6** — Builds on this (M6 groom flow rides the same daemon pickup path when a groom run is queued)
- **TB-7** — Builds on this (M7 settings UI writes the `max_workers` field this introduces)

## Log

- 2026-05-13: Created
- 2026-05-13: Moved to in-progress
- 2026-05-13: Moved to backlog
- 2026-05-13: Groomed — aligned acceptance criteria 1:1 with `docs/FEATURES.md` F5.1–F5.4 and `docs/IMPLEMENTATION.md` M5 task list; decomposed into TB-53..TB-62 (daemon skeleton + Wails lifecycle; worker pool feeding a sync executor; in-memory active-set dedup; persisted `max_workers` 1–4; startup queue scan; watcher subscription for live queued-trigger; `pidAlive` with command-name cross-check; stale-running recovery; cancelled carve-out; graceful shutdown).
- 2026-05-13: Codex adversarial review applied across all 10 children + epic. Critical fixes: (1) TB-54 — verified `RunAgent` rejects queued/running (`agent_run.go:98-100`), uses `context.Background()` (`agent_run.go:148`), and returns async; daemon needs an internal blocking executor that accepts queued and plumbs caller ctx into the runner. Also narrows `s.mu` and adds `agent` to `started` JSONL event. (2) TB-58 — verified atomic CLI rename routes through `board:reloaded`, not `task:updated:<id>` (`watcher.go:201-211`), so the original "skip board:reloaded" AC would have made the daemon miss every CLI edit. Replaced new `Subscribe()` API with an emitter fan-out using the existing `watcher.Emitter` seam. (3) TB-57 — strict startup order: recovery → register watcher sink → scan (closes the race where an edit lands between scan-read and subscription-attached). (4) TB-53 — split lifecycle into `New`/`Activate(boardDir)`/`Deactivate`/`Close`; activation gated on `SettingsService.OpenBoard` since BoardService client + watcher boardDir aren't set until then. Concrete fields replaced with narrow interfaces. (5) TB-60 — `expectedAgent` comes from the `queued` event (which has `agent`), not `started` (`pid` only). Test split into Go in-process fixture vs manual multi-process harness. (6) TB-59 — `claude`/`codex` are npm shebang scripts so `ps -o comm=` returns `node`; added `args`-fallback that accepts a node-wrapper invocation. "Prefix match" replaced with "exact basename match". (7) TB-62 — daemon shutdown depends on TB-54's ctx plumbing; without it, `Daemon.Close()` cannot reach in-flight runs. Cancel-finish work factored into a shared `AgentService` helper with a `reason` parameter. Major fixes: TB-55 needs a public `HasActiveRun` accessor (the field is private); TB-56's `recent.json` extension dropped in favor of a separate `preferences.json`; TB-61 test scenarios split into Go fixture vs manual harness; F5.2 "re-attach to live PID" removed (no owner — deferred). Added explicit owner matrix to this epic and docs-drift AC for `docs/FEATURES.md` F5.2 wording + `docs/ARCHITECTURE.md` Daemon/Cancel-carve-out sections.
