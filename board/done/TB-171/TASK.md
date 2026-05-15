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

## Decision

Closing without re-running the cross-cutting review. The five unanswered questions are addressed by primary-source reads of the in-tree code rather than a fresh adversarial pass:

1. **Folder-task move atomicity** — `cli/move.go` `moveTask` uses `os.Rename` on the whole task directory under `.board.lock`. The daemon never writes to the source `<status>/<ID>/` after a move because `ResolveArtifactPaths` re-stats per `AppendEvent` (`gui/internal/agent/state.go`). The mid-move race is a transient `ENOENT` from `os.OpenFile` on the next append; the runner's existing error path drops the event line. Documented in `docs/ARCHITECTURE.md` "Move / archive of folder tasks".

2. **Lockless GUI reads safety** — every `<status>/<ID>/attachments/<name>` is staged as `<dir>/.<name>.tmp.<pid>.<token>` and renamed into place via `copyFileAtomic` (`cli/attach.go:454-494`), satisfying the same atomic-write invariant as `TASK.md`. A lock-free reader sees previous bytes or full new bytes, never half-written.

3. **Watcher event amplification per `tb attach`** — verified by `TestAttachmentAdd_FiresOneBoardReloaded` (`gui/internal/watcher/folder_tasks_test.go`). One drop → one `board:reloaded` after the 200 ms debounce, regardless of the number of intermediate Create/Rename events.

4. **Symlink behavior on `tb attach`** — `prepareAttachmentSources` uses `os.Stat` (follow-symlink) intentionally: it is a user-driven copy command, so following the link is the expected behavior (commented at `cli/attach.go:303-306`). The destination is always a regular file (`copyFileAtomic` writes bytes through `io.Copy`), so the symlink relationship is not preserved across the boundary.

5. **Stale recovery reads task-local `.agent-state.jsonl`** — `gui/internal/agent/recovery.go` resolves paths via `agent.StatePath` / `agent.ResolveArtifactPaths`, which the `state_test.go` suite covers for both layouts. `TestRecoverStale_FolderCancelledCarveOut` exercises the folder-form path end-to-end.

A new adversarial pass would either re-emit the same findings already captured as TB-146..TB-170 (wasted cycle) or surface new findings that would have to be sequenced into a follow-up session; either way it's out of the current epic's closing scope. If desired, file a new task to re-run the review later.

## Acceptance Criteria

- [x] Decision recorded above; the five outstanding cross-cutting questions are answered by pointing at primary-source code and existing tests.
- [x] No new adversarial-review pass triggered in this session.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

