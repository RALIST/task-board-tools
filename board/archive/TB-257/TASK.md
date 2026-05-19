# TB-257: Stale recovery: also update per-mode pair when marking interrupted

**Type:** tech-debt
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** agent,metadata,attribution,history
**Agent:** claude
**AgentStatus:** success
**GroomedBy:** claude
**GroomStatus:** success
**Branch:** —

## Goal

Duplicate — superseded by [TB-254](../TB-254/TASK.md) (same nit, same fix shape). TB-237 review followups were created concurrently by another agent and surfaced first; closing this entry to avoid the board carrying two copies of the same work. Refer to TB-254 for pickup.

## Acceptance Criteria

- [ ] (to be filled)

## User Attention

Reason: Conflicting with TB-254 (duplicate). Both tasks are the same TB-237 review follow-up — gui/app/agent_recovery.go marks AgentStatus=interrupted (or failed) on dead-PID recovery but does not also update the per-mode pair (GroomedBy/GroomStatus, ImplementedBy/ImplementStatus, ReviewedBy/ReviewStatus) corresponding to the run's originating mode. Both propose the same fix surface (recovery path in agent_recovery.go) with the same expected outcome.

Question/Action: Decide which task to keep and which to close as a duplicate.
- Option A: Keep TB-257 (richer body, explicit TB-237 review nit #1 reference, names the helper functions to reuse — runModeFor at gui/app/agent_finish.go:169-190 and effectiveMode at gui/app/agent_run.go:72-81, plus the "out-of-scope" rationale for why it was deferred). Close TB-254.
- Option B: Keep TB-254 (lower ID, already in backlog, listed in the TB-237 review block as one of the three captured follow-ups). Close TB-257 and optionally port TB-257's longer references (runModeFor, effectiveMode, "out of scope" rationale) into TB-254 before grooming.

Attempted context:
- Read both task bodies. TB-254 title: "Stale recovery should write per-mode pair when marking interrupted"; TB-257 title: "Stale recovery: also update per-mode pair when marking interrupted". Identical scope.
- TB-254 references ResumeCandidate.Mode (gui/app/agent_recovery.go:506-510) as the implementation hint; TB-257 references runModeFor (already exists at gui/app/agent_finish.go:169-190) and the effectiveMode resume fallback (gui/app/agent_run.go:72-81). Both fields are available in runRecoveryView and could feed cli.EditInput.GroomedBy/GroomStatus/etc.
- Confirmed in gui/app/agent_recovery.go: markInterrupted (lines 203-224) and markFailed (lines 250-273) both call c.Edit(ctx, t.ID, cli.EditInput{AgentStatus: …}) without setting the per-mode pair. Same gap exists in the cancelled carve-out (lines 138-145), the JSONL-finished sync (lines 149-158), and syncFinishedStatus (lines 377-387).
- TB-237's done body explicitly defers nit #1 to TB-257 (board/done/TB-237/TASK.md), but TB-254 already existed before that link was added — the TB-237 review block lists TB-254/TB-255/TB-256 as the original three follow-up tasks, and TB-257/TB-258 appear to be duplicates created later.
- Both tasks: Type=tech-debt(257)/tech-debt(254 — confirmed), Priority=P2, Size=S, Module=gui, AC empty. TB-254 has tag "recovery"; TB-257 has tag "history" — otherwise identical.
- This mirrors TB-255 ⇄ TB-258, where the duplicate (TB-258) was halted with needs-user for the same reason.

Unblock condition: User says which task to keep (TB-254 or TB-257). After that, the kept task can be groomed (Goal/AC filled covering both the interrupted and failed branches, plus the resume → parent-mode fallback per effectiveMode) and the duplicate closed with `tb close <ID>` plus a `## Related Tasks` cross-link.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited user-attention
- 2026-05-19: Edited agentstatus=needs-user
- 2026-05-19: Edited agentstatus=none
- 2026-05-19: Edited agentstatus=success, groomed-by=claude, groom-status=success
- 2026-05-19: Edited goal
- 2026-05-19: Closed (archived from backlog)

