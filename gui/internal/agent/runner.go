// Package agent runs external CLI agents (claude, codex) on behalf of the
// GUI. It owns the Runner interface, the embedded prompt templates, and the
// JSONL+log-file state writer that AgentService consumes.
//
// Layering:
//
//	gui/app/agent_service.go        ← coordinator; one struct per service
//	       │
//	       ▼
//	gui/internal/agent              ← this package
//	 ├─ runner.go     (interface + Mode + RunInput/RunResult + RenderPrompt)
//	 ├─ claude.go     (ClaudeRunner: shells out to `claude -p …`)
//	 ├─ codex.go      (CodexRunner:  shells out to `codex exec --prompt …`)
//	 ├─ state.go      (AppendEvent, NewLogWriter, GenerateRunID)
//	 └─ prompts/*.md  (embedded prompt templates)
package agent

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"strings"
	"time"
)

// Mode tells a Runner what variant of prompt to use. ModeImplement landed
// in M4, ModeGroom in M6, ModeResume in TB-130 (continues a prior agent
// session via the agent CLI's native resume flag).
type Mode string

const (
	ModeImplement Mode = "implement"
	ModeGroom     Mode = "groom"
	ModeResume    Mode = "resume"
)

// String makes Mode round-trip cleanly through JSON, logs, and prompt
// substitution.
func (m Mode) String() string { return string(m) }

// Runner is the abstraction every agent CLI plugs into. Implementations live
// in the same package (ClaudeRunner, CodexRunner). The interface is small on
// purpose — cancellation, logging, and lifecycle accounting all live in the
// caller (AgentService), not here.
type Runner interface {
	// Name is the agent identifier the rest of the system uses ("claude",
	// "codex"). It MUST match the value the GUI writes into the task's
	// `Agent:` metadata field so a run can always be replayed.
	Name() string

	// Run blocks until the agent process exits, ctx is cancelled, or
	// RunInput.Timeout elapses. The Runner never returns the output streams
	// directly; it forwards every stdout/stderr line to the writers in
	// RunInput, so the caller can tee them to multiple sinks (JSONL, log
	// file, Wails events) without buffering inside the Runner.
	Run(ctx context.Context, in RunInput) (RunResult, error)
}

// RunInput packages every piece of data a Runner needs. The caller is
// responsible for filling everything in; missing fields produce errors at
// run time rather than silent defaults so a wiring bug fails loudly.
type RunInput struct {
	// TaskID is the task this run targets (e.g. "TB-4"). Only used for log
	// breadcrumbs inside the Runner; the JSONL event writer in state.go
	// records it on every event.
	TaskID string

	// Mode selects the prompt template variant.
	Mode Mode

	// Prompt is the fully-rendered prompt text passed to the agent CLI.
	// Caller renders this via RenderPrompt before invoking Run.
	Prompt string

	// ProjectRoot is the cwd the agent process inherits. Required.
	ProjectRoot string

	// Env is an allowlist of additional environment variables to forward to
	// the agent. The Runner adds these on top of a tiny built-in whitelist
	// (HOME/PATH/USER/LANG/LC_*/TERM); see claude.go and codex.go.
	Env []string

	// Timeout is the deadline for the whole run. Distinct from ctx
	// cancellation — a deadline means "the run is unattended and took too
	// long" and triggers the SIGTERM → SIGKILL escalation inside the
	// Runner. A user-initiated cancel is delivered via ctx and stays a
	// single SIGTERM (the AgentService escalates by reaching into the
	// process group directly; see TB-48).
	//
	// Zero = no timeout, but in practice the caller always sets one (30m
	// default per ARCHITECTURE.md).
	Timeout time.Duration

	// Stdout / Stderr receive line-by-line output from the agent. Each
	// scanned line is written followed by a single '\n'. Writers must be
	// safe for concurrent use across stdout and stderr — the Runner does
	// not serialise the two streams.
	Stdout io.Writer
	Stderr io.Writer

	// OnStarted fires synchronously inside Run after cmd.Start() succeeds
	// and before any output is forwarded. AgentService uses it to populate
	// the in-flight `activeRun` entry (PID/pgid for cancel cascade) BEFORE
	// the Runner blocks on output streaming. If OnStarted is nil, the
	// Runner skips the call.
	//
	// On POSIX, pgid equals pid because the Runner sets Setpgid=true; the
	// callback receives both so the caller doesn't have to know that.
	OnStarted func(pid, pgid int)

	// OnSessionID fires the first time the agent CLI reports a session id
	// (TB-130). For Codex this comes from parsing `codex exec --json`
	// stdout; for Claude the caller pre-allocates via SessionID and the
	// callback path is unused. Wired by TB-136 in runGoroutine; left nil
	// here means "don't notify". Called at most once per run.
	OnSessionID func(sessionID string)

	// SessionID is a caller-supplied agent-side conversation id. Claude
	// accepts `--session-id <uuid>` so the daemon can pre-allocate
	// (TB-130). Empty means "let the agent CLI pick / emit it" (Codex
	// always works this way; Claude only when pre-alloc is disabled).
	SessionID string
}

