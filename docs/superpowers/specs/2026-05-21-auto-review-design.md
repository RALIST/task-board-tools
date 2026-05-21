# Auto-Review Design

## Summary

Auto-review completes the third staged automation leg for the board workflow. When `auto_review_enabled` is on, the GUI daemon reviews every eligible task in `code-review` using the existing review-mode agent lifecycle. A clean review moves the task to `done`; blocking findings move it back to `ready` with `review-failed`; missing review targets stop with `needs-user`.

This is an opt-in automation stage. It does not replace manual review, does not add a new kanban status, and does not let review agents edit implementation files.

## Approved V1 Decisions

- Auto-review reviews every eligible `code-review` task when enabled. There is no v1 filter.
- A clean auto-review passes the task directly to `done`.
- A blocking auto-review fails the task back to `ready` with `review-failed`.
- A missing top-level `ReviewRef` is a `needs-user` handoff, not a silent skip.
- Recovery follows the auto-groom and auto-implement pattern: `interrupted` resumes the captured session, and `lost` starts a fresh review.
- Auto-recovery applies only to JSONL runs whose queued event has `initiator=auto-review`.
- Agent selection mirrors auto-implement: use explicit task `Agent` when set; otherwise persist the configured `default_agent`.
- Dedupe is based on the current code-review submission epoch. A reworked task is reviewable again after resubmission even when `ReviewRef` is unchanged.

## Existing Context

The repository already has most of the shared infrastructure:

- `code-review` is a first-class status.
- `ReviewRef` is parsed, emitted in JSON, and required for code-review submission.
- `AgentService` supports `mode=review` through `ReviewTask` and `ReviewDecorator`.
- JSONL run events already carry `mode`, `initiator`, resume metadata, terminal status, and task-local or board-root artifact paths.
- Auto-groom and auto-implement provide coordinator patterns for settings, watcher-driven scans, worker-budget checks, auto-resume, and visible skip reasons.
- Stage reconciliation exists for deterministic repair and should stay conservative. Auto-review should not depend on prose inference.

## Architecture

Auto-review should be implemented as a coordinator beside `AutoGroomCoordinator` and `AutoImplementCoordinator`.

The coordinator owns candidate selection and queueing. `AgentService` owns the run lifecycle. The CLI owns board mutations. The daemon owns shared worker capacity, stale recovery, startup queue scan, and deterministic stage reconciliation.

The preferred flow is:

1. User enables `auto_review_enabled`.
2. Coordinator scans `code-review`.
3. For each eligible task, the coordinator assigns the explicit or default agent.
4. Coordinator starts a review run through a new auto-review entry point that records `mode=review` and `initiator=auto-review`.
5. Review agent inspects the `ReviewRef` target.
6. Review agent records the result through a managed CLI pass or fail command.
7. Board refresh moves the card out of `code-review`.

## CLI Pass Flow

Add a managed pass command symmetrical with the existing fail flow:

```sh
tb review --pass TB-123 - <<'EOF'
- No blocking findings.
EOF
```

The command must:

- Accept only tasks currently in `code-review`.
- Reject empty pass findings.
- Write or replace `## Review Findings`.
- Append an explicit pass log entry.
- Move the task to `done`.
- Regenerate `BOARD.md`.
- Use existing board locking, atomic writes, folder-form movement, and redaction rules.

Existing manual flows remain valid: `tb review --findings`, `tb done`, and `tb review --fail` continue to work.

## Preferences And Controls

Add `auto_review_enabled` to the existing preferences file.

Defaults and validation:

- Default is `false`.
- Enabling requires `default_agent` to be `claude` or `codex`.
- Validation failure must be typed and actionable.
- Validation failure must not mutate task metadata, JSONL, logs, or board files.

Controls:

- Settings panel gets an auto-review toggle near auto-groom and auto-implement.
- Board header gets a compact auto-review toggle backed by the same preference.
- Header and settings stay in sync through the preferences store.

