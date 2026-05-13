package agent

import "context"

// ClaudeRunner shells out to `claude -p <prompt>` (positional prompt arg
// after the -p flag) in non-interactive mode. The prompt is the full
// rendered text (see RenderPrompt) — Claude reads it once, runs to
// completion, exits.
type ClaudeRunner struct{}

// NewClaudeRunner returns a ready-to-use ClaudeRunner. The runner is
// stateless; one instance can be shared across the process.
func NewClaudeRunner() *ClaudeRunner { return &ClaudeRunner{} }

// Name returns "claude" — the value AssignAgent stores in the task's
// `**Agent:**` metadata field.
func (r *ClaudeRunner) Name() string { return "claude" }

// Run invokes `claude -p <prompt>`. See runExternal for the full lifecycle
// (process group, env whitelist, output streaming, cancel/timeout handling).
func (r *ClaudeRunner) Run(ctx context.Context, in RunInput) (RunResult, error) {
	res, _ := runExternal(ctx, in, "claude", []string{"-p", in.Prompt})
	return res, res.Err
}
