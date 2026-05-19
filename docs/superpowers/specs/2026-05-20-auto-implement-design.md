# Auto-Implement — Design Spec (TB-177 epic)

**Date:** 2026-05-20
**Author:** Claude Opus 4.7 + Aleksander Danilov
**Epic:** TB-177
**Sibling reference:** TB-172 (auto-groom) — shipped commits `f262ec8` + `1335504`

## Goal

Opt-in feature: when enabled, the GUI daemon selects committed `ready` tasks
matching a saved board-filter query, moves them through the canonical
`tb pull` path into `in-progress`, and starts implementation-mode agent runs
using the task's assigned agent or the configured default agent.

Auto-implement is a *commitment automation* — it runs only on tasks the user
has already groomed and explicitly committed via `tb ready`. It deliberately
does **not** add a settle window (the act of running `tb ready` is itself the
"done editing" signal). This is the policy break from auto-groom, which
operates on backlog and therefore needs a settle window.

## Children and order

| Child  | Title                                            | Size | Notes |
|--------|--------------------------------------------------|------|-------|
| TB-268 | Review-failed handoff clears retry-blocking AgentStatus | M | cli/ — ship first; independent |
| TB-178 | Preferences + shared query parser                | M | gui/app + frontend store |
| TB-267 | Epic child order helper                          | M | gui/internal/automation/epicorder |
| TB-179 | Daemon enqueues candidates (+ TB-233 sort)       | M+S | gui/app/auto_implement.go |
| TB-180 | UI controls + feedback                           | S | SettingsPanel + header pill + drawer row |

Shipped as five sequential commits on `main` per user direction, each
preceded by a `fullstack-code-reviewer` (and codex adversarial review where
helpful) and verified with the cli/gui Go test suites plus the frontend
suite.

## Architecture

```
gui/app/preferences.go       +AutoImplementEnabled +AutoImplementQuery
gui/app/settings_service.go  +SetAutoImplementEnabled / +SetAutoImplementQuery
gui/app/auto_implement.go    new AutoImplementCoordinator (peer to AutoGroomCoordinator)
gui/internal/automation/
  query/                     shared parser + matcher (type/priority/size/module/tag/agent/parent/text)
  epicorder/                 pure EligibleForEpicOrder(task, siblings)
gui/main.go                  register coordinator + Wails service
gui/adapters.go              extend composite boardActivator
gui/frontend/src/lib/stores/preferences.ts  +auto-implement fields
gui/frontend/src/lib/stores/autoImplement.ts  status store + event handlers
gui/frontend/src/lib/components/SettingsPanel.svelte  +toggle + query input + validation
gui/frontend/src/routes/+page.svelte  +header pill
gui/frontend/src/lib/components/TaskDrawer.svelte  +info row when enabled
```

### Coordinator state machine

```
Disabled  ── enable + valid prereqs ──►  WaitingForScan
WaitingForScan  ── watcher/board event ──►  Scanning
Scanning
  for each ready task with blank AgentStatus:
    queryMatch?         no → skip
    triage clean?       no → skip (defence-in-depth; ready usually is)
    epic-order OK?      no → skip + record blocker id
    WIP allows move?    no → recordSkip(wip)
    sort by priority desc, then review-failed first within bucket, then id asc
    take top N candidates (≤ daemon worker capacity − active autos)
    for each: tb pull → AgentService.StartImplementWithSelectionHash
WaitingForScan
```

### Dedupe

The coordinator computes a stable `SelectionHash` per task derived from
`(id, status, agent, agentStatus, query)`. Selection hash is written into
the `queued` and `finished` JSONL events for the implement run (mirrors the
auto-groom triage-hash dedupe). On scan start the coordinator skips a task
whose last *successful* implement run has the same hash AND whose body has
not changed since (mtime check on task file).

This protects against tight retry loops on tasks the agent already
implemented but that re-enter `ready` (e.g. because a human moved them) —
the user has to either edit the task or change the saved query to re-arm
auto-implement.

### Cross-feature contracts

- **TB-268 contract:** `tb review --fail` and the review-mode terminal
  recorder must leave `AgentStatus` blank when the resulting status is
  `ready`. Mode-specific fields (`ReviewedBy`, `ReviewStatus`) are
  preserved.
- **TB-267 contract:** `EligibleForEpicOrder` blocks a child when any
  same-parent sibling with a lower numeric ID is not in `done`.
  `AgentStatus ∈ {needs-user, interrupted, cancelled}` on an earlier
  sibling also blocks. Tasks tagged `epic` are always skipped as
  candidates.
- **TB-233 sort:** within the same priority bucket and the same
  query-eligible pool, tasks tagged `review-failed` rank ahead of tasks
  without the tag. The eligibility gates from TB-179 and TB-267 run *before*
  this sort.

## Test plan (per commit)

Each commit lands with at least the test surface called out in its task's
acceptance criteria. Aggregate:

- `cd cli && go test ./...`
- `cd gui && go test ./...`
- `cd gui/frontend && npm run check`
- `cd gui/frontend && npm test -- --run`
- `make lint-go`

A short manual exercise after TB-180 covers Settings enable/disable,
default-agent guard, query `bug, S size, gui`, header quick-toggle,
eligible ready auto-start, backlog skip, epic-order skip, Cancel during
auto-started run, and restart while an auto-started run is queued/running.

## Out of scope

- No settle window (commitment-gated, see Goal).
- No new `AgentStatus` value.
- No new column or kanban edge.
- No CLI surface for auto-implement settings (Settings panel is the
  only edit point; `.tb.yaml` is unchanged).
- No multi-agent rotation; the existing single `default_agent` field
  drives unassigned tasks.
