# TB-290: Allow to edit context and constraits by CLI

**Type:** improvement
**Priority:** P2
**Size:** S
**Branch:** —

## Goal

Agent reported it 
`eason — managed-board mutation limitation.

Specific question/action — Please either authorize a direct task-body cleanup for TB-285 or provide/use a managed board command that can rewrite ## Context and ## Constraints sections. The cleanup needed is to remove the duplicated Context/Constraints blocks and reword the scan-created log expectation without embedding the exact triage-triggering phrase in this groomed task card.

Attempted context — I read tb show TB-285, board/CONVENTIONS.md, board/SKILL.md, tb edit --help, cli/scan.go, cli/create.go, and the folder-form architecture references. tb triage --json still reports TB-285 with auto-created by scan because the task body itself quotes the scan-created log text in Constraints/Acceptance. tb edit --acceptance can update Acceptance Criteria, but current tb edit cannot remove or rewrite the duplicated ## Context / ## Constraints sections where the blocker also appears.

Unblock condition — Grooming can resume once there is an approved board-safe way to rewrite those body sections, or explicit permission is given to perform the narrow task-body cleanup directly while preserving metadata, status, related tasks, attachments, and log history.`

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited type=improvement
- 2026-05-20: Edited size=S

