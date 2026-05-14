# Agent session resume & interrupted-run recovery — design

Status: draft v3.1 (post third Codex review — 0 BLOCKERs remaining)
Owner: TB
Date: 2026-05-14

Changelog:
- v3.1: addressed Codex round-3 — added `TB_`-prefix env allowlist for
  `run_env` persistence (security: don't write API keys to JSONL);
  switched `resumableSessionID` to a `ResumeCandidate` struct return;
  dropped pre-creating sibling follow-up tasks (TB-Y1..Y5 stay as
  documented follow-ups, created only when needed); cited the real
  `NewGroomingDecorator` location at `gui/internal/agent/runner.go:180`
  (Codex round-3 thought it was new — it isn't); cross-referenced
  `exec.go` env-build site for TB-B implementer.
- v3: addressed Codex round-2 — dropped the public/internal validator
  split (use the `cancelled` precedent: invariant lives in code+comment,
  not in the validator); switched env persistence from one top-level
  field per var to a `run_env` map matching `RunInput.Env`; expanded
  TB-A's enumerated sites (agent_finish.go, stores/runs.ts,
  TaskDrawer.svelte); added `resumed_from_run` to support the
  frontend's source-RunID chip; specified resume mode's prompt-rendering
  path (decorator + `PromptResume` constant); promoted "follow-up:
  resume from finished + expiry UX" to a concrete sibling-task
  reference; added explicit "no direct TB-C dep on TB-G" note.
- v2: addressed Codex round-1 review — daemon path coverage, post-PID
  session ordering, cwd/env persistence for TB-114 compatibility, JSON
  tag camelCase, closed-set blast radius, Codex `--json` parity gate,
  resume-only-latest predicate, finished-run resume cut, expiry-UX
  scoped out, fake-runner ACs, build order reshuffled.

## Problem

When the GUI/daemon dies mid-run (user closes the app, crash, OOM, machine
sleep), every `AgentStatus: running` task is currently reconciled by
`gui/app/agent_recovery.go:RecoverStale` to `failed` with reason
"stale after restart". The work the agent did up to that point is lost from
*our* tracking — even though the agent CLI's own session log on disk still
has the full conversation transcript.

We never capture the agent's session id, never persist it, and never use it.
Re-running the task starts from zero with the original prompt, throwing away
context the agent already built.

## Goal

Persist agent session ids alongside our own run ids, and let an explicit
user "Resume" action continue the previous session by id with a short
continuation prompt — instead of starting a fresh agent run.

Two distinct user flows must be supported, and they MUST stay distinct:

1. **Re-run** (existing behavior): start a fresh session with the rendered
   prompt. New session id, new run id, no continuation.
2. **Resume** (new): continue the previous session by id, with a short
   continuation prompt. Same session id (Claude) or a successor id linked
   via `resumed_from` (Codex), new run id.

## Non-goals

- **Auto-resume.** Recovery never silently resumes — a user closed the
  app deliberately; resuming an `rm -rf` mid-flight is bad. Recovery
  surfaces the option (`interrupted` status); user clicks Resume.
- **Hot reattachment.** If the subprocess is somehow alive when recovery
  runs, current behavior wins (skip and leave alone). Resume always means
  spawning a NEW agent process pointed at the saved session id.
- **Resume from finished runs (M1).** Resume is offered only for tasks
  in `interrupted` status. Resuming a `success`/`failed`/`cancelled`
  task to ask a follow-up is a follow-up epic, not this one.
- **Cross-machine session portability.** Both Claude and Codex store
  sessions on the local filesystem (`~/.claude/projects/<cwd-hash>/<uuid>.jsonl`
  and `~/.codex/sessions/`). Resume only works on the host that
  originated the session.
- **Multi-turn UI for arbitrary continuation prompts.** M1 uses a fixed
  short continuation prompt (one to two sentences — see § 9). A
  free-form prompt input is a follow-up.
- **Session expiry / version-drift UX.** Detecting "session expired"
  from a non-zero resume invocation and offering a "start fresh?" path
  is explicitly scoped OUT of this epic. Tracked as a follow-up; until
  then, expired-session resume looks like any other `failed` run.

## Verified CLI surface

`claude -p` (the headless flag we already use) supports:

- `--session-id <uuid>` — **pre-allocate** the session id from the caller.
  Lets us generate the id ourselves in our daemon and write it to
  JSONL alongside the PID, so a crash mid-run still leaves a usable id.
- `-r, --resume <session-id>` — non-interactive resume by id.
- `--fork-session` — when resuming, create a new id instead of reusing
  the original. NOT used in M1; revisit if transcript size or audit
  immutability becomes a real complaint.

`codex exec` supports:

- `codex exec resume <SESSION_ID> [PROMPT]` — non-interactive resume by id.
- `codex exec resume --last [PROMPT]` — resume newest in cwd.
- `codex exec --json` — emit JSONL events to stdout. **Codex does NOT
  accept a pre-allocated session id**; we must parse the id from this
  stream. Today we run `codex exec` *without* `--json`, so we have nothing
  to parse. Switching is required and requires parity tests with
  `mapRunnerOutcome` (`gui/app/agent_run.go:523-541`).

The asymmetry — Claude lets us pre-allocate, Codex does not — drives the
schema and ordering rules in §§ 1–3.

## Design

### 1. JSONL schema additions

Add three fields and one new event to `gui/internal/agent/state.go` Event:

```go
type Event struct {
    // ... existing fields ...
    SessionID      string            `json:"session_id,omitempty"`       // agent-side conversation id
    ResumedFrom    string            `json:"resumed_from,omitempty"`     // session id this run was resumed from (set on `queued`)
    ResumedFromRun string            `json:"resumed_from_run,omitempty"` // run id this run was resumed from (set on `queued`; UI chip)
    Cwd            string            `json:"cwd,omitempty"`              // absolute cwd captured at session-write time
    RunEnv         map[string]string `json:"run_env,omitempty"`          // env-var overrides captured at session-write time
    Event          EventName // existing; add EvSession
}

const EvSession EventName = "session"
```

> Codex round-2 important: `RunInput.Env` (`gui/internal/agent/runner.go:77`)
> is already a map; persist it as a map too. Adding one top-level field
> per env var would not extend forward when TB-114 (or future epics) add
> more keys. The map is the smallest forward-compatible shape.

> **Security allowlist (Codex round-3 important):** `run_env` MUST
> persist ONLY keys with the `TB_` prefix. The agent process inherits
> the daemon's full environment via `runExternal` (see
> `gui/internal/agent/exec.go` env-build path), which can include
> `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `OAUTH_*`, and other
> credentials. JSONL log files live on disk unencrypted; persisting
> credential env vars would be a real leak. The TB-B writer filters
> `RunInput.Env` to keys matching `^TB_` before serialising. Tests
> assert that no key without that prefix lands in the JSONL.

New event:

```
session {ts, run_id, task_id, event:"session", session_id, pid, cwd, run_env}
```

Emitted **exactly once per run, AFTER the `started` event has been
appended** (i.e. PID is known and durable). Same callback, two writes:
`started` first, `session` second. Cwd and TB_BOARD_PATH are persisted
here so resume can replay the exact execution context (TB-114 worktree
path included).

`resumed_from` is set on the `queued` event when a run was kicked off via
the resume path, not the fresh path. Pure metadata for the GUI's run
history; recovery doesn't read it.

**Why post-`started` instead of pre-fork:**

> First Codex round-1 blocker. Writing `session` before `started` would
> leave a `running + session_id + no PID` state on a between-fork-and-
> OnStarted crash. Recovery's `pidAlive` cross-check needs the PID.
> Putting `session` AFTER `started` keeps the recovery contract intact:
> if there is no `started` event, there is no `session` event either,
> and the task falls through to the existing `failed` branch.

### 2. Capture flow per agent

#### Claude (pre-allocated path)

1. `runGoroutine` (single shared entry point — see § 3) generates a
   UUIDv4 BEFORE invoking the runner, stores it on `activeRun.SessionID`.
2. Passes UUID to the runner via `RunInput.SessionID`. Claude runner
   appends `--session-id <uuid>` to its args.
3. Inside `OnStarted` callback (after `cmd.Start` succeeds → PID known),
   the existing JSONL `started` write happens, then a new
   `session{session_id, pid, cwd, run_env}` write happens
   immediately after, holding the same `taskMutex` ordering guarantees.
4. The translator's existing rendering of `session id: …` to the human
   log stays — but the JSONL is now the source of truth.

#### Codex (parsed path)

1. Switch invocation to `codex exec --json <prompt>` (fresh) or
   `codex exec --json resume <session_id> <prompt>` (resume).
2. Add `codexJsonTranslator` mirroring `claude_stream.go`'s
   `claudeTranslator`: parse each JSONL stdout event, render
   human-readable lines into the underlying writer (current behavior of
   plain `codex exec` output, preserved as closely as possible).
3. When the first event carrying `session_id` arrives, the translator
   calls a new `RunInput.OnSessionID(string)` callback (provided by
   `runGoroutine`) — which appends `session{session_id, pid (already
   known from OnStarted), cwd, run_env}` to the JSONL.
4. If Codex never emits a session id (degraded `--json` schema, network
   error pre-init), the run still works — it just isn't resumable. The
   recovery branch in § 7 keeps it on the `failed` path; the UI never
   surfaces a Resume button.

### 3. Single shared session-allocation point (covers daemon path)

> Codex round-1 blocker 1.

Both `RunAgent` (`startAgentRun`) and `RunQueuedAgentSync` already
converge in `runGoroutine` (`gui/app/agent_run.go:366-460`). Session
allocation MUST live there, not in the entry-point functions.

Concretely, at the top of `runGoroutine` (after the cancelled-check,
before opening `logWriter`):

```go
ar.SessionID = uuidv4()  // for Claude; Codex stays empty here
```

And in the `OnStarted` callback, after the `started` event is appended,
add a `session` event with the ar.SessionID (Claude) or — for Codex —
register an `OnSessionID` closure that performs the same write when the
stream supplies the id.

Both `startAgentRun` and `RunQueuedAgentSync` exercise the same
`runGoroutine`, so session capture is automatically covered for both
the manual UI flow and the daemon's queued-task pickup. No new wiring
in `startAgentRun` or `RunQueuedAgentSync` is required.

### 4. Run record

`Run` in `gui/app/agent_runs.go` gains:

```go
SessionID   string `json:"sessionId"`
ResumedFrom string `json:"resumedFrom,omitempty"`
```

> Codex round-1 important 1: existing `Run` JSON tags use lower-camel
> (`runId`, `taskId`, `queuedAt`); new fields match.

`runRecoveryView` in `agent_recovery.go` gains `SessionID string` and the
JSONL reader populates it from the `EvSession` line for the latest run.

### 5. Resumable predicate (latest-run only)

> Codex round-1 important 4.

A "task is resumable" predicate exists in one place:

```go
type ResumeCandidate struct {
    SessionID string
    RunID     string            // surfaced as the `resumed_from` UI chip
    Cwd       string
    Env       map[string]string // TB_-prefixed keys only (see § 1)
}

