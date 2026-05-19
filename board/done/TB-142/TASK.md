# TB-142: Docs sweep: ARCHITECTURE.md + CLAUDE.md + FEATURES.md for resume

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** docs
**Tags:** epic-tb130,docs,resume
**Branch:** —
**Parent:** TB-130

## Goal

Final documentation sweep: update the architecture spec, the
top-level CLAUDE.md invariants, the cli CLAUDE.md invariants, and
the features roadmap to reflect `interrupted` status, resume vs
re-run user model, the post-`started` session-write rule, and the
`TB_`-prefix env allowlist.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 11 (Documentation), § 12 task L.

This task lands LAST so the docs reflect what was actually shipped,
not what was originally planned. Touches code-adjacent invariants
ONLY — the design spec under `docs/superpowers/specs/` stays
historical and is not edited here.

## Acceptance Criteria

- [ ] `docs/ARCHITECTURE.md` "Agent state" section gains:
  - Session id capture flow per agent (Claude pre-alloc / Codex
    parsed-from-`--json`).
  - `EvSession` event in the JSONL schema table, including the
    ordering rule "always after `started`".
  - `interrupted` status with the same invariants as `cancelled`
    (recovery-only writer, validator allows the value).
  - Resume vs re-run user model.
  - `run_env` `TB_`-prefix allowlist.
- [ ] `cli/CLAUDE.md` AgentStatus enum line: appends `| interrupted`.
      Note added for `interrupted` invariant.
- [ ] `CLAUDE.md` "Architecture invariants" → AgentStatus values:
      same. Add bullet for resume capability.
- [ ] `docs/FEATURES.md`: this epic added under M5 (extends crash
      recovery) or as M5.5 — TBD with maintainer at landing time.
      Acceptance criteria from the parent epic are mirrored here.
- [ ] `docs/IMPLEMENTATION.md`: TB-130 marked done with completion
      date and one-line summary.
- [ ] No code changes — docs only. Tests = `markdownlint` clean (if
      configured) and link-check on relative anchors.

## Related Tasks

- **TB-130** — parent epic. This task closes the epic.
- Depends on every other A–K task being merged.

## Log

- 2026-05-14: Created
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done
