# TB-306: Generated conventions and skill should omit autonomous flows

**Type:** bug
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** cli
**Tags:** docs,templates
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Remove autonomous-flow and product-feature detail from the generated board conventions and task-board skill so they describe only portable board policy, kanban methodology, and generic task/agent handoff rules. Keep the autonomous workflow contract in product and architecture docs where the app feature is specified.

## Context

- Current generated docs come from `cli/templates.go` (`conventionsTemplate` and `skillTemplate`) and checked-in copies at `board/CONVENTIONS.md` and `board/SKILL.md`.
- Those generated docs currently include `## Autonomous Stages` with `auto-groom`, `auto-implement`, `auto-review`, daemon housekeeping, epic child ordering, and resume/UI-specific lifecycle details.
- Template parity is covered in `cli/templates_test.go`; existing-board generated-doc refresh behavior is covered in `cli/init_test.go`.
- Product docs intentionally keep autonomous workflow detail in `docs/FEATURES.md` M11 and `docs/ARCHITECTURE.md` under `Staged autonomous workflow`. This task should keep app-specific behavior there instead of in generated board guidance.
- Related tasks: TB-227/TB-230 cover generated board-doc refresh/reconcile and template lockstep behavior. TB-270 covers autonomous prompt-contract cleanup; do not fold prompt rewrites into this task.

## Constraints

- Scope is generated board conventions/skill content and the source templates/tests that produce it.
- Do not change GUI automation behavior, daemon candidate selection, agent prompts, or autonomous workflow implementation.
- Do not remove or weaken product/architecture documentation for the autonomous feature; generated board docs should simply stop carrying that feature-specific contract.
- Keep `CONVENTIONS.md` as a policy guide, not a CLI manual. Keep detailed command syntax in CLI help and the skill's minimal command list.
- Keep generic board/agent handoff guidance that is not autonomous-stage specific, especially `User Attention` / `needs-user` if it is still required for safe agent stops.
- Preserve configured board-path and task-prefix templating in `conventionsTemplate(prefix, boardPath)` and `skillTemplate(prefix, boardPath)`.
- Refresh checked-in generated docs from the templates rather than hand-diverging them; keep `board/CONVENTIONS.md`, `board/SKILL.md`, and template output byte-identical where tests require it.
- Because implementation touches `cli/`, rebuild and relink the local `tb` binary after changes.

## Acceptance Criteria

- [ ] `cli/templates.go` no longer emits a `## Autonomous Stages` section or product-stage strings such as `auto-groom`, `auto-implement`, `auto-review`, `Daemon housekeeping for autonomous stages`, or `auto-implement must not pick a later numeric child` in generated `CONVENTIONS.md` or generated `SKILL.md`.
- [ ] `board/CONVENTIONS.md` and `board/SKILL.md` are refreshed from the updated templates and contain no autonomous-stage, product-feature, daemon-repair, or UI/resume nuance text while still covering kanban flow, source-of-truth rules, task quality, WIP limits, review loop, backlog capture, related tasks, done evidence, and safe user-attention handoff.
- [ ] `.codex/skills/task-board/SKILL.md` is updated too if it is intended to mirror the generated task-board skill used by this repo; otherwise the task log explains why it intentionally differs.
- [ ] Autonomous workflow details remain documented in product/architecture surfaces such as `docs/FEATURES.md` M11 and `docs/ARCHITECTURE.md` `Staged autonomous workflow`; this task does not remove or weaken those feature contracts.
- [ ] Template/unit coverage is updated so tests assert generated docs exclude the autonomous-stage strings while preserving existing policy-focused, portable-skill, board-path, and prefix checks.
- [ ] Verification includes `cd cli && go test ./...`, `cd cli && go build -o tb .`, and a temp-board `tb init` smoke check that generated `CONVENTIONS.md` and `SKILL.md` omit the forbidden autonomous-stage strings.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited priority=P2, type=bug, size=M, module=cli, tags=docs,templates, title=Generated conventions and skill should omit autonomous flows
- 2026-05-20: Edited goal
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success

