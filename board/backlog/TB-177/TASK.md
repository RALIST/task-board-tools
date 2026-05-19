# TB-177: Auto task implementation

**Type:** feature
**Priority:** P0
**Size:** L
**Agent:** codex
**AgentStatus:** success
**Tags:** auto-implement,agent,daemon,settings,filtering,epic
**Module:** gui
**Branch:** —

## Goal

Ship an opt-in auto-implement feature: when enabled, the GUI daemon selects groomed backlog tasks matching a saved board-filter query and starts implementation-mode agent runs using the task's assigned agent or the configured default agent.

## Context

- Existing implement runs are owned by `AgentService` in `gui/app/agent_run.go`; JSONL/log artifacts, Wails events, cancellation, and terminal `AgentStatus` writes should remain centralized there.
- Existing daemon pickup lives in `gui/internal/daemon/daemon.go`: it scans/enqueues tasks, respects `max_workers`, dedupes active runs, and recovers after restart.
- Existing preferences live in `gui/app/preferences.go`, `gui/app/settings_service.go`, `gui/frontend/src/lib/stores/preferences.ts`, and `gui/frontend/src/lib/components/SettingsPanel.svelte`; auto-implement should extend that path.
- Existing board filtering lives in `gui/frontend/src/lib/stores/filter.ts` and `gui/frontend/src/lib/filtering.ts`; the saved auto-implement query should match the same mental model users use on the board, including type, module, size, tags, agent, parent epic, and search text.
- `BoardService.Triage()` / `tb triage --json` identifies tasks that still need grooming. Auto-implementation must treat those tasks as ineligible even when the saved query matches.

**Constraints / non-goals**

- Auto-implement is off by default and must be explicitly enabled.
- Enabling requires both a supported `default_agent` (`claude` or `codex`) and a non-empty valid query; otherwise the user gets an actionable message and preferences remain disabled.
- Only backlog tasks that are groomed, match the query, and have blank `AgentStatus` are eligible for automatic first runs.
- Use the task's assigned `Agent` when present; otherwise use `default_agent` as the effective runner.
- Do not auto-run tasks in in-progress, done, archive, queued, running, success, failed, or cancelled states.
- Do not merge this with auto-groom: auto-groom remains groom mode, auto-implement remains implement mode, and neither feature should trigger the other accidentally.
- Settings is the source of truth. A header quick toggle may exist only as a compact mirror/update path for the same persisted setting.

## Subtasks

- **TB-178** (M) — GUI: persist auto-implement settings and query
- **TB-179** (M) — GUI: enqueue auto-implement candidates from daemon
- **TB-180** (S) — GUI: show auto-implement controls and feedback
- **TB-233** (S) — Auto-implement priority: rank review-failed backlog tasks first
## Acceptance Criteria

- [ ] **TB-178** is done: auto-implement enabled/query preferences are persisted, exposed through SettingsService/Wails/preferencesStore, validated, and covered by backend/frontend tests.
- [ ] **TB-179** is done: daemon activation and watcher events enqueue only groomed backlog tasks matching the saved query, use assigned-agent/default-agent fallback correctly, and avoid duplicate or retry loops.
- [ ] **TB-180** is done: Settings and the header quick toggle surface enabled state, prerequisite errors, query changes, and auto-started run feedback while preserving manual Run/Groom controls.
- [ ] Disabled path: with auto-implement disabled, creating or editing a matching groomed backlog task never enqueues an implementation run automatically.
- [ ] Validation path: with `default_agent=none` or a blank/invalid query, enabling auto-implement is rejected with an actionable message and no task metadata/JSONL/log files are mutated.
- [ ] Enabled path: with auto-implement enabled, a valid query such as `bug, S size, gui`, and `default_agent=codex` or `claude`, a matching groomed backlog task with blank `AgentStatus` starts exactly one visible `mode=implement` run.
- [ ] Grooming guard: a matching task that appears in `tb triage --json` is skipped until it is groomed, regardless of query match.
- [ ] Agent selection: a matching task with an explicit `Agent` uses that agent; an unassigned matching task uses the configured default agent.
- [ ] Verification for the epic includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.
- [ ] Manual test note: exercise Settings enable/disable, missing default-agent message, invalid query message, header quick toggle, query `bug, S size, gui`, eligible auto-start, ungroomed skip, Cancel during an auto-started run, and app restart while an auto-started run is queued/running.

## Related Tasks

- **TB-5** — Existing agent daemon pickup/recovery/shutdown lifecycle this feature extends.
- **TB-76** — Existing backend preferences foundation.
- **TB-81** — Existing Settings panel surface.
- **TB-172** — Sibling auto-groom epic; keep groom-mode automation separate from implement-mode automation.
- **TB-178** — Child: persisted auto-implement settings and query.
- **TB-179** — Child: daemon candidate selection and queueing.
- **TB-180** — Child: controls and feedback.

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

