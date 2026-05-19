# TB-259: Per-task chat panel via claude/codex CLIs

**Type:** feature
**Priority:** P1
**Size:** L
**Module:** gui
**Tags:** agent,chat
**Branch:** —

## Goal

Add an interactive chat panel in tb-gui that lets the user converse with `claude` or `codex` about a specific task, reusing the existing CLI binaries and TB-130 session-resume machinery — not the Agent SDK and not the raw Anthropic/OpenAI APIs. The agent has the full toolbelt so it can manage the board if needed, and is instructed (via `--append-system-prompt`) to use `tb` for board mutations rather than directly editing files under `board/`.

## Context

Today tb-gui shells out to `claude` and `codex` in non-interactive ("autonomous") mode — one prompt in, one task done, with TB-130 capturing the agent CLI's `session_id` to enable resume after crashes. There is no way to have a back-and-forth conversation with the agent about a task: clarify acceptance criteria, brainstorm an approach, ask "what does TB-205 do?", or drive a board mutation interactively from a chat bubble.

**Why drive the existing CLIs instead of the Agent SDK or raw API?** The app already ships, installs, and trusts the user's `claude` / `codex` binaries; their auth, hooks, MCP servers, and skills are already configured. Spawning the same binaries for chat avoids a second auth surface, a second permissions model, and a parallel agent loop to keep in sync with the autonomous one.

**Why spawn-per-turn-with-resume, not a long-lived stdio session?** `claude` has an `--input-format stream-json` mode that opens a bidirectional JSON channel, but it is undocumented and has open bugs (claude-code#24594, #5034). The supported path is one process per user turn:

- claude: `claude -p "<msg>" --resume <sid> --output-format stream-json --verbose`
- codex: `codex exec --json resume <sid> "<msg>"` (codex has no documented stdin protocol)

Sessions live in the agent CLI's own store, so resume is durable across process crashes, panel close/reopen, and full app restarts. Mid-turn cancel is safe — kill the child PID, and the next turn picks up where the agent's session store left off. This is the same pattern TB-130 already implements for autonomous runs.

**Tooling and safety.** The chat agent gets the full toolbelt (`Read`, `Edit`, `Write`, `Bash`, etc.) so it can read source, edit files, and shell out. To preserve the project's locking invariants, the system prompt explicitly steers board mutations through `tb` (which takes `.board.lock` and calls `regenerateBoard`) instead of direct Edit/Write on files under `board/`. CLAUDE.md, board/SKILL.md, and board/CONVENTIONS.md auto-load when cwd is set correctly, so most of this guidance is already in the agent's context — the `--append-system-prompt` is just the chat-specific reinforcement.

**Boundary with autonomous runs.** A chat is not an "agent run". It does not write to `.agent-state.jsonl` / `.agent-logs/`, does not show up in the runs sidebar, and does not interact with the `AgentStatus` lifecycle (`queued | running | success | failed | …`). If a chat session is persisted at all (for resume across restarts), it lives in its own slot, not the autonomous-run store. Board mutations the agent performs via `tb` from inside chat will surface in the kanban via the existing fsnotify watcher — that's the only crossover.

## Acceptance Criteria

- [ ] New Wails service method on the agent runner that accepts `(taskID, sessionID, message)` and streams chat events back over Wails events.
- [ ] Reuses `gui/internal/agent/` machinery for subprocess management, JSONL parsing, and `session_id` capture — no parallel fork of the runner.
- [ ] First turn captures `session_id`; subsequent turns resume the same conversation with `claude -p "<msg>" --resume <sid> --output-format stream-json --verbose --permission-mode bypassPermissions` (or `codex exec --json resume <sid> "<msg>"`).
- [ ] `session_id` is persisted durably per task (e.g. `<status>/<ID>/.chat-session` for folder form, or a TASK.md frontmatter field for file form) so chat resumes across app restarts.
- [ ] cwd is the task directory (folder form) or board root (file form), so `CLAUDE.md` / `board/SKILL.md` / `board/CONVENTIONS.md` auto-load into the agent's context.
- [ ] `--append-system-prompt` carries chat-specific guidance: "use `tb` for any board mutation; do not directly Edit/Write files under `board/`; free-form notes via `tb edit --goal -` / `--acceptance -`".
- [ ] Svelte chat panel: append-only message list, streams token deltas into the in-flight assistant bubble, cancel button kills the subprocess cleanly. Designed via the `frontend-design` skill.
- [ ] Mid-turn cancel is safe — next message resumes from the partial state via the same `session_id`.
- [ ] Subprocess crash recovery — next turn re-resumes from the persisted `session_id`; no orphaned state.
- [ ] Works with both `claude` and `codex`; defaults to the task's existing `Agent:` field, overridable per-chat.
- [ ] Board mutations from inside chat trigger the existing fsnotify watcher (no extra wiring); confirm `BOARD.md` / kanban update in real time.
- [ ] Chat sessions do NOT appear in the autonomous-run history (`.agent-logs/`, `.agent-state.jsonl`) — interactive sessions are a separate concern from one-shot runs. Stored in their own location (e.g. `<status>/<ID>/.chat-log/<turn>.json` or a dedicated chat store) if persisted at all.
- [ ] `--permission-mode bypassPermissions` matches autonomous-run parity; alternatively, surface prompts in the UI via `--permission-prompt-tool` (decide before implementation — bypass is the simpler default).
- [ ] No credential env vars (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.) reach disk — match the TB-130 `TB_`-prefix-only persistence rule.
- [ ] Tests cover: session_id capture, resume across simulated restart, mid-turn cancel, board-mutation propagation via fsnotify, and credential-scrubbing in any persisted state.

## Attachments

## Related Tasks

- **TB-130** — Agent session resume + interrupted-run recovery (foundation: this task extends the same `session_id` capture + `--resume` machinery to interactive chat; shares the credential-scrubbing rule for any persisted state).
- **TB-231** — TB-130 adversarial review findings (reference for security/recovery edge cases to honour in the chat path).
- **TB-237** — Per-mode agent attribution (groom/implement/review) (the "chat" mode is a new attribution bucket; coordinate metadata if chat-driven board edits should be attributed).

## Log

- 2026-05-19: Created
- 2026-05-19: Edited goal
- 2026-05-19: Edited acceptance

