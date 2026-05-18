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
	assertContains(t, conventions, "tb init [path] [--board-path=board] [--prefix=TB] [--refresh-docs]")
	assertContains(t, conventions, "tb board [--json]")
	assertContains(t, conventions, "tb show <TB-NNN> [--json]")
	assertContains(t, conventions, "tb attach <TB-NNN> <path>...")
	assertContains(t, conventions, "tb assign <TB-NNN> <claude|codex>")
	assertContains(t, conventions, "tb create --legacy-file")
	assertContains(t, conventions, "<status>/TB-NNN/TASK.md")

	skill := readFileForTest(t, filepath.Join(boardDir, "SKILL.md"))
	assertContains(t, skill, "tb attach <TB-NNN> <path>...")
	assertContains(t, skill, "tb assign <TB-NNN> <claude|codex>")
	assertContains(t, skill, "tb show <TB-NNN> [--json]")
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
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md")), "tb init [path] [--board-path=board] [--prefix=TB]")
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "SKILL.md")), "All operations use the `tb` CLI")
}

func TestInitExistingBoardPreservesExtraConfigFieldsByDefault(t *testing.T) {
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

	if got := readFileForTest(t, filepath.Join(root, configFileName)); got != config {
		t.Fatalf(".tb.yaml changed:\n%s", got)
	}
	assertPathMissing(t, filepath.Join(root, configFileName+".bak"))
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
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md")), "Legacy path: `<status>/TB-NNN.md`")
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
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "CONVENTIONS.md")), "Generated kanban view")
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "SKILL.md")), "All operations use the `tb` CLI")
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
