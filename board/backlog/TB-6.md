# TB-6: M6: Groom flow for AI-assisted task refinement

**Type:** feature
**Priority:** P2
**Size:** L
**Module:** gui
**Tags:** milestone-m6,agent,groom,epic
**Branch:** —

## Goal

Add a Groom button that runs an agent in grooming mode: refine Goal and Acceptance Criteria via tb edit without writing code. Compose with a GroomingDecorator over the existing runners and a separate prompt.

## Context

Groom mode wraps the existing runners in a decorator that swaps the prompt to refine `## Goal` and `## Acceptance Criteria` via `tb edit` (no code changes). Surface a Groom button in TaskDrawer next to Run. Highlight tasks returned by `tb triage`. See plan M6.

## Acceptance Criteria

- [ ] `GroomingDecorator` composes over `ClaudeRunner` / `CodexRunner` with `prompts/groom.md`
- [ ] `AgentService.GroomTask(id)` queues a groom run distinct from a normal run
- [ ] TaskDrawer shows a Groom button next to Run
- [ ] Tasks surfaced by `tb triage` are visually marked in the kanban
- [ ] Backlog task with empty Goal → click Groom → agent updates fields via `tb edit` → GUI reflects the change

## Related Tasks

- **TB-4** — Prerequisite (runner infrastructure)
- **TB-5** — Prerequisite (daemon scheduling)

## Log

- 2026-05-13: Created
