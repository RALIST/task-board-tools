# TB-304: Auto-groom: respect ready WIP limit

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** gui
**Tags:** auto-groom,wip,daemon
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Make auto-groom treat `wip_limit_ready` as a hard automation cap: when the ready column is already at or over its configured limit, auto-groom should not start new groom runs that would produce more ready work, and successful groom runs should not auto-promote into an already-full ready column.

## Context

- `docs/FEATURES.md` defines per-column WIP limits from `.tb.yaml`; `wip_enforcement: warn` remains permissive for manual CLI moves, while `strict` blocks managed moves.
- `docs/ARCHITECTURE.md` defines auto-groom as backlog triage -> groom run -> managed `tb ready` promotion only after triage is clean.
- `gui/app/auto_groom.go` currently queues triage candidates in `evaluate` without checking ready-column capacity, and `tryPromote` calls `BoardService.ReadyTask` after a successful groom. In warn mode, `tb ready` can still return success while overfilling `ready`.
- `gui/app/auto_implement.go` and TB-300 already establish the automation pattern: use structured board WIP data as a conservative preflight cap while preserving manual CLI warn semantics.
- Related tasks: TB-174 shipped auto-groom queueing/promotion, TB-239 shipped ready/WIP mechanics, TB-300 shipped the auto-implement in-progress WIP guard, and TB-266 owns broader daemon reconciliation/backoff behavior.

## Constraints

- Keep manual CLI behavior unchanged: `tb ready` in warn mode should still warn and allow, and strict mode should still block through the existing managed path.
- Use structured board data such as `BoardService.LoadBoard` / `tb board --json` `wipLimits` and `wipCounts`; do not parse generated `BOARD.md`.
- Keep all task movement on managed board operations (`BoardService.ReadyTask` / `tb ready`). WIP preflight is an early automation skip, not a replacement for the canonical move guard.
- Missing or zero `wip_limit_ready` means no ready-column cap and should preserve existing auto-groom behavior.
- WIP-blocked auto-groom work must leave backlog tasks and agent metadata untouched before queueing, surface a useful skip/diagnostic reason, and avoid retry/event hot loops. Durable reconciliation backoff remains TB-266 scope.

## Acceptance Criteria

- [ ] Auto-groom scan preflights the ready-column WIP state before selecting/queueing backlog triage candidates. When `wip_limit_ready` is configured and `ready` count is at or above the limit, no new groom run is queued, no `Agent` / `AgentStatus` / JSONL metadata is written for the skipped backlog task, and the coordinator records a visible skip reason such as `ready WIP limit full`.
- [ ] Post-groom promotion re-checks ready-column WIP before calling `BoardService.ReadyTask`. If the ready cap is full in warn mode, the groomed task remains in backlog, no over-limit `tb ready` promotion occurs, and a useful `auto-groom` diagnostic/event is emitted for the frontend/status surface.
- [ ] Strict-mode races remain safe: if capacity changes between preflight and the managed `tb ready` call, the existing `ReadyTask` error path records/emits the promotion failure and does not start a duplicate groom run or hot-loop retries.
- [ ] Boards with no `wip_limit_ready` or `wip_limit_ready: 0` keep current auto-groom behavior: eligible triage tasks can queue, successful clean grooming can promote through `tb ready`, and worker/agent-status guards still apply.
- [ ] Tests cover warn-mode ready-WIP preflight skip before queueing, post-groom promotion blocked by ready WIP, strict/managed promotion failure fallback, no-limit behavior, and preservation of manual CLI WIP semantics.
- [ ] Verification passes with `cd gui && go test ./...`.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited priority=P2, type=bug, size=M, module=gui, tags=auto-groom,wip,daemon, title=Auto-groom: respect ready WIP limit
- 2026-05-20: Edited goal
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success

