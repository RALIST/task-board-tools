# TB-192: GUI backend: expose parent reassignment

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** parent-task,api,wails
**Branch:** —
**Parent:** TB-186

## Goal

Expose the CLI parent-reassignment mutation through the GUI backend service, internal CLI wrapper, generated bindings, and typed frontend API wrapper.

## Context

- `gui/app/board_service.go` exposes `EditTask`, but `EditTaskInput` currently includes priority, type, size, module, tags, agent, and agent status only.
- `gui/internal/cli/mutations.go` builds the `tb edit` argument list and classifies mutation errors for GUI callers.
- `gui/frontend/src/lib/api.ts` re-exports the generated binding types and wraps `editTask` for Svelte components.
- This task should be implemented after **TB-191** defines the CLI contract for `tb edit <id> --parent <epic|none>`.

### Constraints

- Do not edit task markdown directly from the GUI backend for parent reassignment; delegate to `tb` so parent and child task files stay consistent.
- Keep the existing `EditTask` call shape unless a separate method is necessary for clear validation or binding generation.
- Preserve existing mutation error handling and watcher-driven refresh behavior.

## Acceptance Criteria

- [ ] `gui/internal/cli.EditInput` and its argument builder support a parent value, including the clear sentinel `none`, and pass it to the CLI as `tb edit <id> --parent <value>`.
- [ ] `gui/app.EditTaskInput` exposes a `parent` field in the Wails JSON contract, and `BoardService.EditTask` forwards it without trimming away the `none` sentinel.
- [ ] Generated bindings and `gui/frontend/src/lib/api.ts` expose the parent field through the typed `EditTaskInput` / `editTask` path used by frontend code.
- [ ] Mutation error classification treats missing parent, invalid parent, and self-parent validation failures as user-facing validation errors rather than unknown failures.
- [ ] Backend tests cover parent argument construction, service pass-through, clear-parent pass-through, and at least one validation error surfaced from the CLI wrapper.
- [ ] Verification passes with `cd gui && go test ./...`; if bindings or frontend types change, also run `cd gui/frontend && npm run check`.

## Related Tasks

- **TB-186** - Parent epic for changing a task's parent from the task page.
- **TB-191** - Prerequisite CLI parent-reassignment mutation.
- **TB-193** - Frontend drawer UX that consumes this API surface.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

