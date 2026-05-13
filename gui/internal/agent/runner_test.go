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
