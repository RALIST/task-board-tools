# TB-131: Closed-set schema sweep: interrupted status + resume mode

**Type:** feature
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** epic-tb130,agent,resume,schema
**Branch:** —
**Parent:** TB-130

## Goal

Widen every closed enum that participates in the agent lifecycle to
include the new `interrupted` AgentStatus and the new `resume` Mode.
Foundation task; no behaviour change yet — only the closed sets
widen so downstream tasks have somewhere to write the new values.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 6 (status enumeration sites) and § 8 (mode enumeration sites).
Codex round-2 found that v1's "just add the constant" framing missed
three frontend sites; this task lists every one explicitly.

`interrupted` is added to `validAgentStatuses` like every other status
(including `cancelled`). The convention "nothing manual writes
`interrupted`" lives in a code comment + `docs/ARCHITECTURE.md`, NOT
in a validator boundary — this matches how `cancelled` is enforced
today (see `cli/task.go:33-41`).

## Acceptance Criteria

**Status sites — `interrupted`:**

- [ ] `cli/task.go:35-41` — `validAgentStatuses` includes `interrupted`,
      with a comment block mirroring the `cancelled` invariant.
- [ ] `cli/edit.go:84-88` — `--agent-status` validator accepts
      `interrupted`; help text updated.
- [ ] `cli/main.go:88` — top-level help string lists `interrupted`.
- [ ] `gui/internal/agent/state.go:35-39` — `StatusInterrupted Status =
      "interrupted"` constant added.
- [ ] `gui/frontend/src/lib/api.ts` — AgentStatus union includes
      `"interrupted"`.

**Mode sites — `resume`:**

- [ ] `gui/internal/agent/runner.go:33` (next to `ModeGroom`) —
      `ModeResume Mode = "resume"` constant.
- [ ] `gui/app/agent_finish.go:91-97` `parseRunMode` — `"resume"`
      round-trips, no longer collapsed to `implement`.
- [ ] `gui/frontend/src/lib/stores/runs.ts:201` — frontend mode
      normalizer rounds-trips `"resume"`.
- [ ] `gui/frontend/src/lib/components/TaskDrawer.svelte:287, 315` —
      hardcoded optimistic rows extended with a `resume` branch.

**Documentation invariant lines (text-only update):**

- [ ] `cli/CLAUDE.md` — AgentStatus enum line appends `| interrupted`.
- [ ] `CLAUDE.md` — "Architecture invariants" → AgentStatus values:
      same.
- [ ] `docs/ARCHITECTURE.md` — single sentence noting `interrupted` is
      recovery-initiated; deeper documentation lands in TB-142.

**Tests:**

- [ ] Round-trip test for every status site (parse + serialise).
- [ ] Round-trip test for every mode site.
- [ ] Existing `cancelled`-related tests still pass (no regression in
      the `cancelled` carve-out).

## Related Tasks

- **TB-130** — parent epic.
- Blocks **TB-137** (recovery writes `interrupted`), **TB-138** /
  **TB-139** (resume mode), **TB-140** (frontend renders both).

## Log

- 2026-05-14: Created
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done
