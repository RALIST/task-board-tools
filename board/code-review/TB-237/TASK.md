# TB-237: Save diffrent agent actions in diffrent fields

**Type:** improvement
**Priority:** P2
**Size:** L
**Agent:** claude
**AgentStatus:** running
**Module:** cli
**Tags:** agent,metadata,attribution,history
**ReviewRef:** main
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

- [ ] `Task` struct in `cli/task.go` gains per-mode pairs (e.g., `GroomedBy`/`GroomStatus`, `ImplementedBy`/`ImplementStatus`, `ReviewedBy`/`ReviewStatus`) with JSON tags, parsed by `parseTaskFile` within `maxMetadataLines` (bump the limit if needed).
- [ ] Status fields validate against the existing enum (`queued|running|success|failed|cancelled|interrupted|needs-user|""`); agent fields validate against `validAgents` plus empty.
- [ ] `tb edit` accepts flags to set/clear each per-mode pair (one flag per side, e.g., `--groomed-by`, `--groom-status`, ...). `cli/edit.go` appends a log entry and regenerates `BOARD.md` as today.
- [ ] Empty per-mode fields are not emitted in task markdown (mirrors existing `Agent`/`AgentStatus` behavior in `cli/create.go` / `cli/edit.go`).
- [ ] `cli/json_output.go` emits the new fields (camelCase) in `tb show --json`, `tb ls --json`, and the board snapshot; empty values render as empty strings.
- [ ] Agent run terminal writes in `gui/app/agent_run.go` / `gui/app/agent_finish.go` update the per-mode pair matching the run's `Mode` (from `gui/internal/agent/runner.go`: `ModeGroom`, `ModeImplement`, `ModeReview`) in addition to the existing single `Agent`/`AgentStatus`.
- [ ] `ModeResume` updates the parent action's per-mode pair — it never introduces a fourth action.
- [ ] Existing single `Agent`/`AgentStatus` fields continue to reflect the most recent run (back-compat for `tb triage`, auto-implement, auto-groom, daemon pickup, and stale recovery).
- [ ] Tasks lacking per-mode fields still parse and render; no migration of existing task files is required.
- [ ] GUI surfaces per-action attribution: TaskDrawer (and Card where it already shows agent state) renders the three pairs when present; missing actions render nothing — no placeholder rows.
- [ ] Verification: `cd cli && go test ./...`, `cd gui && go test ./...`, `cd gui/frontend && npm run check`, `cd gui/frontend && npm test -- --run`.
- [ ] Manual test note: on a fresh task, run groom (claude) → implement (codex) → review (claude); confirm the drawer shows three attributions with their final statuses; restart the GUI/daemon and confirm the per-mode fields survive; trigger a `ModeResume` and confirm it updates the originating action's pair only.

## Review Target

branch: main
scope: per-mode agent attribution (GroomedBy/GroomStatus, ImplementedBy/ImplementStatus, ReviewedBy/ReviewStatus)

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

