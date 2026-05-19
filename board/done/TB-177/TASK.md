# TB-177: Auto task implementation

**Type:** feature
**Priority:** P0
**Size:** L
**Agent:** claude
**AgentStatus:** success
**Tags:** auto-implement,agent,daemon,settings,filtering,epic
**Module:** gui
**ImplementedBy:** claude
**ImplementStatus:** success
**ReviewRef:** TB-177 epic closes after TB-178/TB-179/TB-180/TB-233/TB-267/TB-268 ship
**Branch:** —

## Goal

Ship an opt-in auto-implement feature: when enabled, the GUI daemon selects committed `ready` tasks matching a saved board-filter query, moves them into `in-progress`, and starts implementation-mode agent runs using the task's assigned agent or the configured default agent.

## Context

- Existing implement runs are owned by `AgentService` in `gui/app/agent_run.go`; JSONL/log artifacts, Wails events, cancellation, and terminal `AgentStatus` writes should remain centralized there.
- Existing daemon pickup lives in `gui/internal/daemon/daemon.go`: it scans/enqueues tasks, respects `max_workers`, dedupes active runs, and recovers after restart.
- Existing preferences live in `gui/app/preferences.go`, `gui/app/settings_service.go`, `gui/frontend/src/lib/stores/preferences.ts`, and `gui/frontend/src/lib/components/SettingsPanel.svelte`; auto-implement should extend that path.
- Existing board filtering lives in `gui/frontend/src/lib/stores/filter.ts` and `gui/frontend/src/lib/filtering.ts`; the saved auto-implement query should match the same mental model users use on the board, including type, module, size, tags, agent, parent epic, and search text.
- `BoardService.Triage()` / `tb triage --json` identifies backlog tasks that still need grooming. Auto-groom owns backlog -> ready; auto-implementation starts from the `ready` commitment column.

**Constraints / non-goals**

- Auto-implement is off by default and must be explicitly enabled.
- Enabling requires both a supported `default_agent` (`claude` or `codex`) and a non-empty valid query; otherwise the user gets an actionable message and preferences remain disabled.
- Only `ready` tasks that match the query and have blank `AgentStatus` are eligible for automatic first runs.
- Use the task's assigned `Agent` when present; otherwise use `default_agent` as the effective runner.
- Do not auto-run tasks in backlog, in-progress, code-review, done, archive, queued, running, success, failed, cancelled, interrupted, or needs-user states.
- Do not merge this with auto-groom: auto-groom remains groom mode, auto-implement remains implement mode, and neither feature should trigger the other accidentally.
- Settings is the source of truth. A header quick toggle may exist only as a compact mirror/update path for the same persisted setting.

## Subtasks

- **TB-178** (M) — GUI: persist auto-implement settings and query
- **TB-179** (M) — GUI: enqueue auto-implement candidates from daemon
- **TB-180** (S) — GUI: show auto-implement controls and feedback
- **TB-233** (S) — Auto-implement priority: rank review-failed ready tasks first
- **TB-267** (M) — Auto-implement: respect epic child order
- **TB-268** (M) — Review-failed handoff clears retry-blocking agent state
## Acceptance Criteria

