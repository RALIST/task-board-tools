# TB-141: Fake-runner integration test: kill->interrupted->resume cycle

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb130,agent,resume,testing,integration
**Branch:** —
**Parent:** TB-130

## Goal

End-to-end fake-runner integration test that exercises the full
queue → start → kill-mid-stream → daemon-restart → observe-
`interrupted` → resume → observe-continuation cycle. Two scenarios:
Claude (pre-allocated UUID) and Codex (stream-emitted UUID). CI-
stable — no real `claude` or `codex` binaries.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ "Acceptance criteria for the epic" + § "Manual smoke" (live binaries
NOT in CI per Codex round-2 important 7).

This task is the gate that proves the whole feature works end-to-end
under the existing daemon test harness. Live-binary tests stay as a
manual smoke checklist on the parent epic — they're not gating CI.

## Acceptance Criteria

- [ ] New test file `gui/app/agent_resume_integration_test.go`
      (or extend an existing test file) covering both scenarios.
- [ ] Helper: a fake Claude runner that:
  - Captures the `--session-id` arg passed in.
  - Emits a stub `started` callback with a fake PID.
  - Hangs (or emits one stdout line then hangs) until killed via
    its `RunInput.Cancel` / context cancellation.
- [ ] Helper: a fake Codex runner that:
  - Captures the argv (asserts `--json` for fresh, `resume <uuid>`
    for resume).
  - Emits a synthetic `--json` event carrying a session id, then
    hangs until killed.
- [ ] **Scenario A — Claude:**
  1. Queue a task with `Agent: claude`, kick off a run.
  2. Wait for the JSONL `session` event to land (post-`started`
     hook from TB-133/TB-135).
  3. Kill the runner mid-stream (simulate daemon shutdown).
  4. Run `RecoverStale` against the same boardDir.
  5. Assert `AgentStatus: interrupted`.
  6. Assert latest JSONL event is `finished{interrupted, reason:
     "interrupted by daemon restart"}`.
  7. Call `ResumeAgent(taskID)`.
  8. Assert the fake runner was invoked with `-r <captured-uuid>`,
     `cwd = <persisted-cwd>`, `env["TB_BOARD_PATH"] = <persisted>`.
  9. Assert the resumed run's `queued` JSONL has `resumed_from` and
     `resumed_from_run`.
- [ ] **Scenario B — Codex:** mirror of Scenario A with the Codex
      fake runner; the resumed argv is
      `["exec", "--json", "resume", "<uuid>", "<prompt>"]`; the
      resumed stream emits a NEW session id and the JSONL gains a
      fresh `session` event with that new id.
- [ ] **Negative test:** kill the runner BEFORE any session id is
      captured (Claude case: race the kill against `OnStarted`;
      Codex case: kill before the fake runner emits the session
      event). After `RecoverStale`, status is `failed` (not
      `interrupted`); Resume button predicate (`resumableSessionID`)
      returns ok=false.
- [ ] **Cancelled-carve-out test:** user cancels the run via
      `CancelRun`, kill the daemon, `RecoverStale` runs. Status
      stays `cancelled` (not `interrupted`).
- [ ] **Allowlist test:** caller passes `Env: {"TB_BOARD_PATH":
      "/x", "ANTHROPIC_API_KEY": "secret"}`. JSONL `run_env` has
      ONLY `TB_BOARD_PATH`, never the secret.

## Related Tasks

- **TB-130** — parent epic.
- Depends on **TB-138** + **TB-139** (the resume backends being
  tested). Can develop alongside **TB-140** (frontend) using
  service-only assertions.

## Log

- 2026-05-14: Created
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done
