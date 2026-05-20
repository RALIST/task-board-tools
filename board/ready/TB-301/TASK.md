# TB-301: GUI: add startup grace before automation pickup

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** automation,daemon,startup,ux,auto-implement
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Give users a short, visible grace window after the GUI opens a board before daemon-driven automation starts existing queued or otherwise eligible tasks. Stale-run recovery and board loading should still run immediately, but startup pickup for queued tasks and autonomous stage scans should wait so the user can switch boards, disable automation, or correct board state first.

## Context

- `docs/ARCHITECTURE.md` defines daemon activation as stale recovery, watcher sink registration, then startup queue scan. Preserve that ordering, but delay the pickup/scan portion that can launch work.
- `gui/internal/daemon/daemon.go` currently performs startup queue pickup for tasks with `AgentStatus: queued` after activation.
- `gui/app/auto_groom.go` and `gui/app/auto_implement.go` each schedule an activation scan with the shared short `scanDebounce`; that debounce coalesces events but does not give a human time to intervene after app start or board switch.
- `gui/app/preferences.go`, `gui/app/settings_service.go`, `gui/frontend/src/lib/stores/preferences.ts`, and `gui/frontend/src/lib/components/SettingsPanel.svelte` already own persisted automation/settings controls; any configurable grace knob should use that path.
- Existing auto-groom settle time is per-backlog-task editing protection. This task is a board-activation grace period for daemon/autonomous pickup, not a replacement for auto-groom settle semantics.

## Constraints

- Preserve immediate stale-running recovery on board activation so `running` tasks are reconciled before the user acts.
- Preserve watcher sink registration before any delayed startup scan so edits made during the grace window are not missed.
- Gate only automation pickup/scans. Explicit user actions such as manual Run, Groom, Review, Cancel, Settings, and Open board remain responsive.
- Deactivating or switching boards must cancel pending grace timers for the previous board; a delayed callback must never start work on a board that is no longer active.
- Watcher or settings changes during the grace window should be coalesced and evaluated once against the latest board state when the window expires, unless the configured delay is zero.
- Keep existing auto-groom, auto-implement, WIP, worker-budget, `needs-user`, `cancelled`, `interrupted`, and `lost` gates intact.

## Acceptance Criteria

- [ ] Preferences expose `automation_startup_grace_seconds` with default `30`, clamp range `[0, 300]`, and `0` meaning no delay; backend, Wails bindings, TypeScript store, and Settings UI all use that same default/range.
- [ ] On GUI app start or board switch, activation-time stale recovery still runs immediately and the watcher sink is registered before any delayed scan, but existing `AgentStatus: queued` tasks are not picked up until the grace window expires.
- [ ] Auto-groom and auto-implement activation scans respect the same grace window for existing eligible tasks, including auto-resume/restart work owned by those coordinators; the existing per-task auto-groom settle window continues to apply after the startup grace has elapsed.
- [ ] Board switch/deactivation cancels pending delayed pickups for the previous board. A test proves opening board A with eligible work, switching to board B before the grace expires, and waiting past the original deadline never starts work from board A.
- [ ] Watcher/settings events that happen during the grace window are not lost: they are coalesced and one scan runs against the latest active board state after the grace window; with the setting at `0`, current immediate-pickup behavior is preserved.
- [ ] The GUI surfaces automation-paused/startup-grace state with a compact countdown or status indicator near the existing automation controls, without hiding manual Run/Groom/Review controls.
- [ ] Tests cover preference default/clamping/round-trip, delayed daemon queue pickup, delayed auto-groom/auto-implement activation scans, watcher coalescing during grace, board-switch cancellation, zero-delay behavior, and immediate stale-recovery behavior.
- [ ] Docs that describe daemon activation, startup pickup, and autonomous stages are updated so the grace period and `0` opt-out are explicit.
- [ ] Verification includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.
- [ ] Manual test note: launch the GUI with a recent board that has auto-implement enabled and an eligible ready task plus a queued task; confirm neither starts during the grace window, switch boards before expiry and confirm the old board stays untouched, then let the grace expire on the intended board and confirm eligible automation starts once.

## Related Tasks

- **TB-5 / TB-57** — Original daemon startup queue pickup and activation ordering.
- **TB-172** — Auto-groom stage and its separate per-task settle window.
- **TB-177** — Auto-implement stage that scans ready tasks on activation.
- **TB-291** — Auto-resume behavior that must also respect startup grace for coordinator-owned runs.
- **TB-300** — Worker-budget and WIP preflight that must still gate starts after the grace window.
- **TB-266** — Deterministic reconciliation/backoff must stay separate from this startup pause.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited priority=P2, type=bug, size=M, module=gui, tags=automation,daemon,startup,ux,auto-implement, title=GUI: add startup grace before automation pickup
- 2026-05-20: Edited goal
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success

