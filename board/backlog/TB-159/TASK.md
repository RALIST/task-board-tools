# TB-159: TB-93/GUI: resolveArtifactPaths should normalize taskID to uppercase

**Type:** bug
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,agent-state
**Branch:** —
**Parent:** TB-93

## Goal

gui/internal/agent/state.go:275 in resolveArtifactPaths does strings.TrimSpace(taskID) but not strings.ToUpper. Peer resolvers (resolveTaskDir in attachments.go, findTaskFile in edit_body.go) uppercase the ID. If any caller passes lowercase ('tb-1') the function silently falls through every status dir, returns legacy paths with lowercase basenames (<board>/.agent-state/tb-1.jsonl) and the GUI now has a parallel-universe state file diverged from the canonical TB-1.jsonl. Production callers happen to source IDs from tb show --json which already normalizes, so this is latent. Fix: add taskID = strings.ToUpper(taskID) after the TrimSpace, matching peer resolvers; or assert non-empty uppercase ID and return an error otherwise. Source: GUI backend review finding #5.

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
