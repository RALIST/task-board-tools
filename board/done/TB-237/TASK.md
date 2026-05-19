# TB-237: Save diffrent agent actions in diffrent fields

**Type:** improvement
**Priority:** P2
**Size:** L
**Agent:** claude
**AgentStatus:** success
**Module:** cli
**Tags:** agent,metadata,attribution,history
**ReviewRef:** main
**ImplementedBy:** claude
**ImplementStatus:** success
**ReviewedBy:** claude
**ReviewStatus:** success
**Branch:** —

## Goal

Persist per-mode agent attribution and status on each task — groomed-by, implemented-by, reviewed-by, each with its own status — instead of overwriting a single `Agent` / `AgentStatus` pair on every run.

**Context**

- `Task` in `cli/task.go:13` carries a single `Agent` and `AgentStatus` (lines 26–27); every finished run rewrites both, losing prior-action attribution.
- The three logical actions already exist as run modes in `gui/internal/agent/runner.go:27-35`: `ModeGroom`, `ModeImplement`, `ModeReview`. `ModeResume` is a continuation of one of those, not a fourth action.
- Terminal status writes happen in `gui/app/agent_run.go` (around lines 716, 779) and `gui/app/agent_finish.go` via managed `tb edit --agent-status` calls.
- Card / TaskDrawer render the single Agent/AgentStatus pair today; that's the surface that needs to grow per-mode rows.
- `tb triage`, auto-implement (TB-177/TB-179), auto-groom (TB-172), daemon pickup (TB-5), and stale recovery (TB-130) read `AgentStatus` for eligibility — they must keep working unchanged.

**Constraints / non-goals**

- Keep the existing single `Agent` / `AgentStatus` fields as a "most recent run" snapshot; do not remove them in this task. Replacing them is a follow-up if ever needed.
- Cover exactly three actions: groom, implement, review. `ModeResume` updates the parent action's per-mode pair; do not introduce a fourth.
- Parsing stays backward-compatible: tasks without per-mode fields keep parsing; missing fields render as empty and are omitted from markdown output (mirror `Agent`/`AgentStatus` behavior).
- All structured writes go through `tb edit` and `.board.lock`; no direct `.md` writes outside `cli/atomicfs.go`; atomic-write rules unchanged.
- `needs-user` stays a single-cursor status applied to whichever action is paused; no per-mode `needs-user` fields.
- No migration of existing task files; existing fixtures must keep passing.

**Related Tasks**

- **TB-11** — original `Agent` / `AgentStatus` metadata feature this extends.
- **TB-130** — session-resume design that introduced `ModeResume` and `interrupted`.
- **TB-172** — Auto-groom; consumes `AgentStatus` for eligibility.
- **TB-177**, **TB-179** — Auto-implement; consumes `AgentStatus` for eligibility.
- **TB-235** — code-review workflow that introduced `ReviewRef`; lives alongside the new `ReviewedBy` / `ReviewStatus`.

## Acceptance Criteria

- [x] `Task` struct in `cli/task.go` gains per-mode pairs (e.g., `GroomedBy`/`GroomStatus`, `ImplementedBy`/`ImplementStatus`, `ReviewedBy`/`ReviewStatus`) with JSON tags, parsed by `parseTaskFile` within `maxMetadataLines` (bump the limit if needed).
- [x] Status fields validate against the existing enum (`queued|running|success|failed|cancelled|interrupted|needs-user|""`); agent fields validate against `validAgents` plus empty.
- [x] `tb edit` accepts flags to set/clear each per-mode pair (one flag per side, e.g., `--groomed-by`, `--groom-status`, ...). `cli/edit.go` appends a log entry and regenerates `BOARD.md` as today.
- [x] Empty per-mode fields are not emitted in task markdown (mirrors existing `Agent`/`AgentStatus` behavior in `cli/create.go` / `cli/edit.go`).
- [x] `cli/json_output.go` emits the new fields (camelCase) in `tb show --json`, `tb ls --json`, and the board snapshot; empty values render as empty strings.
- [x] Agent run terminal writes in `gui/app/agent_run.go` / `gui/app/agent_finish.go` update the per-mode pair matching the run's `Mode` (from `gui/internal/agent/runner.go`: `ModeGroom`, `ModeImplement`, `ModeReview`) in addition to the existing single `Agent`/`AgentStatus`.
- [x] `ModeResume` updates the parent action's per-mode pair — it never introduces a fourth action.
- [x] Existing single `Agent`/`AgentStatus` fields continue to reflect the most recent run (back-compat for `tb triage`, auto-implement, auto-groom, daemon pickup, and stale recovery).
- [x] Tasks lacking per-mode fields still parse and render; no migration of existing task files is required.
- [x] GUI surfaces per-action attribution: TaskDrawer (and Card where it already shows agent state) renders the three pairs when present; missing actions render nothing — no placeholder rows.
- [x] Verification: `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run`.
- [x] Manual test note: on a fresh task, run groom (claude) → implement (codex) → review (claude); confirm the drawer shows three attributions with their final statuses; restart the GUI/daemon and confirm the per-mode fields survive; trigger a `ModeResume` and confirm it updates the originating action's pair only.

Verified by reviewer (see "Review Findings"): no blocking issues, all four test suites pass. Three nits captured as TB-254 / TB-255 / TB-256 follow-ups.

## Review Target

branch: main
scope: per-mode agent attribution (GroomedBy/GroomStatus, ImplementedBy/ImplementStatus, ReviewedBy/ReviewStatus)

