# TB-206: Setup golangci-lint for project and initial run it

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** running
**Module:** tooling
**Tags:** lint,go,quality
**Branch:** —

## Goal

Set up golangci-lint for the Go codebase, document the expected contributor/agent lint workflow, run the first lint pass against both Go modules, and capture follow-up tasks for meaningful findings that should not be fixed in the setup pass.

## Context

This repository has two Go modules under a root `go.work`: `cli/` (`module tools/tb`, Go 1.26.1) and `gui/` (`module tools/tb-gui`, Go 1.25). The root is not itself a Go module, so lint commands should either run from each module root or provide a proven root-level wrapper that handles both modules explicitly.

No committed `.golangci.yml` / `.golangci.yaml` was found during grooming. A local `golangci-lint` binary is available at `/Users/ralist/go/bin/golangci-lint`, but the implementation should document whatever command contributors and agents are expected to run.

Contributor and agent-facing instructions may need updates where they mention Go verification, such as AGENTS.md, board/SKILL.md, README files, or docs that list standard checks.

### Constraints

Keep this task limited to Go lint setup and first-pass triage. Do not include frontend ESLint/dead-code work here; that belongs to TB-205.

Prefer high-signal checks that catch correctness, robustness, dead code, unchecked errors, and suspicious constructs. Leave noisy style-only or churn-heavy rules disabled unless they expose a concrete project risk.

Do not turn the first lint pass into a broad cleanup project. Fix only small setup blockers if needed, and create follow-up board tasks for nontrivial findings with the rule name, file path, and why the finding matters.

## Context

This repository has two Go modules under a root `go.work`: `cli/` (`module tools/tb`, Go 1.26.1) and `gui/` (`module tools/tb-gui`, Go 1.25). The root is not itself a Go module, so lint commands should either run from each module root or provide a proven root-level wrapper that handles both modules explicitly.

No committed `.golangci.yml` / `.golangci.yaml` was found during grooming. A local `golangci-lint` binary is available at `/Users/ralist/go/bin/golangci-lint`, but the implementation should document whatever command contributors are expected to run.

### Constraints

Keep this task limited to Go lint setup and first-pass triage. Do not include frontend ESLint/dead-code work here; that belongs to TB-205.

Prefer high-signal checks that catch correctness, robustness, dead code, unchecked errors, and suspicious constructs. Leave noisy style-only or churn-heavy rules disabled unless they expose a concrete project risk.

Do not turn the first lint pass into a broad cleanup project. Fix only small setup blockers if needed, and create follow-up board tasks for nontrivial findings with the rule name, file path, and why the finding matters.

## Acceptance Criteria

- [ ] A golangci-lint configuration or wrapper is committed in the repo and is scoped to the existing Go modules (`cli/` and `gui/`) without assuming the repository root is a standalone Go module.
- [ ] The enabled linters are intentionally limited to high-signal checks, and any disabled/noisy rules are documented in config comments or nearby contributor documentation.
- [ ] Contributor/agent-facing instructions that list Go verification are updated where appropriate so the new lint command is easy to run consistently.
- [ ] The first lint pass is run for both Go modules, with exact commands and results recorded in the task log or implementation summary.
- [ ] Existing Go verification still passes: `cd cli && go test ./...` and `cd gui && go test ./...`.
- [ ] Any meaningful lint finding not fixed during this setup task has a follow-up board task linked from this card, including the rule name, affected file/path, and expected fix direction.

## Related Tasks

- **TB-205** — Setup eslint and deadcode check for frontend (sibling: frontend-only lint/dead-code setup)

## Related Tasks

- **TB-205** — Setup eslint and deadcode check for frontend (sibling: frontend-only lint/dead-code setup)

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited type=tech-debt, module=tooling, tags=lint,go,quality, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited goal
- 2026-05-15: Edited acceptance

