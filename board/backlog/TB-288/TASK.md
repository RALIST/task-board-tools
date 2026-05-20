# TB-288: FilterBar-driven auto-implement query (replaces text DSL)

**Type:** feature
**Priority:** P2
**Size:** M
**Module:** gui
**Tags:** auto-implement,ux,filter,frontend,prereq-tb289
**Branch:** —

## Goal

Replace the freeform text-based auto-implement query with a structured filter that reuses the existing FilterBar UI. The user already has a multi-select filter component on the board (search, type, priority, module, tags, agent, epic, archived); auto-implement should consume the same data shape via a "Save as auto-implement query" button rather than asking the user to learn a text DSL.

The text-format parser introduced by TB-178 ships in M11 and has not had time to grow real-user data, so this task replaces it cleanly rather than maintaining both representations.

Blocked on **TB-289** (extend `tb ls` with multi-value filter flags). The auto-implement coordinator stops carrying its own matcher and instead shells out to `tb ls --json` with the flags TB-289 introduces. Land TB-289 first.

## Acceptance Criteria

- [ ] **Depends on TB-289**: `tb ls` already accepts multi-value `-T`, `-p`, `-m`, `-s`, `-t`, `--parent`, plus `--agent` and `--search`. This task assumes that shape; do not begin until TB-289 is in done.
- [ ] **Storage shape**: `Preferences.AutoImplementQuery` changes from `string` to a structured `AutoImplementFilter` JSON object mirroring the frontend `BoardFilter` shape: `{search: string, types: []string, priorities: []string, modules: []string, sizes: []string, tags: []string, agents: []string, parents: []string}`. Old `string` values in existing `preferences.json` are logged + reset to an empty filter (no real-user data exists yet).
- [ ] **Delete `gui/internal/automation/query`**: the package, its tests, and every import path are removed. No matcher code remains in the GUI repo for the persisted query; the CLI is the single source of truth.
- [ ] **Daemon coordinator**: `gui/app/auto_implement.go` no longer parses or matches in-process. The scan flow becomes: serialize the persisted `AutoImplementFilter` to `tb ls --status ready --json <flags>` arguments → run via a new `BoardService.ListWithFilter(ctx, filter)` helper → use the returned tasks as the candidate pool. Epic-order + active-run + non-blank-AgentStatus gates from TB-179 stay client-side.
- [ ] **Wails surface**: `SettingsService.GetAutoImplementQuery` returns the structured object; `SetAutoImplementQuery` takes it. `ValidateAutoImplementQuery` is removed (the structured form can't fail to parse). Enabling still requires a supported `default_agent` AND a non-empty filter (at least one field has at least one value); the SettingsService rejects the enable otherwise, same UX as today.
- [ ] **Frontend store**: `preferencesStore.autoImplementQuery` is the same shape as `BoardFilter`. `validateAutoImplementQuery` proxy is removed.
- [ ] **FilterBar button**: a "Save as auto-implement query" button appears in the FilterBar (near the existing Clear button), enabled only when at least one filter is active. Click serializes the current `$filter` store to `preferencesStore.setAutoImplementQuery` and surfaces a success toast.
- [ ] **Saved-query indicator** (optional but recommended): when the FilterBar's current state matches the persisted query, the button shows a subtle "saved" affordance so the user knows nothing's drifted.
- [ ] **Size selector in FilterBar**: confirm whether the current FilterBar exposes a Size dropdown. If not, add one (multi-select, S/M/L/XL) alongside Type/Priority/Module. Required so the original TB-178 AC fixture `bug, S size, gui` remains expressible end-to-end.
- [ ] **SettingsPanel cleanup**: remove the freeform text input + the parser-error inline warning. Replace with a read-only human-readable summary of the persisted filter (e.g., "Type: bug, improvement · Module: gui · Tags: macos") plus a "Edit in board filter" button that closes Settings and focuses the FilterBar. The toggle + needs-default-agent warning + needs-query warning (now "no filter saved") remain.
- [ ] **Migration story**: on first load, if the persisted query is the legacy string form, log a one-line warning and reset to an empty filter. Document in `docs/ARCHITECTURE.md` if it touches a shipped invariant. No-op for users on M11 or later who never saved a text query.
- [ ] **Test surface**: (a) `BoardService.ListWithFilter` round-trips empty / single-value / multi-value filter against a seeded board; (b) coordinator scan with a structured filter selects the same set as the existing AC fixture; (c) FilterBar "Save as auto-implement query" test asserts the store call shape; (d) SettingsPanel summary-render test asserts no text input is present and the summary reflects current filter; (e) preferences store legacy-string migration test.
- [ ] **Verification**: `cd gui && go test ./...`; `cd gui/frontend && npm run check`; `cd gui/frontend && npm test -- --run`; `make lint-go`.
- [ ] **Manual test note**: open Settings, observe the summary + "Edit in board filter" button (no text input); apply a filter on the board; click "Save as auto-implement query"; verify persisted filter; toggle auto-implement on; verify a matching ready task auto-starts; clear the FilterBar and confirm the saved query is unchanged until explicitly re-saved.

## Non-goals

- Re-introducing the text DSL as an advanced "expert mode" view. If a power user wants raw access they can hand-edit `preferences.json`.
- Multi-saved-query slots (one saved query per board today; multi-query is a separate future task).
- Cross-board query sharing.
- Implementing the multi-value flags on `tb ls` — TB-289 owns that.

## Related Tasks

- **TB-289** — Hard prerequisite. Adds the multi-value `-T,-p,-m,-s,-t,--parent` + new `--agent`,`--search` flags to `tb ls`. TB-288's coordinator and `ListWithFilter` helper assume those flags exist; do not start TB-288 until TB-289 is in done.
- **TB-177** — Parent auto-implement epic (closed). TB-288 is a follow-up that replaces only the freeform-query slice introduced under TB-178; the rest of the epic stays as shipped.
- **TB-178** — Freeform query parser this task replaces. `gui/internal/automation/query` is the package to delete.
- **TB-179** — Coordinator that currently calls `query.Match`. After TB-288, it shells out via `BoardService.ListWithFilter` instead.
- **TB-180** — Settings UI that needs the text input + parser-error inline warning replaced with a read-only summary + "Edit in board filter" button.
- **No task ID — FilterBar** — `gui/frontend/src/lib/components/FilterBar.svelte` is the UI being repurposed; `gui/frontend/src/lib/stores/filter.ts` is the `BoardFilter` shape the persisted query mirrors; `gui/frontend/src/lib/filtering.ts` is the existing in-process matcher whose semantics the CLI now owns for the persisted-query path.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited priority=P2, type=feature, size=L, tags=auto-implement,ux,filter,frontend,parent-tb177, acceptance
- 2026-05-20: Edited goal, acceptance
- 2026-05-20: Edited size=M, tags=auto-implement,ux,filter,frontend,prereq-tb289

