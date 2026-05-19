# TB-206: Setup golangci-lint for project and initial run it

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** tooling
**Tags:** lint,go,quality
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** main
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

- [x] A golangci-lint configuration or wrapper is committed in the repo and is scoped to the existing Go modules (`cli/` and `gui/`) without assuming the repository root is a standalone Go module.
- [x] The enabled linters are intentionally limited to high-signal checks, and any disabled/noisy rules are documented in config comments or nearby contributor documentation.
- [x] Contributor/agent-facing instructions that list Go verification are updated where appropriate so the new lint command is easy to run consistently.
- [x] The first lint pass is run for both Go modules, with exact commands and results recorded in the task log or implementation summary.
- [x] Existing Go verification still passes: `cd cli && go test ./...` and `cd gui && go test ./...`.
- [x] Any meaningful lint finding not fixed during this setup task has a follow-up board task linked from this card, including the rule name, affected file/path, and expected fix direction.

## Review Target

branch: main (scoped TB-206 changes on shared dirty worktree)

scope:
  - .golangci.yml — root golangci-lint v2 config, intentionally high-signal linters only: errcheck, govet, ineffassign, staticcheck SA*, unused, bodyclose, errorlint, nilerr, nilnesserr, makezero, wastedassign, unconvert. Style/churn-heavy linters are documented as deferred.
  - scripts/lint-go.sh + Makefile — root wrapper enters cli/ and gui/ explicitly; supports GOLANGCI_LINT=/path/to/golangci-lint.
  - AGENTS.md, CLAUDE.md, README.md — Go lint workflow documentation. README frontend lint/deadcode hunk in the shared worktree belongs to TB-205 and is not part of the intended TB-206 commit.
  - board/backlog/TB-249/TASK.md — CLI baseline follow-up from first pass.
  - board/backlog/TB-250/TASK.md — GUI baseline follow-up from first pass.

first-pass lint result:
  - GOLANGCI_LINT=/Users/ralist/go/bin/golangci-lint make lint-go -> cli: 0 issues; gui: 0 issues after temporary baseline suppressions linked to TB-249/TB-250.

verification:
  - live shared tree: GOLANGCI_LINT=/Users/ralist/go/bin/golangci-lint make lint-go -> cli: 0 issues; gui: 0 issues.
  - live shared tree: cd cli && go test ./... -> ok tools/tb 3.850s.
  - live shared tree: cd gui && go test ./... -> failed in tools/tb-gui/internal/agent because unrelated dirty gui/internal/agent/claude.go adds --dangerously-skip-permissions while exec_test.go still expects the old argv length.
  - scoped clean worktree /tmp/tb206-verify.ahAQHG with only TB-206 lint wrapper/config copied in and existing generated gui/frontend/dist precondition restored:
    - GOLANGCI_LINT=/Users/ralist/go/bin/golangci-lint make lint-go -> cli: 0 issues; gui: 0 issues.
    - cd cli && go test ./... -> ok tools/tb 4.721s.
    - cd gui && go test ./... -> ok tools/tb-gui/app 52.056s; ok internal/agent 17.878s; ok internal/cli 16.353s; ok internal/daemon 2.317s; ok internal/redact 1.777s; ok internal/shell 0.401s; ok internal/watcher 11.590s.

baseline follow-ups:
  - TB-249 tracks CLI errcheck/nilerr/unused findings in cli/board.go, cli/scan.go, and cli/ready.go.
  - TB-250 tracks GUI errcheck/errorlint/nilerr/unused findings in gui/app/agent_run.go, gui/app/settings_service_test.go, gui/app/attachments.go, gui/internal/agent/groom_test.go, gui/internal/agent/usage_codex.go, and gui/app/agent_service_test.go.

## Related Tasks

- **TB-205** — Setup eslint and deadcode check for frontend (sibling: frontend-only lint/dead-code setup)
- **TB-249** — Resolve CLI golangci-lint baseline findings (follow-up: CLI first-pass findings from TB-206)
- **TB-250** — Resolve GUI golangci-lint baseline findings (follow-up: GUI first-pass findings from TB-206)

## Related Tasks

- **TB-205** — Setup eslint and deadcode check for frontend (sibling: frontend-only lint/dead-code setup)
- **TB-249** — Resolve CLI golangci-lint baseline findings (follow-up: CLI first-pass findings from TB-206)
- **TB-250** — Resolve GUI golangci-lint baseline findings (follow-up: GUI first-pass findings from TB-206)

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
- 2026-05-15: Edited agentstatus=failed
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited review-target
- 2026-05-19: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewref=main
- 2026-05-19: Submitted to code-review
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agent=codex
