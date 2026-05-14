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

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
