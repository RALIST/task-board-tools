package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitRefreshDocsUsesExistingConfigAndPreservesFolderBoardState(t *testing.T) {
	root, boardDir := seedInitializedBoardForRefresh(t, "TB")
	taskDir := filepath.Join(boardDir, "backlog", "TB-1")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	taskContent := strings.Join([]string{
		"# TB-1: Existing Folder Task",
		"",
		"**Type:** improvement",
		"**Priority:** P1",
		"**Size:** M",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Existing task must not be touched.",
		"",
		"## Attachments",
		"",
		"- evidence.txt",
		"",
		"## Log",
		"",
		"- 2026-05-18: Created",
		"",
	}, "\n")
	writeFileForTest(t, filepath.Join(taskDir, folderTaskFileName), taskContent)
	writeFileForTest(t, filepath.Join(taskDir, "evidence.txt"), "attachment bytes\n")
	archivedTask := "# TB-9: Archived Task\n\n**Type:** bug\n"
	writeFileForTest(t, filepath.Join(boardDir, "archive", "TB-9.md"), archivedTask)
	writeFileForTest(t, filepath.Join(boardDir, "BOARD.md"), "# Board\n\nmanual snapshot\n")
	writeFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"), "# Old Conventions\n\nUse PR-123 examples.\n")
	writeFileForTest(t, filepath.Join(boardDir, "SKILL.md"), "# Old Skill\n\nRun tb create.\n")

	out := captureStdout(t, func() {
		cmdInit([]string{root, "--refresh-docs"})
	})

	assertContains(t, out, "Refreshed board docs")
	assertContains(t, out, "CONVENTIONS.md.bak")
	assertContains(t, out, "SKILL.md.bak")
	if got := readFileForTest(t, filepath.Join(boardDir, ".next-id")); got != "42\n" {
		t.Fatalf(".next-id changed to %q", got)
	}
	if got := readFileForTest(t, filepath.Join(boardDir, "BOARD.md")); got != "# Board\n\nmanual snapshot\n" {
		t.Fatalf("BOARD.md changed to:\n%s", got)
	}
	if got := readFileForTest(t, filepath.Join(taskDir, folderTaskFileName)); got != taskContent {
		t.Fatalf("task content changed:\n%s", got)
	}
	if got := readFileForTest(t, filepath.Join(taskDir, "evidence.txt")); got != "attachment bytes\n" {
		t.Fatalf("attachment changed to %q", got)
	}
	if got := readFileForTest(t, filepath.Join(boardDir, "archive", "TB-9.md")); got != archivedTask {
		t.Fatalf("archive task changed:\n%s", got)
	}

	conventions := readFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"))
	assertContains(t, conventions, "TB-NNN")
	assertNotContains(t, conventions, "PR-NNN")
	assertContains(t, conventions, "policy guide, not a command manual")
	assertContains(t, conventions, "backlog → ready → in-progress → code-review → done → archive")
	assertContains(t, conventions, "Directories are the source of truth")
	assertContains(t, conventions, "## Task Quality")
	assertContains(t, conventions, "## Review Loop")
	assertNotContains(t, conventions, "## CLI Reference")
	assertNotContains(t, conventions, "tb init [path]")
	assertNotContains(t, conventions, "tb create \"Title\"")
	assertNotContains(t, conventions, "<status>/TB-NNN/TASK.md")

	skill := readFileForTest(t, filepath.Join(boardDir, "SKILL.md"))
	assertContains(t, skill, "---\nname: task-board\n")
	assertContains(t, skill, "Compatible with Claude Code and Codex.")
	assertContains(t, skill, "Every `done` task needs evidence")
	assertContains(t, skill, "tb show TB-NNN")
	assertNotContains(t, skill, "### CLI Reference")
}

func TestInitExistingBoardRefreshesDocsByDefault(t *testing.T) {
	root, boardDir := seedInitializedBoardForRefresh(t, "TB")
	writeFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"), "# Stale Conventions\n")
	writeFileForTest(t, filepath.Join(boardDir, "SKILL.md"), "# Stale Skill\n")

	out := captureStdout(t, func() {
		cmdInit([]string{root})
	})

	assertContains(t, out, "Refreshed board docs")
	assertContains(t, out, "CONVENTIONS.md.bak")
	assertContains(t, out, "SKILL.md.bak")
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md")), "Detailed command syntax belongs in CLI help")
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "SKILL.md")), "Use when working with a markdown task board: inspecting board state")
}

