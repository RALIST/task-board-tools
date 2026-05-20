# TB-309: Frontend: remove generic AgentStatus display dependency

**Type:** improvement
**Priority:** P2
**Size:** M
**Module:** gui-frontend
**Tags:** agent-status,per-mode-fields,ui
**Branch:** —
**Parent:** TB-303

## Goal

Update cards, drawer, run controls, and status chips so the UI communicates per-mode groom/implement/review state without depending on a generic AgentStatus field.

## Context

- Parent epic: TB-303.
- Frontend components currently receive and/or render generic task agent status while also showing per-action fields added by TB-237.
- UI work should follow the backend contract from TB-307/TB-308 and make the active/terminal state understandable without a single generic status chip.
- Likely surfaces include `gui/frontend/src/lib/api.ts`, task/run stores, Card, TaskDrawer, run controls, resume/cancel controls, tooltips, and tests.

## Constraints

- Do not hide manual Run, Groom, Review, Cancel, Clear needs-user, or Resume actions; update their enabled/disabled logic to the new state source.
- Show per-mode state compactly enough for dense board cards, and reserve detailed status/history for the drawer.
- Keep accessibility labels, focus behavior, and responsive layout intact.
- Coordinate with TB-308 so the frontend does not invent client-only lifecycle truth.

## Acceptance Criteria

- [ ] Frontend task DTO/types and API wrappers no longer require or render `agentStatus` for current task state.
- [ ] Cards and TaskDrawer show groom/implement/review attribution and active/terminal state from per-mode fields and run history, including queued/running indicators for the active mode.
- [ ] Run, Groom, Review, Cancel, Resume, and needs-user controls enable/disable from the new backend state without regressions in existing manual flows.
- [ ] Vitest coverage is updated for the removed generic field and the replacement per-mode display/control logic.
- [ ] Verification: `cd gui/frontend && npm run check` and `cd gui/frontend && npm test -- --run` pass.
- [ ] Manual test note: in the GUI, run one task through Groom, Implement, Review, Cancel, and Resume paths where practical; confirm cards and drawer never show a generic `AgentStatus` chip and the per-mode state stays understandable after reload.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
