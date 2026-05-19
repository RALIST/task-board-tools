# TB-198: Agent: add review mode and findings section

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** agent
**Tags:** epic-tb194,code-review,agent,prompt
**Branch:** —
**Parent:** TB-194

## Goal

Add a review-mode agent flow so a user can run a reviewer agent on a task in Code Review and have findings written back to the task.

## Context

- Parent epic: TB-194.
- Existing agent modes include implement and groom via `gui/internal/agent` prompts/decorators, `AgentService`, JSONL `mode`, Wails run events, and the daemon executor.
- TB-6/TB-65..TB-68 are the closest pattern: a mode-specific prompt/decorator reuses the same runner, cancellation, JSONL, and daemon lifecycle.
- Review output must land in `## Review Findings` using the managed CLI surface from TB-196.

## Constraints / Non-goals

- Review mode must not edit implementation files. Its job is to inspect the referenced branch/PR/commit/worktree and update the task with findings.
- Use existing runner lifecycle and cancellation paths; do not fork a parallel agent execution system.
- JSONL/Wails run events should carry `mode=review` so history can distinguish implement, groom, and review runs.
- The review prompt must tell agents to write actionable findings to `## Review Findings` through `tb review --findings`, then either leave the task in Code Review for human decision or use the failure flow from TB-199 when explicitly instructed/available.
- Do not implement review-failed marker logic here; TB-199 owns that.

## Related Tasks

- **TB-194** - Parent epic.
- **TB-196** - CLI findings section writer used by review agents.
- **TB-197** - GUI surfaces review-mode controls/history.
- **TB-199** - Failure handoff when findings require rework.
- **TB-6** - Existing groom-mode architecture pattern.

## Acceptance Criteria

- [ ] Agent mode enum/schema accepts `review` and preserves backward compatibility for existing implement/groom JSONL events.
- [ ] A review prompt is embedded and rendered with task ID, title, body, and review target context.
- [ ] `AgentService` exposes a review-run entry point that reuses existing active-run dedup, queued/running/success/failed/cancelled transitions, JSONL writes, Wails events, and Cancel behavior.
- [ ] The daemon executor honors queued review runs by reading `mode=review` from JSONL, matching the groom-mode pattern.
- [ ] The GUI can start a review run for a Code Review task and past-runs history labels it as `review`.
- [ ] A successful review run that writes findings via TB-196 leaves the findings visible in `tb show` and the TaskDrawer after watcher refresh.
- [ ] Tests cover review prompt rendering, JSONL `mode=review`, success/failure/cancel paths, daemon pickup, and no regression to implement/groom mode.
- [ ] Manual test note: create a Code Review task with a fake review target, start a review run, cancel one run, run one to success with stubbed findings, and confirm history plus `## Review Findings` update.
- [ ] Verification includes `cd gui && go test ./...`, `cd gui/frontend && npm run check`, and `cd gui/frontend && npm test -- --run`.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance
- 2026-05-19: Done

