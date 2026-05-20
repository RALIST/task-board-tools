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

## Decision

Descoped. The "8 stats" worst case applies only to a file-form task in `archive/`. The dominant case in practice is a folder-form task in `backlog`, `in-progress`, or `done` — that exits the first loop after 1-3 stats. A naive "single pass per status dir checking both forms" would actually be *worse* for the dominant folder-form case (~2x the stats per non-hit status). Caching the per-task layout in `activeRun` is a real optimisation but requires invalidation on promotion / move / restore — too risky for a P2 polish task without first measuring the actual rate of `AppendEvent` calls under a real agent run.

Filing as documented hot path: callers that emit many `AppendEvent`s for the same task can already cache the result of `ResolveArtifactPaths` themselves (the function is pure given the on-disk state). If a future profile shows this is a real bottleneck, the right surface is a per-call layout cap or a callsite cache in the agent runner — not a generic LRU in this package.

## Acceptance Criteria

- [x] Decision recorded; no code change. Follow-up filed in this task's Decision section.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Started — moved to in-progress
- 2026-05-15: Done
