package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateDefaultsToFolderTask(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	root := filepath.Dir(boardDir)

	var out string
	withWorkingDirForTest(t, root, func() {
		out = captureStdout(t, func() {
			cmdCreate([]string{
				"Folder Task",
				"-m", "cli",
				"-d", "Detailed goal.",
				"-p", "P1",
				"-T", "feature",
				"-s", "S",
				"-t", "alpha,beta",
				"--status", "ip",
			})
		})
	})

	taskPath := filepath.Join(boardDir, "in-progress", "TB-2", folderTaskFileName)
	assertPathExists(t, taskPath)
	assertPathMissing(t, filepath.Join(boardDir, "in-progress", "TB-2.md"))
	assertContains(t, out, filepath.Join("board", "in-progress", "TB-2", folderTaskFileName))

	content := readFileForTest(t, taskPath)
	assertContains(t, content, "# TB-2: Folder Task")
	assertContains(t, content, "**Type:** feature")
	assertContains(t, content, "**Priority:** P1")
	assertContains(t, content, "**Size:** S")
	assertContains(t, content, "**Module:** cli")
	assertContains(t, content, "**Tags:** alpha,beta")
	assertContains(t, content, "## Goal\n\nDetailed goal.")
	assertContains(t, content, "## Acceptance Criteria\n\n- [ ] (to be filled)")
	assertContains(t, content, "## Attachments\n\n## Log")
	assertSectionsInOrder(t, content, []string{"## Goal", "## Acceptance Criteria", "## Attachments", "## Log"})

	nextID := strings.TrimSpace(readFileForTest(t, filepath.Join(boardDir, ".next-id")))
	if nextID != "3" {
		t.Fatalf(".next-id = %q, want 3", nextID)
	}

	boardContent := readFileForTest(t, filepath.Join(boardDir, "BOARD.md"))
	assertContains(t, boardContent, "| TB-2 | Folder Task | P1 | cli |")

	withWorkingDirForTest(t, root, func() {
		showOut := captureStdout(t, func() {
			cmdShow([]string{"2"})
		})
		assertContains(t, showOut, "# TB-2: Folder Task")

		listOut := captureStdout(t, func() {
			cmdList([]string{"--status", "ip"})
		})
		assertContains(t, listOut, "board/in-progress/TB-2/TASK.md")
		assertContains(t, listOut, "Folder Task")
	})
}

func TestCreateFolderTaskUpdatesParentSubtasks(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	root := filepath.Dir(boardDir)
	parentPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	writeFileForTest(t, parentPath, strings.Join([]string{
		"# TB-1: Parent Task",
		"",
		"**Type:** feature",
		"**Priority:** P0",
		"**Size:** XL",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Parent goal.",
		"",
		"## Acceptance Criteria",
		"",
		"- [ ] Parent acceptance.",
		"",
		"## Log",
		"",
		"- 2026-05-14: Created",
		"",
	}, "\n"))

	withWorkingDirForTest(t, root, func() {
		captureStdout(t, func() {
			cmdCreate([]string{"Child Task", "-m", "cli", "-s", "L", "--parent", "1", "-d", "Child goal."})
		})
	})

	childPath := filepath.Join(boardDir, "backlog", "TB-2", folderTaskFileName)
	assertPathExists(t, childPath)
	assertPathMissing(t, filepath.Join(boardDir, "backlog", "TB-2.md"))
	childContent := readFileForTest(t, childPath)
	assertContains(t, childContent, "**Parent:** TB-1")
	assertContains(t, childContent, "## Attachments\n\n## Log")

	parentContent := readFileForTest(t, parentPath)
	assertContains(t, parentContent, "**Tags:** epic\n**Branch:** -")
	assertContains(t, parentContent, "## Subtasks\n\n- **TB-2** (L) — Child Task")
	assertContains(t, parentContent, "Child Task\n\n## Acceptance Criteria")
}

func TestCreateLegacyFileFlag(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	root := filepath.Dir(boardDir)

	var out string
	withWorkingDirForTest(t, root, func() {
		out = captureStdout(t, func() {
			cmdCreate([]string{"Legacy Task", "--legacy-file", "-m", "cli"})
		})
	})

	taskPath := filepath.Join(boardDir, "backlog", "TB-2.md")
	assertPathExists(t, taskPath)
	assertPathMissing(t, filepath.Join(boardDir, "backlog", "TB-2", folderTaskFileName))
	assertContains(t, out, filepath.Join("board", "backlog", "TB-2.md"))

	content := readFileForTest(t, taskPath)
	assertContains(t, content, "# TB-2: Legacy Task")
	assertNotContains(t, content, "## Attachments")
}

func withWorkingDirForTest(t *testing.T, dir string, fn func()) {
	t.Helper()

	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	fn()
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s to be absent", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertSectionsInOrder(t *testing.T, content string, headings []string) {
	t.Helper()
	prev := -1
	for _, heading := range headings {
		idx := strings.Index(content, heading)
		if idx == -1 {
			t.Fatalf("missing %q in:\n%s", heading, content)
		}
		if idx <= prev {
			t.Fatalf("heading %q appears out of order in:\n%s", heading, content)
		}
		prev = idx
	}
}
