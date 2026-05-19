# TB-266: Daemon: reconcile autonomous stage transitions

**Type:** improvement
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** automation,daemon,housekeeping,kanban
**Branch:** —

## Goal

Add soft deterministic reconciliation for the staged autonomous flow so daemon-owned runs help tasks land in the correct column when an agent omits a safe board move.

## Context

- The intended autonomous flow is staged: auto-groom moves backlog -> ready, auto-implement moves ready -> in-progress -> code-review, and auto-review moves code-review -> done or ready.
- Prompts should ask agents to move tasks, but agents are nondeterministic. The daemon can safely repair some objective board states without guessing product intent.
- Existing daemon code already has startup scan, watcher rescan, periodic stale recovery, active-set dedupe, and `AgentStatus` inspection.

## Constraints / Non-goals

- "Soft" means only perform deterministic moves when objective markers prove the intended state. Otherwise log/diagnose and leave the task alone.
- Use managed CLI operations (`tb ready`, `tb pull <ID>`, `tb review --submit`, `tb review --fail`, `tb done`, or `tb edit`) so locks, atomic writes, WIP checks, and `BOARD.md` regeneration stay centralized.
- Do not parse arbitrary prose to decide pass/fail. Use status, run mode, terminal state, tags, ReviewRef, managed review pass/fail commands, Review Findings presence, and triage eligibility only.
- Never override `needs-user`, `cancelled`, unresolved `interrupted`, or `lost` states.
- Reconciliation skips must be durable enough to avoid hot loops on every watcher event, especially when WIP strict mode blocks a managed move after earlier metadata writes.
- Do not create a second scheduler. This task adds reconciliation to the existing daemon hooks.

## Acceptance Criteria

- [ ] Daemon reconciliation runs after activation, after relevant watcher reloads, and after daemon-owned run terminal events without racing active worker state.
- [ ] Auto-groom repair: if a daemon-owned groom run succeeded, the task is still in backlog, and `tb triage --json` no longer reports it, the daemon promotes it with `tb ready <ID>`.
- [ ] Auto-implement start repair: if a daemon-owned implement run is queued/running for a ready task, the daemon moves it to in-progress with the canonical pull path before or at run start.
- [ ] Auto-implement submit repair: if a daemon-owned implement run succeeded, the task is still in in-progress, and `ReviewRef` is non-empty, the daemon submits it with `tb review --submit <ID>`; if `ReviewRef` is missing, it records an actionable skip/diagnostic and does not invent a ref.
- [ ] Review-failed repair: if a task has `review-failed` plus non-empty `## Review Findings` but remains in code-review, the daemon moves it to ready using a managed review/fail-safe path or a documented no-op if the CLI cannot safely reapply findings.
- [ ] Review pass repair is deliberately conservative: the daemon does not move code-review -> done unless TB-272 or another managed marker from review mode proves pass; if no marker exists, the review prompt remains responsible.
- [ ] WIP-blocked or partially-applied reconciliation attempts record a durable skip/backoff marker keyed to task + attempted transition + relevant fingerprint so the daemon does not retry immediately on every watcher reload.
- [ ] Review-failed repair handles the `tb review --fail` partial-write corner case: if findings/tag were written but strict WIP blocked the code-review -> ready move, the daemon surfaces/backoffs the blocked move instead of repeatedly rewriting findings/tag.
- [ ] Reconciliation clears no state except where a sibling task explicitly defines it (TB-268 owns retry-blocking AgentStatus cleanup).
- [ ] Tests cover each repair path, each skip path, WIP-limit behavior for ready/in-progress/code-review, WIP-blocked backoff/no-hot-loop behavior, and `needs-user`/`cancelled`/`interrupted`/`lost` preservation.
- [ ] Verification includes `cd gui && go test ./...`.

## Related Tasks

- **TB-172** — Auto-groom stage.
- **TB-177** — Auto-implement stage.
- **TB-262** — Auto-review stage.
- **TB-239** — Ready column, pull mechanics, and failed-review return-to-ready behavior.
- **TB-268** — Clears retry-blocking agent state after failed review.
- **TB-269** — Documents reconciliation limits.
- **TB-270** — Prompt ownership must match reconciliation ownership.
- **TB-272** — Managed review pass flow gives the daemon an objective pass marker.

## Attachments

## Log

- 2026-05-19: Created
