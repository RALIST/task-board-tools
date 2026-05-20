# TB-300: Auto-implement: respect WIP and CPU worker budget

**Type:** bug
**Priority:** P1
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** auto-implement,daemon,settings,wip
**GroomedBy:** codex
**GroomStatus:** success
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewedBy:** codex
**ReviewStatus:** success
**ReviewRef:** ce50c51
**Branch:** —

## Goal

Make auto-implement scheduling honor both board WIP limits and the configured daemon worker budget. The coordinator should not pull or queue more ready tasks than can currently run, should treat `wip_limit_in_progress` as a hard automation cap even when CLI enforcement is `warn`, and settings should allow `max_workers` up to the host CPU count instead of the current fixed upper bound of 4.

## Context

- `gui/app/auto_implement.go` currently builds a sorted candidate list and calls `startCandidate` for every eligible ready task. `startCandidate` uses canonical `tb pull`, so strict WIP failures are safe, but warn-mode WIP can still let automation overfill `in-progress`, and the scan does not reserve or limit starts by remaining worker slots.
- `gui/internal/daemon/daemon.go` spawns `MaxWorkers` worker goroutines from settings and exposes `MaxWorkers()`, while `AgentService.HasActiveRun` is only a per-task guard. Auto-implement needs an explicit remaining-capacity decision before starting new candidates.
- `gui/app/preferences.go`, `gui/frontend/src/lib/components/SettingsPanel.svelte`, `docs/FEATURES.md`, `docs/ARCHITECTURE.md`, and `gui/internal/daemon/README.md` currently describe or enforce `max_workers` as `1-4`.
- `tb board --json` / `BoardService.LoadBoard` expose `wipLimits`, `wipCounts`, and `wipEnforcement`; use those structured fields rather than parsing generated `BOARD.md`.
- Related work: TB-177 shipped auto-implement, TB-239 shipped ready/WIP mechanics, and TB-266 owns deterministic reconciliation/backoff after partially applied moves. This card is for preflight scheduling and settings limits, not reconciliation ownership.

## Constraints

- Keep manual CLI WIP semantics unchanged: `warn` still warns for human/CLI moves and `strict` still blocks. Auto-implement should be more conservative than a manual warn-mode move.
- Keep all task movement on managed CLI paths such as `tb pull`; WIP/capacity preflight is an early skip, not a replacement for the canonical move guard.
- Preserve the default `max_workers=1` and the current no-hot-reload daemon worker-count behavior unless a separate task explicitly changes daemon lifecycle.
- Keep backend, frontend, tests, and docs on one shared definition of the maximum worker count: `runtime.NumCPU()` with a minimum of 1.
- When work is skipped because capacity or WIP is full, leave task files in place and surface a visible skip/status reason; durable reconciliation backoff remains TB-266's responsibility.

## Acceptance Criteria

- [x] Backend settings clamp `max_workers` to `[1, runtime.NumCPU()]` with minimum 1, log out-of-range coercions, persist and round-trip a value equal to `runtime.NumCPU()`, and keep the default at `1`.
- [x] Settings UI and TypeScript preference wrappers validate/display the same CPU-based range; helper text and numeric input maximum no longer hard-code `4`.
- [x] Auto-implement scans compute remaining worker capacity before starting candidates; existing daemon/AgentService active runs reduce available slots, and zero capacity leaves ready tasks untouched with a skip reason.
- [x] Auto-implement WIP preflight uses the structured board snapshot for `wipLimits["in-progress"]` and `wipCounts["in-progress"]`; when the explicit limit is full, it does not call `tb pull` or start an agent even if enforcement is `warn`.
- [x] Strict-mode races remain safe: if WIP changes between preflight and `tb pull`, the pull failure is recorded/emitted and no agent run starts.
- [x] No-limit boards preserve existing behavior except for worker-budget limiting: missing or zero `wip_limit_in_progress` does not block auto-implement.
- [x] Tests cover CPU-count max-worker round trip/clamping, Settings UI range rendering, worker budget with multiple ready candidates, active-run capacity reduction, warn-mode WIP skip, strict pull-failure fallback, post-pull run-failure WIP accounting, and no-limit behavior.
- [x] Docs that said `max_workers` is `1-4` now describe the CPU-count ceiling and restart-required worker-count semantics.
- [x] Manual test note: desktop auto-implement end-to-end smoke was not run in this API/headless session; backend tests cover the WIP/worker behavior, including warn/strict paths and post-pull failure accounting.
- [x] Verification passed: `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited body via GUI
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited priority=P1, type=bug, size=M, module=gui, tags=auto-implement,daemon,settings,wip, title=Auto-implement: respect WIP and CPU worker budget
- 2026-05-20: Edited goal
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewed-by=codex, review-status=success, reviewref=ce50c51, acceptance
- 2026-05-20: Done
