# TB-134: Codex --json switch + codexJsonTranslator + parity tests

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb130,agent,resume,codex
**Branch:** —
**Parent:** TB-130

## Goal

Switch the default Codex invocation from `codex exec <prompt>` to
`codex exec --json <prompt>` and add a `codexJsonTranslator`
mirroring `gui/internal/agent/claude_stream.go`'s `claudeTranslator`:
parse each JSONL stdout event, render a human-readable line for the
log file, pass-through unknown events.

This task is the hard prerequisite for TB-136 (Codex session capture)
and TB-139 (Codex resume) — neither can land safely without parity
verification of the new flow.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ "Verified CLI surface" (Codex section), § 12 task D, § "Risks"
(Codex `--json` parity is the highest-risk change — it modifies an
already-working code path).

Codex round-1 important 3 flagged this as a real behaviour change for
the existing run flow, not just the resume path. Switching stdout to
JSONL changes log content and may change non-zero / timeout / error
behaviour unless `mapRunnerOutcome` (`gui/app/agent_run.go:523-541`)
is verified against the new output.

## Acceptance Criteria

- [ ] `gui/internal/agent/codex.go:31` — invocation changes from
      `["exec", in.Prompt]` to `["exec", "--json", in.Prompt]`.
- [ ] New file `gui/internal/agent/codex_stream.go` (mirror of
      `claude_stream.go`) implements `codexJsonTranslator`:
  - Wraps an `io.Writer` (the line sink).
  - Parses each line as JSON; unknown shapes pass through unchanged.
  - Renders human-readable lines for known event types — output
    should be visually close to today's plain `codex exec` output.
  - Returns `len(p)` from `Write` even when the rendered output is
    longer (honest accounting from the runner's POV — same contract
    as `claudeTranslator`).
- [ ] `CodexRunner.Run` wraps `in.Stdout` with the new translator
      before passing to `runExternal`.
- [ ] **Parity tests** for `mapRunnerOutcome` against the new flow:
  - Zero-exit success → `success`.
  - Non-zero exit → `failed{non-zero exit}`.
  - Binary not found → `failed{binary not found}`.
  - Timeout (`agent.ErrTimeout`) → `failed{timeout}`.
  - Context-cancelled → `failed{<err>}` (cancel-path normally
    intercepted before this).
- [ ] Translator unit tests: feed sample `--json` events captured
      from a real `codex exec --json` run (or hand-crafted samples)
      and assert the rendered output matches expected text.
- [ ] Stub-binary integration test (mirroring
      `gui/internal/agent/exec_test.go:310, 349`): assert the
      `--json` flag is in argv.

## Related Tasks

- **TB-130** — parent epic.
- Blocks **TB-136** (Codex session capture parses `--json` events),
  **TB-139** (Codex resume invocation builds on the same flow).

## Log

- 2026-05-14: Created
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Done
