# TB-191: CLI: safely reassign a task parent

**Type:** feature
**Priority:** P2
**Size:** M
**Module:** cli
**Tags:** parent-task,cli
**Branch:** —
**Parent:** TB-186

## Goal

Add a CLI-supported parent reassignment path so a task can be moved from one parent epic to another, or detached from an epic, without leaving stale `Parent` metadata or `## Subtasks` entries.

## Context

- Parent links are represented by child task metadata (`**Parent:** TB-NNN`) and parent epic `## Subtasks` bullets written by `tb create --parent`.
- `cli/create.go` already validates a parent, auto-tags it as `epic`, and appends the child to the parent's `## Subtasks` section.
- `tb edit` can update several metadata fields, but it cannot currently change `Parent` or maintain the old and new parent task bodies.
- GUI structured mutations should delegate this relationship change to the CLI instead of rewriting markdown directly.

### Constraints

- Hold `.board.lock` for the whole relationship mutation, write every touched task atomically, and regenerate `BOARD.md` after a successful change.
- Support both legacy file-form tasks and folder-form `TASK.md` tasks.
- Preserve unrelated parent task body content, metadata, attachments, and log history.
- Do not auto-remove the `epic` tag from the old parent when its last child is removed; epic-ness is an explicit task tag.

## Acceptance Criteria

- [ ] `tb edit <child> --parent <epic>` accepts prefixed or numeric IDs, normalizes the stored child metadata to the board prefix form, and rejects missing parents, self-parenting, and invalid IDs with validation errors.
- [ ] Reassigning from no parent to a parent adds or updates `**Parent:** <epic>` on the child, auto-adds the `epic` tag to the new parent when needed, and writes exactly one matching child bullet in the new parent's `## Subtasks` section.
- [ ] Reassigning from one parent to another removes the child bullet from the old parent's `## Subtasks` section, adds it to the new parent's section, and preserves unrelated bullets and surrounding markdown in both parents.
- [ ] `tb edit <child> --parent none` clears the child's `Parent` metadata and removes the child bullet from the old parent without adding a new parent.
- [ ] Reassigning to the already-current parent is idempotent: it does not duplicate `Parent` metadata, duplicate `## Subtasks` bullets, or create extra log noise beyond the edit command's normal log entry.
- [ ] Table-driven CLI tests cover add, change, clear, same-parent no-op, missing parent, self-parent, file-form tasks, folder-form tasks, and mixed boards.
- [ ] Verification passes with `cd cli && go test ./...`.

## Related Tasks

- **TB-186** - Parent epic for changing a task's parent from the task page.
- **TB-192** - GUI backend/API wiring depends on this CLI mutation.
- **TB-193** - TaskDrawer parent-edit UX depends on this CLI mutation.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

