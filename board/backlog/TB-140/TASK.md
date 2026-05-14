# TB-140: Frontend: Resume button, interrupted pill, resumed_from chip

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb130,agent,resume,frontend
**Branch:** —
**Parent:** TB-130

## Goal

Surface the resume capability in the Svelte UI: a Resume button
distinct from Run, an `interrupted` status pill in a colour distinct
from `failed`, and a `resumed_from` chip on resumed runs in the run
history.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 10 (Frontend), § 12 task J.

Resume must be visually distinct from Run so users don't confuse
"continue conversation" with "start fresh" (the latter loses the
agent's accumulated context). Per spec scope, M1 only offers Resume
when AgentStatus is `interrupted`; resume from finished runs is
documented as a follow-up (§ 13).

## Acceptance Criteria

- [ ] `gui/frontend/src/lib/api.ts` exports `ResumeAgent(id)` →
      bound to the Wails-generated `AgentService.ResumeAgent`.
- [ ] `Card.svelte` (`gui/frontend/src/lib/components/Card.svelte`):
      when `metadata.agentStatus === "interrupted"` AND
      `metadata.agent` is set, render a Resume icon button
      (distinct icon from Run — e.g. play-with-arrow vs play).
      Tooltip: "Resume the previous agent session".
- [ ] `TaskDrawer.svelte`
      (`gui/frontend/src/lib/components/TaskDrawer.svelte`): same
      Resume button in the action row, ONLY visible when
      AgentStatus is `interrupted`. Greyed-out / hidden otherwise
      with a tooltip explaining the M1 scope.
- [ ] AgentStatus pill colour for `interrupted`: a neutral-warm tone
      (e.g. amber/orange) — distinct from `failed`'s red and
      `cancelled`'s grey. Matches the existing pill component
      styling system.
- [ ] Run history rows: when a Run has `resumedFromRun`, render a
      chip "↻ resumed from r_xxxx" (uses the runID, not the internal
      sessionID). Click on the chip scrolls/jumps to the source run
      in the history.
- [ ] Optimistic UI: clicking Resume immediately reflects
      `AgentStatus: queued` and a placeholder run row with `mode:
      "resume"` (matches existing pattern for Run / Groom — see
      `TaskDrawer.svelte:287, 315`).
- [ ] Frontend tests (Vitest):
  - Card renders Resume button only for `interrupted` status.
  - TaskDrawer renders Resume button only for `interrupted` status.
  - `resumedFromRun` chip renders with the expected runID.
  - Clicking Resume calls `api.ResumeAgent(taskID)` exactly once.

## Related Tasks

- **TB-130** — parent epic.
- Depends on **TB-131** (frontend AgentStatus / mode unions and
  `TaskDrawer.svelte` mode-row hooks), **TB-137** (recovery produces
  `interrupted`), **TB-138** + **TB-139** (`ResumeAgent` backend).

## Log

- 2026-05-14: Created
