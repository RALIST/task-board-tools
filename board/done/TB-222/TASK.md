# TB-222: Refresh README for current repo layout

**Type:** improvement
**Priority:** P1
**Size:** S
**Module:** docs
**Tags:** docs,repo-layout,readme
**Branch:** main

## Goal

Update stale README layout and usage notes now that the CLI lives in cli/, the GUI lives in gui/, and go.work ties the modules together.

## Acceptance Criteria

- [x] Root README no longer describes the pre-M1 `tb/` separate-repo layout
- [x] Quick start and build commands match the current `cli/`, `gui/`, and `go.work` structure
- [x] README status reflects shipped CLI/GUI/folder-task milestones without over-claiming completed backlog

## Attachments

## Log

- 2026-05-17: Created
- 2026-05-17: Started — moved to in-progress
- 2026-05-17: Confirmed README was stale against current repo layout, CLI README, GUI Taskfile, and root go.work.
- 2026-05-17: Updated root README quick start, current layout, build/test commands, prerequisites, and status section; verified stale pre-M1 phrases are gone and `git diff --check` is clean.
- 2026-05-17: Done
