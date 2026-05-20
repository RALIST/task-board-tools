# TB-314: GUI: opening a board can start queued agent runs during smoke/load tests

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** gui,daemon,agents,performance
**Branch:** —

## Goal

Starting Wails dev with Writer Studio as the recent board attached the watcher, ran stale recovery, and then launched a real `codex exec --yolo --json` grooming run for queued task WS-2942. Merely opening/switching boards should not surprise-start autonomous work without an explicit enabled mode or a clear pause/safe-mode for manual UI smoke testing. Evidence from /tmp/tb-gui-dev-smoke.log on 2026-05-20: daemon rescan enqueued count=1 followed by a codex exec child process under tb-gui.

## Acceptance Criteria

- [ ] Opening or switching boards in the GUI has a clear safe/default mode that does not start queued autonomous agent runs unexpectedly.
- [ ] If queued runs are intentionally resumed by the GUI daemon, the behavior is gated by an explicit user-visible setting and documented in the UI.
- [ ] Manual smoke/load testing can open large boards without launching `codex`/`claude` child processes.
- [ ] Regression coverage proves board activation does not spawn agent workers when the safe/manual mode is active.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited acceptance

