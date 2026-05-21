# TB-340: Auto-review: prevent successful review runs from staying in code-review

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** gui/app
**Tags:** agent,review,automation,stuck
**Branch:** —

## Goal

Review-mode or resumed-review runs can exit 0 and stamp ReviewedBy/ReviewStatus success while the task remains in code-review if the agent did not run tb review --pass or --fail. Add a post-run invariant so review success cannot leave ambiguous code-review state; either finalize through managed pass/fail or surface a needs-user/protocol-failure handoff with a clear recovery path.

## Context

- TB-322 ended with `ReviewedBy: codex`, `ReviewStatus: success`, and latest run `finished{status: success}` while the task still lived in `code-review`.
- Attached evidence: `TB-322-review-state-evidence.md` cites `board/done/TB-322/.agent-state.jsonl` for run `r_a20a3806`.
- JSONL lines 742-745 show the run was `mode=resume`; line 1004 shows only process success (`finished{mode: resume, status: success}`).
- JSONL lines 950-968 show the agent was doing board cleanup and resubmission: it cleared stale review metadata, pulled TB-322 to `in-progress`, then ran `tb review --submit TB-322`.
- JSONL lines 976 and 987 show the agent intentionally stopped with TB-322 in `code-review`; the log does not show `tb review --pass` or `tb review --fail` after resubmission.
- `recordTerminal` maps process exit 0 to per-mode `ReviewStatus: success`; for resumed sessions it attributes success to the parent mode. That lets a resumed review session mark review success even if the managed review transition never happened.
- `autoReviewAlreadyQueued` can dedupe the same review epoch, so a code-review task left behind after a successful review run may be skipped instead of re-reviewed.
- Daemon startup already runs stale recovery then `StageReconciler.ReconcileActive`; that reconciler is the right daemon-side safety net for detecting and repairing this stuck state.

## Acceptance Criteria

- [ ] Add coverage for a review-mode or resumed-review run that exits 0 while the task remains in `code-review`; expected result must not be silent `ReviewStatus: success` plus unchanged column.
- [ ] `StageReconciler.ReconcileActive` checks `code-review` tasks for latest effective review/resume terminal success without a managed `tb review --pass` or `tb review --fail` transition.
- [ ] Daemon activation/startup reconciliation invokes that check so stuck tasks are repaired without waiting for a watcher event.
- [ ] Recovery is explicit: successful review only counts when task is in `done`; failed/incomplete review protocol moves to `ready` with `review-failed` or marks `needs-user` with actionable context when automation cannot safely classify it.
- [ ] Auto-review scan does not permanently dedupe or skip a `code-review` task whose last review run violated the pass/fail contract.
- [x] Prompt/conventions/skills make clear that review agents must use `tb review --pass` or `tb review --fail`; process exit 0 alone is not a semantic review decision.
- [ ] Verification includes focused GUI backend tests for the new invariant plus `cd gui && go test ./app`.

## Attachments

- TB-322-review-state-evidence.md

## Log

- 2026-05-21: Created
- 2026-05-21: Edited context
- 2026-05-21: Edited acceptance
- 2026-05-21: Attached TB-322-review-state-evidence.md
- 2026-05-21: Edited context
- 2026-05-21: Edited acceptance
- 2026-05-21: Edited acceptance

