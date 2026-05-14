# TB-138: Resume backend Claude: ResumeDecorator + -r flag + cwd/env replay

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** epic-tb130,agent,resume,claude
**Branch:** —
**Parent:** TB-130

## Goal

Add the `ResumeAgent(ctx, taskID)` service method, the
`agent.ResumeDecorator` runner wrapper (mirror of
`groomingDecorator` at `gui/internal/agent/runner.go:180-200`), the
`agent.PromptResume` template, and the Claude-specific resume args
(`-r <uuid>`). Resume replays the original execution context — `cwd`
and `TB_`-prefixed env vars persisted in the JSONL `session` event.

## Context

Spec: `docs/superpowers/specs/2026-05-14-agent-session-resume-design.md`
§ 8 (Resume API), § 9 (continuation prompt), § 12 task H.

Codex round-2 confirmed the decorator pattern: `groomingDecorator`
exists today and overrides `RunInput.Prompt` from inside its `Run`
method. `ResumeDecorator` mirrors the structure, so `runGoroutine`'s
prompt-render call stays unchanged.

Resume must run with the SAME `cwd` and `TB_BOARD_PATH` the original
run used. TB-114 (worktree isolation) is independent — if it lands
first, the persisted cwd is the worktree path; if not, it's the
repo root. Either way, replay works.

## Acceptance Criteria

- [ ] New file `gui/internal/agent/prompts/resume.md` with the fixed
      continuation prompt (one to two sentences per spec § 9):
      > The previous run was interrupted before completion. Inspect
      > the current worktree and board state, then continue from
      > where you left off. Do not restart the task.
- [ ] `agent.PromptResume` constant in
      `gui/internal/agent/prompts.go` (or wherever `PromptImplement`
      / `PromptGroom` live).
- [ ] New `agent.ResumeDecorator` in
      `gui/internal/agent/runner.go` mirroring `groomingDecorator`:
      overrides `in.Prompt` with `RenderPrompt(PromptResume, vars)`,
      forwards everything else.
- [ ] `gui/app/agent_run.go:runnerForMode` switch gains:
      ```go
      case agent.ModeResume:
          return agent.NewResumeDecorator(runner, promptVarsFromDetail(detail))
      ```
- [ ] `RunInput` gains `Cwd string` and `Env map[string]string`
      (consumed by `runExternal`'s `cmd.Dir` and env-append logic
      at `gui/internal/agent/exec.go`).
- [ ] New service method:
      ```go
      func (s *AgentService) ResumeAgent(ctx context.Context, taskID string) (string, error)
      ```
  - Calls `resumableSessionID` (TB-132). Returns `ErrNotResumable`
    if `ok=false`.
  - Validates `AgentStatus == "interrupted"`. Returns
    `ErrCannotResume` otherwise.
  - Sets up `RunInput.SessionID = candidate.SessionID` (so
    `--session-id` is NOT passed; resume reads back).
  - Sets up `RunInput.Cwd = candidate.Cwd`, `RunInput.Env =
    candidate.Env`.
  - Calls the same path `startAgentRun` uses (factored shared body).
  - Appends `queued{resumed_from: <sessionID>, resumed_from_run:
    <runID>, mode: "resume"}` to JSONL.
- [ ] `ClaudeRunner.Run` appends `-r <in.SessionID>` (NOT
      `--session-id`) when `in.SessionID != "" && in.Mode ==
      ModeResume`. (For non-resume runs the existing `--session-id`
      from TB-135 stays.)
- [ ] `RunAgent` is REJECTED when `AgentStatus == "interrupted"` (a
      fresh run is allowed, but the user must explicitly choose Run
      vs Resume — frontend disambiguation in TB-140).
- [ ] Tests use a fake Claude runner that asserts:
  - argv contains `-r <expected-uuid>` and NOT `--session-id`.
  - `cmd.Dir` matches the persisted cwd.
  - Env contains `TB_BOARD_PATH=<expected>`.
  - Prompt body matches `prompts/resume.md` rendered.
  - `queued` JSONL has `resumed_from` and `resumed_from_run`.
- [ ] Test: `ResumeAgent` on `failed` task → `ErrCannotResume`.
- [ ] Test: `ResumeAgent` with no captured session id →
      `ErrNotResumable`.

## Related Tasks

- **TB-130** — parent epic.
- Depends on **TB-131** (`ModeResume`), **TB-132** (`ResumeCandidate`),
  **TB-135** (Claude session capture), **TB-137** (`interrupted`
  status to resume FROM).
- Blocks **TB-140** (frontend wires `ResumeAgent`), **TB-141**
  (integration test exercises this path).

## Log

- 2026-05-14: Created
