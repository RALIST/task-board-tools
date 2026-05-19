# TB-271: Fix Codex post-tool hook timeout

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** tooling
**Tags:** codex,hook
**Branch:** main

## Goal

Keep the Codex apply_patch post-hook scoped and fast so it does not run full-project checks or exceed hook timeouts during single-file edits.

## Context

- This was discovered while editing board task files: the post-tool hook reacted to a small `apply_patch` operation by creating this follow-up, which suggests the hook is doing too much work synchronously.
- Hook work should be narrowly scoped to the changed paths and should fail advisory rather than blocking ordinary board/document edits.

## Constraints / Non-goals

- Do not disable useful safety checks outright.
- Keep any expensive full-repo checks as explicit user/developer commands, not default post-edit hook behavior.
- Preserve Codex hook compatibility and existing hook configuration layout.

## Acceptance Criteria

- [x] Remove the stale repo-local Codex PostToolUse hook files that still run formatter/Svelte checks.
- [x] Confirm the user-level Codex hook config remains in ~/.codex/hooks.json and does not run the stale Svelte check path.
- [x] Verify the repo no longer has local hook files that can trigger the 30s timeout.

## Related Tasks

- **TB-86** — Existing Codex post-edit hook adaptation.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited acceptance
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Pulled into in-progress
- 2026-05-19: Removed stale repo-local Codex hook config/script; confirmed global PostToolUse hook stays scoped and does not invoke Svelte checks.
- 2026-05-19: Done

