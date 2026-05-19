package agent

import "context"

// CodexRunner shells out to `codex exec --json <prompt>`.
//
// CLI shape (verified against codex-cli 0.130.0 — `codex exec --help`):
//
//	codex exec [OPTIONS] [PROMPT]
//	  ↑ PROMPT is positional; if absent (or "-") the prompt is read from
//	  stdin. There is no `--prompt` flag despite what TB-44's original
//	  draft assumed; we pass the prompt positionally, which is the
//	  documented contract.
//
// TB-130 switched from `codex exec` to `codex exec --json`: codex does
// NOT accept a pre-allocated session id, so we have to parse one out of
// its `--json` event stream. The translator (codex_stream.go) renders
// the events back into human-readable lines for the per-run log and
// fires the optional RunInput.OnSessionID callback the first time it
// observes a UUIDv4-shaped session id. Exit-code mapping in
// mapRunnerOutcome is unchanged by the switch — only the on-disk log
// content and session capture are new.
type CodexRunner struct{}

// NewCodexRunner returns a ready-to-use CodexRunner. Stateless.
func NewCodexRunner() *CodexRunner { return &CodexRunner{} }

// Name returns "codex" — the value AssignAgent stores in the task's
// `**Agent:**` metadata field.
func (r *CodexRunner) Name() string { return "codex" }

// Run invokes `codex exec --json <prompt>` with the rendered prompt as
// the single positional argument. See runExternal for the lifecycle
// contract.
func (r *CodexRunner) Run(ctx context.Context, in RunInput) (RunResult, error) {
	if in.Stdout != nil {
		in.Stdout = newCodexJsonTranslator(in.Stdout, in.OnSessionID)
	}
	args := []string{"exec", "--json", in.Prompt}
	res, _ := runExternal(ctx, in, "codex", args)
	return res, res.Err
}
