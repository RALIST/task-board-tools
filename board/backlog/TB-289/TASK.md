# TB-289: Extend tb ls with multi-value filter flags + --agent + --search

**Type:** feature
**Priority:** P2
**Size:** M
**Module:** cli
**Tags:** cli,filter,auto-implement,blocks-tb288
**Branch:** —

## Goal

Extend `tb ls` filters to accept comma-separated values for OR-within-field matching, plus add the `--agent` and `--search` flags missing today, so the CLI can express the same multi-select filter the GUI FilterBar offers. The auto-implement coordinator (TB-179) and the FilterBar-driven query work (TB-288) will then both delegate to `tb ls --json` instead of carrying their own matcher.

Each flag stays AND-across-flags; commas inside a single flag are OR-within. Single-value invocations (`-T bug`) remain valid and produce identical output, so existing callers and tests are unaffected.

## Acceptance Criteria

- [ ] **Multi-value flags**: `-T`, `-p`, `-m`, `-s`, `-t`, `--parent` accept comma-separated lists. Each value is trimmed; empty segments after split are ignored; case rules match the current single-value behaviour (type/priority/size: enum-validated case-insensitive; module/tag/parent: exact-match on the canonical value).
- [ ] **OR semantics within a flag**: a task matches the flag when any of the supplied values matches that field. Across flags the existing AND semantics is preserved.
- [ ] **Tag flag audit**: confirm `-t` semantics — current help reads "filter by tag (exact match per tag)". If it already does OR-on-any-tag, document that and add explicit tests; if it currently AND's multiple tags, decide deliberately (recommend OR for symmetry with the other multi-value flags) and update help + tests + a one-line CHANGELOG note about the semantic change.
- [ ] **New `--agent` flag**: accepts comma-separated agent names (`claude`, `codex`, `none` to match unassigned). OR-within-flag. Exact-match on the canonical (lowercased) agent field; unknown agents return an error mirroring the existing enum-flag rejections.
- [ ] **New `--search` flag**: accepts a free-text term. Case-insensitive substring match against `id` and `title` (matches the GUI FilterBar's search box semantics). Single-value only (no comma split — commas are valid in titles).
- [ ] **Help text**: `tb ls --help` shows the multi-value-capable usage, e.g. `-T type[,type…]` instead of `-T type`. Each flag's docstring states "comma-separated, matches any" where applicable. The single positional example in the usage line stays the common case.
- [ ] **JSON output unchanged**: the existing `--json` shape stays byte-identical for single-value calls so frontend consumers (BoardService.LoadBoard) don't need a JSON-schema migration. Multi-value calls produce the same shape with the broader result set.
- [ ] **Backwards-compatibility**: every existing test in `cli/` that uses `-T`/`-p`/`-m`/`-s`/`-t`/`--parent` with a single value passes unchanged.
- [ ] **Tests**: dedicated table-driven tests in `cli/list_test.go` (or a new `cli/list_multi_test.go`) covering: (a) `-T bug,improvement` matches both; (b) `-p P0,P1` matches both; (c) `-m gui,cli` matches both; (d) `-s S,M` matches both; (e) `-t macos,window` matches tasks with either tag; (f) `--parent TB-1,TB-2` matches children of either; (g) `--agent claude,codex` matches both; (h) `--search router` finds the term in id or title case-insensitively; (i) multi-flag AND: `-T bug -p P1,P2 -m gui` returns the intersection; (j) empty/whitespace segments are tolerated.
- [ ] **Verification**: `cd cli && go test ./...`; the existing tb binary integration tests in gui/ (`buildTbForIntegration`) keep passing because the JSON shape is unchanged.
- [ ] **Help / docs**: update `tb ls`'s usage one-liner and per-flag descriptions. Add a short example to `cli/CLAUDE.md` ("Multi-value filter flags accept commas: `tb ls -T bug,improvement -p P0,P1 --status ready --json`"). No README/skill changes needed — this is additive.

## Non-goals
- Implementing the FilterBar-driven save button (TB-288 owns that, with a hard dependency on this task).
- Boolean operators beyond OR (no AND-within-field, no NOT). Comma is the only separator.
- A new query DSL (the goal is to keep flags simple and shell-friendly).
- Updating the GUI's `gui/internal/automation/query` package — that deletion is TB-288's scope.

## Related Tasks

- **TB-288** — Downstream consumer. TB-288 is blocked on this task: its plan is to delete `gui/internal/automation/query` and shell out to `tb ls --json` with the multi-value flags introduced here. Ship this first; TB-288 cannot land without it.
- **TB-178** — Originally introduced the in-GUI text-DSL parser this work supersedes. The parser package will be deleted as part of TB-288, not this task.
- **TB-179** — Auto-implement coordinator that currently calls `gui/internal/automation/query`. After TB-288 lands, the coordinator will instead call `BoardService.LoadBoard(... with flag args ...)` (or a new variant).

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited priority=P2, type=feature, size=M, tags=cli,filter,auto-implement,prereq-tb288, acceptance
- 2026-05-20: Edited tags=cli,filter,auto-implement,blocks-tb288

