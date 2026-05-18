# TB-227: CLI: refresh generated board docs for existing boards

**Type:** improvement
**Priority:** P1
**Size:** M
**Module:** cli
**Tags:** docs,init,templates
**Branch:** main

## Goal

Add a supported way to refresh generated board CONVENTIONS.md and SKILL.md in projects that already have an initialized tb board, without disturbing task files or project-specific customizations.

## Acceptance Criteria

- [x] `tb` exposes an explicit opt-in path to refresh generated board docs for an already initialized board, such as `tb init --refresh-docs` or a dedicated docs command.
- [x] The refresh path reads the existing `.tb.yaml` board path and prefix by default, and writes docs with the correct project prefix instead of falling back to `PR`.
- [x] The refresh path updates the generated `board/CONVENTIONS.md` and `board/SKILL.md` templates to the current CLI surface, including folder-form tasks, `--legacy-file`, `tb edit`, `tb attach`, `tb assign`, and JSON-capable commands where applicable.
- [x] Existing task files, attachments, `.next-id`, archive contents, and `BOARD.md` task state are not modified except for an intentional regenerate when needed.
- [x] The command is safe for customized docs: it either shows a diff/dry-run and requires confirmation/force for overwrites, writes backups, or clearly documents a supported manual merge workflow.
- [x] Regression coverage proves refresh behavior for a new-style folder board, a legacy file-form board, and a board whose docs already contain local customizations.
- [x] Documentation/help text explains how existing projects can refresh generated docs without reinitializing or losing local board conventions.

## Context

Found during a 2026-05-18 audit of initialized boards under `/Users/ralist/projects`.

- `/Users/ralist/projects/task-board-tools` has `.tb.yaml` prefix `TB`, but its checked-in `board/CONVENTIONS.md` and `board/SKILL.md` still contain `PR` examples in multiple places and are behind `cli/templates.go`.
- `/Users/ralist/projects/books/publishing-platform` has generated docs that are close to the current templates, but they are missing newer template lines for `tb edit`, `triage --json`, folder-form default creation details, and `--legacy-file` examples.
- `/Users/ralist/projects/books/writer-studio` has heavily customized board docs and a Claude skill at `.claude/skills/task-board/SKILL.md`; it intentionally differs from generic generated templates and should be handled as a custom-docs merge case, not a blind overwrite.
- `cli/init.go` currently skips template generation when `board/.next-id` already exists, so rerunning `tb init` updates config only and cannot refresh docs in existing projects.

## Related Tasks

- **TB-97** — Folder-form task creation changed the default generated board layout this refresh path needs to propagate.
- **TB-223** — Secondary docs audit is adjacent repo-doc hygiene; this task is specifically about generated board docs in initialized projects.
- **TB-225** — GUI board initialization should remain aligned with whatever generated docs the CLI owns.

## Attachments

## Log

- 2026-05-18: Created
- 2026-05-18: Groomed from cross-project audit; added evidence from task-board-tools, publishing-platform, and Writer Studio initialized boards.
- 2026-05-18: Started — moved to in-progress
- 2026-05-18: Implemented `tb init --refresh-docs`, refreshed generated docs, added regression coverage, and rebuilt installed CLI binaries.
- 2026-05-18: Done

