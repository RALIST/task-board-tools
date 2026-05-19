package agent

import "context"

// ClaudeRunner shells out to `claude -p <prompt> --output-format stream-json
// --verbose` in non-interactive mode. The prompt is the full rendered text
// (see RenderPrompt) — Claude reads it once, runs to completion, exits.
//
// stream-json is the structured event stream Claude emits when --verbose is
// also set; we wrap RunInput.Stdout with a translator (claude_stream.go) that
// converts each event into a Codex-style human-readable line. The JSONL
// state stream still captures the translated lines verbatim, and the
// per-run .log file is what the user actually reads. Codex's plain output
// is already human-readable, so it stays untouched.
type ClaudeRunner struct{}

// NewClaudeRunner returns a ready-to-use ClaudeRunner. The runner is
// stateless; one instance can be shared across the process.
func NewClaudeRunner() *ClaudeRunner { return &ClaudeRunner{} }

// Name returns "claude" — the value AssignAgent stores in the task's
// `**Agent:**` metadata field.
func (r *ClaudeRunner) Name() string { return "claude" }

// Run invokes `claude -p <prompt> --output-format stream-json --verbose`.
// See runExternal for the full lifecycle (process group, env whitelist,
// output streaming, cancel/timeout handling).
//
// When RunInput.SessionID is non-empty (Claude pre-allocation per
// TB-130), `--session-id <uuid>` is appended so the daemon's
// pre-allocated id becomes Claude's actual conversation id. A
// subsequent ResumeAgent run (TB-138) replaces this with `-r <uuid>`
// against the same id.
func (r *ClaudeRunner) Run(ctx context.Context, in RunInput) (RunResult, error) {
	if in.Stdout != nil {
		in.Stdout = newClaudeTranslator(in.Stdout)
	}
	args := []string{"-p", in.Prompt, "--output-format", "stream-json", "--verbose"}
	if in.SessionID != "" {
		args = append(args, "--session-id", in.SessionID)
	}
	res, _ := runExternal(ctx, in, "claude", args)
	return res, res.Err
}
