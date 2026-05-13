# TB-1: M1: CLI extensions for GUI integration

**Type:** feature
**Priority:** P1
**Size:** XL
**Module:** cli
**Tags:** milestone-m1,cli,epic
**Branch:** —

## Goal

Extend the existing tb CLI with everything the upcoming GUI needs: --json output, Agent/AgentStatus task fields, atomic writes, regenerate consistency, and clearer archive/active/all status semantics. CLI must remain the single source of truth for structured mutations.

## Subtasks

- **TB-8** (S) — Rename tb/ to cli/ and add go.work
- **TB-9** (S) — Add cli/atomicfs.go writeFileAtomic helper
- **TB-10** (S) — Migrate task .md writes to writeFileAtomic
- **TB-11** (M) — Add Agent and AgentStatus task metadata fields
- **TB-12** (M) — Add --json output to ls, show, and board
- **TB-13** (S) — Call regenerateBoard at end of create and edit
- **TB-14** (S) — Implement active/archive/all status semantics
- **TB-15** (S) — Add flag.NewFlagSet and reorderArgs to tb show

## Context

CLI is the single source of truth for structured mutations. GUI will exec the CLI for create/move/edit/close and parse output. To make that work cleanly the CLI needs JSON output, two new agent metadata fields, atomic writes (so lock-free GUI reads are safe), and BOARD.md must regenerate after every mutation. Reference: `/Users/ralist/.claude/plans/misty-dazzling-raven.md` (M1) and `docs/IMPLEMENTATION.md`.

## Acceptance Criteria

- [ ] All M1 sub-tasks (TB-8..TB-15) closed
- [ ] `cd cli && go build -o tb .` produces a working binary
- [ ] `go build ./cli` works from repo root via go.work
- [ ] `tb ls --json` returns `[]` for empty selections and well-formed JSON otherwise
- [ ] `tb ls --status archive --json | jq .` works
- [ ] `tb edit 1 -a claude --agent-status queued` writes the new metadata fields
- [ ] `tb create "X" -m m` and `tb edit X-N -p P1` update BOARD.md without a manual regenerate
- [ ] `tb show 1 --json` and `tb show --json 1` both work
- [ ] Existing smoke flow (create → start → done → close) still passes
- [ ] docs/IMPLEMENTATION.md M1 markers flipped to ☑

## Related Tasks

- **TB-8..TB-15** — sub-tasks (children)
- **TB-2** — blocked by this epic

## Log

- 2026-05-13: Created
