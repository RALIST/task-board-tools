# TB-229: CLI: reconcile .tb.yaml with annotated config template

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** cli
**Tags:** init,config,templates
**Branch:** main

## Goal

Make tb init expand minimal .tb.yaml files into the full supported config surface, keeping optional keys commented with defaults and backing up previous config before rewrite.

## Acceptance Criteria

- [x] `tb init` expands a minimal `.tb.yaml` into the full supported config surface with comments.
- [x] Optional config keys are commented with defaults when unset.
- [x] Existing configured optional values remain active instead of being replaced by commented defaults.
- [x] `.tb.yaml` is backed up before reconcile rewrites it.
- [x] CLI help, generated board docs, and README describe config refresh behavior.

## Attachments

## Log

- 2026-05-18: Created
- 2026-05-18: Started — moved to in-progress
- 2026-05-18: Added annotated `.tb.yaml` renderer and tests for minimal config expansion, active optional fields, and backup behavior.
- 2026-05-18: Done

