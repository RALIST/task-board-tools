# TB-6: M6: Groom flow for AI-assisted task refinement

**Type:** feature
**Priority:** P2
**Size:** L
**Module:** gui
**Tags:** milestone-m6,agent,groom,epic
**Branch:** —

## Goal

Surface a **Groom** button in `TaskDrawer.svelte` that runs the assigned agent in "grooming mode" — refines `## Goal` and `## Acceptance Criteria` of the current task via the narrow body-edit CLI surface added in TB-75 (`tb edit --goal -` / `tb edit --acceptance -`) without writing code. Composes a `GroomingDecorator` over the existing `ClaudeRunner` / `CodexRunner` to swap the prompt; reuses M4 / M5 lifecycle plumbing (JSONL events, Wails fan-out, daemon pickup, cancel ordering) so groom runs participate in the same crash-safety guarantees. Cards flagged by `tb triage` (placeholder Goal / missing module / auto-created by scan) wear a "needs grooming" indicator.

## Context

The Groom flow is intentionally a thin layer over the M4 runner + M5 daemon, not a parallel system:

- **Prompt swap, not behaviour swap**: `GroomingDecorator` wraps a concrete `Runner` (captures `PromptVars` at construction — see TB-66), renders the embedded `prompts/groom.md` against those vars at `Run` time, overwrites `RunInput.Prompt`, and delegates. `RunInput` schema stays byte-stable. `claude.go` / `codex.go` stay mode-agnostic; JSONL `started`/`finished` still record `agent: "claude"|"codex"` so M5's `pidAlive(pid, expectedAgent)` cross-check is unaffected (TB-60 contract).
- **Body-edit surface**: the groom prompt cannot ship without a CLI that lets the agent rewrite `## Goal` / `## Acceptance Criteria`. `cli/edit.go` today only mutates metadata. TB-75 adds the narrow flags (`tb edit --goal -` / `tb edit --acceptance -`) that go through `lockBoard` + `writeFileAtomic` + `regenerateBoard`. The prompt (TB-65) wires the agent to exactly that surface — no direct file edits, no `tb start`/`tb done`.
- **Mode plumbing**: `agent.ModeGroom` already exists in `runner.go`; M6 just propagates it through `activeRun.Mode`, the JSONL `Event.Mode` field (already wired in M4 `state.go`), and the `agent:run-queued`/`-started`/`-finished` Wails payloads. The daemon's `RunQueuedAgentSync` (TB-54) reads `Mode` from the queued JSONL event so a CLI-side `tb edit X --agent-status queued` on a task whose intended mode was groom still picks up the decorator.
- **Cancel is shared**: `CancelRun(id)` cancels whichever run is active. No second cancel surface.
- **Triage is data-driven**: `tb triage --json` (TB-69) is the source of truth. `BoardService.Triage()` (TB-70) caches and invalidates on watcher events so cards don't re-shell `tb` per render. The frontend stores the set in `triageStore` and Card.svelte reads it as O(1).

See `docs/FEATURES.md` § M6 (F6.1, F6.2), `docs/IMPLEMENTATION.md` § M6, `docs/ARCHITECTURE.md` (GroomingDecorator + BoardService + JSONL mode field).

## Subtasks

- **TB-65** (S) — `prompts/groom.md` + embedded `agent.PromptGroom` constant
- **TB-66** (M) — `agent.GroomingDecorator` wraps a `Runner` and swaps the prompt
- **TB-67** (M) — `AgentService.GroomTask(id)` — RunAgent twin that records `Mode=groom` and applies the decorator
- **TB-68** (S) — `RunQueuedAgentSync` reads `Mode` from the queued JSONL event and applies the decorator on daemon pickup
- **TB-69** (S) — CLI: `tb triage --json` structured output (empty → `[]`)
- **TB-70** (M) — `BoardService.Triage()` wraps `tb triage --json`, caches, invalidates on `board:reloaded` / `task:updated:<id>`
- **TB-71** (S) — Frontend `api.ts` + `triageStore.ts`: typed `groomTask`, `getTriage`; watcher-driven refresh; `mode` field on Run types
- **TB-72** (M) — TaskDrawer: **Groom** button next to **Run**; shared Cancel; mode-labelled past runs; visual emphasis when triage flags the task
- **TB-73** (M) — Card: "needs grooming" indicator + tooltip with reasons; click opens drawer and emphasises Groom button
- **TB-74** (S) — Flip M6 markers in `docs/IMPLEMENTATION.md`, `docs/FEATURES.md` (F6.1, F6.2), `docs/ARCHITECTURE.md`
- **TB-75** (M) — CLI: tb edit --goal / --acceptance — body-section writes via writeFileAtomic

## Subtask → Feature ownership matrix

| F6 / invariant                                             | Owner(s)                              |
|------------------------------------------------------------|---------------------------------------|
| F6.1 prompt template                                       | TB-65                                 |
| F6.1 mode-aware Runner composition                         | TB-66                                 |
| F6.1 GUI-initiated groom run (button → JSONL → AgentStatus)| TB-67                                 |
| F6.1 daemon-initiated groom run (CLI queues, daemon picks) | TB-68                                 |
| F6.1 streaming + Cancel UX                                 | reused from TB-50 / TB-51 / TB-48 (no new code) |
| F6.2 triage data plane (CLI side)                          | TB-69                                 |
| F6.2 triage data plane (GUI service + cache)               | TB-70                                 |
| F6.2 triage data plane (frontend store)                    | TB-71                                 |
| F6.2 indicator + click-to-suggest                          | TB-73                                 |
| F6.2 drawer visual emphasis when triage flagged            | TB-72 (last AC)                       |
| F6.1 body-edit CLI surface the prompt depends on           | TB-75                                 |
| Doc flips + completed-work-log entry                       | TB-74                                 |
| **Out of scope**                                           | new cancel surface for groom; mode-aware runner factory signature; live re-attach (deferred from M5); generic body-section editing (TB-75 is intentionally narrow to Goal + AC) |

