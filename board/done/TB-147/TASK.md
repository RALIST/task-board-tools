# TB-147: TB-93/CLI: implement startup recovery sweep for stale .promote/.attach staging dirs OR amend doc

**Type:** tech-debt
**Priority:** P1
**Size:** S
**Module:** cli
**Tags:** epic-tb93,review-tb93,docs,recovery
**Branch:** —
**Parent:** TB-93

## Goal

docs/ARCHITECTURE.md (File->folder promotion section) claims partially-built staging dirs left by a crash 'can be GC'd by startup recovery' and references 'an explicit tb-side recovery sweep on startup' for dual-form orphans. grep -nE 'promote\.|\.attach\.|startup' cli/*.go finds no such sweep - stale .<ID>.promote.<pid>.<rand>/ and .attach.<pid>.<rand>/ directories accumulate forever in <status>/ and <status>/<ID>/. Functionally invisible (dot-prefixed, skipped by readers) but doc materially overstates implementation. Fix: either implement init-time / loadProjectConfig-time sweep that deletes stale dot-prefixed staging dirs older than ~1h and where PID is dead, or amend the doc to say 'left until manual cleanup'. Source: CLI grand review finding #3.

## Decision

Amend docs. Implementing a startup sweep is out of scope for an S-size hygiene task: stale `.<ID>.promote.<pid>.<rand>/` and `.attach.<pid>.<rand>/` directories are dot-prefixed and ignored by every reader and by `BOARD.md` regeneration. They are pure disk cruft, not a correctness risk. The doc was overpromising.

## Acceptance Criteria

- [x] `docs/ARCHITECTURE.md` no longer claims `tb` performs a startup recovery sweep. The "File → folder promotion" and "Resolution order" sections describe what actually happens: dual-form is self-healed on the next structured mutation; partially-built staging dirs are inert and left until manually cleaned up.
- [x] No code change required for staging dir cleanup. A future opportunistic sweep is acknowledged but not implemented here.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done

