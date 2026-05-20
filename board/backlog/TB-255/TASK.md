# TB-255: TaskDrawer marks stale per-action attribution during same-mode run

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** agent,metadata,attribution,ux,quick-win
**Agent:** codex
**AgentStatus:** success
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Show when a TaskDrawer per-action attribution row is stale because a new run for that same action is currently queued or running, while preserving the previous terminal attribution value until the backend writes the next terminal per-mode pair.

**Context**

- Follow-up from TB-237, which added `GroomedBy` / `GroomStatus`, `ImplementedBy` / `ImplementStatus`, and `ReviewedBy` / `ReviewStatus` as terminal per-mode attribution fields.
- Current `TaskDrawer.svelte` builds `perActionAttributions` from task metadata and renders the "Per action" list, so a row like `Groomed: claude · success` can remain visually prominent while `AgentStatus` and run history show that a new groom run is in flight.
- The legacy `Agent` / `AgentStatus` chip remains the live most-recent-run snapshot; this task is only about making the stale per-action row visually honest.
- Relevant files: `gui/frontend/src/lib/components/TaskDrawer.svelte`, `gui/frontend/src/lib/components/TaskDrawer.test.ts`, and `gui/frontend/src/lib/stores/runs.ts` for active run mode/status data.

**Constraints / non-goals**

- Frontend UX fix only; do not change when backend terminal writes update the per-mode metadata fields.
- Mark only the row whose mode matches the active queued/running run (`groom`, `implement`, or `review`). Rows for other modes stay normal.
- Keep the previous terminal agent/status visible; add a stale/re-running hint rather than hiding or overwriting it.
- Preserve existing busy gating, run history, resume, and `needs-user` behavior.

**Related Tasks**

- TB-237 — introduced per-mode attribution and recorded this as follow-up nit #2.

**Manual test note**

On a task that already has a terminal per-action row, start the same action again from the GUI. While the run is queued/running, confirm the matching row shows a stale/re-running hint and other per-action rows remain normal; after the run finishes and metadata refreshes, confirm the stale hint clears.

## Acceptance Criteria

- [ ] TaskDrawer derives the active run mode for the selected task from the queued/running run history and treats the matching per-action attribution row as stale while task `AgentStatus` is `queued` or `running`.
- [ ] The stale row remains readable but visibly non-current, with accessible text such as a `stale`, `re-running`, or `updating` badge; the previous terminal agent/status values are still shown.
- [ ] Only the row matching the active run mode is marked stale; non-matching per-action rows and terminal/no-active-run states render normally.
- [ ] `gui/frontend/src/lib/components/TaskDrawer.test.ts` covers the stale same-mode case plus at least one negative case where the row must not be marked stale.
- [ ] Verification: `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- src/lib/components/TaskDrawer.test.ts` pass.
- [ ] Manual test note above is completed or explicitly deferred in the task log before closing.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited tags=agent,metadata,attribution,ux,quick-win, title=TaskDrawer marks stale per-action attribution during same-mode run, goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=success, groomed-by=codex, groom-status=success
