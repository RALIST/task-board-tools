# TB-273: CLI: make tb init interactive

**Type:** improvement
**Priority:** P1
**Size:** M
**Agent:** codex
**AgentStatus:** lost
**Module:** cli
**Tags:** init,ux,templates,skills
**ImplementedBy:** codex
**ImplementStatus:** lost
**Branch:** —

## Goal

Make `tb init` offer a guided interactive setup for humans while preserving the current scriptable initialize-or-reconcile behavior for automation.

## Context

`cli/init.go` currently implements `tb init [path] --board-path=board --prefix=PR --refresh-docs`: fresh boards get status directories, `.next-id`, `BOARD.md`, `CONVENTIONS.md`, `SKILL.md`, and `.tb.yaml`; existing boards are reconciled using `.tb.yaml` values, refreshed generated docs, annotated config output, `.bak` backups, and byte-identical no-op behavior. `cli/templates.go` owns the rendered board `SKILL.md` content that should be reused for any agent-skill installs.

This task builds on the completed init-reconcile slice from TB-227, TB-228, TB-229, and TB-230. It is adjacent to, but separate from, TB-225's GUI init flow.

## Constraints

- Keep `tb init` safe for scripts: non-interactive stdin/stdout, CI-style use, and explicit skip/yes behavior must never wait for input.
- Existing flags remain supported and override prompt defaults; existing `.tb.yaml` values seed defaults when reconciling an initialized board.
- Preserve existing reconcile guarantees: do not disturb task files, attachments, `.next-id`, archive contents, or generated board state beyond the current documented init behavior.
- Optional project-local skill installs must be opt-in and must not overwrite customized `.claude` or `.codex` files silently.
- Non-goal: redesign the GUI missing-board init flow from TB-225.

## Acceptance Criteria

- [ ] On an uninitialized project in an interactive terminal, `tb init` prompts for project root, board directory, task ID prefix, and optional project-local task-board skill install for Claude Code and Codex; defaults are shown and pressing Enter accepts them.
- [ ] Scripted/non-interactive use remains backward-compatible: `tb init`, `tb init <path>`, `--board-path`, `--prefix`, and `--refresh-docs` still work without blocking for input, and a documented flag or mode can explicitly skip prompts in a TTY.
- [ ] On an existing board, the interactive prompts are prefilled from the existing `.tb.yaml`; accepting defaults preserves the current initialize-or-reconcile behavior, including generated docs/config refresh, `.bak` handling, and byte-identical no-op output.
- [ ] Input validation is explicit and default-aware: pressing Enter on a prompt with a displayed default uses that default, while whitespace-only or otherwise empty values after default resolution, absolute board directories, and board directories that escape the project root are rejected before writes; interactive mode re-prompts and non-interactive mode exits with an actionable error.
- [ ] Opting into Claude Code skill install creates or updates `.claude/skills/task-board/SKILL.md` from the rendered board skill template, creating parent directories as needed and leaving unrelated `.claude` files untouched.
- [ ] Opting into Codex skill install creates or updates `.codex/skills/task-board/SKILL.md` from the rendered board skill template, creating parent directories as needed and leaving unrelated `.codex` files untouched.
- [ ] Existing project-local skill files are handled safely: byte-identical content is a no-op, customized content is either confirmed before overwrite or backed up before replacement, and declining/skipping leaves the file unchanged.
- [ ] CLI help and user-facing docs describe the interactive prompts, the non-interactive/scriptable path, and the optional Claude/Codex project skill install destinations.
- [ ] Regression coverage exercises fresh interactive init, existing-board interactive defaults, non-interactive/scripted init, invalid-answer handling, and Claude/Codex skill install no-op/backup/skip cases.
- [ ] Manual smoke: in a throwaway terminal project, run interactive `tb init` with defaults and with custom prefix/board directory; verify the board opens with `tb ls`, the selected skill installs land at `.claude/skills/task-board/SKILL.md` and `.codex/skills/task-board/SKILL.md`, and a piped/non-TTY invocation does not prompt.

## Related Tasks

- **TB-225** — GUI missing-board init flow; keep CLI defaults and artifact layout aligned, but do not redesign the GUI here.
- **TB-227** — Existing-board generated docs refresh path that this task must preserve.
- **TB-228** — Default existing-board reconcile behavior that interactive mode must keep intact.
- **TB-229** — Annotated `.tb.yaml` reconcile behavior that should remain the config surface.
- **TB-230** — Byte-identical no-op backup suppression that also applies to generated skill installs.

## Related Tasks

- **TB-225** — GUI missing-board init flow; keep CLI defaults and artifact layout aligned, but do not redesign the GUI here.
- **TB-227** — Existing-board generated docs refresh path that this task must preserve.
- **TB-228** — Default existing-board reconcile behavior that interactive mode must keep intact.
- **TB-229** — Annotated `.tb.yaml` reconcile behavior that should remain the config surface.
- **TB-230** — Byte-identical no-op backup suppression that also applies to generated skill installs.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited priority=P1, type=improvement, size=M, module=cli, tags=init,ux,templates,skills, title=CLI: make tb init interactive
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited acceptance
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=lost, implemented-by=codex, implement-status=lost
