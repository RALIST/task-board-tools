# TB-221: Prepare repository for GitHub push

**Type:** tech-debt
**Priority:** P1
**Size:** S
**Module:** tooling
**Tags:** repo-hygiene,github,security
**Branch:** main

## Goal

Audit git state, ignore rules, agent artifacts, and secret exposure before pushing to GitHub.

## Acceptance Criteria

- [x] Git remote is configured for `https://github.com/RALIST/task-board-tools`
- [x] Ignore rules cover local agent/runtime/build artifacts
- [x] Secret and sensitive-file scans show no committed exposure
- [x] Verification commands pass or blockers are documented
- [x] Changes are committed, pushed, and the working tree is clean

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Started — moved to in-progress
- 2026-05-17: Began GitHub push readiness audit for remotes, ignore rules, artifacts, secrets, verification, and clean working tree.
- 2026-05-17: Configured GitHub origin, purged agent logs/state from reachable history, kept `.claude/` via `.gitkeep` while ignoring its local contents, and verified no committed agent-log paths or token patterns remain.
- 2026-05-17: Verification passed: `cd cli && go build -o tb .`; `cd cli && go test ./...`; `cd gui && go test ./...`; `cd gui/frontend && npm run check`; `cd gui/frontend && npm test -- --run`.
- 2026-05-17: Pushed sanitized `main` to GitHub and set the repository default branch to `main`.
- 2026-05-17: Done

