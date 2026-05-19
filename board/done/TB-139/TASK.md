# TB-139: Resume backend Codex: codex exec --json resume <uuid> wiring

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb130,agent,resume,codex
**Branch:** —
**Parent:** TB-130

## Goal

Wire the Codex side of `ResumeAgent`: when the runner is
`CodexRunner` and `RunInput.SessionID` is set in resume mode, invoke
`codex exec --json resume <uuid> <prompt>` instead of the fresh
`codex exec --json <prompt>`. The new session id that appears in the
resumed stream is captured via the existing TB-136 pipeline.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 8 (Resume API, Codex branch), § 12 task I.

Codex's resume creates a successor session id; `resumed_from` on
the queued event tracks the parent. The TB-136 `OnSessionID`
callback fires for the new id and writes a fresh `session` event,
which is what future resumes will use.

## Acceptance Criteria

- [ ] `CodexRunner.Run` (`gui/internal/agent/codex.go`) branches:
  - When `in.SessionID == "" || in.Mode != ModeResume`: existing
    `codex exec --json <prompt>` invocation (TB-134 default).
  - When `in.SessionID != "" && in.Mode == ModeResume`:
    `codex exec --json resume <in.SessionID> <prompt>`.
- [ ] The `--json` translator (TB-134) is wrapped around stdout in
      both branches.
- [ ] The `OnSessionID` callback (TB-136) fires for the new session
      id emitted by the resumed stream, writing a fresh `EvSession`
      event into the JSONL. Future resumes use this new id.
- [ ] `cwd` and `env` plumbing from TB-138's shared
      `ResumeAgent` body work identically — Codex resume runs in
      the persisted cwd with persisted `TB_`-env.
- [ ] Tests use a fake Codex runner that asserts:
  - argv is `["exec", "--json", "resume", "<expected-uuid>", "<prompt>"]`.
  - `cmd.Dir` matches the persisted cwd.
  - Env contains `TB_BOARD_PATH=<expected>`.
  - The fake runner emits a synthetic session-init event with a
    DIFFERENT uuid; assert the JSONL gains a fresh `session` event
    with that new uuid.
  - `queued` JSONL has `resumed_from: <original-uuid>` and
    `resumed_from_run: <original-runid>`.
- [ ] Test: positional argument order matches the verified
      `codex exec resume [SESSION_ID] [PROMPT]` shape from
      `codex exec resume --help`.

## Related Tasks

- **TB-130** — parent epic.
- Depends on **TB-131** (`ModeResume`), **TB-132**
  (`ResumeCandidate`), **TB-134** (Codex `--json`), **TB-136** (Codex
  session capture), **TB-137** (`interrupted` status), **TB-138**
  (shared `ResumeAgent` body — Claude lands the service method
  first, this task adds the Codex branch).
- Blocks **TB-140** (frontend), **TB-141** (integration test
  exercises this path).

## Log

- 2026-05-14: Created
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done
