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

// Run invokes `codex exec --json <prompt>` (fresh) or `codex exec
// --json resume <uuid> <prompt>` (resume). See runExternal for the
// lifecycle contract.
//
// Resume args per TB-130: when Mode == ModeResume and SessionID is
// non-empty, the `resume <uuid>` positional pair is inserted between
// `--json` and the prompt. Codex then continues the named session and
// emits a NEW session_id on its own — the translator's OnSessionID
// callback captures that as the resumable id for any future resume
// (the chain is traceable via the queued event's `resumed_from`).
//
// An empty SessionID in ModeResume is a wiring bug — the caller MUST
// supply one from resumableSessionID. We don't second-guess it here;
// the runner would just invoke `codex exec --json resume "" <prompt>`
// and codex would fail with a clear error.
func (r *CodexRunner) Run(ctx context.Context, in RunInput) (RunResult, error) {
	if in.Stdout != nil {
		in.Stdout = newCodexJsonTranslator(in.Stdout, in.OnSessionID)
	}
	var args []string
	if in.Mode == ModeResume && in.SessionID != "" {
		args = []string{"exec", "--json", "resume", in.SessionID, in.Prompt}
	} else {
		args = []string{"exec", "--json", in.Prompt}
	}
	res, _ := runExternal(ctx, in, "codex", args)
	return res, res.Err
}
