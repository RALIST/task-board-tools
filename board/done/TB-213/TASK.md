# TB-213: QA probe agent run

**Type:** bug
**Priority:** P3
**Size:** S
**Module:** gui
**Tags:** manual-qa,probe,agent
**Agent:** codex
**AgentStatus:** failed
**Branch:** —

## Goal

Manual QA probe for M4 manual agent run. Agent prompt should make no source edits and report status only.

## Context

Manual QA probe. Do not modify source, board, config, generated files, or git state. The expected agent behavior is to inspect the task, report that this is a no-op QA probe, and exit successfully.

## Acceptance Criteria

- [ ] Manual Run from the GUI writes queued, started, stdout/stderr if any, and finished events.
- [ ] AgentStatus ends in success for a completed no-op run, or cancelled for a user-cancelled run.
- [ ] A per-run log is visible in the drawer and on disk.

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Edited agent=codex
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Edited agentstatus=failed
- 2026-05-17: Moved to done

