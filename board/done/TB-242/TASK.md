# TB-242: Agent runner blocks on stdout EOF when child processes inherit pipes

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** agent,runner,daemon,reliability
**Agent:** codex
**AgentStatus:** success
**ImplementedBy:** codex
**ImplementStatus:** success
**Branch:** —

## Goal

Ensure the agent runner records a `finished` event and clears `AgentStatus` even when the agent's child processes keep its stdout/stderr pipes open after the parent's work is done.

## Acceptance Criteria

- [x] Reproduce: a claude grooming run that spawns an MCP server child (e.g. pencil) finishes its work but the wrapper process does not exit; the runner currently blocks at `streamWG.Wait()` / `cmd.Wait()` in `gui/internal/agent/exec.go:146-147` and never writes the `finished` JSONL event. Captured by `TestRunExternal_ReturnsWhenChildInheritsOutputPipes`.
- [x] Fix the runner so it does not depend on stdout/stderr EOF from grandchildren to terminate the run. Direct parent exit is authoritative; inherited pipes get a bounded grace window before reader close.
- [x] The chosen fix preserves Setpgid semantics on cancel/timeout and does not regress the existing exit-code, timeout, and cancel behavior in `gui/internal/agent/exec_test.go`.
- [x] After the fix, hung-but-finished runs produce a `finished` JSONL event within a bounded grace window after parent exit and `tb edit --agent-status …` updates the task markdown through the existing terminal path.
- [x] Document the new behaviour in `docs/ARCHITECTURE.md` under the agent runner section, including the grace window and child-process expectation.
- [x] `cd gui && go test ./...` passes.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance
- 2026-05-19: Committed — moved to ready
- 2026-05-19: Edited agent=codex
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Started — moved to in-progress
- 2026-05-19: Edited agentstatus=failed
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Implementation complete — runner now treats direct parent exit as terminal, gives inherited stdout/stderr pipes a 1s drain grace, then closes readers so terminal JSONL/task status writes can proceed; architecture docs and regression coverage added.
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited agentstatus=queued
- 2026-05-19: Edited agentstatus=running
- 2026-05-19: Edited agentstatus=interrupted
- 2026-05-19: Edited agentstatus=success, implemented-by=codex, implement-status=success
- 2026-05-19: Done

