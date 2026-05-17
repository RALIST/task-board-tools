# TB-217: Manual QA: attachment removal mis-parses dash-leading filename

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** cli
**Tags:** manual-qa,attachments,folder-tasks
**Branch:** —

## Goal

Fix `tb attach --rm` so a dash-leading attachment filename can be removed using a `--` terminator without being parsed as the task ID.

## Acceptance Criteria

- [x] User-visible: `tb attach --rm TB-212 -- -dash.txt` removes the `-dash.txt` attachment instead of reporting a bogus task ID.
- [x] Command/state: after removal, `board/<status>/TB-212/attachments/-dash.txt` is gone and `TB-212`'s `## Attachments` section no longer lists it.
- [x] Regression: removing ordinary attachment names still works, and invalid/missing attachment names still return nonzero validation errors.

## Context

Manual QA test case: TB-93/M1 attachment error path.

Expected: a dash-leading attachment added with `tb attach TB-212 -- /tmp/tb-manual-qa/-dash.txt` can be removed with `tb attach --rm TB-212 -- -dash.txt`.

Actual: the removal command fails with `error: task TB--DASH.TXT not found in requested status scope (backlog, in-progress, done, archive). Verify the ID with \`tb ls --status all\``. Running `tb attach --rm TB-212 -dash.txt` also fails with the generic usage error.

Repro steps:

1. `printf 'dash attachment\n' > /tmp/tb-manual-qa/-dash.txt`
2. `./cli/tb attach TB-212 -- /tmp/tb-manual-qa/-dash.txt`
3. `./cli/tb attach --rm TB-212 -- -dash.txt`

Evidence task: TB-212 still has `board/backlog/TB-212/attachments/-dash.txt`.

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Started — moved to in-progress
- 2026-05-17: Done — fixed `reorderArgs` in `cli/create.go` to respect the `--` terminator (emits it directly after collected flags so the FlagSet consumes it and post-terminator args remain positional, even when leading with `-`). Added regression `TestAttachRemoveDashLeadingFilenameAfterTerminator`. End-to-end repro on standalone QA board confirms `tb attach QA-1 -- /tmp/-dash.txt` followed by `tb attach --rm QA-1 -- -dash.txt` now removes the file.