// RunResult is the terminal record a Runner returns. ExitCode is set even on
// signal-kill (Go's os/exec turns -1 into the conventional "process did not
// exit normally" value). Err is non-nil only for:
//
//   - spawn failure (ErrBinaryNotFound),
//   - IO error reading the agent's pipes,
//   - the deadline path (ErrTimeout),
//   - context cancellation (returns ctx.Err()).
//
// A non-zero ExitCode with Err == nil means the agent itself returned that
// exit code — that's a failed run, not a Runner bug, and AgentService maps
// it to AgentStatus: failed.
type RunResult struct {
	ExitCode int
	Err      error
}

// Sentinel errors returned by the Runner so AgentService can branch on them
// for the error → AgentStatus mapping (see TB-47 contract).
var (
	// ErrBinaryNotFound means the agent binary isn't on PATH and PathLookup
	// failed inside the Runner before cmd.Start().
	ErrBinaryNotFound = errors.New("agent binary not found on PATH")

	// ErrTimeout means RunInput.Timeout elapsed and the Runner sent
	// SIGTERM + SIGKILL to the process group.
	ErrTimeout = errors.New("agent run timed out")
)

// --- Prompt templating -----------------------------------------------------

// PromptImplement is the embedded "implement this task" template. The bytes
// are baked into the binary at compile time; the file is the source of
// truth.
//
//go:embed prompts/implement.md
var PromptImplement string

// PromptGroom is the embedded "groom this task" template. It is intentionally
// separate from PromptImplement because grooming is a markdown-only task-body
// refinement flow with a narrower mutation contract.
//
//go:embed prompts/groom.md
var PromptGroom string

// PromptVars are the values RenderPrompt substitutes into a template. The
// set is intentionally tiny — adding a new placeholder requires editing the
// templates AND this struct AND RenderPrompt, so reviewers always see the
// new surface in one place.
type PromptVars struct {
	TaskID    string
	TaskTitle string
	TaskBody  string
}

// RenderPrompt substitutes {{TASK_ID}}, {{TASK_TITLE}}, {{TASK_BODY}} into
// the given template by exact-string replacement. No regex, no template
// engine — anything that looks like a placeholder but isn't one of these
// three tokens passes through unchanged.
//
// Multiple occurrences of the same token are all replaced.
func RenderPrompt(template string, vars PromptVars) string {
	s := template
	s = strings.ReplaceAll(s, "{{TASK_ID}}", vars.TaskID)
	s = strings.ReplaceAll(s, "{{TASK_TITLE}}", vars.TaskTitle)
	s = strings.ReplaceAll(s, "{{TASK_BODY}}", vars.TaskBody)
	return s
}

type groomingDecorator struct {
	inner Runner
	vars  PromptVars
}

// NewGroomingDecorator returns a Runner that replaces the incoming prompt with
// the rendered grooming prompt before delegating to inner.
func NewGroomingDecorator(inner Runner, vars PromptVars) Runner {
	return &groomingDecorator{inner: inner, vars: vars}
}

func (r *groomingDecorator) Name() string {
	return r.inner.Name()
}

func (r *groomingDecorator) Run(ctx context.Context, in RunInput) (RunResult, error) {
	in.Prompt = RenderPrompt(PromptGroom, r.vars)
	return r.inner.Run(ctx, in)
}
