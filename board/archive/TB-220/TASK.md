# TB-220: Manual QA: agent probe corrupts implement prompt template

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** agent
**Tags:** manual-qa,agent,prompt
**Branch:** —

## Goal

Fix real-agent run behavior so a no-op QA probe cannot silently corrupt the embedded implement prompt template or leave source prompt changes outside the intended task scope.

## Context

Manual QA test M4/M6 real-agent probe on 2026-05-17.

Expected: TB-213 and TB-215 were no-op QA probes instructing the real Codex agent not to modify source, board, config, generated files, or git state except for normal agent run state/log artifacts.

Actual: after the Codex probe runs, `gui/internal/agent/prompts/implement.md` was modified. The diff malformed the `{{TASK_ID}}` placeholder into `{{TASK_ID}` and added new completion requirements including commit wording with a typo. This source prompt mutation was outside the QA probe scope and would break or distort future agent runs if left unnoticed.

Evidence:
- Source diff: `git diff -- gui/internal/agent/prompts/implement.md`
- Agent probes: TB-213 (`r_26fbd621`) and TB-215 (`r_af108e79`)
- State files: `board/done/TB-213/.agent-state.jsonl`, `board/done/TB-215/.agent-state.jsonl`

Repro steps:
1. Create a no-op QA probe task assigned to codex that explicitly tells the agent not to modify source/config/generated files.
2. Run or Groom the task from the Wails GUI.
3. After the run reaches a terminal state, inspect `git diff -- gui/internal/agent/prompts/implement.md`.
4. Observe prompt-template source changes unrelated to the probe task.

## Acceptance Criteria

- [ ] User-visible verification: running a no-op QA probe does not leave unexpected source prompt changes in the working tree.
- [ ] Command/state verification: `git diff -- gui/internal/agent/prompts/implement.md` stays empty after a no-op probe run, while `.agent-state.jsonl` still records queued, started, and terminal events.
- [ ] Regression verification: the embedded implement prompt template keeps a valid `{{TASK_ID}}` placeholder.

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Closed (archived from backlog)

