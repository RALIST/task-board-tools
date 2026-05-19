# TB-130: Agent session resume + interrupted-run recovery

**Type:** feature
**Priority:** P1
**Size:** XL
**Module:** gui
**Tags:** agent,daemon,recovery,resume,epic
**Branch:** —

## Goal

When the GUI/daemon dies mid-run, today every `AgentStatus: running` task
is reconciled to `failed` (`gui/app/agent_recovery.go:154`) and the work
the agent already did is lost from our tracking — even though the agent
CLI's own session log on disk still has the full conversation transcript.

Capture the agent CLI's session id alongside our run id, add a new
`interrupted` AgentStatus that recovery sets when a captured session is
present, and let the user click **Resume** to spawn a continuation
(`claude -r <uuid>` / `codex exec --json resume <uuid> <prompt>`)
instead of starting from scratch with the original prompt.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
(v3.1, three rounds of Codex adversarial review — 0 BLOCKERs remaining
after round 3).

Key invariants the spec preserves:

- **Markdown is the source of truth.** AgentStatus widening to include
  `interrupted` follows the existing `cancelled` precedent — validator
  accepts it; convention "nothing manual writes interrupted" lives in
  code comments + docs, not in a validator boundary.
- **Single shared session-allocation point.** `runGoroutine` is the
  only writer; both `RunAgent` (manual) and `RunQueuedAgentSync`
  (daemon) converge there.
- **Session JSONL event always trails `started`.** PID is durable
  before the session id is recorded so recovery's `pidAlive` cross-check
  stays meaningful.
- **`run_env` filtered to `TB_`-prefixed keys only.** API tokens never
  land in JSONL log files.
- **Resume only from latest run.** `resumableSessionID` does NOT walk
  backward to older runs; if the latest run failed before capturing a
  session id, resume is disabled.

## Subtasks

- **TB-131** (S) — Closed-set schema sweep: `interrupted` status + `resume` mode
- **TB-132** (M) — JSONL schema additions + `ResumeCandidate` helper
- **TB-133** (S) — Shared post-`started` session-write hook in `runGoroutine`
- **TB-134** (M) — Codex `--json` switch + `codexJsonTranslator` + parity tests
- **TB-135** (S) — Claude session capture via `--session-id` pre-allocation
- **TB-136** (M) — Codex session capture via `--json` `OnSessionID` callback
- **TB-137** (M) — Recovery: dead-PID + SessionID → `interrupted` (`markInterrupted`)
- **TB-138** (M) — Resume backend Claude: `ResumeDecorator` + `-r` + cwd/env replay
- **TB-139** (M) — Resume backend Codex: `codex exec --json resume <uuid>` wiring
- **TB-140** (M) — Frontend: Resume button, `interrupted` pill, `resumed_from` chip
- **TB-141** (M) — Fake-runner integration test: kill → `interrupted` → resume cycle
- **TB-142** (S) — Docs sweep: `ARCHITECTURE.md` + `CLAUDE.md` + `FEATURES.md`

Build order (each builds on the previous; some can parallelize once
TB-133 and TB-134 land):

1. **TB-131** — Closed-set sweep. Foundation; blocks every site that
   reads/writes the new status or mode. Spec § 6, § 12 task A.
2. **TB-132** — JSONL schema (Event fields, `EvSession`,
   `ResumeCandidate`, `resumableSessionID`, TB_-prefix env filter).
   Pure schema + readers; no writers yet. Spec § 1, § 5, § 12 task B.
3. **TB-133** — Shared session-write hook in `runGoroutine`'s
   `OnStarted` callback (post-`started`, both manual + daemon paths).
   Spec § 3, § 12 task C.
4. **TB-134** — Codex `--json` switch + translator + `mapRunnerOutcome`
   parity tests. Hard prereq for any Codex session capture or resume.
   Spec § 12 task D.
5. **TB-135** — Claude UUID pre-alloc + `--session-id`. Depends on
   TB-132 + TB-133. Spec § 12 task E.
6. **TB-136** — Codex `OnSessionID` parsing + write. Depends on
   TB-133 + TB-134. Spec § 12 task F.
7. **TB-137** — `recoverOne` two-branch transition + `markInterrupted`.
   Depends on TB-131 + TB-132 + TB-135 (uses TB-133 transitively).
   Spec § 7, § 12 task G.
8. **TB-138** — `ResumeDecorator` + `ResumeAgent` + Claude `-r`
   wiring. Depends on TB-131 + TB-132 + TB-135 + TB-137. Spec § 8,
   § 9, § 12 task H.
9. **TB-139** — Codex resume invocation. Depends on TB-131 + TB-132 +
   TB-136 + TB-137. Spec § 12 task I.
10. **TB-140** — Frontend Resume UI. Depends on TB-131 + TB-137 +
    TB-138 + TB-139. Spec § 10, § 12 task J.
11. **TB-141** — Fake-runner integration test. Depends on TB-138 +
    TB-139 (can develop alongside TB-140 with stubbed UI). Spec § 12
    task K.
12. **TB-142** — Documentation sweep. Lands last as the final gate.
    Spec § 11, § 12 task L.

## Acceptance Criteria

- [x] All 12 sub-tasks (TB-131 through TB-142) merged.
- [x] **Fake-runner contract:** killing the fake runner mid-stream
      after a SessionID is captured leaves the task in `interrupted`,
      not `failed`.
- [x] **Fake-runner contract:** killing the fake runner mid-stream
      BEFORE a SessionID is captured leaves the task in `failed`
      (existing behaviour, unchanged).
- [x] **Fake-runner contract:** clicking Resume on an `interrupted`
      task spawns a new fake run with the expected resume flag
      (`-r <uuid>` for Claude, `resume <uuid>` for Codex), the
      expected `Cwd`, the expected `Env["TB_BOARD_PATH"]`, and the
      resume prompt body. The resumed run's `queued` JSONL event
      carries `resumed_from: <uuid>` and `resumed_from_run: <runid>`.
- [x] `RunAgent` and `ResumeAgent` produce visibly distinct entries in
      run history (different `mode`, `resumed_from` chip on Resume).
- [x] Cancelled carve-out unchanged: a user-cancelled task with a
      SessionID still becomes `cancelled` on recovery, never
      `interrupted`.
- [x] No regression: `worktrees.enabled: false` boards still work.
      Resume runs in the persisted `cwd` (verified via fake runner
      assertion); a future TB-114 board hands the worktree path
      through unchanged.
- [x] Documentation invariants in `CLAUDE.md`, `cli/CLAUDE.md`, and
      `docs/ARCHITECTURE.md` reflect `interrupted`, the resume flow,
      the post-`started` session-write rule, and the `TB_`-prefix env
      allowlist.
- [x] `run_env` JSONL never contains a key without a `TB_` prefix
      (asserted by TB-132 and TB-141).

## Related Tasks

- **TB-5** — M5 agent-daemon epic; this extends recovery beyond the
  M5 stale-failed pattern.
- **TB-114** — Worktree-isolated agent execution. The `cwd` and
  `TB_BOARD_PATH` resume relies on come from TB-114 when worktree
  mode is on; resume works regardless of TB-114 landing first or
  second (persisted cwd is the worktree path or the repo root).

## Log

- 2026-05-14: Created. Spec finalized after 3 rounds of Codex
  adversarial review (1 BLOCKER closed in v2, 1 BLOCKER closed in v3,
  4 IMPORTANTs and several NITs closed in v3.1).
- 2026-05-19: Done
