# TB-214: QA probe daemon pickup

**Type:** bug
**Priority:** P3
**Size:** S
**Module:** gui
**Tags:** manual-qa,probe,daemon
**Agent:** codex
**AgentStatus:** failed
**Branch:** —

## Goal

Manual QA probe for M5 queued daemon pickup. Agent prompt should make no source edits and report status only.

## Context

Manual QA probe. Do not modify source, board, config, generated files, or git state. The expected daemon-picked agent behavior is to inspect the task, report that this is a no-op QA probe, and exit successfully.

## Acceptance Criteria

- [ ] CLI queueing via AgentStatus=queued is picked up by the running GUI daemon.
- [ ] AgentStatus transitions through running to a terminal state.
- [ ] A JSONL state file and per-run log are written at the task-local folder path.

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Edited agent=codex
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Edited agentstatus=failed
- 2026-05-17: Moved to done

