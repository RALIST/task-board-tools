# TB-252: Allow Resume when session_id is present regardless of AgentStatus

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** agent,resume,session,ux
**Branch:** —

## Goal

Surface the Resume action whenever the latest run has a captured `session_id`, regardless of `AgentStatus`, so users can recover any daemon-lost run without first having to manually flip the status.

## Acceptance Criteria

- [ ] `ResumeAgent` in `gui/app/agent_run.go` (around the `ErrCannotResume` check) no longer gates on `AgentStatus == "interrupted"` alone. Eligibility is: latest run from `resumableSessionID` returned ok=true (`gui/app/agent_recovery.go:414-436`) AND status is one of `{interrupted, failed, cancelled, success}` (i.e. any terminal state with a captured session). `queued` / `running` / `needs-user` remain blocked.
- [ ] `ErrCannotResume` is kept for the no-session case (so the message remains accurate) and reworded; the comment at `gui/app/agent_service.go:49-52` is updated to reflect the new policy.
- [ ] Frontend (`gui/frontend/src/lib/components/TaskDrawer.svelte` or equivalent) surfaces the Resume button whenever the backend reports a resumable candidate — driven by a service call or the existing `agent:run-finished` payload, not by a hardcoded status string.
- [ ] Resuming from `failed`, `success`, or `cancelled` does NOT silently clear the user's intent — show the source status next to the button (e.g. "Resume failed run") so the action is intentional.
- [ ] Backend tests: extend `gui/app/agent_run_test.go` to cover resume eligibility from each of the four terminal states; assert that the new run links `ResumedFromRun` correctly.
- [ ] Frontend test: the Resume button renders for `failed` runs with a session_id and stays hidden for `failed` runs without one.
- [ ] `cd cli && go test ./...`, `cd gui && go test ./...`, and `cd gui/frontend && npm test` pass.
- [ ] Coordinate with TB-251 — if the recovery taxonomy is changed there first, this task only needs the policy widening, not the status-string list.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance

