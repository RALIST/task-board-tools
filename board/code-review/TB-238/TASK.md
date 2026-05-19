# TB-238: Update implement.md agent prompt to set ReviewRef before submit

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** workflow
**Tags:** agents,workflow,docs,review-failed
**Agent:** codex
**AgentStatus:** success
**ReviewRef:** main
**Branch:** —

## Goal

Update `gui/internal/agent/prompts/implement.md` so autonomous agents set `**ReviewRef:**` via `tb edit --review-ref` before calling `tb review --submit`, matching TB-235's gating.

## Acceptance Criteria

- [x] `gui/internal/agent/prompts/implement.md` includes a `tb edit {{TASK_ID}} --review-ref <branch|PR|commit>` step before `tb review --submit {{TASK_ID}}` in the code-review submission block (currently lines ~24–31).
- [x] The prompt explicitly distinguishes `## Review Target` (human-readable prose) from `**ReviewRef:**` (gating metadata required by TB-235); both are mentioned so agents do not drop the prose section.
- [x] Updated guidance is consistent with `## Defenition of Done` (line ~61): the submit instructions there reference the same `tb review --submit` command, so any wording change above stays compatible with that block.
- [x] No behavioral code changes — this is a prompt-only edit. Surrounding sections (`## Role`, `## Working contract`, `## User Attention handoff`, `## Defenition of Done`, the `tb start` footer) remain untouched except for the targeted review-submit block.
- [x] Manual verification: render the prompt for a sample task (e.g. via the GUI agent runner or by reading the file directly) and confirm the new `tb edit --review-ref` step appears before `tb review --submit` and that the `## Review Target` vs `**ReviewRef:**` distinction is clear.
- [x] Optional: if `prompts/groom.md` or other agent prompts reference the same submit-to-code-review flow, mirror the wording; otherwise note in the Log that only `implement.md` needed the change. — Confirmed via `grep -n "review --submit\|ReviewRef\|review --target" gui/internal/agent/prompts/*.md`: only `implement.md` carries the submit flow, so no other prompt needed mirroring.

## Review Target

branch: main
ReviewRef metadata: main

Surface area to verify:
- gui/internal/agent/prompts/implement.md (lines 24-31): prompt includes `tb edit {{TASK_ID}} --review-ref <branch|PR|commit>` before `tb review --submit {{TASK_ID}}` and distinguishes `## Review Target` prose from `**ReviewRef:**` metadata.
- board/BOARD.md: regenerated/staged from the committed task directories so it no longer lists task IDs in columns without matching committed task files.
- No behavioral code changes.

Mirror check: `grep -n "review --submit\|ReviewRef\|review --target" gui/internal/agent/prompts/*.md` still confirms only `implement.md` carries the submit flow.

## Review Findings

- `board/BOARD.md` in the submitted commits is not regenerated from the committed task directories: it lists TB-239 as an in-progress epic and lists TB-202/TB-237/TB-239 under `## In Progress`, but the commit tree for the reviewed state contains no `board/in-progress/TB-202`, `board/in-progress/TB-237`, or `board/in-progress/TB-239` task files (only `board/backlog/TB-202/TASK.md` and `board/code-review/TB-238/TASK.md` for those IDs via `git ls-tree -r HEAD board`). Regenerate/stage `BOARD.md` and `.next-id` from the actual committed board source of truth, or remove the unrelated generated board churn before resubmitting TB-238.

## Related Tasks

- **TB-235** — Parent: introduced the ReviewRef gate; this task aligns the agent prompt with the new workflow.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Pulled into in-progress
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited review-target
- 2026-05-19: Edited reviewref=main@fd1233e
- 2026-05-19: Submitted to code-review
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Failed code review — moved to ready with review-failed marker
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited review-target
- 2026-05-19: Edited reviewref=main
- 2026-05-19: Submitted to code-review
- 2026-05-19: Edited agentstatus=success

