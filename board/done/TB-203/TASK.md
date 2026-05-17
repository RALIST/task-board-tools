# TB-203: obfuscation agents logs and tasks

**Type:** bug
**Priority:** P1
**Size:** M
**Agent:** codex
**AgentStatus:** success
**Module:** gui
**Tags:** security,agent,logging
**Branch:** —

## Goal

Prevent agent-run output and board task text from leaking secrets into the git-tracked board while preserving useful redacted diagnostics.

## Context

Agent runtime artifacts are stored in two layouts documented in `docs/ARCHITECTURE.md`: file-form tasks use `board/.agent-state/<ID>.jsonl` and `board/.agent-logs/<ID>/<run_id>.log`; folder-form tasks use `<status>/<ID>/.agent-state.jsonl` and `<status>/<ID>/.agent-logs/<run_id>.log`.

Current persistence fan-out lives in `gui/app/agent_run.go`: `lineSink.Write` sends each stdout/stderr line to JSONL, the per-run log file, and Wails `agent:run-log` events. `gui/internal/agent/state.go` owns `AppendEvent`, `NewLogWriter`, `StatePath`, and `LogPath`.

Current repo hygiene is incomplete: `.gitignore` ignores `/board/.agent-state` and has a singular `/board/.agent-log` entry, but it misses `/board/.agent-logs/` and all folder-form task-local `.agent-state.jsonl` / `.agent-logs/` paths. The worktree already contains agent artifacts under those paths, and some legacy `board/.agent-logs/**` paths have been tracked or staged before; treat every such file as potentially sensitive.

Constraints / non-goals:

- Never print, paste, or commit raw secret values discovered during the fix. Use fake secrets in tests and notes.
- Do not rewrite git history or rotate real credentials in this task. If a real credential is confirmed, create an incident/follow-up task and keep the value out of the board.
- Do not delete the user's local runtime history just to satisfy git hygiene; remove tracked/indexed runtime artifacts from git tracking and ignore future ones.
- Preserve board invariants: task markdown remains the source of truth, structured task writes stay atomic and regenerate `BOARD.md`, and GUI-owned `.agent-state` / `.agent-logs` paths keep their file-form and folder-form layouts.
- TB-132/TB-141 already cover `RunEnv` allowlisting for resume work; keep that contract intact instead of duplicating schema work here.

## Acceptance Criteria

- [x] `.gitignore` ignores every production agent artifact layout: `board/.agent-state/`, `board/.agent-logs/`, `board/**/.agent-state.jsonl`, and `board/**/.agent-logs/`. `git check-ignore` covers at least one file-form sample path and one folder-form sample path.
- [x] No production agent run artifacts remain tracked by git: `git ls-files board | rg '(^|/)\.agent-(state|logs?|state\.jsonl)'` returns no runtime paths. If a fixture must remain tracked, it lives outside the production board runtime paths and contains fake data only.
- [x] A shared redaction helper covers common secret forms: password/token/api-key assignment text, bearer-token style headers, and known credential env names such as `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, and `GITHUB_TOKEN`; tests assert only the sensitive value is replaced with `[REDACTED]`.
- [x] Agent stdout/stderr are redacted before reaching every persisted or replayed sink: `.agent-state*.jsonl`, `.agent-logs`, Wails `agent:run-log` payloads, and `GetRunLog` readback. Backend tests assert a raw fake secret never appears in those sinks.
- [x] Managed task-text writes used by agents are redacted before task markdown is written, at minimum `tb create -d`, `tb edit --goal`, `tb edit --acceptance`, and log entries that include user-supplied values. CLI tests assert a fake token in stdin is redacted in the task file and regenerated board output.
- [x] Existing runner env allowlisting still excludes ambient credentials, and any `RunEnv` work from TB-132/TB-141 remains compatible with the redactor.
- [x] Verification includes `cd cli && go test ./...`, `cd gui && go test ./...`, and the `git check-ignore` / `git ls-files` checks above.
- [ ] Manual smoke note: run or groom a test task using fake secret-looking text in agent output and task edit input; confirm the drawer live log, past-run log, task body/card, JSONL, and log file show `[REDACTED]` and never the raw fake value. *(deferred to manual run; CLI + Go backend tests cover every sink including the GetRunLog readback)*

## Related Tasks

- **TB-102** — defines the file-form versus folder-form agent artifact paths that must be ignored and preserved.
- **TB-132** — complementary `RunEnv` schema work; its env allowlist prevents API tokens from entering resume JSONL.
- **TB-141** — complementary fake-runner integration coverage for the `RunEnv` allowlist.

## Attachments

## Log

- 2026-05-15: Created
- 2026-05-15: Edited body via GUI
- 2026-05-15: Edited agent=codex
- 2026-05-15: Edited agentstatus=queued
- 2026-05-15: Edited agentstatus=running
- 2026-05-15: Edited priority=P1, type=bug, size=M, module=gui, tags=security,agent,logging, goal
- 2026-05-15: Edited acceptance
- 2026-05-15: Edited agentstatus=success
- 2026-05-17: Started — moved to in-progress
- 2026-05-17: Done — added `redact` helper in `gui/internal/redact/` (and mirror at `cli/redact.go`) covering generic `apiKey/secret/password/passwd/token/bearer` key=value assignments, Bearer header tokens, and a hard-coded `KnownEnvNames` allow-list (OPENAI/ANTHROPIC/GITHUB/GH/AWS/SLACK/GOOGLE) — only the value is replaced with `[REDACTED]`. Wired into `lineSink.Write` in `gui/app/agent_run.go` so the same redacted text reaches the log file, JSONL state, Wails `agent:run-log` event, and `GetRunLog` readback; new `TestRunAgent_RedactsSecretsAcrossAllSinks` proves it. CLI paths: `cmdCreate -d`, `cmdEdit --goal/--acceptance`, and a wrapper inside `appendLogEntry` so user-supplied label fragments get sanitized; new `TestCreate*/TestEdit*RedactsSecrets` + `TestAppendLogEntryRedactsUserSuppliedValues` cover task body and BOARD.md. `.gitignore` tightened: replaced ambiguous singulars with explicit `/board/.agent-state/`, `/board/.agent-logs/`, `/board/**/.agent-state.jsonl`, `/board/**/.agent-logs/`; `git check-ignore` verified for both file-form and folder-form layouts; `git ls-files board` shows no tracked runtime artifacts. All `cli/` and `gui/` go tests pass. Manual GUI smoke deferred to user.

