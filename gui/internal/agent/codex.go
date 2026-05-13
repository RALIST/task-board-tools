package agent

import "context"

// CodexRunner shells out to `codex exec <prompt>`.
//
// CLI shape (verified against codex-cli 0.130.0 — `codex exec --help`):
//
//	codex exec [OPTIONS] [PROMPT]
//	  ↑ PROMPT is positional; if absent (or "-") the prompt is read from
//	  stdin. There is no `--prompt` flag despite what TB-44's original
//	  draft assumed; we pass the prompt positionally, which is the
//	  documented contract.
//
// If a future codex release drops the positional argument and we get a
// usage error from cmd.Wait, the fallback path is `codex exec -` with the
// prompt written to stdin (see runExternalWithStdin) — but the positional
// form is what's tested and shipped.
type CodexRunner struct{}

// NewCodexRunner returns a ready-to-use CodexRunner. Stateless.
func NewCodexRunner() *CodexRunner { return &CodexRunner{} }

// Name returns "codex" — the value AssignAgent stores in the task's
// `**Agent:**` metadata field.
func (r *CodexRunner) Name() string { return "codex" }

// Run invokes `codex exec <prompt>` with the rendered prompt as the single
// positional argument. See runExternal for the lifecycle contract.
func (r *CodexRunner) Run(ctx context.Context, in RunInput) (RunResult, error) {
	res, _ := runExternal(ctx, in, "codex", []string{"exec", in.Prompt})
	return res, res.Err
}