func resumableSessionID(taskID, boardDir string) (ResumeCandidate, bool)
```

> Codex round-3 NIT: a 5-tuple bare return is over the Go style
> threshold. A named struct + bool is the canonical shape.

Looks at the **latest run only**. If the latest run has no `session`
event, returns `ok=false` and resume is disabled — the helper does NOT
walk backward to older runs. This avoids resuming a stale conversation
when a more recent attempt failed too early to capture the id. If the latest run has no
`session` event (no SessionID captured), the helper returns `ok=false`
and resume is disabled — the helper does NOT walk backward to older
runs. This avoids resuming a stale conversation when a more recent
attempt failed too early to capture the id.

Used by both UI ("show Resume button?") and recovery ("transition to
interrupted instead of failed?").

### 6. New AgentStatus value: `interrupted`

> Codex round-1 important 2: closed-set blast radius is wider than
> "just add the constant". Treat the schema update as one task that
> sweeps every site.

Add `interrupted` (status) and `resume` (mode) to every enumerated
closed set. Codex round-2 found the v2 list was incomplete:

**Status sites:**

- `cli/task.go` `validAgentStatuses` (line 35-41 enum).
- `cli/edit.go` `--agent-status` validator help text (line 84-88).
- `cli/main.go:88` top-level help string.
- `gui/internal/agent/state.go` `Status` constants (line 35-39):
  add `StatusInterrupted Status = "interrupted"`.
- `gui/frontend/src/lib/api.ts` AgentStatus type union.
- Documentation: `cli/CLAUDE.md`, `docs/ARCHITECTURE.md`, `CLAUDE.md`
  invariant line.

**Mode sites:** (Codex round-2 important 6 — three sites, not one)

- `gui/app/agent_finish.go:91-97` `parseRunMode` — currently collapses
  unknown modes to `implement`; must round-trip `resume`.
- `gui/frontend/src/lib/stores/runs.ts:201` — the frontend mode
  normalizer (NOT `api.ts`).
- `gui/frontend/src/lib/components/TaskDrawer.svelte:287, 315` —
  hardcoded optimistic `implement` / `groom` rows; add an optimistic
  `resume` branch for the Resume button.

All sites enumerated in TB-A's task body so nothing slips through.

Semantics, in lockstep with the existing carve-outs:

- Set by `RecoverStale` ONLY when:
  - latest run has no `finished` event,
  - PID is dead per `pidAlive`,
  - **and** a SessionID is recorded for that run (via § 5 predicate).
- If SessionID is missing → keep current behavior (`failed`, reason
  "stale after restart"). Resume isn't possible anyway.
- A user action `ResumeAgent` transitions `interrupted → queued → running`
  via the same path as `RunAgent`, just with the resume flag set.
- `interrupted` is OS-recovery-initiated, mirroring how `cancelled` is
  user-initiated. The anti-overwrite invariant follows the existing
  `cancelled` precedent: the validator allows the value (so recovery
  can write it via `c.Edit` / `tb edit --agent-status` like every
  other status), and the convention "nothing manual writes
  `interrupted`" lives in a code comment + docs/ARCHITECTURE.md, NOT
  in the validator. This matches how `cancelled` is enforced today
  (`cli/task.go:33-41` — the comment says "M5 stale-recovery must
  never overwrite it"; the validator does not encode that).
  > Codex round-2 blocker: v2's "public/internal validator split" was
  > hand-waved — `Client.Edit` shells out to public `tb edit`, which
  > validates against `validAgentStatuses`. Splitting would require a
  > new internal CLI flag and a wider GUI change. The `cancelled`
  > precedent is simpler, lands in TB-A's same one-line allowlist
  > addition, and trusts the same convention the codebase already
  > trusts.
- A `ResumeAgent` call is REJECTED if AgentStatus ∈ {queued, running} —
  same gate as `RunAgent`. Resume is REJECTED if AgentStatus ≠
  `interrupted` (M1 scope; resume-from-finished is documented as a
  follow-up in § 13, not implemented here).

### 7. Recovery transition

`recoverOne` in `agent_recovery.go` line 154 changes from one branch to two:

```go
// dead PID branch
if latest.SessionID != "" {
    return r.markInterrupted(ctx, c, boardDir, t, latest.RunID,
        "interrupted by daemon restart")
}
return r.markFailed(ctx, c, boardDir, t, latest.RunID, "stale after restart")
```

`markInterrupted` is a new helper: appends `finished{interrupted}` JSONL
(yes, `finished` with status=`interrupted` — Status enum gains
`StatusInterrupted`), edits `--agent-status interrupted` via the same
`c.Edit(...)` path `markFailed` uses (the validator now accepts
`interrupted` per § 6 — convention prevents misuse, not the validator),
emits `agent:run-finished{status:"interrupted"}` Wails event.

The TB-61 cancelled carve-out still wins: latest event is
`finished{cancelled}` → reconcile to `cancelled`, never `interrupted`.

### 8. Resume API

New service method on `AgentService`:

```go
func (s *AgentService) ResumeAgent(ctx context.Context, taskID string) (string, error)
```

- Reads the resumable session id, cwd, and board path via § 5
  predicate. If `ok=false` → return `ErrNotResumable`.
- Validates `AgentStatus == "interrupted"` (M1 scope). Returns
  `ErrCannotResume` otherwise.
- Otherwise mirrors `startAgentRun` step-for-step except:
  - `RunInput.ResumeFromSessionID = <uuid>`.
  - `RunInput.Cwd = <persisted cwd>` (overrides `c.Cwd()` so resume
    runs in the original execution directory — critical for TB-114
    worktrees AND for Claude's cwd-keyed session lookup).
  - `RunInput.Env = {"TB_BOARD_PATH": <persisted board path>}` if set.
  - `RunInput.Prompt` becomes the **resume prompt** (template body
    lives at `gui/internal/agent/prompts/resume.md`). See § 9.
  - `queued` JSONL event includes `resumed_from: <uuid>`.
  - For Claude: `-r <uuid>` is appended (NOT `--session-id`); the
    same UUID stays the session id (no `--fork-session` in M1).
  - For Codex: invocation becomes
    `codex exec --json resume <uuid> <prompt>`.
    A *new* session id will appear in the `--json` stream; capture it as
    a fresh `session` event. The new id is what future resumes use.

A new `Mode` is introduced: `agent.ModeResume`. Distinct from
`ModeImplement` so prompt rendering, JSONL `mode` field, and frontend
filtering all stay clean. All three mode-normalizer sites enumerated in
§ 6 must handle `resume` (Codex round-2 important 6).

**Prompt rendering for resume mode** (Codex round-2 important 10):

`runGoroutine` currently always renders `PromptImplement`
(`gui/app/agent_run.go:393`); grooming uses a runner decorator that
overrides the prompt (`runnerForMode` at `gui/app/agent_run.go:47`,
implementation `groomingDecorator` at
`gui/internal/agent/runner.go:180-200`, exposed via
`NewGroomingDecorator`). Resume follows the same decorator pattern for
symmetry:

- Add `agent.PromptResume` constant pointing at
  `gui/internal/agent/prompts/resume.md`.
- Add `agent.ResumeDecorator` (mirror of `GroomingDecorator`) that
  overrides `RunInput.Prompt` with the rendered resume template.
- `runnerForMode` switch gains a third branch:
  ```go
  case agent.ModeResume:
      return agent.NewResumeDecorator(runner, promptVarsFromDetail(detail))
  ```
- Resume's prompt template uses NO template variables in M1 (the
  resumed conversation already has the original task context). The
  template body is the fixed string from § 9.

This keeps `runGoroutine` unchanged — every mode goes through
`runnerForMode → decorator → runner` uniformly.

### 9. Continuation prompt

> Codex round-1 verdict on prompt template.

Use a short fixed continuation prompt — neither empty (undefined CLI
behaviour) nor the full original task prompt (risks restarting from
scratch). One to two sentences:

> The previous run was interrupted before completion. Inspect the
> current worktree and board state, then continue from where you left
> off. Do not restart the task.

Lives at `gui/internal/agent/prompts/resume.md`. No template variables
needed in M1 — the resumed conversation already has the original task
context.

### 10. Frontend

- `gui/frontend/src/lib/api.ts`: export `ResumeAgent(id)`. AgentStatus
  type union gains `"interrupted"`. Mode normalizer gains `"resume"`.
- `Card.svelte`: when `metadata.agentStatus === "interrupted"`, render a
  Resume icon button next to the existing run controls.
- `TaskDrawer.svelte`: same Resume button in the action row, ONLY for
  `interrupted` status (M1 scope; finished-run resume was cut).
  Tooltip explains "Resume continues the previous agent session" vs
  "Run starts a fresh conversation".
- Run history rows show a `resumed_from: r_xxxx` chip on resumed runs
  (the SessionID is internal — surface the source RunID in the UI).
- Status pill colours: add a neutral-warm tone for `interrupted`
  (distinct from `failed`'s red and `cancelled`'s grey).

### 11. Documentation

- `docs/ARCHITECTURE.md` "Agent state" section gains:
  - Session id capture flow per agent.
  - `EvSession` event in the JSONL schema table, including ordering
    rule (always after `started`).
  - `interrupted` status with the same invariants as `cancelled`.
  - Resume vs re-run user model.
- `cli/CLAUDE.md` AgentStatus enum line: append `| interrupted`.
- `CLAUDE.md` "Architecture invariants" → AgentStatus values: same.
- `docs/FEATURES.md`: this epic added under M5 (extends crash recovery)
  or as M5.5; TBD with maintainer.

## Build order — revised after Codex round 1

> Replaces the original 9-task list. Closed-set schema first, shared
> hook second, Codex `--json` parity third. Resume-side and frontend
> work last and can parallelize.

1. **TB-A** (S, cli + gui + frontend, ~0.5d) — Closed-set schema sweep
   for `interrupted` (status) and `resume` (mode) at every enumerated
   site in § 6:
   - Status: `cli/task.go:35-41`, `cli/edit.go:84-88`,
     `cli/main.go:88` help, `gui/internal/agent/state.go:35-39`
     (`StatusInterrupted`), `gui/frontend/src/lib/api.ts` AgentStatus
     union.
   - Mode: `gui/app/agent_finish.go:91-97` `parseRunMode`,
     `gui/frontend/src/lib/stores/runs.ts:201`,
     `gui/frontend/src/lib/components/TaskDrawer.svelte:287, 315`.
   - `interrupted` is added to `validAgentStatuses` like `cancelled`
     is — convention prevents misuse, not the validator (see § 6).
   - Documentation invariant lines: `cli/CLAUDE.md`,
     `docs/ARCHITECTURE.md`, `CLAUDE.md`.
   No behaviour change yet — only closed sets widen. Tests cover
   round-trip of every site touched.

2. **TB-B** (M, gui, ~0.5d) — JSONL schema additions:
   `EvSession` constant, `SessionID / ResumedFrom / ResumedFromRun /
   Cwd / RunEnv` Event fields, `Run.SessionID / ResumedFrom /
   ResumedFromRun` (camelCase tags), reader populates
   `runRecoveryView.SessionID / Cwd / RunEnv`, `resumableSessionID`
   helper returning `(sessionID, runID, cwd, env, ok)` from the LATEST
   run only. Pure schema + readers; no writer change yet. Tests cover
   JSONL round-trip + helper, including "latest run has no session id
   → ok=false" guarantee.

3. **TB-C** (S, gui, ~0.5d) — Shared post-`started` session-write hook
   in `runGoroutine`'s OnStarted callback. Writes `session{session_id,
   pid, cwd, tb_board_path}` immediately after `started`. Only fires
   if `ar.SessionID != ""` (Claude case for now; Codex remains empty
   until TB-F lands its OnSessionID callback). Tests cover both manual
   (`startAgentRun`) and daemon (`RunQueuedAgentSync`) entry paths,
   verifying the session event lands on both.

4. **TB-D** (M, gui, ~1d) — Codex `codex exec --json` switch +
   `codexJsonTranslator` (mirrors `claudeTranslator`). Parity tests
   with `mapRunnerOutcome`: zero-exit/non-zero-exit/timeout/binary-not-
   found all behave identically to the pre-`--json` invocation. Hard
   prereq for TB-F.

5. **TB-E** (S, gui, ~0.5d) — Claude session capture: pre-allocate
   UUIDv4 in `runGoroutine`, plumb via `RunInput.SessionID`, runner
   appends `--session-id <uuid>`. Smoke test: spawn Claude (real
   binary, behind a build tag), kill it after the first stream-json
   event, confirm JSONL has `session_id` matching the value passed via
   `--session-id`. Depends on TB-B + TB-C.

6. **TB-F** (M, gui, ~1d) — Codex session capture: parse `session_id`
   from `--json` stream, fire `OnSessionID` callback, callback writes
   `session` event using PID already known from OnStarted. Depends on
   TB-C + TB-D.

7. **TB-G** (M, gui, ~1d) — Recovery `interrupted` transition:
   `recoverOne` two-branch split (SessionID → markInterrupted; no
   SessionID → existing markFailed). `markInterrupted` helper,
   `StatusInterrupted` enum value already added in TB-A. Tests cover
   every branch + cancelled carve-out (must still win) + worktree path
   (cwd persisted in session event used as the cwd for any future
   resume — verify the field round-trips). Depends on TB-A + TB-B +
   TB-E.

8. **TB-H** (M, gui, ~1d) — Resume backend Claude:
   `agent.ModeResume`, `agent.PromptResume`, `agent.ResumeDecorator`
   (mirror of `GroomingDecorator`), `prompts/resume.md`,
   `ResumeAgent` service method, Claude runner appends `-r <uuid>`,
   `RunInput.Cwd` / `RunInput.Env` plumbed end-to-end through
   `runExternal`'s existing `cmd.Dir` / env append (`exec.go:43`),
   `queued` JSONL gets `resumed_from` AND `resumed_from_run`.
   `runnerForMode` switch gains the `ModeResume` branch. Tests use
   a fake Claude runner that asserts expected args + cwd + env +
   prompt body. Depends on TB-A + TB-B + TB-E + TB-G.

9. **TB-I** (M, gui, ~1d) — Resume backend Codex: same as TB-H but
   for Codex (`codex exec --json resume <uuid> <prompt>`). New session
   id from the resumed stream is captured via the existing TB-F
   pipeline; `resumed_from` on the queued event tracks the parent
   session. Depends on TB-A + TB-B + TB-F + TB-G.

10. **TB-J** (M, gui+frontend, ~1d) — Frontend resume UI:
    `ResumeAgent` API binding, Resume button on Card and TaskDrawer
    (only for `interrupted` status in M1), `interrupted` status pill,
    `resumed_from` chip in run history. Depends on TB-A + TB-G + TB-H +
    TB-I.

11. **TB-K** (M, gui+test, ~1d) — Fake-runner integration tests:
    full queue→start→kill-mid-stream→restart→observe-`interrupted`→
    resume→observe-continuation cycle. Two scenarios: Claude fake
    runner with pre-allocated UUID, Codex fake runner with stream-emit
    UUID. CI-stable (no real Claude/Codex binaries). Depends on TB-H +
    TB-I (can develop in parallel with TB-J using stubs for the UI).

12. **TB-L** (S, docs, ~0.5d) — Documentation sweep: ARCHITECTURE.md,
    CLAUDE.md, FEATURES.md, the resume.md prompt template review, plus
    a one-line user-facing entry in any release notes. Depends on
    everything else landing.

Total: 12 sub-tasks, ~9 ideal-developer-days. Most are S/M slices to
match the codebase's history of 1-day-per-task units (Codex round-1
right-sizing verdict).

> Note (Codex round-2): TB-G's listed deps (A + B + E) make TB-C a
> *transitive* dep via TB-E. That's intentional — explicit listing
> would only be a readability nit, not a DAG fix.

## 13. Documented follow-ups (NOT created as tasks at epic-creation time)

> Codex round-3 NIT: pre-creating sibling follow-ups is unconventional
> for this repo (board CONVENTIONS.md says "create backlog tasks when
> you encounter out-of-scope work", not prospectively). These stay as
> documented intent here; they get real IDs only when someone is ready
> to pick them up.

- **Resume from finished runs.** Lifts the M1 restriction that
  `ResumeAgent` rejects unless AgentStatus == `interrupted`. Drives
  the UX where a `failed` run from session expiry can still be retried
  with continuity.
- **Session expiry detection + "session expired; start fresh?" UX.**
  Detects the failure post-launch (resume command exits non-zero
  quickly with a stderr signature), surfaces a one-click "Start fresh
  instead" path.
- **Free-form continuation prompt input.** Lets the user type a custom
  continuation message instead of the fixed template.
- **`--fork-session` toggle.** For immutable original sessions
  (audit/transcript-size knob).
- **Auto-resume policy for power users.** An opt-in setting that turns
  `interrupted → resume` into a single recovery step on daemon
  restart.

## Acceptance criteria for the epic

> Codex round-1 important 7: every AC must be testable with fake
> runners. Live-agent runs are a manual smoke checklist appended at
> the end, NOT acceptance criteria.

- [ ] All 12 sub-tasks (A–L) merged.
- [ ] **Fake-runner contract:** killing the fake runner mid-stream after
      a SessionID is captured leaves the task in `interrupted` status,
      not `failed`.
- [ ] **Fake-runner contract:** killing the fake runner mid-stream
      BEFORE a SessionID is captured leaves the task in `failed`
      status (existing behaviour, unchanged).
- [ ] **Fake-runner contract:** clicking Resume on an `interrupted`
      task spawns a new fake run that receives the expected resume
      flag (`-r <uuid>` for Claude, `resume <uuid>` for Codex), the
      expected `Cwd`, the expected `Env["TB_BOARD_PATH"]`, and the
      resume prompt body. The resumed run's `queued` JSONL event
      carries `resumed_from: <uuid>`.
- [ ] **Closed-set sweep:** `tb edit --agent-status interrupted` is
      rejected (validator allowlist); `RecoverStale` writes
      `interrupted` via the internal write path.
- [ ] `RunAgent` and `ResumeAgent` produce visibly distinct entries in
      run history (different `mode`, `resumed_from` chip on Resume).
- [ ] Cancelled carve-out unchanged: a user-cancelled task with a
      SessionID still becomes `cancelled` on recovery, never
      `interrupted`.
- [ ] No regression: `worktrees.enabled: false` boards still work.
      Resume in a worktree-enabled board (when TB-114 lands) launches
      the resume command in the persisted cwd — verified via fake
      runner cwd assertion.
- [ ] Documentation invariants in `CLAUDE.md` and
      `docs/ARCHITECTURE.md` reflect `interrupted`, the resume flow,
      and the post-`started` session-write rule.

**Manual smoke (NOT an AC — a checklist for the maintainer):**

1. Real Claude run, kill daemon mid-run, restart, click Resume,
   confirm the session continues from the last assistant turn.
2. Same for Codex.
3. Kill mid-run before any session_id arrives → `failed`, no Resume
   button.
4. Cancelled carve-out: `tb edit X --agent-status cancelled` while
   running, kill daemon, restart, status remains `cancelled`.

## Risks and open questions (post round-1)

- **Codex `--json` parity is the highest-risk change.** It modifies an
  already-working code path. TB-D's parity tests are the gate; if any
  failure mode (timeout, binary-not-found, non-zero exit) maps
  differently we cannot proceed. Mitigation: TB-D is its own task, has
  to land green before TB-F/I depend on it.
- **Pre-allocated UUID validity.** `claude --session-id` requires a
  valid UUID. Add a `crypto/rand`-based UUIDv4 helper in the `agent`
  package; do NOT reuse `GenerateRunID` (32-bit hex, not UUID-shaped).
- **Worktree cwd interaction.** Persisted in the `session` event, so
  resume replays the exact cwd used at `started` time. Works regardless
  of TB-114 landing first or second. If TB-114 lands first, persisted
  cwd will be the worktree path; if it lands later, persisted cwd is
  the repo root and resume continues to work.
- **Session id GC by the agent CLI.** Both tools may eventually
  garbage-collect sessions. M1 punts to follow-up: expired-session
  resume looks like any other `failed` run. Documented in non-goals
  AND in the sub-task L docs sweep.
- **Concurrency with `taskMutex`.** `started` and `session` are written
  back-to-back inside `OnStarted`. Both go through `AppendEvent`, which
  takes the per-task mutex per call. The two writes are not atomic
  together — a crash between them leaves `started` with no `session`,
  same as if SessionID had never been allocated. That is an acceptable
  state: recovery sees no SessionID → `failed`. No new invariant
  violated.
- **Daemon-only `tb edit` path for `interrupted`.** `markFailed`
  already calls `c.Edit(ctx, …, AgentStatus: "failed")` via the CLI.
  Adding `interrupted` to the validator allowlist would let any caller
  set it manually, breaking the "recovery-only" invariant. The fix is
  to either (a) split the validator: a public allowlist for `tb edit`
  that excludes `interrupted`, and an internal allowlist used by
  recovery; or (b) keep the CLI validator open and document the
  invariant in code/comments. Decision: (a) — explicit is better than
  trust-based for status invariants. TB-A includes the validator split.
