# TB-339: CLI: fix init EOF errorlint regression

**Type:** bug
**Priority:** P2
**Size:** S
**Module:** cli
**Tags:** lint,go,quality
**Branch:** —

## Goal

Restore the Go lint gate by replacing the direct `err != io.EOF` comparison in `cli/init.go` with wrapped-error-safe handling, likely `!errors.Is(err, io.EOF)`, while preserving existing init-doc refresh behavior.

## Acceptance Criteria

- [ ] `make lint-go` reports 0 CLI issues for `cli/init.go` and does not introduce new CLI lint findings.
- [ ] `cd cli && go test ./...` passes.
- [ ] The fix is scoped to the EOF comparison behavior and does not change board initialization or generated-doc refresh semantics.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited goal
- 2026-05-21: Edited acceptance

