# TB-267: Auto-implement: respect epic child order

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** auto-implement,daemon,epic,ordering
**ImplementedBy:** claude
**ImplementStatus:** success
**ReviewRef:** TB-267 ships in next commit
**Branch:** —
**Parent:** TB-177

## Goal

Block auto-implementation of a child task when an earlier sibling in the same epic is not yet done, treating child task IDs as implementation order.

## Context

- Auto-implement candidate selection cannot infer arbitrary dependency graphs.
- The board already models child tasks through top-level `**Parent:** TB-NNN` metadata and `tb epic <ID>` progress.
- The working assumption for epics is that child tasks are created in implementation order. Numeric task ID is the available deterministic ordering key.

## Constraints / Non-goals

- This applies to automatic implementation pickup only. Manual `tb pull <ID>`, manual Run, and human board moves remain possible escape hatches.
- Tasks without a parent are unaffected.
- The parent epic card itself should not be auto-implemented when it is tagged `epic`; automation should pick leaf child tasks.
- The rule is hard for auto-implement: if any lower-ID sibling with the same parent is not in `done`, the candidate is skipped.
- Do not invent dependency syntax in this task. A richer dependency model can be a future feature.

## Acceptance Criteria

- [ ] Auto-implement candidate selection loads same-parent siblings from the active board and sorts them by numeric task ID.
- [ ] A child task is eligible only when every same-parent sibling with a lower numeric ID is in `done`.
- [ ] Siblings whose status directory is backlog, ready, in-progress, or code-review block later siblings; siblings in done unblock them.
- [ ] Siblings with `AgentStatus=needs-user`, `interrupted`, or `cancelled` block later siblings even if their status directory might otherwise look eligible.
- [ ] If the epic implies a lower-ID sibling but the task file is missing/deleted or cannot be parsed, automation blocks later siblings with a diagnostic rather than silently skipping the unknown predecessor.
- [ ] Archived earlier siblings are treated conservatively: either block with a clear diagnostic or are explicitly documented as closed; choose one behavior and cover it in tests.
- [ ] Review-failed retry obeys the same rule. A later child tagged `review-failed` does not outrank an unfinished earlier sibling.
- [ ] Tasks tagged `epic` are skipped as implementation candidates unless a future explicit override exists.
- [ ] Diagnostics/logging identify the blocking earlier sibling ID so the GUI can explain why a task was skipped.
- [ ] Unit tests cover no-parent, first child, blocked later child, unblocked later child, review-failed later child, archived earlier sibling, missing/deleted lower-ID sibling, sibling with blocking `AgentStatus`, and epic-card skip.
- [ ] Verification includes `cd gui && go test ./...`.

## Review Target

- gui/internal/automation/epicorder/epicorder.go (new): pure EligibleForEpicOrder helper. Treats archive as closed (per board CONVENTIONS.md "archive is for obsolete/superseded/dropped work"). Blocks on missing predecessors (Status==""), epic-tagged candidates, and AgentStatus in {needs-user, interrupted, cancelled}.
- gui/internal/automation/epicorder/epicorder_test.go: 14 tests covering no-parent, first child, blocked-by-backlog, unblocked-when-all-done, review-failed later child still blocked, archived treated as closed, missing predecessor blocks, needs-user/interrupted/cancelled all block, epic card not a candidate, unrelated parents ignored, higher-id ignored, ParseNumeric edge cases including double-dash rejection.

## Related Tasks

- **TB-177** — Parent auto-implement epic.
- **TB-179** — Candidate selector where this rule should be enforced.
- **TB-233** — Review-failed priority boost must run after this dependency gate.
- **TB-204** — Existing GUI epic progress surface.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited implemented-by=claude, implement-status=success, reviewref=TB-267 ships in next commit
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited review-target
- 2026-05-20: Done

