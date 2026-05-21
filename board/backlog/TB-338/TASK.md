# TB-338: Agent: restore PromptGroom required placeholders

**Type:** bug
**Priority:** P2
**Size:** S
**Module:** gui/internal/agent
**Tags:** testing,agent,prompt
**Agent:** codex
**Branch:** —

## Goal

Full GUI suite currently fails in untouched internal/agent: `go test ./internal/agent -run TestPromptGroom_NonEmptyAndUsesOnlySupportedPlaceholders` reports PromptGroom missing `{{TASK_TITLE}}` and `{{TASK_BODY}}`. Restore the groom prompt placeholder contract or update the test/contract intentionally.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited agent=codex

