# TB-258: TaskDrawer: hint when per-mode row is stale during a fresh same-action run

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** agent,metadata,attribution,ux
**Agent:** claude
**Branch:** —

## Goal

Duplicate — superseded by [TB-255](../TB-255/TASK.md) (same nit, same fix shape). TB-237 review followups were created concurrently by another agent and surfaced first; closing this entry to avoid the board carrying two copies of the same work. Refer to TB-255 for pickup.

## Acceptance Criteria

- [ ] (to be filled)

## User Attention

Reason: Conflicting with TB-255 (duplicate). Both tasks are TB-237 follow-up nits about the same UX issue — per-mode chip in TaskDrawer is stale while a fresh same-mode run is in flight — and propose the same fix (dim/hint the per-mode row while the run is queued/running).

Question/Action: Decide which task to keep and which to close as a duplicate.
- Option A: Keep TB-258 (richer Goal/context, explicit Fix idea pointing at TaskDrawer.svelte's per-action list, queued + running gating). Close TB-255.
- Option B: Keep TB-255 (lower ID, already in backlog). Close TB-258 and (optionally) port TB-258's longer context block into TB-255 before grooming.

Attempted context:
- Read both: TB-258 goal explicitly cites TB-237 review nit #2 and proposes dimming + "(updating…)" hint on the matching per-action row when AgentStatus is running/queued.
- TB-255 goal is the same shape but shorter: "TaskDrawer surfaces a hint (e.g. dim/strike or 'stale — re-running' badge) on the per-mode row whose mode matches AgentStatus=running."
- Both: Type=improvement, Priority=P2, Size=S, Module=gui, overlapping tags (agent, metadata, attribution, ux on TB-258; agent, metadata, attribution, ux on TB-255).
- Code surface confirmed in gui/frontend/src/lib/components/TaskDrawer.svelte:384-393 (perActionAttributions derivation) and 1694-1707 (per-action list render). Active-run signal already derived at 353-355 (taskHasActiveRun via effectiveRuns).
- The TB-237 review block in this branch lists TB-254/TB-255/TB-256 as the three captured follow-ups; TB-258 was created later and reproduces TB-255's scope.

Unblock condition: User says which task to keep (TB-255 or TB-258). After that, the kept task can be groomed (Goal/AC filled) and the duplicate closed with `tb close <ID>` plus a `## Related Tasks` cross-link.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited user-attention
- 2026-05-19: Edited agentstatus=needs-user
- 2026-05-19: Edited agentstatus=none
- 2026-05-19: Edited goal
- 2026-05-19: Closed (archived from backlog)

