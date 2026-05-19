package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConventionsTemplateStaysPolicyFocused(t *testing.T) {
	content := conventionsTemplate("TB")

	for _, want := range []string{
		"# Board Conventions",
		"backlog → ready → in-progress → code-review → done → archive",
		"Directories are the source of truth",
		"ready",
		"WIP",
		"Related Tasks",
		"AgentStatus",
	} {
		assertContains(t, content, want)
	}

	for _, forbidden := range []string{
		"## CLI Reference",
		"## Project Refresh",
		"**Examples:**",
		"tb init [path]",
		"tb create \"Title\"",
		"tb ls --status",
		"tb edit <TB-NNN>",
		"tb attach <TB-NNN>",
		"tb regenerate",
		"<status>/TB-NNN/TASK.md",
		"<status>/TB-NNN.md",
		"Folder-form tasks",
		"Legacy path:",
	} {
		assertNotContains(t, content, forbidden)
	}

	if strings.Contains(content, "\ntb ") || strings.Contains(content, "\n  tb ") {
		t.Fatalf("conventions template should not include exact tb command recipes:\n%s", content)
	}
}

func TestCheckedInConventionsMatchesTemplate(t *testing.T) {
	path := filepath.Join("..", "board", "CONVENTIONS.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checked-in conventions: %v", err)
	}

	want := conventionsTemplate("TB")
	if string(got) != want {
		t.Fatalf("%s is out of sync with conventionsTemplate(\"TB\")", path)
	}
}
