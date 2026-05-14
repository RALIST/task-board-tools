# TB-132: JSONL schema: SessionID/ResumedFrom/Cwd/RunEnv + ResumeCandidate helper

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb130,agent,resume,schema,jsonl
**Branch:** —
**Parent:** TB-130

## Goal

Extend the JSONL `Event` schema with the fields needed to capture and
later replay an agent session. Add the `ResumeCandidate` reader.
Pure schema + readers — no writer changes yet; downstream tasks
(TB-133, TB-135, TB-136) wire the writers.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 1 (event schema), § 4 (Run record), § 5 (resumable predicate).

Codex round-2 caught that one-field-per-env-var would not extend
forward; round-3 added an explicit `TB_`-prefix allowlist for the
`run_env` map so API tokens never land in JSONL log files. The
resumable predicate looks at the **latest run only** — no walking
backward (round-1 important 4).

## Acceptance Criteria

**Event schema (`gui/internal/agent/state.go`):**

- [ ] New `EventName` constant `EvSession EventName = "session"`.
- [ ] `Event` struct gains five fields with `omitempty` JSON tags:
      `SessionID string`, `ResumedFrom string`, `ResumedFromRun string`,
      `Cwd string`, `RunEnv map[string]string`.
- [ ] `AppendEvent` writer filters `RunEnv` to keys matching `^TB_`
      before serialising. Test asserts that `ANTHROPIC_API_KEY=x` in a
      caller-supplied env map does NOT land in the JSONL.

**Run record (`gui/app/agent_runs.go:25-36`):**

- [ ] `Run` struct gains `SessionID string`,
      `ResumedFrom string`, `ResumedFromRun string` with **lowerCamel**
      JSON tags (`sessionId`, `resumedFrom`, `resumedFromRun`) matching
      the existing convention (`runId`, `taskId`, `queuedAt`).

**Recovery view (`gui/app/agent_recovery.go:202-214`):**

- [ ] `runRecoveryView` gains `SessionID string`, `Cwd string`,
      `RunEnv map[string]string`.
- [ ] `readLatestRun` populates the new fields from the `EvSession`
      event for the latest run.

**Resumable predicate (new in `gui/app/agent_run.go` or
`agent_runs.go`):**

- [ ] New struct:
      ```go
      type ResumeCandidate struct {
          SessionID string
          RunID     string
          Cwd       string
          Env       map[string]string
      }
      ```
- [ ] New helper `func resumableSessionID(taskID, boardDir string)
      (ResumeCandidate, bool)`:
  - Reads the latest run only.
  - Returns `ok=false` if the latest run has no `EvSession` line
    (no walking backward).
- [ ] Test: latest run has session_id → ok=true.
- [ ] Test: latest run has no session_id, older run does → ok=false
      (anti-stale-conversation guarantee).
- [ ] Test: no runs at all → ok=false.

**Tests:**

- [ ] JSONL round-trip for every new field.
- [ ] `RunEnv` allowlist test (no `TB_` prefix → dropped).
- [ ] Existing `Event` tests still pass (no regression on existing
      fields).

## Related Tasks

- **TB-130** — parent epic.
- Blocks **TB-133** (writer hook needs the `EvSession` constant),
  **TB-137** (recovery reads `SessionID`), **TB-138** / **TB-139**
  (resume reads `ResumeCandidate`).

## Log

- 2026-05-14: Created
