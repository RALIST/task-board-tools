# TB-312: GUI: Open project selection does not switch board

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** gui,board-switch,regression
**ReviewRef:** worktree
**Branch:** —

## Goal

Selecting another project from the Wails Open project flow should activate that project board and refresh the visible kanban. Current report: start wails3 dev, click Open project, select Writer Studio, nothing changes.

## Acceptance Criteria

- [x] Open project accepts a selected project root that contains `.tb.yaml` and activates it.
- [x] The board service and frontend refresh to the selected project instead of silently leaving the previous board visible.
- [x] Switching to a large board avoids duplicate load/reconcile bursts and does not render thousands of cards at once.
- [x] Regression coverage proves the project-open path handles Writer Studio-style board roots.
- [x] Relevant Go/Svelte checks pass for the changed surface.

## Review Target

Current commit: TB-312 fix GUI board switching
Scope: GUI project picker, board refresh/load coalescing, stage reconciliation startup scan, and large-column render cap.

## Reviewer Notes

Verification:
- cd gui/frontend && npm test
- cd gui/frontend && npm run check
- cd gui/frontend && npm run lint
- cd gui && go test ./...
- Follow-up correction: cd gui/frontend && npm test -- src/lib/columnVisibility.test.ts
- Follow-up correction: cd gui/frontend && npm run check
- Follow-up correction after live smoke found Column.svelte update loop: cd gui/frontend && npm run lint
- Follow-up correction after live smoke found Column.svelte update loop: cd gui/frontend && npm test -- src/lib/columnVisibility.test.ts
- Follow-up correction after live smoke found Column.svelte update loop: cd gui/frontend && npm run check
- Manual smoke before CPU concern: fresh Wails dev loaded Writer Studio and rendered the board (~2,871 active tasks / 2,313 done) after the StageReconciler snapshot reuse fix.
- Manual smoke on 2026-05-20 with isolated config opened Task Board Tools: watcher attached to /Users/ralist/projects/task-board-tools/board, startup scan enqueued=0, tb-gui held around 0% CPU and ~124 MB RSS; Vite dev server held around 0% CPU and ~521 MB RSS.

Review note:
- Batch rendering is intentionally limited to done/archive only; backlog, ready, in-progress, and code-review render all tasks so grooming and selection workflows remain visible.
- Live smoke found and fixed a Svelte effect_update_depth_exceeded loop in Column.svelte by untracking state read during item synchronization and skipping no-op item assignments.
- Opening the real Writer Studio board is not safe for repeated manual smoke yet: startup recovery/daemon mutated Writer Studio board files and launched a real codex child process for WS-2942. Follow-up tracked as TB-314.
- Three subagent review attempts were started and shut down after timeouts. No successful subagent review result was produced, so this task is submitted to code-review rather than marked done.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited size=M, acceptance
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited review-target
- 2026-05-20: Edited reviewer-notes
- 2026-05-20: Edited reviewref=worktree
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited reviewer-notes
- 2026-05-20: Edited reviewer-notes

