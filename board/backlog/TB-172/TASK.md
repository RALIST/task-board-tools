# TB-172: Auto-groom

**Type:** feature
**Priority:** P1
**Size:** L
**Agent:** codex
**AgentStatus:** failed
**Tags:** auto-groom,groom,triage,settings,daemon,epic
**Module:** gui
**Branch:** —

## Goal

Ship an opt-in auto-groom feature: when enabled, the GUI automatically starts groom-mode agent runs for backlog tasks returned by `tb triage`; when disabled or unavailable, users can still groom tasks manually from the drawer. 
UI: header should have toggle - enable\disable autogroom right from the board

## Context

- M6 already shipped the manual groom path: `AgentService.GroomTask`, `GroomingDecorator`, `mode=groom` JSONL/Wails events, `BoardService.Triage()`, `triageStore`, the Card needs-grooming indicator, and the TaskDrawer Groom button (TB-6, TB-67..TB-73).
- M5 already shipped daemon pickup for `AgentStatus=queued` tasks with an `Agent` set, plus active-run dedupe, recovery, and shutdown behavior (TB-5).
- M7 already shipped persisted preferences, `default_agent`, `preferencesStore`, and the Settings panel (TB-76, TB-80, TB-81). Auto-groom should extend that settings path instead of adding a new config file.
- `tb triage` currently scans backlog tasks only and can report no priority, no module, placeholder Goal, placeholder Acceptance Criteria, and auto-created-by-scan reasons.

**Constraints / non-goals**

- Auto-groom is off by default and requires an explicit `Enable auto groom` setting.
- A valid `default_agent` (`claude` or `codex`) is required before automation starts. If it is missing, the GUI must tell the user to set it instead of failing silently.
- Reuse the existing groom-mode lifecycle; do not introduce a second runner, second run-history model, second cancel path, or direct task-file writes from the GUI.
- Do not auto-groom tasks outside backlog, tasks already queued/running, or unchanged tasks that have already completed an auto-groom attempt for the same triage state.
- Manual Groom remains available when auto-groom is off, skipped by a guard, or the user wants an explicit retry.

## Subtasks

- **TB-173** (M) — GUI: persist auto-groom setting and toggle
- **TB-174** (M) — GUI: auto-groom triage tasks via groom-mode daemon runs
- **TB-175** (S) — GUI: surface auto-groom feedback and manual fallback
## Acceptance Criteria

- [ ] **TB-173** is done: `auto_groom_enabled` is persisted, exposed through SettingsService/Wails/preferencesStore, rendered in Settings, defaulted off, and covered by backend/frontend tests.
- [ ] **TB-174** is done: when auto-groom is enabled and `default_agent` is set, triage-reported backlog tasks are queued as `mode=groom` runs through the existing daemon/AgentService lifecycle with durable dedupe and no implement-mode fallback.
- [ ] **TB-175** is done: users can see auto-groom state, get an actionable no-default-agent message, and still use the manual Groom button when automation is disabled, skipped, or manually retried.
- [ ] Disabled path: with `auto_groom_enabled=false`, creating or editing a triage-worthy backlog task never enqueues a run automatically; the Card indicator and TaskDrawer Groom button remain available.
- [ ] Enabled path: with `auto_groom_enabled=true` and `default_agent=codex` or `claude`, creating or editing a triage-worthy backlog task starts exactly one visible groom-mode run, writes normal JSONL/log artifacts, and clears or records a guarded skip without duplicate reruns.
- [ ] No-default path: with `auto_groom_enabled=true` and `default_agent=none`, no task metadata/JSONL is mutated and the GUI tells the user to set a default agent in Settings.
- [ ] Verification for the epic includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.
- [ ] Manual test note: exercise Settings toggle on/off, default-agent missing/configured, auto queue from a placeholder backlog card, manual Groom fallback, Cancel during an auto-groom run, and app restart while a groom run is queued/running.

## Related Tasks

- **TB-5** — Existing daemon pickup/recovery/shutdown lifecycle this feature reuses.
- **TB-6** — Existing manual groom and triage highlighting epic.
- **TB-70** — `BoardService.Triage()` source for automation candidates.
- **TB-72** — Existing manual Groom drawer behavior.
- **TB-73** — Existing needs-grooming card indicator.
- **TB-76** — Existing backend preferences foundation.
- **TB-81** — Existing Settings panel surface.
- **TB-88** — Triage-unavailable fallback for stale CLI binaries must remain advisory.
- **TB-173** — Child: persisted setting and Settings toggle.
- **TB-174** — Child: daemon-side auto-groom queueing.
- **TB-175** — Child: frontend feedback and manual fallback.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited priority=P0
- 2026-05-15: Edited priority=P1, type=feature, size=L, module=gui, tags=auto-groom,groom,triage,settings,daemon,epic, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=failed
- 2026-05-15: Edited body via GUI
