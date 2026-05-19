# TB-8: Rename tb/ to cli/ and add go.work

**Type:** tech-debt
**Priority:** P0
**Size:** S
**Module:** cli
**Tags:** milestone-m1,repo-layout
**Branch:** —
**Parent:** TB-1

## Goal

Move the current tb/ Go module to cli/ via git mv so it merges into this repo, and add a root go.work file with use ./cli. Foundation for every other M1 task and a prerequisite for adding ./gui in M2.

## Context

The `tb/` directory is still its own git repo today and is `.gitignored` from this monorepo. M1 starts by merging it back in via `git mv tb cli` and adding a root `go.work`. Every other M1 task assumes paths under `cli/`. CLAUDE.md and docs need their `tb/` path references updated.

## Acceptance Criteria

- [ ] `git mv tb cli` (preserves history)
- [ ] `/Users/ralist/projects/task-board-tools/go.work` created with `use ./cli`
- [ ] Root `.gitignore` no longer ignores `tb/`
- [ ] `cd cli && go build -o tb .` succeeds
- [ ] `go build ./cli` from repo root succeeds
- [ ] `cd cli && go test ./...` still passes (`board_test.go`)
- [ ] References in CLAUDE.md, docs/ARCHITECTURE.md, docs/IMPLEMENTATION.md to `tb/` paths updated to `cli/`

## Related Tasks

- **TB-1** — Parent epic
- **TB-9, TB-10, TB-11, TB-12, TB-13, TB-14, TB-15** — All assume paths under `cli/`

## Log

- 2026-05-13: Created
- 2026-05-13: Started — moved to in-progress
- 2026-05-13: Done
- 2026-05-19: Moved to code-review
- 2026-05-19: Moved to done

