# TB-228: CLI: make init refresh existing boards by default

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** cli
**Tags:** init,templates,config
**Branch:** main

## Goal

Make tb init reconcile an already initialized project by refreshing generated project files with .bak backups instead of requiring a separate refresh flag.

## Acceptance Criteria

- [x] Plain `tb init` on an existing board refreshes generated project docs with `.bak` backups.
- [x] `--refresh-docs` remains accepted for scripts, but is no longer required for refresh behavior.
- [x] Existing `.tb.yaml` files are not rewritten when no config change is needed, preserving extra fields byte-for-byte.
- [x] Explicit config changes write a `.tb.yaml.bak` backup and preserve unrelated config fields.
- [x] Generated docs and CLI help describe `tb init` as initialize-or-reconcile behavior.

## Attachments

## Log

- 2026-05-18: Created
- 2026-05-18: Started — moved to in-progress
- 2026-05-18: Implemented default existing-board refresh with `.bak` backups and config-preserving reconciliation.
- 2026-05-18: Done

