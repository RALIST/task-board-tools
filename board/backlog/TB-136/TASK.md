# TB-136: Codex session capture via --json OnSessionID callback

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb130,agent,resume,codex
**Branch:** —
**Parent:** TB-130

## Goal

Parse the Codex session id from the `--json` stream (TB-134's
translator) and route it via `RunInput.OnSessionID` (TB-133) into a
JSONL `session` event. Unlike Claude, Codex does NOT accept a
pre-allocated session id; we must observe it from the output.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 2 (Codex parsed path), § 12 task F.

Codex's `exec --json` emits events to stdout including a session id
(exact field name to be enumerated in TB-134's translator work). The
first event carrying it triggers the callback. If Codex never emits
one (degraded `--json`, network error pre-init), the run still
works — it just isn't resumable. Recovery sees no SessionID →
`failed` branch, exactly the TB-133 graceful-degradation path.

## Acceptance Criteria

- [ ] `codexJsonTranslator` (from TB-134) gains a wired
      `OnSessionID func(string)` callback. When the first event
      carrying a `session_id` (exact field-name verified during
      TB-134) is observed, fire the callback exactly once.
- [ ] `CodexRunner.Run` passes `in.OnSessionID` into the translator.
- [ ] `runGoroutine` provides an `OnSessionID` closure that calls
      `agent.AppendEvent(EvSession, ...)` with the captured id and
      the PID already known from `OnStarted`.
- [ ] Idempotency: if `OnSessionID` fires multiple times (it
      shouldn't, but defensive), only the first call writes the
      JSONL event; subsequent calls log a warning and no-op.
- [ ] Test (translator): feed a `--json` stream containing a
      session-init event; assert OnSessionID is called with the
      expected id and exactly once.
- [ ] Test (translator): feed a `--json` stream missing the
      session-init event; assert OnSessionID is NEVER called and the
      runner still completes normally.
- [ ] Integration test (fake runner, no real codex): assert the
      JSONL has the expected `session` event when OnSessionID fires.
- [ ] Integration smoke (behind a build tag): real `codex exec
      --json` round-trip; confirm session event lands in JSONL.

## Related Tasks

- **TB-130** — parent epic.
- Depends on **TB-133** (`OnSessionID` consumer hook), **TB-134**
  (Codex `--json` translator).
- Blocks **TB-137** (recovery uses captured Codex session id),
  **TB-139** (Codex resume reads it back).

## Log

- 2026-05-14: Created