func TestInitGeneratedConventionsUseConfiguredBoardPath(t *testing.T) {
	root := t.TempDir()

	captureStdout(t, func() {
		cmdInit([]string{root, "--board-path", ".workflow/tasks", "--prefix", "WS"})
	})

	boardDir := filepath.Join(root, ".workflow", "tasks")
	conventions := readFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"))
	assertContains(t, conventions, "This board root is configured in `.tb.yaml` as `.workflow/tasks`.")
	assertContains(t, conventions, "generated views such as `.workflow/tasks/BOARD.md`")
	assertContains(t, conventions, "Task IDs use the `WS-NNN` shape")
	assertNotContains(t, conventions, "generated views such as `board/BOARD.md`")
}

func TestInitExistingBoardExpandsMinimalConfigWithAnnotatedDefaults(t *testing.T) {
	root, boardDir := seedInitializedBoardForRefresh(t, "TB")
	writeFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"), "# Stale Conventions\n")
	writeFileForTest(t, filepath.Join(boardDir, "SKILL.md"), "# Stale Skill\n")

	out := captureStdout(t, func() {
		cmdInit([]string{root})
	})

	assertContains(t, out, "Config updated")
	assertContains(t, out, ".tb.yaml.bak")
	if got := readFileForTest(t, filepath.Join(root, configFileName+".bak")); got != "board: board\nprefix: TB\n" {
		t.Fatalf(".tb.yaml backup = %q", got)
	}
	updated := readFileForTest(t, filepath.Join(root, configFileName))
	assertContains(t, updated, "# Board directory relative to this file.")
	assertContains(t, updated, "board: board")
	assertContains(t, updated, "# Task ID prefix.")
	assertContains(t, updated, "prefix: TB")
	assertContains(t, updated, "# Canonical kanban WIP limits.")
	assertContains(t, updated, "# wip_limit: 2")
	assertContains(t, updated, "# wip_limit_ready: 5")
	assertContains(t, updated, "# wip_limit_in_progress: 2")
	assertContains(t, updated, "# wip_limit_code_review: 3")
	assertContains(t, updated, "# wip_enforcement: warn")
	assertContains(t, updated, "# File extensions scanned by `tb scan`.")
	assertContains(t, updated, "# scan_extensions: .go,.ts,.svelte,.js,.tsx,.jsx")
}

func TestInitExistingBoardBacksUpConfigBeforeChangingExplicitFields(t *testing.T) {
	root, boardDir := seedInitializedBoardForRefresh(t, "TB")
	config := strings.Join([]string{
		"board: board",
		"prefix: TB",
		"wip_limit: 5",
		"",
	}, "\n")
	writeFileForTest(t, filepath.Join(root, configFileName), config)
	writeFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"), "# Stale Conventions\n")
	writeFileForTest(t, filepath.Join(boardDir, "SKILL.md"), "# Stale Skill\n")

	out := captureStdout(t, func() {
		cmdInit([]string{root, "--prefix", "WS"})
	})

	assertContains(t, out, "Config updated")
	assertContains(t, out, ".tb.yaml.bak")
	if got := readFileForTest(t, filepath.Join(root, configFileName+".bak")); got != config {
		t.Fatalf(".tb.yaml backup = %q", got)
	}
	updated := readFileForTest(t, filepath.Join(root, configFileName))
	assertContains(t, updated, "prefix: WS")
	assertContains(t, updated, "wip_limit: 5")
}

func TestInitExistingBoardKeepsConfiguredOptionalFieldsActive(t *testing.T) {
	root, boardDir := seedInitializedBoardForRefresh(t, "TB")
	config := strings.Join([]string{
		"board: board",
		"prefix: TB",
		"wip_limit: 5",
		"scan_extensions: .go,.md",
		"",
	}, "\n")
	writeFileForTest(t, filepath.Join(root, configFileName), config)
	writeFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"), "# Stale Conventions\n")
	writeFileForTest(t, filepath.Join(boardDir, "SKILL.md"), "# Stale Skill\n")

	captureStdout(t, func() {
		cmdInit([]string{root})
	})

	updated := readFileForTest(t, filepath.Join(root, configFileName))
	assertContains(t, updated, "wip_limit: 5")
	assertContains(t, updated, "scan_extensions: .go,.md")
	assertNotContains(t, updated, "# wip_limit: 2")
	assertNotContains(t, updated, "# scan_extensions: .go,.ts,.svelte,.js,.tsx,.jsx")
}

