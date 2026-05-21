# TB-319: CLI: clear review-failed when submitting retried in-progress work

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** workflow
**Tags:** code-review,review-failed,automation
**Branch:** —

## Goal

When a failed review returns to ready with review-failed, auto-implement or tb pull can move it to in-progress while the tag remains. tb review --submit clears review-failed only from ready/backlog retry sources, so an in-progress retry can enter code-review still tagged review-failed. Stage reconciliation can then treat it as an objective failed-review repair and move it back to ready immediately. Expected: submitting retried in-progress work clears review-failed before or during the move to code-review.

## Acceptance Criteria

- [ ] Reproduces an in-progress task with `review-failed` entering code-review through `tb review --submit`.
- [ ] `tb review --submit` clears `review-failed` for retried in-progress work before or during the move to code-review.
- [ ] Ready/backlog legacy retry behavior stays covered, and normal first-submit behavior is unchanged.
- [ ] Verification includes `cd cli && go test ./...`.

## Attachments

## Log

- 2026-05-21: Created
- 2026-05-21: Edited acceptance