## Candidate Eligibility

A task is eligible when all of these are true:

- It is in `code-review`.
- It has non-empty top-level `ReviewRef`.
- It is not already active in `AgentService` or queued/running through the shared worker budget.
- Its current submission epoch has not already had an auto-review attempt.
- Its generic `AgentStatus` is blank or otherwise non-blocking for a new review.
- If explicit `Agent` is set, it is supported.
- If explicit `Agent` is blank, `default_agent` is supported and can be persisted before queueing.

Blocking statuses:

- `queued` and `running` are skipped because work is already in flight.
- `needs-user` is skipped until a human clears it.
- `cancelled` is skipped unless a later explicit user action requeues it.
- `interrupted` and `lost` are handled only by the auto-review recovery sweep when the latest queued initiator is `auto-review`.

Wrong-column tasks are never reviewed by auto-review, including `ready` tasks tagged `review-failed`. Rework belongs to auto-implement.

## Missing ReviewRef

If a `code-review` task is otherwise in scope but lacks top-level `ReviewRef`, the coordinator writes a user-attention handoff:

- `Reason`: missing review target.
- `Question/Action`: set `ReviewRef` to a concrete branch, PR URL, commit SHA, worktree path, or other machine-readable target.
- `Attempted context`: auto-review found the task in `code-review` but could not determine a safe target.
- `Unblock condition`: set `ReviewRef`, then clear `AgentStatus` with `tb edit <ID> --agent-status none`.

Then it sets `AgentStatus: needs-user` and stops. It does not launch a reviewer agent.

## Submission-Epoch Dedupe

Auto-review dedupe must be keyed to the current code-review submission epoch, not just `ReviewRef`.

This matters because a stable branch name such as `feature/foo` can receive new commits without changing `ReviewRef`. A reworked task that returns to `code-review` should be reviewed again.

The coordinator should treat each fresh arrival or resubmission into `code-review` as a new epoch. Within one epoch:

- Watcher bursts must not queue duplicate review runs.
- App restart must not queue a second review if the epoch already has a terminal auto-review attempt.
- `interrupted` and `lost` are recovery states for the same epoch, not a new epoch.

Changing `ReviewRef` while still in `code-review` should create a new reviewable fingerprint because the target changed.

## Review Run Lifecycle

Add an auto-review entry point equivalent to the existing coordinator-owned run helpers:

- It starts `mode=review`.
- It records `initiator=auto-review`.
- It writes the queued JSONL event before setting `AgentStatus: queued`.
- It uses the existing review decorator, cancellation, timeout, log streaming, terminal recording, and per-mode attribution.
- It reserves shared worker capacity before launching direct `AgentService` work.

The daemon's normal `RunQueuedAgentSync` path must continue to respect queued review runs by reading the queued JSONL mode and applying `ReviewDecorator`.

## Recovery

Auto-review recovery mirrors auto-groom and auto-implement.

During each scan:

- If a task in `code-review` has `AgentStatus: interrupted` and the latest queued initiator is `auto-review`, resume the captured session with `ResumeAgentAs(..., initiator=auto-review)`.
- If a task in `code-review` has `AgentStatus: lost` and the latest queued initiator is `auto-review`, start a fresh review run.
- Manual or user-triggered review runs are not auto-resumed by this stage.
- Persistent resume failures use the same cooldown pattern as the other coordinators.
- Worker-budget limits apply before resume or restart.

## Pass And Fail Outcomes

Pass:

- Review agent calls `tb review --pass`.
- Task moves from `code-review` to `done`.
- `ReviewStatus` records success through existing terminal attribution.
- The coordinator does not requeue the task after board refresh.

Fail:

- Review agent calls `tb review --fail`.
- Task moves from `code-review` to `ready`.
- `review-failed` tag is present.
- Generic `AgentStatus` is cleared so auto-implement can retry rework.
- Review findings stay visible in the drawer.

