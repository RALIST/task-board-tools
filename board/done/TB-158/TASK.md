# TB-158: TB-93/GUI: insert '--' before user paths in tb attach mutations to prevent flag confusion

**Type:** bug
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,security
**Branch:** —
**Parent:** TB-93

## Goal

gui/internal/cli/mutations.go:347-359 - Attach / RemoveAttachments append user-supplied paths/names to args without a -- flag terminator. A path or attachment name beginning with '-' is parsed as a flag by the CLI's flag.Parse(reorderArgs(args)). Example: tb attach TB-1 -malware.txt could be treated as -m alware.txt. Fix: insert '--' before user-controlled args, e.g. args := []string{'attach', id, '--'}; args = append(args, paths...) - same for remove. Confirm cli/attach.go's flag parser respects '--'; if not, fix the CLI side first. Source: GUI backend review finding #4.

## Acceptance Criteria

- [x] `Attach` and `RemoveAttachments` in `gui/internal/cli/mutations.go` insert a `--` terminator between the task ID and user-controlled paths/names before exec.
- [x] CLI side: `containsAttachRemoveFlag` stops scanning at `--`, so a path literally named `--rm` cannot retarget the command into the remove path; `runAttach`'s add branch strips the optional leading `--` from positional paths before passing to `attachTask`.
- [x] New CLI tests cover `tb attach TB-X -- -leading-dash.txt` and `tb attach TB-X -- --rm` add-path scenarios; existing GUI tests updated to assert the `--` token in argv.
- [x] `cd cli && go test ./` and `cd gui && go test ./...` pass.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done
