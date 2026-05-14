# TB-205: Setup esling and deadcode check for frontend

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** running
**Module:** tooling
**Tags:** lint,frontend,quality,dead-code
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

## Acceptance Criteria

- [ ] `gui/frontend` has committed ESLint dependencies, configuration, and an `npm run lint` script that works with the existing Svelte 5/SvelteKit + TypeScript project.
- [ ] `gui/frontend` has a committed dead-code check, exposed as `npm run deadcode` or an equivalently documented npm script, with ignores/entry points covering generated Wails bindings, SvelteKit files, tests, and intentional public API surfaces.
- [ ] Enabled rules/checks are intentionally high-signal; noisy style-only rules, broad generated-file checks, or known false positives are disabled or documented in config comments or nearby contributor docs.
- [ ] The first lint/dead-code pass is run, and exact commands plus results are recorded in the task log or implementation summary.
- [ ] Any meaningful lint or dead-code finding not fixed during setup has a follow-up board task linked from this card, including the rule/tool name, affected path, and expected fix direction.
- [ ] Existing frontend verification still passes: `cd gui/frontend && npm run check` and `cd gui/frontend && npm test`.
- [ ] Contributor/agent-facing docs that list frontend verification commands are updated to include the new lint/dead-code workflow and when to run it.

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

