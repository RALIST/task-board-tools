# TB-322 Review-State Evidence

Source: `board/done/TB-322/.agent-state.jsonl`
Run: `r_a20a3806`

## Timeline

- Lines 742-745: run queued and started as `mode=resume`, resumed from session `019e4ae5-0cb5-7e71-8683-2624b32d50b9`.
- Line 950: agent said this was "Board-only cleanup", not a final review verdict.
- Lines 952-955: agent cleared `ReviewedBy` / `ReviewStatus`, set `AgentStatus: success`, and wrote findings ending with "Pending fresh review."
- Lines 956-957: `tb review --submit TB-322` failed because task was in `ready` without `review-failed`.
- Line 961: agent diagnosed that it had cleared `review-failed` too early and chose normal path: pull to `in-progress`, then submit.
- Lines 965-968: agent ran `tb pull TB-322`, then `tb review --submit TB-322`; this moved task to `code-review`.
- Line 976: agent explicitly said `TB-322 stable in code-review now, no review-failed tag`.
- Line 987: final summary said `TB-322 now back in code-review, review-failed tag cleared, ReviewRef: working-tree`.
- Line 1004: runner recorded `finished{mode: resume, status: success}`.

## Interpretation

JSONL does not show a semantic review pass. It shows the resumed agent cleaning stale failed-review state and resubmitting TB-322 to `code-review`.

The task became misleading because the resumed run exited 0, and terminal attribution mapped that success back to review metadata (`ReviewedBy: codex`, `ReviewStatus: success`) even though the agent never ran `tb review --pass` or `tb review --fail`.

## Prevention Target

Daemon/stage reconciliation should detect a `code-review` task whose latest effective review/resume run finished success but did not perform a managed pass/fail transition. It must not leave that task silently in `code-review` or let auto-review dedupe it forever.

Valid recovery should be explicit: finalize via managed pass/fail when proven, move to `ready` with `review-failed` for protocol failure, or mark `needs-user` with actionable context if automation cannot safely classify it.
