# TB-323: Prevent kanban drag-start crash

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** gui-frontend
**Tags:** ui,dnd,crash
**GroomedBy:** codex
**GroomStatus:** success
**AgentStatus:** needs-user
**Branch:** —

## Goal

Prevent kanban card drag-start from crashing the GUI with `TypeError: null is not an object (evaluating 'originDropZone.closest')`. A card drag should either start normally or cancel cleanly when the source DOM node disappears before `svelte-dnd-action` finishes startup; the board must remain usable.

## Context

- Observed console error: `TypeError: null is not an object (evaluating 'originDropZone.closest')` from `handleDragStart` / `handleMouseMoveMaybeDragStart` in the compiled frontend bundle.
- The active kanban DnD surface is `gui/frontend/src/lib/components/Column.svelte`: it passes `items` to `svelte-dnd-action`, renders each card inside an `<li>`, handles `consider` / `finalize`, and disables DnD for virtualized columns.
- `gui/frontend/src/lib/components/Board.svelte` wires draggable active columns and keeps Archive non-draggable unless separate archive-DnD work changes that contract.
- `gui/frontend/src/lib/boardDrop.ts` owns drop routing, optimistic move, revert, and readable failure toasts.
- `svelte-dnd-action@0.9.69` sets `originDropZone = originalDragTarget.parentElement` and immediately calls `originDropZone.closest(...)`; this crashes when the dragged item has been detached before drag startup completes.
- Related tasks: TB-34 introduced kanban card DnD; TB-104/TB-126 hardened file-drop attachment behavior that must stay separate from card drags; TB-298 is sibling archive-DnD work; TB-313 is sibling virtualization work that explicitly guards unsupported DnD.

## Constraints

- Keep the fix scoped to frontend DnD lifecycle handling unless investigation proves the backend or CLI move path is involved.
- Do not solve by disabling kanban drag/drop globally; active-column card moves should still work where supported.
- Preserve existing drop routing: backlog -> ready uses `readyTask`, ready -> in-progress uses `pullTask`, other active-column moves use `moveTask`, with optimistic revert and readable toast on failure.
- Preserve file-drop attachment behavior from TB-104/TB-126: card elements still need `data-file-drop-target` / `data-task-id`, and OS file drops must remain distinct from card drags.
- Do not fold TB-298 archive-DnD or TB-313 virtualization scope into this bug. If either task's in-flight state blocks full frontend gates, record that as verification context instead of broadening this task.

## Acceptance Criteria

- [ ] A focused frontend regression test reproduces the drag-start crash path or an equivalent detached-source DnD startup case, and fails before the fix.
- [ ] Starting a kanban card drag no longer throws `originDropZone.closest` when the source item is detached or the board snapshot refreshes before drag startup; the drag either proceeds or cancels without breaking the board.
- [ ] Normal active-column DnD still works after the fix, including backlog -> ready, ready -> in-progress, and generic active-column moves through the existing `boardDrop.ts` routing.
- [ ] Existing virtualized-column guard remains intact: unsupported virtualized DnD stays visibly disabled instead of silently crashing or moving the wrong task.
- [ ] File-drop attachment affordances on cards and the task drawer still expose the expected `data-file-drop-target` / `data-task-id` attributes and are not treated as kanban card drags.
- [ ] Frontend verification passes for the relevant surface: `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run src/lib/components/Column.test.ts src/lib/boardDrop.test.ts`, and `cd gui/frontend && npm run lint`; if unrelated TB-313/TB-318 WIP blocks a full gate, record the blocker and the scoped passing command.
- [ ] Manual test: run `cd gui && task dev`, open a populated board, drag cards across active columns, repeat the selected-element crash repro if known, and confirm no console error mentioning `originDropZone.closest` appears and the board remains interactive.

## User Attention

**Reason:** verification and commit blocker.

**Specific question or action:** Please either (1) close/clear the existing `tb-gui` single-instance dev app and run the manual drag smoke, or authorize this task to skip/manual-defer that smoke; and (2) provide a clean index/worktree or authorize an isolated worktree/commit strategy so TB-323 can be committed without sweeping sibling WIP.

**Attempted context:** TB-323 is still in `in-progress`. Implemented a scoped frontend guard in `gui/frontend/src/lib/components/Column.svelte` and focused regression coverage in `Column.test.ts`/`boardDrop.test.ts`. The required frontend gates pass: `npm run check`, `npm test -- --run src/lib/components/Column.test.ts src/lib/boardDrop.test.ts`, `npm run lint`; full frontend `npm test -- --run` also passes (304 tests). Manual smoke was attempted with `cd gui && XDG_CONFIG_HOME=/tmp/tb323-config.v88BMr task dev` against a temp populated board at `/tmp/tb323-smoke.Ar6vXH`, but an existing single-instance `tb-gui` process reused the current repo board instead of the isolated temp board. Browser runtime tools were not callable; Playwright could only load the browser preview and hit Wails runtime 404s; Computer Use timed out reading the `tb-gui` app. Commit is also unsafe right now: `git diff --cached` already contains sibling `Column.test.ts` virtualization hunks, and the worktree has other TB-313/TB-318/TB-322 board/app WIP. Committing now would include unrelated user/sibling changes or require disturbing the existing index.

**Unblock condition:** A human completes the Wails manual drag smoke or explicitly accepts automated coverage in its place, and the workspace is made clean enough (or isolated-worktree commit is authorized) for an atomic TB-323 commit/review submission.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited agent=codex
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited priority=P2, type=bug, size=M, module=gui-frontend, tags=ui,dnd,crash, title=Prevent kanban drag-start crash
- 2026-05-21: Edited goal
- 2026-05-21: Edited context
- 2026-05-21: Edited constraints
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited context
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Committed — moved to ready
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited user-attention
- 2026-05-21: Edited agentstatus=needs-user

