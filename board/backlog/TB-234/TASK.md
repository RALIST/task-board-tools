# TB-234: Daemon should not auto-pick up tasks in code-review

**Type:** bug
**Priority:** P3
**Size:** S
**Module:** gui
**Tags:** daemon,code-review,epic-tb194
**Branch:** —

## Goal

Currently isReadyForDaemon only checks AgentStatus=queued and a non-empty Agent. If a user runs 'tb assign <ID> claude' on a code-review task the daemon will spawn implement-mode against an already-reviewed task. Daemon eligibility should ignore tasks in code-review (and arguably done/archive too) so reassigning an already-reviewed task does not regress it. Same fix should apply to ResumeAgent and manual RunAgent paths.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-19: Created
