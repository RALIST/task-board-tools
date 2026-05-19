# TB-206: Setup golangci-lint for project and initial run it

**Type:** tech-debt
**Priority:** P2
**Size:** M
**Agent:** claude
**AgentStatus:** success
**Module:** tooling
**Tags:** lint,go,quality
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** main
**ReviewedBy:** claude
**ReviewStatus:** success
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

## Review Findings

No blocking findings. Implementation matches the Review Target scope and meets all acceptance criteria.

Verified during this review:
- `.golangci.yml`, `scripts/lint-go.sh`, and `Makefile` provide a root wrapper that enters `cli/` and `gui/` explicitly without treating the repo root as a Go module. v2 config schema is valid against golangci-lint v2.11.3.
- Enabled linters (errcheck, govet, ineffassign, staticcheck SA*, unused, bodyclose, errorlint, nilerr, nilnesserr, makezero, wastedassign, unconvert) match the "high-signal" goal; the deferred families (revive, wsl_v5, cyclop, funlen, gocognit, misspell, varnamelen, staticcheck ST/QF/S) are documented in config comments at .golangci.yml:13-16.
- AGENTS.md, CLAUDE.md, and README.md each gained a `make lint-go` line under the verification block.
- `GOLANGCI_LINT=/Users/ralist/go/bin/golangci-lint make lint-go` → cli: 0 issues, gui: 0 issues (re-run during this review).
- `cd cli && go test ./...` → ok tools/tb (cached).
- TB-249 and TB-250 exist on disk under board/backlog/ and their acceptance criteria match every baseline suppression in `.golangci.yml` 1:1 (CLI: errcheck cli/board.go syscall.Flock, errcheck cli/scan.go filepath.Walk, nilerr in cli/board.go|ready.go|scan.go, unused writeSimpleYAML/statusFromTaskPath; GUI: errcheck gui/app/agent_run.go and settings_service_test.go, errorlint gui/app/attachments.go and groom_test.go, nilerr usage_codex.go, unused hasActiveRunID/recordingEmitter.names).

Non-blocking observations:
- (nit) README.md:94 hardcodes `GOLANGCI_LINT=/Users/ralist/go/bin/golangci-lint make lint-go`, embedding the author's home path in the public README. AGENTS.md correctly uses `/path/to/golangci-lint`. Replace the README example with the same placeholder for parity.
- (nit) Commit subject is "tooling: TB-206 add Go lint workflow" but no `.github/workflows/*.yml` was added — "workflow" here means the developer workflow, not CI. Acceptance criteria don't require CI gating, so this is fine; if CI enforcement is desired, capture it as a follow-up so the wording isn't misleading.
- (nit) The baseline suppressions in `.golangci.yml` use file-path + message-text patterns, so genuinely new findings matching the same message in the same file would be silently masked while TB-249/TB-250 are open. Worth a sanity-check when those follow-ups remove the rules — golangci-lint's default `unused-rules` warning would also catch stale rules and could be enabled to surface that.
- (nit) `scripts/lint-go.sh` is sound (per-module loop, status accumulation, `command -v` for both absolute paths and PATH names). One small ergonomic gap: `set -e` plus `|| status=$?` correctly continues across modules, but if golangci-lint emits a configuration error (exit ≥ 2 with no findings), the script still proceeds to the next module. Acceptable for a baseline gate; consider an early-exit on non-finding errors only if this becomes confusing in practice.

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
- 2026-05-19: Edited agent=claude
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited review-findings
- 2026-05-19: Edited agentstatus=success, reviewed-by=claude, review-status=success
- 2026-05-19: Moved to done

