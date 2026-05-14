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

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