func TestInitExistingBoardDoesNotBackUpIdenticalGeneratedFiles(t *testing.T) {
	root, boardDir := seedInitializedBoardForRefresh(t, "TB")
	currentConfig := string(renderConfigTemplate(map[string]string{
		"board":  "board",
		"prefix": "TB",
	}))
	writeFileForTest(t, filepath.Join(root, configFileName), currentConfig)
	for _, doc := range generatedBoardDocs("TB", "board") {
		writeFileForTest(t, filepath.Join(boardDir, doc.name), doc.content)
	}

	out := captureStdout(t, func() {
		cmdInit([]string{root})
	})

	assertContains(t, out, "Config already current")
	assertContains(t, out, "Board docs already current")
	assertPathMissing(t, filepath.Join(root, configFileName+".bak"))
	assertPathMissing(t, filepath.Join(boardDir, "CONVENTIONS.md.bak"))
	assertPathMissing(t, filepath.Join(boardDir, "SKILL.md.bak"))
}

func TestInitRefreshDocsPreservesLegacyFileFormBoard(t *testing.T) {
	root, boardDir := seedInitializedBoardForRefresh(t, "TB")
	legacyTask := strings.Join([]string{
		"# TB-1: Existing Legacy Task",
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** S",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Legacy task must stay in file form.",
		"",
	}, "\n")
	writeFileForTest(t, filepath.Join(boardDir, "backlog", "TB-1.md"), legacyTask)
	writeFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"), "# Old\n")
	writeFileForTest(t, filepath.Join(boardDir, "SKILL.md"), "# Old\n")

	captureStdout(t, func() {
		cmdInit([]string{root, "--refresh-docs"})
	})

	if got := readFileForTest(t, filepath.Join(boardDir, "backlog", "TB-1.md")); got != legacyTask {
		t.Fatalf("legacy task changed:\n%s", got)
	}
	assertPathMissing(t, filepath.Join(boardDir, "backlog", "TB-1", folderTaskFileName))
	conventions := readFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"))
	assertContains(t, conventions, "A task entry exists in one status only")
	assertNotContains(t, conventions, "Legacy path:")
}

func TestInitRefreshDocsBacksUpCustomizedDocs(t *testing.T) {
	root, boardDir := seedInitializedBoardForRefresh(t, "TB")
	customConventions := "# Team Board Rules\n\nKeep the deployment checklist here.\n"
	customSkill := "## Local Agent Workflow\n\nUse the team's custom board ritual.\n"
	writeFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md"), customConventions)
	writeFileForTest(t, filepath.Join(boardDir, "SKILL.md"), customSkill)

	captureStdout(t, func() {
		cmdInit([]string{root, "--refresh-docs"})
	})

	if got := readFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md.bak")); got != customConventions {
		t.Fatalf("custom conventions backup = %q", got)
	}
	if got := readFileForTest(t, filepath.Join(boardDir, "SKILL.md.bak")); got != customSkill {
		t.Fatalf("custom skill backup = %q", got)
	}
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md")), "generated board view")
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "SKILL.md")), "Use when working with a markdown task board: inspecting board state")
}

func seedInitializedBoardForRefresh(t *testing.T, prefix string) (string, string) {
	t.Helper()

	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, status := range allStatusDirs {
		if err := os.MkdirAll(filepath.Join(boardDir, status), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", status, err)
		}
	}
	writeFileForTest(t, filepath.Join(root, configFileName), "board: board\nprefix: "+prefix+"\n")
	writeFileForTest(t, filepath.Join(boardDir, ".next-id"), "42\n")
	writeFileForTest(t, filepath.Join(boardDir, "BOARD.md"), "# Board\n\n")
	return root, boardDir
}
