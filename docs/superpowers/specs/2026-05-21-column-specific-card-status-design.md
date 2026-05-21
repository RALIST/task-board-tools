# Column-Specific Card Status Design

## Goal

Board cards should show the lifecycle status that belongs to the card's current column, so users can quickly see whether the relevant stage is queued, running, stale, lost, blocked, or terminal without reading a previous-stage agent/status pair.

## Scope

This is a TB-303/TB-309 frontend display requirement. It changes card display only. It does not change the drawer, backend run lifecycle, daemon scheduling, JSONL history, or per-mode status fields.

## Current Problem

Cards can show multiple per-mode chips or fall back to generic task-level agent state. That makes it easy to read a stale previous mode as current state. Example: a task in `ready` should communicate implement readiness/retry state, not the last groom or review agent.

## Column Mapping

Cards display one column-specific mode chip:

- `backlog` shows `GroomedBy` / `GroomStatus`.
- `ready` shows `ImplementedBy` / `ImplementStatus`.
- `in-progress` shows `ImplementedBy` / `ImplementStatus`.
- `code-review` shows `ReviewedBy` / `ReviewStatus`.
- `done` shows `ReviewedBy` / `ReviewStatus`.

`archive` follows `done` if rendered in an archived board view, because archived tasks should preserve the final review/done signal rather than imply active implementation.

## Display Rules

The card chip uses only the mode-specific agent/status pair selected by the column mapping. It must not fall back to generic `Agent`, generic `AgentStatus`, or another mode's `*By` / `*Status` pair.

If the selected pair is empty, the card shows no stage chip or a neutral empty stage state, depending on the existing card layout. It must not show a previous mode's status to fill the space.

`review-failed` remains a separate marker. A `ready` task with the `review-failed` tag still shows the implementation chip, plus the existing review-failed marker.

The drawer continues to show the full G/I/R history. This design intentionally affects only dense card scanning.

## Data Flow

The card receives the existing task DTO fields:

- `groomedBy`, `groomStatus`
- `implementedBy`, `implementStatus`
- `reviewedBy`, `reviewStatus`
- `status`
- `tags`

Card logic derives a single display model from `status`, then renders the selected mode label, agent, and status. No extra backend field is required.

After TB-303 removes generic `agentStatus`, card code should still compile because the current-column chip has no dependency on that field.

## Testing

Frontend tests should cover:

- backlog card renders groom status only.
- ready card renders implement status only.
- in-progress card renders implement status only.
- code-review card renders review status only.
- done card renders review status only.
- ready card with `review-failed` still renders implement status and the review-failed marker.
- card does not render a previous mode's status when the selected column mode is blank.
- card tests do not require `agentStatus` for current lifecycle display.

Manual GUI smoke for TB-309 should confirm card scanning across all columns after reload: cards show the column-relevant G/I/R status, and no generic `AgentStatus` chip appears.

## Open Decisions

None. User approved cards-only behavior and the `review-failed` ready-column rule on 2026-05-21.
