# TB-289: Extend tb ls with multi-value filter flags + --agent + --search

**Type:** feature
**Priority:** P2
**Size:** M
**Module:** cli
**Tags:** cli,filter,auto-implement,blocks-tb288
**Agent:** codex
**AgentStatus:** success
**GroomedBy:** codex
**GroomStatus:** success
**ImplementedBy:** claude
**ImplementStatus:** success
**ReviewRef:** 76c161b
**Branch:** —

## Goal

Extend `tb ls` filters to accept comma-separated values for OR-within-field matching, plus add the `--agent` and `--search` flags missing today, so the CLI can express the same multi-select filter the GUI FilterBar offers. The auto-implement coordinator (TB-179) and the FilterBar-driven query work (TB-288) will then both delegate to `tb ls --json` instead of carrying their own matcher.

Each flag stays AND-across-flags; commas inside a single flag are OR-within. Single-value invocations (`-T bug`) remain valid and produce identical output, so existing callers and tests are unaffected.

## Acceptance Criteria

- [ ] **Multi-value flags**: `-T`, `-p`, `-m`, `-s`, `-t`, and `--parent` accept comma-separated lists. Each value is trimmed; empty segments after splitting are ignored.
- [ ] **Preserve current single-value matching**: each parsed value uses the current `tb ls` matcher for that field: `-T`, `-p`, and `-s` are case-insensitive equality checks; `-m` remains case-insensitive substring matching; `-t` remains an exact tag-name match against any task tag using the existing trim + case-insensitive comparison; `--parent` normalizes each supplied ID with `normalizeTaskID` and compares case-insensitively.
- [ ] **OR semantics within a flag**: a task matches a multi-value flag when any supplied value matches that field. Across flags, the existing AND semantics is preserved. Do not add multi-value support to `--status` in this task.
- [ ] **Existing-flag compatibility**: single-value calls for `-T`/`-p`/`-m`/`-s`/`-t`/`--parent` produce the same result set, stdout/stderr shape, and exit behavior as today, including unknown existing-filter values that currently just return no matches.
- [ ] **Tag flag behavior**: `-t macos,window` matches tasks that have either `macos` or `window`; a task with multiple tags still matches when any one task tag equals any supplied filter tag. Update help and add explicit tests for this OR behavior.
- [ ] **New `--agent` flag**: accepts comma-separated agent names (`claude`, `codex`, `none` to match unassigned). OR-within-flag. Match `claude`/`codex` against the canonical lowercased `Agent` field; `none` matches a blank `Agent`. Unknown agents return an error using the same valid-agent set as `tb edit -a` plus the `none` sentinel.
- [ ] **New `--search` flag**: accepts one free-text term. It is not comma-split because commas are valid title text. Match case-insensitive substrings against `id` and `title`, matching the GUI FilterBar search-box semantics.
- [ ] **Help text**: `tb ls --help` shows the multi-value-capable usage, e.g. `-T type[,type...]` instead of `-T type`. Each multi-value flag's description states "comma-separated, matches any" and preserves field-specific behavior such as module substring matching. The positional usage line stays focused on the common case.
- [ ] **JSON output unchanged**: the existing `--json` shape stays byte-identical for single-value calls so frontend consumers such as `BoardService.LoadBoard` do not need a JSON-schema migration. Multi-value calls produce the same shape with the broader result set.
- [ ] **Tests**: add dedicated table-driven tests in `cli/list_test.go` or a new `cli/list_multi_test.go` covering: (a) `-T bug,improvement` matches both; (b) `-p P0,P1` matches both; (c) `-m gui,cli` matches both and preserves substring/case-insensitive matching; (d) `-s S,M` matches both; (e) `-t macos,window` matches tasks with either tag; (f) `--parent TB-1,TB-2` matches children of either and accepts numeric forms that normalize; (g) `--agent claude,codex` matches both and `--agent none` matches unassigned tasks; (h) `--search router` finds the term in id or title case-insensitively; (i) multi-flag AND: `-T bug -p P1,P2 -m gui` returns the intersection; (j) empty/whitespace segments are tolerated; (k) unknown existing-filter values keep the current no-match behavior, while unknown `--agent` values fail.
- [ ] **Verification**: `cd cli && go test ./...`; `cd cli && go build -o tb .`; rebuild/relink the local `tb` binary per repo rules after CLI changes. Existing GUI tb-binary integration coverage (`buildTbForIntegration`) keeps passing because the JSON shape is unchanged.
- [ ] **Help / docs**: update `tb ls`'s usage one-liner and per-flag descriptions. Add a short example to `cli/CLAUDE.md` (for example: `tb ls -T bug,improvement -p P0,P1 --status ready --json`). No README or board-skill changes are needed; this is additive.

## Non-goals

- Implementing the FilterBar-driven save button (TB-288 owns that, with a hard dependency on this task).
- Boolean operators beyond OR (no AND-within-field, no NOT). Comma is the only separator.
- A new query DSL (the goal is to keep flags simple and shell-friendly).
- Updating the GUI's `gui/internal/automation/query` package - that deletion is TB-288's scope.

## Related Tasks

- **TB-288** — Downstream consumer. TB-288 is blocked on this task: its plan is to delete `gui/internal/automation/query` and shell out to `tb ls --json` with the multi-value flags introduced here. Ship this first; TB-288 cannot land without it.
- **TB-178** — Originally introduced the in-GUI text-DSL parser this work supersedes. The parser package will be deleted as part of TB-288, not this task.
- **TB-179** — Auto-implement coordinator that currently calls `gui/internal/automation/query`. After TB-288 lands, the coordinator will instead call `BoardService.LoadBoard(... with flag args ...)` (or a new variant).

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited priority=P2, type=feature, size=M, tags=cli,filter,auto-implement,prereq-tb288, acceptance
- 2026-05-20: Edited tags=cli,filter,auto-implement,blocks-tb288
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=lost, groomed-by=codex, groom-status=lost
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=lost, groomed-by=codex, groom-status=lost
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited implemented-by=claude, implement-status=success
- 2026-05-20: Edited reviewref=76c161b
- 2026-05-20: Done

