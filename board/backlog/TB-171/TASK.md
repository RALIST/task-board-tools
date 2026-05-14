# TB-171: TB-93/REVIEW: re-run Codex cross-cutting architectural review (previous run stalled)

**Type:** spike
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,codex
**Branch:** —
**Parent:** TB-93

## Goal

During the TB-93 grand review, three Claude code-reviewer agents delivered findings (CLI, GUI backend, GUI frontend - now tracked as TB-146..TB-170). The codex-rescue cross-cutting reviewer read all in-scope CLI files plus most GUI backend files but then stalled in the generation phase and never produced the structured 10-verdict output. Partial confirmation: Codex did note that the CLI tests cover all-file/all-folder/mixed logical reads and byte-identical BOARD.md snapshots (criteria 2 and 7 PASS). To get the missing cross-cutting verification, re-run with /codex:adversarial-review or resume the existing session 019e2833-c474-7ba1-bfb4-4e383d3bb14b with a tighter 'output findings immediately' nudge. Specific questions still unanswered with primary-source rigor: (1) move atomicity for folder tasks (os.Rename of whole dir vs copy+delete, daemon mid-append race), (2) lockless GUI reads safety under folder form (is attachments/foo copied via writeFileAtomic?), (3) watcher event amplification end-to-end count for one attachment add operation, (4) symlinks on attach add behavior, (5) stale recovery for folder-form tasks reads task-local .agent-state.jsonl. Source: codex-rescue agent stalled in 13m generation phase per agentId ad6c1f362fc888dd4.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