## Acceptance Criteria

- [ ] **F6.1** Drawer **Groom** button next to **Run** (TB-72) → `AgentService.GroomTask` (TB-67) appends JSONL `queued{mode: groom}`, emits Wails `agent:run-queued{mode: groom}`, sets `AgentStatus=queued` → goroutine runs `agent.NewGroomingDecorator(factory(agent), PromptVars{…})` (TB-66) which renders the embedded `PromptGroom` (TB-65) and overwrites `RunInput.Prompt`; `started`/`finished` JSONL records carry `mode=groom`; `AgentStatus` flips to `running` then `success|failed`. End-to-end smoke: backlog task with placeholder Goal → click Groom → agent calls `tb edit <ID> --goal -` / `--acceptance -` (TB-75) → after the agent finishes, the task's `## Goal` and `## Acceptance Criteria` are non-placeholder on disk; GUI reflects the change via the existing watcher fan-out.
- [ ] **F6.1 (daemon path)** `tb edit X --agent-status queued` on a task whose JSONL trail has a `queued{mode: groom}` event causes `RunQueuedAgentSync` (TB-68) to apply the decorator without GUI intervention. CLI-direct queues (no JSONL event) still default to `mode=implement`.
- [ ] **F6.1 (cancel)** Cancelling a groom run via the existing `CancelRun` flows through the same 5-step ordering (TB-48); JSONL `finished{cancelled, mode: groom}`; stale-recovery (TB-60/TB-61) never overwrites `cancelled`.
- [ ] **F6.2** `tb triage --json` (TB-69) returns the same set the tabwriter view emits — keyed by ID, reasons preserved. `BoardService.Triage()` (TB-70) caches the result, invalidates on `board:reloaded` and per-ID on `task:updated:<id>`. `triageStore` (TB-71) mirrors both invalidation paths on the frontend so inline-metadata edits don't leave stale badges. `Card.svelte` (TB-73) wears a "needs grooming" indicator when the store flags it; tooltip lists reasons; clicking opens the drawer with the Groom button visually emphasised (TB-72).
- [ ] **F6.2 (scope)** Indicator only renders on backlog cards — matches `tb triage`'s own scope (cli/triage.go iterates `backlog` only).
- [ ] **Crash-safety regression** Existing M4 (TB-43..TB-52) and M5 (TB-53..TB-62) tests pass unchanged — `-race` clean. New tests cover groom-specific paths (TB-66, TB-67, TB-68, TB-69, TB-70, TB-71, TB-72, TB-73).
- [ ] All M6 sub-tasks (TB-65..TB-75) closed.
- [ ] `docs/IMPLEMENTATION.md` M6 markers flipped to ☑ (via TB-74); `docs/FEATURES.md` F6.1 + F6.2 marked done with implementation pointers; `docs/ARCHITECTURE.md` GroomingDecorator + `BoardService.Triage` sections current.

## Related Tasks

- **TB-4** — Prerequisite (Runner interface, JSONL event schema with `mode` field, `RunAgent` / `CancelRun` lifecycle this builds on)
- **TB-5** — Prerequisite (daemon `RunQueuedAgentSync` + mode-aware pickup hook in TB-68)
- **TB-2** — Prerequisite (kanban + drawer + watcher event names)
- **TB-3** — Prerequisite (mutations + drawer + Card scaffold)
- **TB-7** — Builds on this (M7 settings UI may expose a default mode toggle)

## Log

- 2026-05-13: Created
- 2026-05-14: Groomed — aligned acceptance criteria 1:1 with `docs/FEATURES.md` F6.1–F6.2 and `docs/IMPLEMENTATION.md` M6 task list; decomposed into TB-65..TB-74 (groom prompt + embed; GroomingDecorator over existing Runners; `AgentService.GroomTask` mirroring `RunAgent`; daemon honours JSONL `mode=groom` on pickup; `tb triage --json`; `BoardService.Triage()` with watcher-driven invalidation; frontend `api.ts` + `triageStore`; TaskDrawer Groom button with shared Cancel + mode-labelled past runs; Card "needs grooming" indicator wired to drawer focus signal; M6 doc flips). Out-of-scope items called out in the ownership matrix: no new cancel surface, no mode-aware runner factory signature, no live re-attach (deferred from M5).
- 2026-05-14: Codex review fixes — (1) **P1**: groom prompt had no allowed body-write surface because `cli/edit.go` only mutates metadata; added TB-75 (narrow `tb edit --goal -` / `--acceptance -` flags through `lockBoard` + `writeFileAtomic` + `regenerateBoard`); TB-65 prompt now wires the agent to that surface with an explicit heredoc example. (2) **P2**: `GroomingDecorator` had no source for `PromptVars`; locked TB-66's constructor to `NewGroomingDecorator(inner Runner, vars PromptVars)` — vars captured at construction, `RunInput` schema stays byte-stable; TB-67 and TB-68 updated to pass vars at the wrap call site. (3) **P2**: `triageStore` only refreshed on `board:reloaded`, leaving stale badges after inline metadata edits; TB-71 now also subscribes to `task:updated:<id>` and per-ID refreshes (add / replace / remove), mirroring TB-70's backend cache invalidation.
- 2026-05-14: Started — moved to in-progress
- 2026-05-14: Done