No result from free-form text:

- The daemon must not infer pass or fail from prose.
- A successful review-mode process that leaves the task in `code-review` without a managed pass or fail is treated as incomplete by board state, not as a pass.

## User Interface

The UI should make auto-review visible without adding a tutorial surface.

Settings and board header:

- Show enabled or disabled state.
- Show missing-default-agent feedback when enabling is blocked.

Code-review cards and drawer:

- Show queued/running review runs through existing run history labels.
- Show skipped reasons for missing `ReviewRef`, unsupported agent, worker capacity, active run, duplicate epoch, and `needs-user`.
- Preserve the manual Review button when auto-review is off or skipped.

Pass and fail refresh:

- Pass removes the card from `code-review` and shows it in `done`.
- Fail moves the card to `ready`, keeps findings visible, and shows the existing `review-failed` marker.

## Error Handling

- Missing default agent blocks enabling and leaves board state untouched.
- Missing `ReviewRef` writes `needs-user` because it requires human action.
- Unsupported explicit agent skips visibly and does not overwrite the task's agent choice.
- WIP or worker-capacity blockers record visible skip reasons and retry on a meaningful state change.
- Existing `needs-user`, `cancelled`, unresolved `interrupted`, and unrelated `lost` states are preserved.
- Board mutations always go through the managed CLI, so locks and atomic writes stay centralized.

## Testing

CLI tests:

- `tb review --pass` happy path.
- Reject non-`code-review` tasks.
- Reject empty pass findings.
- Folder-form task movement to `done`.
- No regression to `tb review --fail`.

Backend tests:

- Preference default, persistence, validation failure, enable, disable, and restart load.
- Disabled auto-review does not mutate tasks.
- No-default path does not mutate tasks.
- Eligible code-review task queues `mode=review` with `initiator=auto-review`.
- Explicit agent override.
- Default-agent fallback.
- Missing `ReviewRef` writes `User Attention` and `needs-user`.
- `AgentStatus=success` from prior implement run does not block first review.
- Wrong columns are skipped.
- `needs-user` is skipped.
- Unsupported explicit agent is visible and non-mutating.
- Submission-epoch dedupe blocks watcher/restart duplicates.
- Resubmission with the same `ReviewRef` is eligible again.
- Changed `ReviewRef` while in code-review is eligible again.
- `interrupted` auto-review resumes.
- `lost` auto-review starts fresh review.
- User-initiated interrupted or lost review does not auto-resume.
- Worker budget gates fresh review and recovery starts.

Frontend tests:

- Preferences store includes auto-review.
- Settings toggle works and displays missing-default-agent feedback.
- Header toggle stays in sync with settings.
- Enabled/disabled state renders on the board.
- Skipped reason rendering for missing target and active/duplicate states.
- Pass refresh removes stale code-review row.
- Fail refresh shows ready `review-failed` marker and findings.
- Manual Review remains usable when auto-review is disabled or skipped.

Verification commands:

```sh
cd cli && go test ./...
cd gui && go test ./...
cd gui/frontend && npm run check
cd gui/frontend && npm test -- --run
```

## Non-Goals

- No saved auto-review filter in v1.
- No new kanban status.
- No new `AgentStatus`.
- No implementation-file writes from review-mode agents.
- No semantic pass/fail inference from free-form findings text.
- No automatic review of `ready` tasks tagged `review-failed`.
- No multi-reviewer quorum or approval policy.

## Implementation Order

1. Implement `TB-272`: managed `tb review --pass`.
2. Implement `TB-263`: persisted setting and controls.
3. Implement `TB-264`: auto-review coordinator, JSONL initiator, recovery, dedupe, and needs-user handoff.
4. Implement `TB-265`: visible runtime state, skips, and pass/fail UI refresh.
5. Close `TB-262`: update product docs, board guidance, and final verification evidence.
