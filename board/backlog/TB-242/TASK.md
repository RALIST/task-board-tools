# TB-242: Agent runner blocks on stdout EOF when child processes inherit pipes

**Type:** bug
**Priority:** P1
**Size:** M
**Module:** gui
**Tags:** agent,runner,daemon,reliability
**Branch:** —

## Goal

Ensure the agent runner records a `finished` event and clears `AgentStatus` even when the agent's child processes keep its stdout/stderr pipes open after the parent's work is done.

## Acceptance Criteria

- [ ] Reproduce: a claude grooming run that spawns an MCP server child (e.g. pencil) finishes its work but the wrapper process does not exit; the runner currently blocks at `streamWG.Wait()` / `cmd.Wait()` in `gui/internal/agent/exec.go:146-147` and never writes the `finished` JSONL event. Capture this as a unit/integration test that hangs without the fix.
- [ ] Fix the runner so it does not depend on stdout/stderr EOF from grandchildren to terminate the run. Acceptable approaches: (a) detect parent exit via `cmd.Process.Wait()` in a separate goroutine and close our copies of the pipes when the parent process is gone; (b) start the agent in its own pgid and tear the pgid down on a configurable post-exit grace window; (c) attach pdeathsig / equivalent so children die with the parent.
- [ ] The chosen fix preserves Setpgid semantics on cancel/timeout and does not regress the existing exit-code, timeout, and cancel behavior in `gui/internal/agent/exec_test.go`.
- [ ] After the fix, hung-but-finished runs produce a `finished` JSONL event within a bounded grace window (e.g. ≤ 5s after parent exit) and `tb edit --agent-status …` updates the task markdown.
- [ ] Document the new behaviour in `docs/ARCHITECTURE.md` under the agent runner section, including the grace window and child-process expectation.
- [ ] `cd gui && go test ./...` passes.

## Attachments

## Log

- 2026-05-19: Created
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance

