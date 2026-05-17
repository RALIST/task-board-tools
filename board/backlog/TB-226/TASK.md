# TB-226: CLI: preserve literal command examples in task creation

**Type:** bug
**Priority:** P1
**Size:** S
**Agent:** codex
**AgentStatus:** success
**Module:** cli
**Tags:** shell-quoting,quick-win
**Branch:** —

## Goal

Preserve command examples as literal task text when `tb create` receives them, and make the shell quoting boundary clear enough that Markdown code spans are not accidentally executed before `tb` starts.

## Context

Reported repro:

```sh
tb create "Try to init board with `tb init` and check if command passes" -d "Some description included command tb --help"
```

In POSIX shells, backticks inside double quotes are command substitution, so `tb init` runs before the CLI receives the title. The CLI cannot recover the original backtick text after the shell has replaced argv.

Relevant surfaces:

- `cli/create.go` builds the task from argv values and redacts `-d`; when literal backticks arrive in argv, they should be saved unchanged.
- `tb create --help`, CLI docs, and generated usage guidance should show a safe literal-command example, such as single-quoted title/description values or a follow-up `tb edit --goal - <<'EOF'` body edit for richer Markdown.
- If investigation finds a GUI/agent create path constructing shell strings instead of argv, file or link a separate GUI follow-up; this card is for the CLI-facing create behavior and guidance.

Constraints / Non-goals:

- Do not try to detect or undo command substitution that already happened in the caller shell.
- Do not strip, escape, or rewrite Markdown backticks in saved task titles or descriptions.
- Preserve board mutation invariants: `.board.lock`, atomic task writes, `BOARD.md` regeneration, and existing redaction behavior for user-supplied task text.

## Acceptance Criteria

- [ ] Add a CLI regression test that passes a title containing literal `` `tb init` `` and a description containing literal `` `tb --help` `` directly as argv to the create path; the created task markdown preserves both code spans verbatim and does not contain command output.
- [ ] Add or update a shell-facing smoke test/manual check using a temporary board and safe quoting, for example ``tb create 'Try to init board with `tb init` and check if command passes' -m cli -d 'Some description included command `tb --help`'``, then `tb show <new-id>` shows the literal backtick text.
- [ ] `tb create --help` and the canonical CLI usage docs/guidance include a safe example for Markdown command spans and explicitly note that backticks inside double quotes are evaluated by the caller shell before `tb` runs.
- [ ] Existing create behavior remains unchanged for normal titles/descriptions, parent tasks, folder-form output, `BOARD.md` regeneration, and description redaction.
- [ ] `cd cli && go test ./...` passes.

## Related Tasks

- **TB-39** — Related Markdown literal/section-boundary robustness; command examples in task text must remain content, not structure.
- **TB-203** — Related user-supplied task text redaction; keep redaction intact while preserving literal command spans.

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Edited body via GUI
- 2026-05-17: Edited agent=codex
- 2026-05-17: Edited agentstatus=queued
- 2026-05-17: Edited agentstatus=running
- 2026-05-17: Edited priority=P1, module=cli, tags=shell-quoting,quick-win, title=CLI: preserve literal command examples in task creation
- 2026-05-17: Edited goal
- 2026-05-17: Edited acceptance
- 2026-05-17: Edited acceptance
- 2026-05-17: Edited acceptance
- 2026-05-17: Edited agentstatus=success

