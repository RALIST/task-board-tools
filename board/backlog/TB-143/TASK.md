# TB-143: Add semver to cli tool

**Type:** feature
**Priority:** P2
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** cli
**Tags:** release
**Branch:** —

## Goal

Expose a stable SemVer version surface for the `tb` CLI so humans, scripts, and release packaging can identify the binary they are running. 

## Context

`cli/main.go` routes all top-level commands through one switch. `help`, `-h`, `--help`, and `init` are handled before project config is loaded, but there is no version command or flag today: `cd cli && go run . --version` currently exits as `Unknown command: --version`.

Relevant files:

- `cli/main.go` - top-level command routing and usage text.
- `cli/README.md` - CLI install and command reference.
- `cli/*_test.go` - command-level tests live beside the CLI package.

## Constraints and Non-goals

- Keep the CLI zero-dependency and in the existing single `package main` style.
- `tb version` and `tb --version` must work without `.tb.yaml` or `TB_BOARD_DIR`; they should be config-free like help.
- Local/dev builds should still produce a valid SemVer value, and release builds should be able to inject the release version at build time.
- Do not add release automation, Git tag creation, packaging pipelines, or GUI About-window version display in this task.

## Acceptance Criteria

- [ ] `tb version` and `tb --version` both print exactly one line in the shape `tb <semver>` and exit 0.
- [ ] The printed version token is valid SemVer, including the default local-build value (for example `0.0.0-dev`).
- [ ] Release builds can inject a SemVer value through Go linker flags; smoke example: `cd cli && go build -ldflags "-X main.version=1.2.3" -o tb . && ./tb --version` prints `tb 1.2.3`.
- [ ] Version output does not require a board config: running the built binary from a temporary directory with no `.tb.yaml` still succeeds for `version` and `--version`.
- [ ] CLI usage/help and `cli/README.md` document the version command/flag and the release-build injection form.
- [ ] Focused tests cover the default version output, injected-version output, and config-free behavior; `cd cli && go test ./...` passes.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited type=feature, size=M, module=cli, tags=release, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success
- 2026-05-15: Edited body via GUI
