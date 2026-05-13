# TB-9: Add cli/atomicfs.go writeFileAtomic helper

**Type:** feature
**Priority:** P1
**Size:** S
**Module:** cli
**Tags:** milestone-m1,atomic-writes
**Branch:** —
**Parent:** TB-1

## Goal

Introduce cli/atomicfs.go exposing writeFileAtomic(path, data, perm) that writes to a temp file in the same directory then os.Rename to the target. This is the only sanctioned write path for task .md files going forward and makes lock-free GUI reads safe.

## Context

GUI will parse task `.md` files without holding the board lock. If the CLI writes via plain `os.WriteFile`, a watcher event can fire mid-write and the GUI parser will see a truncated file. Atomic temp+rename gives POSIX atomicity guarantees inside the same directory. The helper lives in a single file so the invariant is enforceable by `grep`.

## Acceptance Criteria

- [ ] `cli/atomicfs.go` exports `writeFileAtomic(path string, data []byte, perm os.FileMode) error`
- [ ] Implementation writes a temp file in the same directory (suffix like `.tmp.<pid>.<rand>`), sets perm, `fsync`s, then `os.Rename`s onto the target
- [ ] Cleans up the temp file on any error path before returning
- [ ] Top-of-file comment documents this as the only sanctioned write path for task `.md` files
- [ ] Unit test covers: happy path, target-dir-missing, rename collision

## Related Tasks

- **TB-1** — Parent epic
- **TB-8** — Prerequisite (paths under `cli/`)
- **TB-10** — Consumer of this helper

## Log

- 2026-05-13: Created
