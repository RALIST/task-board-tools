# TB-175: GUI: surface auto-groom feedback and manual fallback

**Type:** feature
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** auto-groom,frontend,ux,groom
**Branch:** —
**Parent:** TB-172

## Goal

Make auto-groom visible and understandable in the GUI while preserving the existing manual Groom button whenever automation is disabled, unavailable, or not desired for a specific task.

## Context

- `gui/frontend/src/lib/components/Card.svelte` already shows a backlog-only needs-grooming indicator from `triageStore`.
- `gui/frontend/src/lib/components/TaskDrawer.svelte` already has a manual Groom button, default-agent display fallback, run history, and shared Cancel behavior.
- `gui/frontend/src/lib/stores/runs.ts` already tracks `mode: 'implement' | 'groom'`; auto-groom should reuse those events so the user can see what happened.
- TB-173 adds the user-facing setting; TB-174 adds the backend automation and no-default diagnostic.

**Constraints / non-goals**

- Do not remove or hide the manual Groom button. Turning auto-groom off must leave today's M6 manual flow intact.
- Do not add a second run-history model or a separate cancel path. Auto-groom runs are ordinary `mode=groom` runs in the existing list.
- UI copy should be actionable and short: if no default agent is configured, tell the user to set one in Settings.

## Acceptance Criteria

- [ ] With auto-groom disabled, triage indicators still open the drawer and emphasize the existing manual Groom button; clicking Groom starts a `mode=groom` run exactly as M6 does today.
- [ ] With auto-groom enabled and a valid default agent, a triaged card/drawer shows clear queued/running/success/failure feedback for the automatic groom run using existing run events and run history; Cancel still cancels the active groom run.
- [ ] With auto-groom enabled but `default_agent=none`, the GUI shows an actionable message that the user must set a default agent in Settings; no silent failure and no raw backend error is shown.
- [ ] The manual Groom button remains available for explicit user retries even if the task was skipped by the auto-groom dedupe guard.
- [ ] Vitest/Svelte tests cover disabled mode, enabled-with-default mode, enabled-without-default message, and the manual Groom fallback staying available.
- [ ] Verification passes: `cd gui/frontend && npm run check`; `cd gui/frontend && npm test -- --run`.
- [ ] Manual test: set `default_agent=codex`, enable auto-groom, create a placeholder backlog task, confirm it visibly queues/runs as `groom`; disable auto-groom, create another placeholder task, confirm only the needs-grooming indicator/manual Groom path appears; set default agent to `none` while enabled and confirm the warning appears.

## Related Tasks

- **TB-72** — Existing TaskDrawer manual Groom button and run-history mode label.
- **TB-73** — Existing Card needs-grooming indicator.
- **TB-80** — Preferences store/API path.
- **TB-81** — Settings panel surface.
- **TB-173** — Adds the setting consumed here.
- **TB-174** — Provides backend automation state and diagnostics.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

