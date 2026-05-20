# TB-296: Refactor GUI header: condense controls so they fit on one row

**Type:** improvement
**Priority:** P2
**Size:** M
**Tags:** ui,gui
**Agent:** claude
**AgentStatus:** running
**Module:** gui
**GroomedBy:** claude
**GroomStatus:** success
**Branch:** —

## Goal

Redesign the GUI top bar in `gui/frontend/src/routes/+page.svelte` so it fits on a single row at typical window widths without wrapping, while keeping all current actions reachable. The current bar (project title + path, two agent-usage pills, refresh, `+ New`, `Auto-groom`, `Auto-impl`, `Settings`, `Open board…`) is overcrowded and wraps `Open board…` to a second line — see the attached screenshot. Reorganise/group/condense the controls (e.g. collapse low-frequency actions into an overflow menu, merge the two agent-usage chips into a more compact form, move the project path under the title, or similar) so the header looks calm, scannable, and proportionate.

## Acceptance Criteria

- [ ] At a 1280px-wide window, the entire top bar renders on a single row with no wrapping; `Open board…` no longer drops to a second line.
- [ ] All current header affordances remain reachable: project title, project path (full path still visible on hover/tooltip), claude + codex usage, refresh-usage, `+ New`, Auto-groom toggle, Auto-implement toggle, Settings, Open board….
- [ ] Auto-groom and Auto-implement controls keep their existing semantics (on/off state, `needs-default` warning style, disabled-while-busy, tooltips, `data-testid="auto-implement-pill"`).
- [ ] `AgentUsageHeader` still shows claude and codex usage with severity colouring and the unavailable/unknown states, but in a more compact layout (e.g. a single combined chip group, icon-only labels, or a dropdown summary).
- [ ] No regressions in existing tests: `npm run check`, `npm run lint`, `npm test` in `gui/frontend/` all pass.
- [ ] Manual test note recorded under `## Log` after verification: run `cd gui && task dev`, confirm header fits on one row at standard window widths, toggle Auto-groom and Auto-impl, open Settings, open another board via the picker, hover the project path to see full path, and confirm both usage chips still render and refresh.
- [ ] Double-click on the bar still triggers `onTopbarDblClick` (window drag/maximise behaviour preserved).

## Attachments

- Снимок экрана 2026-05-20 в 11.43.35.png

## Log

- 2026-05-20: Created
- 2026-05-20: Attached Снимок экрана 2026-05-20 в 11.43.35.png
- 2026-05-20: Edited body via GUI
- 2026-05-20: Edited type=improvement
- 2026-05-20: Edited tags=ui
- 2026-05-20: Edited tags=Ui
- 2026-05-20: Edited tags=Uiui
- 2026-05-20: Edited tags=Uiuiui
- 2026-05-20: Edited agent=claude
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited module=gui, tags=ui,gui, title=Refactor GUI header: condense controls so they fit on one row
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=claude, groom-status=success
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Implemented refactor. AgentUsageHeader chips render as `agent N% · N%` (window labels + "used" suffix moved into tooltip); title and project path stack vertically with path capped at max-width 220px (ellipsis on overflow, full path on hover); Auto-groom/Auto-impl labels lose the ": on/off" suffix (colored dot + aria-pressed convey state); pills/buttons get `white-space: nowrap`; the macOS `.actions { flex-wrap: wrap }` rule that wrapped "Open board…" to a second line is removed. `needs-default` warning styling, tooltips, disabled-while-busy semantics, and `data-testid="auto-implement-pill"` all preserved. `onTopbarDblClick` untouched. Verified `npm run check` / `npm run lint` / `npm run build` / `npm run deadcode` clean; `npm test` 229 pass with 2 pre-existing Card.test.ts resume-gating failures unrelated to this change (confirmed by re-running tests with these edits stashed). Layout fit at 1280px confirmed via an isolated CSS mock screenshot with all topbar items present. Full live `task dev` Wails session and chip-refresh interaction not exercised in this autonomous run; reviewer should hover the project path, toggle both pills, open Settings + the board picker, double-click the bar for window zoom, and confirm both usage chips refresh.

