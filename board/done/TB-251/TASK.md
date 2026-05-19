# TB-251: Distinguish agent-failed from daemon-lost in recovery

**Type:** improvement
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** agent,daemon,recovery,reliability
**Agent:** codex
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**Branch:** —

## Goal

Make recovery surface daemon-side run loss as a distinct, resumable state rather than overloading `AgentStatus=failed`, which today conflates "agent CLI exited non-zero" with "daemon never saw a `finished` event".

## Acceptance Criteria

- [x] Audit `RecoveryService.recoverOne` in `gui/app/agent_recovery.go`: natural JSONL `finished{status: failed}` remains the only recovery route to `failed`; dead PID/no session, no JSONL, and recovered orphan exit now route to `lost`.
- [x] Chose design (b): introduce `lost`. Documented the AgentStatus invariant in `docs/ARCHITECTURE.md`, `CLAUDE.md`, `AGENTS.md`, `docs/PROJECT.md`, `docs/FEATURES.md`, `docs/IMPLEMENTATION.md`, `cli/CLAUDE.md`, generated board conventions, and daemon smoke notes.
- [x] `markFailed` is gone from recovery. Rule 2 syncs existing `finished{failed}` from JSONL; rule 5 and rule 6 use `markLost` / `StatusLost`.
- [x] Wails `agent:run-finished` payloads emit distinct `lost` versus `interrupted` statuses; interrupted still includes `session_id`, lost does not. Frontend run/card/drawer status handling and styling now includes `lost`.
- [x] Stale-recovery tests in `gui/app/agent_recovery_test.go` cover natural finished sync, cancelled carve-out, interrupted with session, lost without session, no JSONL lost, live PID skip/monitor, older-run terminalization, and non-active buckets.
- [x] Verification passed: `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run`.
- [x] Coordinated TB-252 so resume follow-up treats `lost` as terminal and keeps Resume tied to captured session IDs.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Pulled into in-progress
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-20: Moved to done

