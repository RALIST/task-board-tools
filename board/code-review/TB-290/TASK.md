# TB-290: CLI: edit Context and Constraints sections

**Type:** improvement
**Priority:** P2
**Size:** S
**Agent:** codex
**AgentStatus:** success
**Module:** cli
**Tags:** edit,body-sections,quick-win
**GroomedBy:** codex
**GroomStatus:** success
**ReviewRef:** 3ebef31
**ImplementedBy:** codex
**ImplementStatus:** success
**Branch:** —

## Goal

Allow `tb edit` to replace or insert the `## Context` and `## Constraints` sections of a task through managed CLI flags, so grooming agents can clean up body sections without direct markdown edits.

## Context

- TB-285 is blocked in `needs-user` because the current managed edit path cannot remove duplicated `## Context` / `## Constraints` sections or reword text inside them.
- `tb edit --help` currently exposes section edits for `--goal`, `--acceptance`, and `--user-attention`, but not `--context` or `--constraints`.
- `cli/edit.go` already has managed body-section replacement helpers for a small allowlist of headings; this task should extend that surface instead of introducing a separate mutation path.
- Board conventions prefer managed tooling for structured changes and require task markdown writes to preserve metadata, status, attachments, and logs.

## Constraints

- CLI-only improvement; do not change GUI behavior except through any regenerated bindings only if the implementation explicitly extends GUI usage later.
- Preserve the existing metadata block, status directory, related tasks, attachments, agent state/log files, and task log history when editing body sections.
- Continue to use the board lock and atomic task-file write path used by existing `tb edit` body-section updates.
- Section parsing must remain safe for fenced code blocks and literal markdown examples that contain `## Context` or `## Constraints` text.
- Do not add a free-form arbitrary-body rewrite command in this task; scope the feature to the two named sections.

## Acceptance Criteria

- [ ] `tb edit --help` documents `--context file|-` and `--constraints file|-` alongside the existing managed section-edit flags.
- [ ] `tb edit TB-NNN --context -` replaces an existing `## Context` section from stdin and inserts it in the canonical body order when the section is missing.
- [ ] `tb edit TB-NNN --constraints -` replaces an existing `## Constraints` section from stdin and inserts it after Context and before Acceptance Criteria when the section is missing.
- [ ] Updating Context or Constraints preserves task metadata, status, attachments, related sections, existing log history, and any unrelated body sections.
- [ ] Section replacement remains fenced-heading-aware: literal `## Context` / `## Constraints` text inside fenced code blocks or section content does not corrupt surrounding sections.
- [ ] CLI tests cover replace, insert, stdin/file input, leading-heading stripping, canonical placement, and fenced-heading examples for both new flags.
- [ ] `cd cli && go test ./...` passes.
- [ ] `cd cli && go build -o tb .` has been run so the local untracked `tb` binary matches the CLI change.

## Review Target

commit: 3ebef31
verification: cd cli && go test -count=1 ./...; cd cli && go build -o tb .
notes: managed --context/--constraints support added to tb edit; subagent review attempts stalled, so this is submitted to code-review rather than marked done.

## Related Tasks

- **TB-285** — blocked grooming cleanup that needs a managed way to edit Context and Constraints sections.

## Attachments

## Log

- 2026-05-20: Created
- 2026-05-20: Edited type=improvement
- 2026-05-20: Edited size=S
- 2026-05-20: Edited body via GUI
- 2026-05-20: Edited body via GUI
- 2026-05-20: Edited agent=codex
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited module=cli, tags=edit,body-sections,quick-win, groomed-by=codex, groom-status=running, title=CLI: edit Context and Constraints sections
- 2026-05-20: Edited goal
- 2026-05-20: Edited acceptance
- 2026-05-20: Edited agentstatus=success, groom-status=success
- 2026-05-20: Committed — moved to ready
- 2026-05-20: Pulled into in-progress
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited agentstatus=interrupted
- 2026-05-20: Edited agentstatus=queued
- 2026-05-20: Edited agentstatus=running
- 2026-05-20: Edited review-target
- 2026-05-20: Edited reviewref=3ebef31
- 2026-05-20: Submitted to code-review
- 2026-05-20: Edited agentstatus=success, implemented-by=codex, implement-status=success

