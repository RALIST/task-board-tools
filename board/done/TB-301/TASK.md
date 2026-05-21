# TB-301: GUI: add startup grace before automation pickup

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** automation,daemon,startup,ux,auto-implement
**GroomedBy:** codex
**GroomStatus:** success
**AgentStatus:** needs-user
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** working-tree
**ReviewedBy:** codex
**ReviewStatus:** success
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

## User Attention

Reason: board status mismatch prevents managed review failure handoff.
Question/Action: TB-301 is currently in `done`, but this review found blocking findings that should return to rework. Please move/restore the task to `code-review` or otherwise authorize the correct board repair, then run `tb review --fail TB-301 -` with the recorded findings.
Attempted context: read `tb show TB-301`, confirmed top-level `ReviewRef: working-tree`, inspected the TB-301 GUI startup-grace implementation, and wrote blocking findings to `## Review Findings`. Verified `tb review --fail` only accepts `code-review`, while `tb ls --status done` shows `board/done/TB-301/TASK.md`.
Unblock condition: TB-301 is back in a state where the managed review fail flow can move it to `ready` with `review-failed`, or a human applies the equivalent board repair.

## Review Target

Implementation scope:
- Backend preference `automation_startup_grace_seconds` with default 30, clamp 0..300, and zero-delay opt-out.
- Daemon activation keeps stale recovery immediate, registers watcher sinks, then delays startup pickup during grace.
- Auto-groom, auto-implement, and auto-review activation scans respect same startup grace and cancel on board switch/deactivation.
- Frontend settings/store/API and compact header grace indicator wired to persisted preference.
- Architecture, feature, and implementation docs updated.

Verification:
- cd gui && go test ./...
- cd gui/frontend && npm run check
- cd gui/frontend && npm test -- --run
- git diff --check
- wails3 generate bindings -ts

## Review Findings

- Blocking: `gui/frontend/src/routes/+page.svelte:101-102` derives the header grace pill only from persisted `automationStartupGraceSeconds`, and `gui/frontend/src/routes/+page.svelte:438-441` always renders `Grace {startupGraceSeconds}s` whenever the preference is nonzero. This is not startup-grace state or a countdown; after the grace window expires it still says `Grace 30s`, so the AC requiring a compact countdown/status indicator near automation controls is not met. Add active board-activation grace state/remaining time and hide or update the indicator after expiry.
- Blocking: delayed auto coordinator callbacks are not tied to an activation generation. `gui/app/auto_implement.go:154-160` clears state and stops the timer on deactivation, but an already-started callback from `gui/app/auto_implement.go:258-260` still calls `runScan`; `gui/app/auto_implement.go:264-272` then trusts the current active board. If board A's grace timer fires while switching to board B, the stale callback can scan board B before B's own grace expires. Apply the same generation/board guard used by the daemon to auto-implement, auto-groom, and auto-review delayed scans, and add board-switch race coverage.
- Blocking: auto-groom now blocks ready promotion on warn-mode WIP limits. `gui/app/auto_groom.go:29-31` treats any full ready limit as blocked without checking `WipEnforcement`, `gui/app/auto_groom.go:319-330` skips promotion before calling ready, and `gui/app/board_service.go:510-515` forces `ReadyStrictWIP`. Existing `warn` enforcement should warn but still allow the move; this breaks the AC to keep existing WIP gates intact. Only block when enforcement is strict, and preserve warn-mode promotion behavior.

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
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited user-attention
- 2026-05-21: Edited agentstatus=needs-user
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewref=working-tree, user-attention
- 2026-05-21: Edited review-target
- 2026-05-21: Submitted to code-review
- 2026-05-21: Passed code review
- 2026-05-21: Edited reviewed-by=codex, review-status=success
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited review-findings
- 2026-05-21: Edited user-attention
- 2026-05-21: Edited agentstatus=needs-user

