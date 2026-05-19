# TB-241: GUI: Resume button enabled for interrupted tasks with no captured session

**Type:** bug
**Priority:** P2
**Size:** S
**Agent:** claude
**AgentStatus:** success
**Module:** gui
**Tags:** agent,resume,frontend,ui
**Branch:** —

## Goal

Stop the GUI from offering or invoking Resume on tasks whose `AgentStatus: interrupted` has no captured session id, so clicking Resume no longer produces an `ERR Binding call failed: Bound method returned an error: task has no resumable session` log line for an expected user-recoverable state.

### Context

- The error originates at `gui/app/agent_service.go:47` — `ErrNotResumable = errors.New("task has no resumable session")` — and is returned from `AgentService.ResumeAgent` in `gui/app/agent_run.go:198` after `resumableSessionID(boardDir, id)` finds no session event in the run's JSONL.
- The frontend Resume gates (`gui/frontend/src/lib/components/Card.svelte:101` `canResumeOnCard` and `gui/frontend/src/lib/components/TaskDrawer.svelte:364` `canResume`) only check `task.agentStatus === 'interrupted'`. They never consult resumability, so the button is enabled in any state Wails returns as `interrupted`.
- Recovery (`gui/app/agent_recovery.go:171`) only marks a stale running task `interrupted` when `latest.SessionID != ""`. So the **normal** path is consistent. The mismatch shows up via:
  - `tb edit <ID> --agent-status interrupted` set manually (the CLI validator accepts it; see `cli/CLAUDE.md` notes on `edit.go`).
  - JSONL state (`board/.agent-state/<ID>.jsonl` or `<status>/<ID>/.agent-state.jsonl`) cleared, renamed, or pre-TB-130 — no `session` event recorded.
- Wails3 logs any binding-returning-non-nil-error as `ERR Binding call failed: ...`, which looks like a fault even though the frontend already shows a friendly toast.
- The screenshot attached to this task is the drawer for TB-176, status `ready`, with the Resume control visible — the immediate repro that surfaced the log line.

### Constraints / non-goals

- Do not silently rewrite `AgentStatus` from `interrupted` to `failed` to "fix" the mismatch — preserve manual state set via `tb edit --agent-status interrupted`.
- Do not break the existing `ResumeAgent` contract or `tb edit --agent-status interrupted` CLI validation.
- Do not require a separate Wails round-trip on every card render — surface resumability through the existing task snapshot used by Card/Drawer.
- Out of scope: redesigning resume from finished runs (still deferred per `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md` § 13) and changing pill colours / icons (TB-140 territory).

### Related Tasks

- **TB-130** — Agent session resume + interrupted-run recovery (parent design; sets the resumable-session contract).
- **TB-138** — Resume backend Claude (defines the call site this task gates).
- **TB-139** — Resume backend Codex (same).
- **TB-140** — Frontend Resume button / interrupted pill (introduced the current `canResume` derivation that needs the new flag).
- **TB-176** — Track PID of launched agents (the task whose drawer surfaced the report; complementary, not blocking).

## Acceptance Criteria

- [ ] `BoardService.GetTask` (and the task snapshot used to render Card.svelte) exposes a boolean — e.g. `agentResumable` — populated by reading the same `resumableSessionID` helper (`gui/app/agent_recovery.go`) that `ResumeAgent` already consults. File-form and folder-form tasks both produce the correct value.
- [ ] `Card.svelte:101` (`canResumeOnCard`) and `TaskDrawer.svelte:364` (`canResume`) include the new flag in their derivation. Resume is disabled with a tooltip such as "No captured session for this interrupted run — use Run to start fresh." when `AgentStatus === "interrupted"` AND the flag is false.
- [ ] A request that still reaches `AgentService.ResumeAgent` from a stale state continues to return `ErrNotResumable`, but the Wails service log no longer emits an `ERR Binding call failed` line for that error path. The user-visible toast remains unchanged.
- [ ] Backend tests cover `BoardService.GetTask` snapshot for (a) interrupted + session captured → `agentResumable=true`, (b) interrupted + no session in JSONL → `agentResumable=false`, (c) any other `AgentStatus` → `agentResumable=false`.
- [ ] Vitest tests cover `Card.svelte` and `TaskDrawer.svelte` Resume gating against the new flag (enabled vs disabled with tooltip), and that clicking Resume in the disabled state issues no Wails binding call.
- [ ] Manual UI test: with a folder-form task, run `tb edit <ID> --agent-status interrupted` (no JSONL session present), open the drawer, confirm Resume is disabled with the explanatory tooltip and no `ERR Binding call failed` line appears in the daemon log when hovering/clicking; clear with `tb edit <ID> --agent-status none` and confirm the button disappears.

## Attachments

- Снимок экрана 2026-05-19 в 20.28.50.png

## Log

- 2026-05-19: Created
- 2026-05-19: Attached Снимок экрана 2026-05-19 в 20.28.50.png
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited size=S, module=gui, tags=agent,resume,frontend,ui, title=GUI: Resume button enabled for interrupted tasks with no captured session
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited goal
- 2026-05-19: Edited agentstatus=success

