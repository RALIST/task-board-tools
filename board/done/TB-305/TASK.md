# TB-305: CLI: install project task-board skills during init

**Type:** improvement
**Priority:** P2
**Size:** M
**Agent:** codex
**Module:** cli
**Tags:** init,templates,skills,claude,codex
**GroomedBy:** codex
**GroomStatus:** success
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**ReviewRef:** working-tree
**ReviewedBy:** codex
**ReviewStatus:** success
**Branch:** —

## Goal

Make `tb init` seed project-local task-board skills for Claude Code and Codex from the same rendered board skill template, so a newly initialized board gives both agents the board workflow instructions without manual copying.

## Context

`cli/init.go` currently initializes or reconciles a board by creating the board status directories, `.next-id`, generated `BOARD.md`, generated `CONVENTIONS.md`, generated `SKILL.md`, and `.tb.yaml`. `cli/templates.go` owns the rendered task-board skill content, and `cli/docs_refresh.go` already provides byte-identical no-op and `.bak` backup behavior for generated board docs.

Official docs checked during grooming on 2026-05-20:
- Claude Code project skills live at `.claude/skills/<skill-name>/SKILL.md`; `task-board` should therefore install to `.claude/skills/task-board/SKILL.md`.
- Codex repo skills are discovered from `.agents/skills` between the launch directory and repository root; `task-board` should therefore install to `.agents/skills/task-board/SKILL.md` for Codex. Do not assume `.codex/skills/task-board/SKILL.md` is the current documented Codex repo-skill path unless implementation re-verifies newer docs and records the changed source.

Related work: TB-273 covers the broader interactive `tb init` flow and already includes optional Claude/Codex skill install criteria. If TB-273 is still active when this starts, either fold this task's doc-correct destination/default behavior into that implementation or close one task as duplicate with evidence. TB-306 may change the generated board skill/conventions content; this task should install whatever the current rendered template is, not duplicate stale prose.

## Constraints

- Keep scripted and non-interactive `tb init` deterministic: it must not hang waiting for skill-install input.
- Do not silently overwrite customized project-local skill files. Byte-identical installed skill content should be a no-op; changed/custom content needs explicit confirmation in interactive mode or a `.bak` backup path in non-interactive/flag-driven mode.
- Derive installed skill files from the rendered board skill template instead of maintaining separate Claude/Codex copies.
- Preserve existing init/reconcile guarantees: do not disturb task files, attachments, `.next-id`, archive contents, generated board state beyond the documented init refresh behavior, or unrelated files under `.claude`, `.codex`, or `.agents`.
- Rebuild and relink the local `tb` binary after `/cli/` changes.

## Acceptance Criteria

- [ ] Fresh interactive `tb init` offers project-local task-board skill setup for Claude Code and Codex, defaults are clear to the user, and there is an explicit way to skip installation.
- [ ] Scripted/non-interactive `tb init` remains backward-compatible and never prompts; documented flags or modes can explicitly install or skip project-local skills.
- [ ] Claude install creates or refreshes `.claude/skills/task-board/SKILL.md` from the rendered task-board skill template, creating parent directories as needed and leaving unrelated `.claude` files untouched.
- [ ] Codex install creates or refreshes `.agents/skills/task-board/SKILL.md` from the rendered task-board skill template, creating parent directories as needed and leaving unrelated `.agents` / `.codex` files untouched.
- [ ] Existing installed skill files are handled safely: byte-identical content is a no-op with no backup, customized content is preserved via confirmation or `.bak`, and declining/skipping leaves the file unchanged.
- [ ] CLI help and user-facing docs describe the skill-install behavior, non-interactive controls, and current Claude/Codex project skill destinations.
- [ ] Regression coverage exercises fresh init, existing-board reconcile, byte-identical no-op, customized-file backup/skip, non-interactive no-prompt behavior, and the exact Claude/Codex destination paths.
- [ ] Manual smoke: in a throwaway repo, run the default interactive flow and an explicit scripted flow; verify `tb ls` works, the selected skill files exist at the documented destinations, rerunning `tb init` is a no-op when bytes match, and customized skill files are not overwritten silently.
- [ ] Verification includes `cd cli && go test ./...`, rebuilding `cd cli && go build -o tb .`, and relinking the local `tb` binary used by this repo.

## Review Target

Implementation scope:
- `tb init` supports `--install-skills=auto|all|claude|codex|none`.
- Interactive init can prompt for project skill install; scripted init remains non-interactive unless flag explicitly installs.
- Claude skill installs to `.claude/skills/task-board/SKILL.md`; Codex skill installs to `.agents/skills/task-board/SKILL.md` from current rendered task-board skill template.
- Byte-identical files are no-op; customized files are preserved with prompt/skip or `.bak` backup behavior.
- CLI help, README, and CLI README describe behavior.

Verification:
- cd cli && go test ./...
- cd cli && go build -o tb .
- relinked /Users/ralist/go/bin/tb and /Users/ralist/.local/bin/tb
- throwaway init smoke with --install-skills=all and no-op rerun
- git diff --check

## Review Findings

Second review: Critical issues none. Important issues none.

Verification passed:
- cd cli && go test ./...
- cd cli && go build -o tb .
- explicit init skill smoke passed: selected project skill files created, rerun no-op, local tb relinked
- git diff --check

## Related Tasks

- **TB-273** — Broader interactive `tb init` work; coordinate to avoid duplicate implementations.
- **TB-227/TB-230** — Existing generated-doc refresh and byte-identical no-op behavior that skill installs should mirror.
- **TB-306** — Generated board skill/conventions content cleanup; installed skill content should come from the current template after any cleanup lands.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited priority=P2, type=improvement, size=M, module=cli, tags=init,templates,skills,claude,codex, agentstatus=running, groomed-by=codex, groom-status=running, title=CLI: install project task-board skills during init
- 2026-05-20: Edited goal
- 2026-05-20: Edited context
- 2026-05-20: Edited constraints
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=success, groom-status=success
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Edited agentstatus=success, groomed-by=codex, groom-status=success
- 2026-05-21: Pulled into in-progress
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=interrupted
- 2026-05-21: Edited agentstatus=queued
- 2026-05-21: Edited agentstatus=running
- 2026-05-21: Edited agentstatus=success, implemented-by=codex, implement-status=success, reviewref=working-tree
- 2026-05-21: Edited review-target
- 2026-05-21: Submitted to code-review
- 2026-05-21: Passed code review
- 2026-05-21: Edited reviewed-by=codex, review-status=success

