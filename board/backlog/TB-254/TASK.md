# TB-254: Stale recovery should write per-mode pairs for recovered terminal runs

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** agent,metadata,attribution,recovery
**Agent:** codex
**AgentStatus:** interrupted
**Branch:** —

## Goal

Update stale-run recovery so the latest dead-PID run writes TB-237 per-mode attribution alongside the legacy `AgentStatus` snapshot for both task-level recovery outcomes: captured-session runs become `interrupted`, and no-session stale runs become `lost`.

**Context**

- Related follow-up from TB-237. Per-mode task metadata is `GroomedBy/GroomStatus`, `ImplementedBy/ImplementStatus`, and `ReviewedBy/ReviewStatus`; legacy `Agent/AgentStatus` still reflects the most recent run for compatibility.
- `gui/app/agent_recovery.go` already distinguishes dead PID + captured session from no-session stale runs. The latest captured-session branch calls `markInterrupted`; the latest no-session branch calls `markLost`; older stale rows are JSONL-only.
- `markInterrupted` and `markLost` currently append the synthetic `finished` event and edit only legacy `AgentStatus`, so the action-specific status can remain stale or empty after recovery.
- The recovery reader already records the queued JSONL mode on `runRecoveryView.Mode`; missing legacy mode should follow the existing resume fallback and target implement.

**Constraints / non-goals**

- Only change the paths where `RecoverStale` writes task-level recovery outcomes for the latest stale run: `AgentStatus=interrupted` when a session id exists and `AgentStatus=lost` when it does not.
- Do not change alive-PID handling, older non-latest JSONL-only terminalization, or the cancelled carve-out.
- Keep task metadata writes on the managed `cli.Client.Edit` / `tb edit` path; no direct task markdown writes.
- Do not introduce a per-mode `needs-user` behavior or a fourth `resume` attribution slot.
- Preserve file-form and folder-form task support through the existing task resolution and recovery paths.

**Related Tasks**

- **TB-237** - introduced per-mode agent attribution.
- **TB-130** - introduced resumable interrupted recovery.
- **TB-251** - split daemon-lost recovery from real agent failures.

## Acceptance Criteria

- [ ] Captured-session dead-PID recovery still appends synthetic `finished{status: interrupted, reason: "interrupted by daemon restart"}` and writes legacy `AgentStatus: interrupted` for the latest stale run.
- [ ] No-session dead-PID recovery still appends synthetic `finished{status: lost, reason: "stale after restart"}` and writes legacy `AgentStatus: lost` for the latest stale run.
- [ ] Each task-level recovery edit also writes the matching per-mode pair for the recovered run: `ModeGroom` -> `GroomedBy/GroomStatus`, `ModeImplement` or missing mode -> `ImplementedBy/ImplementStatus`, and `ModeReview` -> `ReviewedBy/ReviewStatus`, with the recovered agent name and terminal status (`interrupted` or `lost`).
- [ ] Cancelled recovery still wins over interrupted/lost, live-PID tasks remain untouched, and older non-latest stale rows are terminalized in JSONL only without changing task metadata.
- [ ] Tests cover recovered `interrupted` and recovered `lost` writes for groom, implement/default, and review modes, including assertions that unrelated per-mode fields are not overwritten.
- [ ] Verification: `cd gui && go test ./app -run 'TestRecoverStale_.*Interrupted|TestRecoverStale_.*Lost|TestRecoverStale_.*PerMode'` and `cd gui && go test ./...` pass.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited title=Stale recovery should write per-mode pairs for recovered terminal runs
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-20: Edited goal/acceptance after TB-251 introduced `lost`
