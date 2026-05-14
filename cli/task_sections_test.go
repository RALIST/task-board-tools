package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAddChildToSubtasksIgnoresQuotedAndFencedHeadings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TB-1.md")
	initial := strings.Join([]string{
		"# TB-1: Parent",
		"",
		"> ## Subtasks",
		"> - fake quoted child",
		"",
		"```md",
		"## Subtasks",
		"- fake fenced child",
		"```",
		"",
		"## Acceptance Criteria",
		"",
		"- [ ] Keep acceptance.",
		"",
		"## Log",
		"",
		"- 2026-05-14: Created",
		"",
	}, "\n")
	writeFileForTest(t, path, initial)

	if err := addChildToSubtasks(path, "TB-2", "S", "Child Task"); err != nil {
		t.Fatalf("addChildToSubtasks: %v", err)
	}

	content := readFileForTest(t, path)
	assertContains(t, content, "> ## Subtasks\n> - fake quoted child")
	assertContains(t, content, "```md\n## Subtasks\n- fake fenced child\n```")
	assertContains(t, content, "```\n\n## Subtasks\n\n- **TB-2** (S) ")
	assertContains(t, content, "- **TB-2** (S) ")
	assertContains(t, content, "Child Task\n\n## Acceptance Criteria")
}

func TestAddChildToSubtasksMatchesTrailingSpaceHeading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TB-1.md")
	initial := strings.Join([]string{
		"# TB-1: Parent",
		"",
		"## Subtasks   ",
		"",
		"- **TB-1** (S) - Existing",
		"",
		"## Acceptance Criteria",
		"",
		"- [ ] Keep acceptance.",
		"",
	}, "\n")
	writeFileForTest(t, path, initial)

	if err := addChildToSubtasks(path, "TB-2", "M", "Child Task"); err != nil {
		t.Fatalf("addChildToSubtasks: %v", err)
	}

	content := readFileForTest(t, path)
	assertContains(t, content, "## Subtasks   \n\n- **TB-1** (S) - Existing\n- **TB-2** (M) ")
	if strings.Count(content, "\n## Subtasks") != 1 {
		t.Fatalf("unexpected duplicate Subtasks section:\n%s", content)
	}
}

func TestAppendLogEntryUsesStructuralLogSection(t *testing.T) {
	entry := "- 2026-05-14: Moved\n"
	content := strings.Join([]string{
		"# TB-1: Task",
		"",
		"```md",
		"## Log",
		"- fake fenced log",
		"```",
		"",
		"## Log   ",
		"",
		"- 2026-05-13: Created",
		"",
		"~~~",
		"## Context",
		"still log body",
		"~~~",
		"",
		"## Context",
		"",
		"Keep context.",
		"",
	}, "\n")

	got := appendLogEntry(content, entry)
	assertContains(t, got, "```md\n## Log\n- fake fenced log\n```")
	assertContains(t, got, "- 2026-05-13: Created\n\n~~~\n## Context\nstill log body\n~~~\n- 2026-05-14: Moved\n\n## Context")
}

func TestTriagePlaceholderDetectionUsesStructuralSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TB-1.md")
	content := strings.Join([]string{
		"# TB-1: Needs Goal",
		"",
		"> ## Goal",
		"> This quoted heading is not the task goal.",
		"",
		"```md",
		"## Goal",
		"This fenced heading is not the task goal.",
		"```",
		"",
		"## Acceptance Criteria   ",
		"",
		"- [ ] (to be filled)",
		"",
	}, "\n")
	writeFileForTest(t, path, content)

	reasons := checkNeedsGrooming(path, Task{Priority: "P2", Module: "cli"})
	want := []string{"no goal", "no acceptance criteria"}
	if !reflect.DeepEqual(reasons, want) {
		t.Fatalf("grooming reasons = %#v, want %#v", reasons, want)
	}
}

func readFileForTest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
