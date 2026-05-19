# TB-238: Update implement.md agent prompt to set ReviewRef before submit

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** workflow
**Tags:** agents,workflow,docs
**Branch:** —

## Goal

TB-235 review nit: gui/internal/agent/prompts/implement.md still tells autonomous agents to call 'tb review --target' then 'tb review --submit'. With TB-235's gate, submit fails unless **ReviewRef:** metadata is set via 'tb edit --review-ref ...'. Update the prompt to include a 'tb edit {{TASK_ID}} --review-ref <branch|PR|commit>' step before 'tb review --submit' and clarify that '## Review Target' is human-readable prose while '**ReviewRef:**' is the gating metadata.

## Acceptance Criteria

- [ ] (to be filled)

## Related Tasks

- **TB-235** — Parent: introduced the ReviewRef gate; this task aligns the agent prompt with the new workflow.

## Attachments

## Log

- 2026-05-19: Created
