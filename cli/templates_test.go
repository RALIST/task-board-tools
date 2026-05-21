package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConventionsTemplateStaysPolicyFocused(t *testing.T) {
	content := conventionsTemplate("TB", "board")

	for _, want := range []string{
		"# Board Conventions",
		"backlog → ready → in-progress → code-review → done → archive",
		"Directories are the source of truth",
		"ready",
		"WIP",
		"Related Tasks",
		"AgentStatus",
		"`lost`",
		"`needs-user`",
		"Every done task needs evidence",
		"Implementation tasks should point to a commit or review artifact that includes the task ID",
		"Spikes should link or attach the investigation result",
		"Archive is only for closing work that should leave the active board",
		"`tb review --pass`",
		"Review metadata is runner evidence, not the review decision",
		"does not pass review unless `tb review --pass` moved the task to `done`",
		"Agent stopped because user input is required",
		"User Attention",
		"stop cleanly",
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
		"## Autonomous Stages",
		"`auto-groom`",
		"`auto-implement`",
		"`auto-review`",
		"`auto_review_enabled`",
		"auto-implement must not pick a later numeric child",
		"a later numeric child must not be selected while an earlier child is still active outside `done`",
		"Daemon housekeeping for autonomous stages",
		"`initiator=auto-review`",
		"Auto-review recovery",
		"Resume is offered",
		"captured session id",
		"UI labels",
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

	want := conventionsTemplate("TB", "board")
	if string(got) != want {
		t.Fatalf("%s is out of sync with conventionsTemplate(\"TB\", \"board\")", path)
	}
}

func TestConventionsTemplateUsesConfiguredBoardPath(t *testing.T) {
	content := conventionsTemplate("WS", ".workflow/tasks")

	assertContains(t, content, "This board root is configured in `.tb.yaml` as `.workflow/tasks`.")
	assertContains(t, content, "For this board, generated views such as `.workflow/tasks/BOARD.md` live under that configured root.")
	assertContains(t, content, "Task IDs use the `WS-NNN` shape")
	assertNotContains(t, content, "generated views such as `board/BOARD.md`")
}

func TestSkillTemplateIsPortableAgentSkill(t *testing.T) {
	content := skillTemplate("TB", "board")

	for _, want := range []string{
		"---\nname: task-board\n",
		"description:",
		"Use when working with a markdown task board: inspecting board state",
		"# Task Board Workflow",
		"Compatible with Claude Code and Codex.",
		"Read `board/CONVENTIONS.md` before changing board state",
		"Directories are status",
		"never edit `BOARD.md` by hand",
		"Pull from `ready` before coding",
		"Do not move backlog directly to `in-progress`",
		"Every `done` task needs evidence",
		"Implementation tasks should cite a commit or review artifact that includes `TB-NNN`",
		"Spike tasks should link or attach the investigation result",
		"Review runs must finish through an explicit board handoff",
		"it does not pass a task that remains in `code-review`",
		"Use `archive` only to close obsolete, duplicate, superseded, or intentionally dropped tasks",
		"## Backlog Capture",
		"`needs-user`",
		"Reason:",
		"Question/Action:",
		"Attempted context:",
		"Unblock condition:",
		"## Minimal Commands",
	} {
		assertContains(t, content, want)
	}

	for _, forbidden := range []string{
		"### CLI Reference",
		"**Examples:**",
		"Based on the argument, perform one of",
		"## Board Management",
		"tb init [path]",
		"tb board [--json]",
		"tb attach <TB-NNN> <path>...",
		"| Command | Aliases | Description |",
		"Claude-only",
		"Codex-only",
		"@board/",
		"## Autonomous Stages",
		"`auto-groom`",
		"`auto-implement`",
		"`auto-review`",
		"`auto_review_enabled`",
		"auto-implement must not pick a later numeric child",
		"Daemon housekeeping is deterministic repair only",
		"`initiator=auto-review`",
		"Auto-review recovery",
	} {
		assertNotContains(t, content, forbidden)
	}
}

func TestCheckedInSkillMatchesTemplate(t *testing.T) {
	path := filepath.Join("..", "board", "SKILL.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checked-in skill: %v", err)
	}

	want := skillTemplate("TB", "board")
	if string(got) != want {
		t.Fatalf("%s is out of sync with skillTemplate(\"TB\", \"board\")", path)
	}
}
