# TB-205: Setup ESLint and dead-code check for frontend

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Agent:** claude
**AgentStatus:** running
**Module:** tooling
**Tags:** lint,frontend,quality,dead-code,epic
**ReviewRef:** TB-205-frontend-lint
**Branch:** —

## Goal

Set up high-signal ESLint and dead-code checks for `gui/frontend`, wire them into the documented frontend verification workflow, run the first pass, and capture follow-up tasks for meaningful findings that are too large for the setup pass.

## Context

The frontend package lives under `gui/frontend/` and currently has SvelteKit/Svelte 5, TypeScript, Vite, Vitest, `svelte-check`, `npm run check`, and `npm test`, but no committed ESLint or dead-code configuration was found during grooming.

Relevant files to inspect first: `gui/frontend/package.json`, `gui/frontend/package-lock.json`, `gui/frontend/tsconfig.json`, `gui/frontend/svelte.config.js`, `gui/frontend/vite.config.ts`, and existing frontend tests under `gui/frontend/src/`.

Contributor and agent-facing instructions that list frontend verification commands may need updates, especially `AGENTS.md`, `board/SKILL.md`, README/docs references, or any local prompt/skill guidance that still says frontend completion is only `npm run check` plus `npm test`.

### Constraints

Keep this task limited to frontend lint/dead-code setup and first-pass triage. Do not turn the initial run into a broad cleanup project; fix only small setup blockers and create follow-up board tasks for nontrivial findings.

Prefer rules that catch correctness, robustness, unsafe imports, unused code, and likely runtime defects. Avoid style-only churn unless a rule protects a real project invariant.

Use Svelte/TypeScript-aware tooling compatible with the current Svelte 5/SvelteKit setup. The dead-code tool must account for Wails-generated bindings, SvelteKit entry points, tests, and intentional public component/store/API surfaces so the first pass does not drown in false positives.

Do not change Go lint setup here; that is TB-206.

## Subtasks

- **TB-247** (S) — Triage TB-205 knip first-pass findings

## Acceptance Criteria

- [ ] `gui/frontend` has committed ESLint dependencies, configuration, and an `npm run lint` script that works with the existing Svelte 5/SvelteKit + TypeScript project.
- [ ] `gui/frontend` has a committed dead-code check, exposed as `npm run deadcode` or an equivalently documented npm script, with ignores/entry points covering generated Wails bindings, SvelteKit files, tests, and intentional public API surfaces.
- [ ] Enabled rules/checks are intentionally high-signal; noisy style-only rules, broad generated-file checks, or known false positives are disabled or documented in config comments or nearby contributor docs.
- [ ] The first lint/dead-code pass is run, and exact commands plus results are recorded in the task log or implementation summary.
- [ ] Any meaningful lint or dead-code finding not fixed during setup has a follow-up board task linked from this card, including the rule/tool name, affected path, and expected fix direction.
- [ ] Existing frontend verification still passes: `cd gui/frontend && npm run check` and `cd gui/frontend && npm test`.
- [ ] Contributor/agent-facing docs that list frontend verification commands are updated to include the new lint/dead-code workflow and when to run it.

## Review Target

branch: main (working tree on host)

scope:
  - gui/frontend/package.json — added eslint, eslint-plugin-svelte,
    typescript-eslint, globals, @eslint/js, svelte-eslint-parser, knip
    (devDeps), plus `npm run lint` and `npm run deadcode` scripts.
  - gui/frontend/eslint.config.js (new) — Svelte 5 / TS flat config,
    rule policy documented inline.
  - gui/frontend/knip.config.js (new) — dead-code config; ignores +
    entry list intentionally narrow with inline rationale.
  - gui/frontend/src/lib/components/FilterBar.svelte — inline disable
    of `svelte/prefer-writable-derived` with TB-247 follow-up reference.
    No behavioral change.
  - AGENTS.md, CLAUDE.md — frontend verification commands updated.
    (These hunks were present from a prior partial run; reviewing
    confirms they describe the lint/deadcode workflow correctly.)
  - board/backlog/TB-247/TASK.md (new) — follow-up for first-pass
    findings.

verification (cd gui/frontend &&):
  - npm run lint   → 0 errors
  - npm run check  → 411 files, 0 errors, 0 warnings
  - npm test       → 17 files, 190 tests pass
  - npm run deadcode → 13 findings (verbatim below)

deadcode baseline (npm run deadcode):
  Unused dependencies (2)
    @types/dompurify  package.json:23:6
    codemirror        package.json:25:6
  Unused devDependencies (1)
    @types/marked  package.json:35:6
  Unused exports (10)
    pullNext                   function  src/lib/api.ts:134:23
    setReviewTarget            function  src/lib/api.ts:148:23
    setReviewerNotes           function  src/lib/api.ts:152:23
    setReviewFindings          function  src/lib/api.ts:156:23
    failReview                 function  src/lib/api.ts:160:23
    regenerate                 function  src/lib/api.ts:172:23
    isAlreadyInitializedError  function  src/lib/api.ts:320:17
    getBoardInfo               function  src/lib/api.ts:348:23
    totalTaskCount                       src/lib/stores/board.ts:148:14
    runsStore                            src/lib/stores/runs.ts:46:14

reviewer notes:
  - All findings above are real, not false positives — captured in
    TB-247 with per-item triage direction.
  - Concurrent unrelated work in the working tree (TB-237 et al.,
    TB-178/241/243/244/245/249/250 task files) is intentionally NOT in
    this commit and is not part of this review.

## Related Tasks

- **TB-206** — Setup golangci-lint for project and initial run it (sibling: Go-only lint setup)

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited priority=P2, type=tech-debt, size=M, module=tooling, tags=lint,frontend,quality,dead-code, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=failed
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited title=Setup ESLint and dead-code check for frontend
- 2026-05-19: Edited agentstatus=success
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited reviewref=TB-205-frontend-lint
- 2026-05-19: Edited review-target
- 2026-05-19: Edited review-target
- 2026-05-19: Submitted to code-review

