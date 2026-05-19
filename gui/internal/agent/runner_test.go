package agent

import (
	"strings"
	"testing"
)

func TestMode_String(t *testing.T) {
	if ModeImplement.String() != "implement" {
		t.Fatalf("ModeImplement: %q", ModeImplement)
	}
	if ModeGroom.String() != "groom" {
		t.Fatalf("ModeGroom: %q", ModeGroom)
	}
	if ModeReview.String() != "review" {
		t.Fatalf("ModeReview: %q", ModeReview)
	}
}

func TestPromptReview_LocksReviewContract(t *testing.T) {
	if strings.TrimSpace(PromptReview) == "" {
		t.Fatal("PromptReview is empty")
	}
	for _, tok := range []string{"{{TASK_ID}}"} {
		if !strings.Contains(PromptReview, tok) {
			t.Errorf("PromptReview missing placeholder %s", tok)
		}
	}
	for _, tok := range []string{"{{TASK_TITLE}}", "{{TASK_BODY}}"} {
		if strings.Contains(PromptReview, tok) {
			t.Errorf("PromptReview should not duplicate task context placeholder %s; review runs read live task data via tb show", tok)
		}
	}
	// TB-198: review-mode runs MUST be read-only against implementation
	// code, MUST write findings through the managed `tb review` surface,
	// and MUST use the failure handoff (--fail) for blocking findings.
	for _, text := range []string{
		"tb review --findings {{TASK_ID}}",
		"tb review --fail {{TASK_ID}}",
		"Do NOT change implementation code",
	} {
		if !strings.Contains(PromptReview, text) {
			t.Errorf("PromptReview missing required text %q", text)
		}
	}
}

func TestPromptImplement_NonEmptyAndContainsPlaceholders(t *testing.T) {
	if strings.TrimSpace(PromptImplement) == "" {
		t.Fatal("PromptImplement is empty")
	}
	for _, tok := range []string{"{{TASK_ID}}", "{{TASK_TITLE}}", "{{TASK_BODY}}"} {
		if !strings.Contains(PromptImplement, tok) {
			t.Errorf("PromptImplement missing placeholder %s", tok)
		}
	}
	// TB-182: implement prompt must describe the user-attention handoff
	// (stop cleanly when blocked instead of guessing or silently retrying).
	for _, text := range []string{
		"--user-attention",
		"--agent-status needs-user",
		"Unblock condition",
	} {
		if !strings.Contains(PromptImplement, text) {
			t.Errorf("PromptImplement missing user-attention handoff text %q", text)
		}
	}
}

func TestRenderPrompt_ReplacesAllOccurrences(t *testing.T) {
	tmpl := "id={{TASK_ID}} title={{TASK_TITLE}} body={{TASK_BODY}} id-again={{TASK_ID}}"
	got := RenderPrompt(tmpl, PromptVars{
		TaskID:    "TB-99",
		TaskTitle: "Hello",
		TaskBody:  "Body text",
	})
	want := "id=TB-99 title=Hello body=Body text id-again=TB-99"
	if got != want {
		t.Fatalf("RenderPrompt:\n got %q\nwant %q", got, want)
	}
}

func TestRenderPrompt_UnknownPlaceholdersPassThrough(t *testing.T) {
	tmpl := "id={{TASK_ID}} unknown={{TASK_FOO}} brace={NOT_TOKEN}"
	got := RenderPrompt(tmpl, PromptVars{TaskID: "TB-1"})
	want := "id=TB-1 unknown={{TASK_FOO}} brace={NOT_TOKEN}"
	if got != want {
		t.Fatalf("RenderPrompt should pass through unknown tokens:\n got %q\nwant %q", got, want)
	}
}

func TestRenderPrompt_RealTemplate(t *testing.T) {
	got := RenderPrompt(PromptImplement, PromptVars{
		TaskID:    "TB-42",
		TaskTitle: "Fix crash on empty input",
		TaskBody:  "The crash happens when …",
	})
	if strings.Contains(got, "{{TASK_") {
		t.Errorf("rendered prompt still has placeholders: %s", got)
	}
	if !strings.Contains(got, "TB-42") || !strings.Contains(got, "Fix crash on empty input") {
		t.Errorf("rendered prompt missing substituted values: %s", got)
	}
}
