# TB-232: tb-gui usage tap: chain to user's original statusline instead of replacing it

**Type:** bug
**Priority:** P1
**Size:** S
**Module:** gui
**Tags:** cli,review
**Branch:** feat/TB-130-agent-session-resume (piggy-backed)

## Goal

When EnableClaudeUsageTap patches settings.local.json, it points statusLine.command at a script that only echoes 'tb-gui tap'. This silently replaces any existing statusLine (notably the user's global one in ~/.claude/settings.json), so a nicely formatted statusline (model + git branch + context %) gets lost when the tap is enabled. Fix: capture the existing non-ours statusLine command at install time (settings.local.json first, then ~/.claude/settings.json), persist it in a sidecar, and generate a tap script that saves tb-gui-usage.json AND pipes stdin to the captured command, forwarding its output. On Disable, restore the original to settings.local.json only if that's where it lived; otherwise just remove our entry (unmasks the global).

## Acceptance Criteria

- [x] Enable captures existing non-ours statusLine: settings.local.json wins over ~/.claude/settings.json
- [x] Sidecar `tb-gui-statusline-original.json` records the captured command + source ("local"/"global")
- [x] Generated tap script saves `tb-gui-usage.json` AND pipes the same stdin to the captured command via `bash -c`, forwarding stdout
- [x] No chain captured → script falls back to the minimal `'tb-gui tap'` line
- [x] Re-Enable preserves the previously-captured local original (doesn't try to capture our own entry)
- [x] Disable with source=local: writes the original entry back into settings.local.json
- [x] Disable with source=global: removes our entry only, unmasking the global statusLine
- [x] Sidecar is gitignored; gitignore is idempotent
- [x] Single-quote-safe embedding (`'\''` escaping) verified by unit test + integration test
- [x] All existing tap tests pass; tests are hermetic (HOME isolated)

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Implemented chain-on-Enable + restore-on-Disable in `gui/app/claude_tap.go`; sidecar `tb-gui-statusline-original.json` records captured command + source. `buildTapScript` now embeds the captured command via `shellSingleQuote` and runs it through `bash -c` with the same stdin piped through.
- 2026-05-19: Added 8 new tests in `gui/app/claude_tap_test.go` (capture-local, capture-global, restore-local, unmask-global, re-enable preserves sidecar, end-to-end chain, single-quote safety, helper unit test). Existing 7 tests updated to isolate `$HOME` for hermeticity. All 15 tests pass. Full `gui/app` suite passes (ok ~44s).
- 2026-05-19: Verified end-to-end against the developer's actual `~/.claude/settings.json` (`sh /Users/ralist/.claude/statusline-command.sh`): chained output produces the user's colored multi-segment statusline (`~/path (branch) [model | context%]`) while `tb-gui-usage.json` still records the JSON payload.
- 2026-05-19: Done
