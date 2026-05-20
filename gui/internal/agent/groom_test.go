package agent

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestPromptGroom_NonEmptyAndUsesOnlySupportedPlaceholders(t *testing.T) {
	if strings.TrimSpace(PromptGroom) == "" {
		t.Fatal("PromptGroom is empty")
	}

	seen := map[string]bool{}
	allowed := map[string]bool{
		"{{TASK_ID}}":    true,
		"{{TASK_TITLE}}": true,
		"{{TASK_BODY}}":  true,
	}
	for _, token := range regexp.MustCompile(`\{\{[^}]+\}\}`).FindAllString(PromptGroom, -1) {
		if !allowed[token] {
			t.Fatalf("PromptGroom uses unsupported placeholder %s", token)
		}
		seen[token] = true
	}
	for token := range allowed {
		if !seen[token] {
			t.Errorf("PromptGroom missing placeholder %s", token)
		}
	}
}

func TestPromptGroom_StatesGroomingMutationContract(t *testing.T) {
	required := []string{
		"tb show {{TASK_ID}}",
		"tb edit {{TASK_ID}} --goal -",
		"tb edit {{TASK_ID}} --acceptance -",
		"stdin heredoc",
		"Do not change code",
		"Do not write directly to markdown files",
		"`Log`",
		"`Related Tasks`",
		"`Context`",
		"Do not move the task",
		// TB-182: groom prompt must describe the user-attention handoff
		// so an agent that cannot finish grooming stops cleanly instead
		// of guessing or silently retrying.
		"--user-attention",
		"--agent-status needs-user",
		"Unblock condition",
	}
	for _, text := range required {
		if !strings.Contains(PromptGroom, text) {
			t.Errorf("PromptGroom missing contract text %q", text)
		}
	}
}

func TestGroomingDecorator_NameDelegates(t *testing.T) {
	inner := &stubRunner{name: "codex"}
	runner := NewGroomingDecorator(inner, PromptVars{})

	if got := runner.Name(); got != "codex" {
		t.Fatalf("Name() = %q, want %q", got, "codex")
	}
}

func TestGroomingDecorator_RunRendersPromptAndForwardsInput(t *testing.T) {
	runErr := errors.New("runner failed")
	wantResult := RunResult{ExitCode: 23, Err: runErr}
	inner := &stubRunner{name: "claude", result: wantResult, err: runErr}
	vars := PromptVars{
		TaskID:    "TB-65",
		TaskTitle: "Prompt grooming",
		TaskBody:  "Clarify grooming constraints.",
	}
	runner := NewGroomingDecorator(inner, vars)

	ctx := context.WithValue(context.Background(), testContextKey{}, "ctx")
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	var startedPID, startedPGID int
	input := RunInput{
		TaskID:      "TB-65",
		Mode:        ModeImplement,
		Prompt:      "original prompt must be replaced",
		ProjectRoot: "/tmp/project",
		Env:         []string{"FOO=bar", "BAZ=qux"},
		Timeout:     3 * time.Minute,
		Stdout:      stdout,
		Stderr:      stderr,
		OnStarted: func(pid, pgid int) {
			startedPID = pid
			startedPGID = pgid
		},
	}

	gotResult, gotErr := runner.Run(ctx, input)

	if gotResult != wantResult {
		t.Fatalf("Run result = %#v, want %#v", gotResult, wantResult)
	}
	if !errors.Is(gotErr, runErr) {
		t.Fatalf("Run error = %v, want %v", gotErr, runErr)
	}
	if inner.calls != 1 {
		t.Fatalf("inner Run called %d times, want 1", inner.calls)
	}
	if inner.ctx != ctx {
		t.Fatal("Run did not forward context unchanged")
	}

	gotInput := inner.input
	wantPrompt := RenderPrompt(PromptGroom, vars)
	if gotInput.Prompt != wantPrompt {
		t.Fatalf("forwarded prompt:\n got %q\nwant %q", gotInput.Prompt, wantPrompt)
	}
	if gotInput.Prompt == input.Prompt {
		t.Fatal("decorator did not overwrite original prompt")
	}
	if strings.Contains(gotInput.Prompt, "{{TASK_") {
		t.Fatalf("forwarded prompt still contains placeholders: %s", gotInput.Prompt)
	}

	if gotInput.TaskID != input.TaskID {
		t.Errorf("TaskID = %q, want %q", gotInput.TaskID, input.TaskID)
	}
	if gotInput.Mode != input.Mode {
		t.Errorf("Mode = %q, want %q", gotInput.Mode, input.Mode)
	}
	if gotInput.ProjectRoot != input.ProjectRoot {
		t.Errorf("ProjectRoot = %q, want %q", gotInput.ProjectRoot, input.ProjectRoot)
	}
	if gotInput.Timeout != input.Timeout {
		t.Errorf("Timeout = %v, want %v", gotInput.Timeout, input.Timeout)
	}
	if len(gotInput.Env) != len(input.Env) {
		t.Fatalf("Env length = %d, want %d", len(gotInput.Env), len(input.Env))
	}
	for i := range input.Env {
		if gotInput.Env[i] != input.Env[i] {
			t.Errorf("Env[%d] = %q, want %q", i, gotInput.Env[i], input.Env[i])
		}
	}
	if gotInput.Stdout != stdout {
		t.Fatal("Stdout writer was not forwarded unchanged")
	}
	if gotInput.Stderr != stderr {
		t.Fatal("Stderr writer was not forwarded unchanged")
	}
	if gotInput.OnStarted == nil {
		t.Fatal("OnStarted was not forwarded")
	}
	gotInput.OnStarted(11, 12)
	if startedPID != 11 || startedPGID != 12 {
		t.Fatalf("OnStarted callback did not match original: pid=%d pgid=%d", startedPID, startedPGID)
	}
}

type testContextKey struct{}

type stubRunner struct {
	name   string
	result RunResult
	err    error

	calls int
	ctx   context.Context
	input RunInput
}

func (r *stubRunner) Name() string {
	return r.name
}

func (r *stubRunner) Run(ctx context.Context, in RunInput) (RunResult, error) {
	r.calls++
	r.ctx = ctx
	r.input = in
	return r.result, r.err
}
