# TB-133: Shared post-started session-write hook in runGoroutine

**Type:** feature
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** epic-tb130,agent,resume,daemon
**Branch:** —
**Parent:** TB-130

## Goal

Add the single shared session-write hook inside `runGoroutine`'s
`OnStarted` callback. The hook fires AFTER the existing `started`
event is appended (PID known, durable on disk), so a crash between
`started` and `session` leaves the existing recovery contract
intact: no `session` event → `failed` branch, exactly as today.

This task wires the hook for the Claude side; Codex follows in
TB-136 via the `OnSessionID` callback the hook accepts.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 2 (Claude / Codex capture flows), § 3 (single shared point).

Codex round-1 blocker 1 was that v1 put session allocation in
`startAgentRun`, which the daemon's `RunQueuedAgentSync` bypasses.
v2 moved it into `runGoroutine` — the convergence point both entry
paths use. This task lands the hook itself; the UUID generator and
the runner-arg plumbing land in TB-135 (Claude) / TB-136 (Codex).

Codex round-1 blocker 2 was that writing `session` BEFORE `started`
would leave `running + session_id + no PID` after a between-fork
crash. This task enforces "session always trails started".

## Acceptance Criteria

- [ ] In `gui/app/agent_run.go:runGoroutine`'s `OnStarted` callback,
      AFTER the existing `agent.AppendEvent(EvStarted, ...)` call:
  - If `ar.SessionID != ""` (Claude case), append
    `EvSession{session_id: ar.SessionID, pid, cwd, run_env}`
    immediately. Holds same `taskMutex` ordering as other writes.
  - If `ar.SessionID == ""` (Codex / not-yet-captured case), no-op.
    The Codex `OnSessionID` callback (TB-136) will write the event
    later when the id arrives in the stream.
- [ ] `RunInput` gains `OnSessionID func(string)` callback that, when
      invoked, appends an `EvSession` event with the same fields and
      the PID stored on `ar` (already known from OnStarted).
- [ ] `Cwd` field on `EvSession` is captured from
      `RunInput.ProjectRoot` (or `RunInput.Cwd` once that lands in
      TB-138 — pick whichever is present at write-time).
- [ ] `RunEnv` field is captured from `RunInput.Env`, filtered to
      `TB_`-prefixed keys (re-using the TB-132 filter).
- [ ] Tests cover BOTH entry paths:
  - `startAgentRun` (manual UI) → set `ar.SessionID = "uuid-x"` via
    a fake runner, kill mid-run, assert JSONL has the session event.
  - `RunQueuedAgentSync` (daemon) → same; assert the daemon path
    also writes the session event.
- [ ] Test: ar.SessionID empty + OnSessionID called later → session
      event still lands.
- [ ] Test: ar.SessionID empty + OnSessionID never called → no
      session event (graceful degradation, run still completes).

## Related Tasks

- **TB-130** — parent epic.
- Depends on **TB-132** (`EvSession` constant + Event fields).
- Blocks **TB-135** (Claude UUID source), **TB-136** (Codex
  OnSessionID consumer).

## Log

- 2026-05-14: Created
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done
