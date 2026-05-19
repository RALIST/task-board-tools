# TB-239: Canonical Kanban: add ready column + WIP/pull mechanics

**Type:** feature
**Priority:** P1
**Size:** XL
**Module:** core
**Tags:** epic,refactor
**ReviewRef:** branch:main (no PR yet — uncommitted local changes)
**Branch:** —

## Goal

Add a 'ready' column between backlog and in-progress, generalize WIP limits to per-column with warn/strict modes, introduce tb ready and tb pull commands, update GUI + docs + agent prompts in lockstep.

## Acceptance Criteria

- [x] CLI: add `ready` to `statusDirs`/`allStatusDirs` + alias `r`; update `resolveStatus`/`resolveStatusFilter`/`statusRank`/help text.
- [x] CLI: per-column WIP config (`wip_limit_ready`, `wip_limit_in_progress`, `wip_limit_code_review`, `wip_enforcement: warn|strict`) with legacy `wip_limit` compat.
- [x] CLI: new commands `tb ready <ID>` (triage gate) and `tb pull [<ID>]` (priority-ordered).
- [x] CLI: `tb start` warns on backlog source; `tb mv` enforces WIP; `tb review --fail` returns to ready.
- [x] CLI: BOARD.md adds `## Ready` section with `(n/m ⚠)` headers; JSON snapshot adds `ready`, `wipLimits`, `wipCounts`, `wipEnforcement`.
- [x] CLI tests: new `ready_test.go` and `pull_test.go`; alias + filter tests; existing fixtures updated.
- [x] GUI Go: `Ready` bucket in `BoardSnapshot`; `ReadyTask`/`PullNext`/`PullTask` Wails methods; `Ready`/`Pull` cli.Client wrappers; watcher and attachments updated.
- [x] GUI Svelte: Ready column in `Board.svelte` + grid; WIP badge in `Column.svelte`; store/api/filtering/page handlers updated; bindings regenerated.
- [x] Docs + prompts: CLAUDE.md, README.md, board/CONVENTIONS.md + SKILL.md (via cli/templates.go), docs/PROJECT.md, docs/ARCHITECTURE.md, docs/FEATURES.md, docs/IMPLEMENTATION.md (M10 entry).
- [x] Verification: cli go test, gui go test, frontend npm check + npm test all green; CLI smoke (ready/pull/start/board --json/BOARD.md) verified.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited reviewref=branch:main (no PR yet — uncommitted local changes)
- 2026-05-19: Moved to code-review
- 2026-05-19: Moved to done

