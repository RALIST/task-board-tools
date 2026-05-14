# TB-170: TB-93/GUI: resolveArtifactPaths hot path - 8 stats per agent log line, cache layout

**Type:** improvement
**Priority:** P2
**Size:** S
**Module:** gui
**Tags:** epic-tb93,review-tb93,performance,agent-state
**Branch:** —
**Parent:** TB-93

## Goal

gui/internal/agent/state.go:264-296 - resolveArtifactPaths iterates all four status dirs twice (once for folder form, once for file form). For a task in archive/, that's 8 stat calls. This is a hot path on every AppendEvent (stdout line writes during an agent run). Fix: single pass per status dir checking both forms; or cache the resolved layout in activeRun so per-line sinks don't re-stat. The per-task-mutex taskMutex already serializes writes for one task, so adding an LRU of recent resolutions wouldn't introduce concurrency hazards. Source: GUI backend review finding #13 (LOW).

## Acceptance Criteria

- [ ] (to be filled)

## Attachments

## Log

- 2026-05-15: Created