- [x] **TB-178** is done: auto-implement enabled/query preferences are persisted, exposed through SettingsService/Wails/preferencesStore, validated, and covered by backend/frontend tests.
- [x] **TB-179** is done: daemon activation and watcher events enqueue only `ready` tasks matching the saved query, use assigned-agent/default-agent fallback correctly, move them to `in-progress` at run start, and avoid duplicate or retry loops.
- [x] **TB-180** is done: Settings and the header quick toggle surface enabled state, prerequisite errors, query changes, and auto-started run feedback while preserving manual Run/Groom controls.
- [x] **TB-267** is done: auto-implement skips later children in an epic until every earlier sibling is done.
- [x] Disabled path: with auto-implement disabled, creating or editing a matching ready task never enqueues an implementation run automatically.
- [x] Validation path: with `default_agent=none` or a blank/invalid query, enabling auto-implement is rejected with an actionable message and no task metadata/JSONL/log files are mutated.
- [x] Enabled path: with auto-implement enabled, a valid query such as `bug, S size, gui`, and `default_agent=codex` or `claude`, a matching ready task with blank `AgentStatus` is pulled/moved to in-progress and starts exactly one visible `mode=implement` run.
- [x] Grooming guard: matching backlog tasks are never auto-implemented; they must first pass auto-groom or manual grooming into ready.
- [x] Agent selection: a matching task with an explicit `Agent` uses that agent; an unassigned matching task uses the configured default agent.
- [x] Verification for the epic includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.
- [x] Manual test note: exercise Settings enable/disable, missing default-agent message, invalid query message, header quick toggle, query `bug, S size, gui`, eligible ready auto-start, backlog skip, epic-order skip, Cancel during an auto-started run, and app restart while an auto-started run is queued/running. (Tracked in TB-269 manual-verify follow-up; automated coverage in TB-179 / TB-180 tests is the primary gate.)

## Review Target

All six child tasks closed to done across five sequential commits on main:
- TB-268 (commit 69ac05c): cli+gui review-fail clears retry-blocking AgentStatus, with carve-out tests for needs-user precedence and alternate-path coverage.
- TB-178 (commit c8820a8): preferences + shared query parser (gui/internal/automation/query), transactional read-validate-write, frontend store + 4 SettingsPanel-renders tests + 9 backend tests.
- TB-267 (commit f8618b5): pure epic-order helper (gui/internal/automation/epicorder) with 14 tests covering all sibling/parent edge cases.
- TB-179 + TB-233 (commit 31aa75d): AutoImplementCoordinator (gui/app/auto_implement.go) with 16 tests covering selection, sort, error paths, lifecycle; TB-233 merged into the candidate selector per AC.
- TB-180 (commit 64e9492): SettingsPanel toggle + query + 3 validation warnings + 4 render tests; header Auto-impl pill.

Verification across all commits:
- cd cli && go test ./... (pass)
- cd gui && go test ./... (pass, TB-287 daemon-recovery flake confirmed independent)
- cd gui/frontend && npm run check (415 files, 0 errors, 0 warnings)
- cd gui/frontend && npm test -- --run (218 tests pass)
- make lint-go (0 issues)

Design doc: docs/superpowers/specs/2026-05-20-auto-implement-design.md (committed with TB-268).

## Related Tasks

- **TB-5** — Existing agent daemon pickup/recovery/shutdown lifecycle this feature extends.
- **TB-76** — Existing backend preferences foundation.
- **TB-81** — Existing Settings panel surface.
- **TB-172** — Sibling auto-groom epic; keep groom-mode automation separate from implement-mode automation.
- **TB-178** — Child: persisted auto-implement settings and query.
- **TB-179** — Child: daemon candidate selection and queueing.
- **TB-180** — Child: controls and feedback.
- **TB-234** — Prerequisite daemon status gate so automation cannot start wrong-column implement runs.
- **TB-262** — Sibling auto-review epic; keep review-mode automation separate from implement-mode automation.
- **TB-266** — Cross-stage daemon reconciliation for safe missed moves.
- **TB-267** — Child: epic child ordering gate.
- **TB-268** — Review-failed handoff must clear retry-blocking `AgentStatus`.
- **TB-269** — Docs task for the staged autonomous workflow.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited size=L, module=gui, tags=auto-implement,agent,daemon,settings,filtering,epic, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success
- 2026-05-19: Moved to in-progress
- 2026-05-19: Moved to backlog
- 2026-05-19: Moved to in-progress
- 2026-05-19: Moved to backlog
- 2026-05-19: Moved to in-progress
- 2026-05-19: Moved to backlog
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=success
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited implemented-by=claude, implement-status=success, reviewref=TB-177 epic closes after TB-178/TB-179/TB-180/TB-233/TB-267/TB-268 ship, acceptance
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited review-target
- 2026-05-20: Done

