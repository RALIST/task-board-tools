# TB-135: Claude session capture via --session-id pre-allocation

**Type:** feature
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** epic-tb130,agent,resume,claude
**Branch:** —
**Parent:** TB-130

## Goal

Pre-allocate a UUIDv4 in `runGoroutine` before invoking the Claude
runner, plumb it via `RunInput.SessionID`, and have `ClaudeRunner`
append `--session-id <uuid>` to its args. Combined with TB-133's
post-`started` hook this captures a stable Claude session id into
the JSONL on every run.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 2 (Claude pre-allocated path), § 12 task E, § "Risks" (UUID
validity).

The current `claude_stream.go:104-105` already extracts `session_id`
from the `system/init` event but only renders it as text. Pre-
allocating means we know the id before the process starts and can
write it to JSONL the moment `started` lands — no race window where
the id is in flight but unrecorded.

`GenerateRunID`'s 32-bit hex (`gui/internal/agent/state.go:333-347`)
is NOT a valid UUID. We need a real `crypto/rand`-based UUIDv4
helper.

## Acceptance Criteria

- [ ] New helper in `gui/internal/agent/state.go` (or a new
      `uuid.go`):
      ```go
      func NewSessionUUID() string  // returns RFC 4122 v4 UUID, lowercase
      ```
      Uses `crypto/rand`; format `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`.
- [ ] `RunInput` gains `SessionID string` field.
- [ ] `runGoroutine` (`gui/app/agent_run.go:366`) generates a UUID
      before invoking the runner and stores it on `ar.SessionID`.
      ALWAYS for Claude; SKIP for Codex (Codex captures it from the
      stream, see TB-136).
- [ ] `ClaudeRunner.Run` (`gui/internal/agent/claude.go:32`)
      appends `--session-id <uuid>` to args when `in.SessionID != ""`.
- [ ] Post-`started` hook (TB-133) writes the `session` JSONL event
      using `ar.SessionID`.
- [ ] Stub-binary integration test (mirror
      `gui/internal/agent/exec_test.go`): assert
      `--session-id <uuid>` is in argv when `RunInput.SessionID` is
      set.
- [ ] Integration smoke (behind a build tag — NOT in default CI):
      spawn real Claude via `claude -p ... --session-id <uuid>`,
      kill after the first `system/init` arrives, confirm the JSONL
      `session` event matches the value passed via `--session-id`.

## Related Tasks

- **TB-130** — parent epic.
- Depends on **TB-132** (Event schema) + **TB-133** (shared hook).
- Blocks **TB-137** (recovery needs the JSONL `SessionID` to mark
  `interrupted`), **TB-138** (Claude resume reads the same id back).

## Log

- 2026-05-14: Created
