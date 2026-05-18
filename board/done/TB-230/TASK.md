# TB-230: CLI: avoid backups when init content is unchanged

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** cli
**Tags:** init,config,templates
**Branch:** main

## Goal

Ensure tb init does not create .bak files when generated docs or config content is byte-identical to the desired current content.

## Acceptance Criteria

- [x] `tb init` does not create `.tb.yaml.bak` when the existing config content is already byte-identical to the rendered template.
- [x] `tb init` does not create `CONVENTIONS.md.bak` or `SKILL.md.bak` when generated board docs are already byte-identical to the current templates.
- [x] Command output reports config/docs already current in the no-op path.
- [x] Regression coverage exercises the end-to-end existing-board no-op path.

## Attachments

## Log

- 2026-05-18: Created
- 2026-05-18: Started — moved to in-progress
- 2026-05-18: Added regression coverage proving byte-identical config/docs do not create `.bak` files.
- 2026-05-18: Done

