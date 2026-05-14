# TB-146: TB-93/CLI: attach promotion orphans file-form agent state + logs

**Type:** bug
**Priority:** P0
**Size:** M
**Module:** cli
**Tags:** epic-tb93,review-tb93,attach,promotion
**Agent:** codex
**AgentStatus:** running
**Branch:** —
**Parent:** TB-93

## Goal

promoteFileTaskWithAttachments at cli/attach.go:322-374 does not migrate board-root .agent-state/<ID>.jsonl or .agent-logs/<ID>/ into the new task folder during file->folder promotion. After promotion the daemon reads the empty task-local paths while prior run history sits orphaned at board root - silent loss of run history. Violates TB-93 criterion 4 (preserves metadata, log history, board rendering). Fix: inside the lock, before os.Rename(stagingDir, taskDir), copy/move board-root agent artifacts into <staging>/.agent-state.jsonl and <staging>/.agent-logs/ if they exist; remove originals after rename succeeds. Add a test (TestAttachPromotesLegacyFileTask currently asserts only metadata + attachment bytes + BOARD.md rendering) that seeds both board-root artifacts on a file-form task, runs attachTask, and asserts the artifacts now live inside <status>/<ID>/ and are gone from board root. Source: CLI grand review finding #1+#2.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running