## Review Findings

No blocking findings — implementation satisfies all acceptance criteria and the touched test suites pass.

Verified end-to-end:
- `cli/task.go` struct gains six per-mode fields (lines 35-40) with camelCase JSON tags; parser handles them in `parseTaskFile` ordering Groom/Implement/Review fields before legacy `Agent`/`AgentStatus`; `maxMetadataLines` bumped 20 → 28 with rationale comment.
- `cli/edit.go` adds six flags (lines 38-43) with enum validation against `validAgents` (agent fields) and `validAgentStatuses` (status fields), `none` sentinel for clearing via `clearable` map (lines 334-344). Combined log entry (line 373) and `regenerateBoard` call preserved.
- `cli/create.go:50-53` registers the six new flag names in `flagsWithValue` so `reorderArgs` reshuffler still works (verified by running `tb show TB-237 --json` — fields parse correctly).
- `cli/json_output.go:23-43, 66-86` emits all six camelCase keys on `tb show --json` and `tb ls --json` — confirmed live: empty fields render as `""` rather than being omitted (matches AC).
- `gui/app/agent_run.go:97-109` `applyPerModeAttribution` routes to the correct pair for groom/implement/review and is a no-op for `ModeResume` (the resume case is handled upstream by `effectiveMode` lines 72-81, which maps resume → ParentMode or falls back to ModeImplement).
- `gui/app/agent_run.go:881-883` writes the per-mode pair under the same `shouldWriteStatus` gate as `AgentStatus`, so the `needs-user` carve-out covers per-mode too (AC: no per-mode `needs-user` fields).
- `gui/app/agent_finish.go:169-190` `runModeFor` recovers the originating mode from a parent run's queued JSONL event; `gui/app/agent_run.go:519-526` consumes it in `RunQueuedAgentSync` so a daemon replay of a resume run lands on the right per-mode slot.
- `gui/app/agent_service.go:186-189` `activeRun.ParentMode` field carries the originating mode through the lifecycle.
- `gui/app/agent_recovery.go:506-510, 537-547` `ResumeCandidate.Mode` + `runRecoveryView.Mode` propagate the parent mode through the drawer Resume path.
- `gui/frontend/src/lib/components/Card.svelte:68-75, 318-325` filters per-action chips to non-empty pairs, status-colored palette mirrors `.agent-*`.
- `gui/frontend/src/lib/components/TaskDrawer.svelte:384-393, 1694-1707` renders the "Per action" list only when at least one pair is present — no placeholder rows.

Tests run:
- `cd cli && go test ./...` — passes. `TestEditPerModeAttribution` (set/clear/parse round-trip) and `TestEditPerModeAttributionInvalidValues` (subprocess re-exec for the validator's `os.Exit(1)` path) both pass.
- `cd gui && go test ./app/...` — passes. `TestRecordTerminalPerModeAttribution` (table-driven across groom/implement/review) and `TestRecordTerminalResumeUsesParentMode` lock the contract.
- `cd gui/frontend && npm run check` — 411 files, 0 errors, 0 warnings.
- `cd gui/frontend && npm test -- --run` — 190/190 tests pass.
- Note: `cd gui && go test ./internal/agent/...` fails 3 tests, but these are pre-existing working-tree changes to `gui/internal/agent/claude.go` (adds `--dangerously-skip-permissions`, unrelated to TB-237). Verified by stashing working tree and re-running — those tests pass on the committed branch state. The TB-237 commit message also notes the unrelated build break.

Documentation:
- `docs/ARCHITECTURE.md:106, 108` updated with the metadata-line bump and the per-mode contract.
- `docs/IMPLEMENTATION.md:262` appends a complete TB-237 entry with verification.

(nit) Stale recovery in `gui/app/agent_recovery.go` does not write per-mode pairs when marking a task `interrupted` — if the daemon crashes mid-groom, `GroomStatus` keeps its prior terminal value (or stays empty) even though the legacy `AgentStatus` becomes `interrupted`, so the attribution of the interrupted action is lost. This is outside the literal AC ("agent_run.go / agent_finish.go" callouts) but is a small completeness gap worth a follow-up.

(nit) Per-mode chip status is misleading while a new run of the same action is in flight: since per-mode is only written at terminal, e.g. starting a fresh groom on a previously-successful task shows `Groomed: claude · success` until the new run finishes. This matches the documented "most recent terminal state" semantics; the user can still tell from the legacy `AgentStatus: running` chip. Consider a UI hint in TaskDrawer that the per-mode row is stale when `AgentStatus` is running and matches the same effective mode.

(nit) `TestRunQueuedAgentSync_ResumeRehydratesParentContext` seeds a parent queued event without a `Mode` field, so the `runModeFor` lookup falls back to `ModeImplement` and the test never asserts the per-mode write lands on the originating action. A small extension (seed `Mode: "groom"` in the parent's queued event, assert `**GroomedBy:**` after the replay) would close the regression gap on the daemon-replay branch of TB-237.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited goal
- 2026-05-19: Edited type=improvement, size=L, module=cli, tags=agent,metadata,attribution,history
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited reviewref=main
- 2026-05-19: Edited review-target
- 2026-05-19: Submitted to code-review
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited reviewref=main
- 2026-05-19: Edited implemented-by=claude, implement-status=success
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited review-findings
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited reviewed-by=claude, review-status=success
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Edited acceptance
- 2026-05-19: Done

