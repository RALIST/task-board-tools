package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanApplyCreatesFolderTaskAndKeepsTriageMarker(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	root := filepath.Dir(boardDir)
	sourceDir := filepath.Join(root, "internal", "scan")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	sourcePath := filepath.Join(sourceDir, "todo.go")
	writeFileForTest(t, sourcePath, strings.Join([]string{
		"package scan",
		"",
		"func work() {",
		"\t// TODO: wire scan storage",
		"}",
		"",
	}, "\n"))

	var dryRunOut string
	withWorkingDirForTest(t, root, func() {
		dryRunOut = captureStdout(t, func() {
			cmdScan([]string{"--path", "internal"})
		})
	})

	assertContains(t, dryRunOut, "Found 1 untagged comment")
	assertContains(t, dryRunOut, "internal/scan/todo.go:4")
	assertPathMissing(t, filepath.Join(boardDir, "backlog", "TB-2"))
	assertPathMissing(t, filepath.Join(boardDir, "backlog", "TB-2.md"))
	assertContains(t, readFileForTest(t, sourcePath), "// TODO: wire scan storage")

	withWorkingDirForTest(t, root, func() {
		captureStdout(t, func() {
			cmdScan([]string{"--apply", "--path", "internal"})
		})
	})

	taskPath := filepath.Join(boardDir, "backlog", "TB-2", folderTaskFileName)
	assertPathExists(t, taskPath)
	assertPathMissing(t, filepath.Join(boardDir, "backlog", "TB-2.md"))

	content := readFileForTest(t, taskPath)
	assertContains(t, content, "# TB-2: wire scan storage")
	assertContains(t, content, "**Type:** tech-debt")
	assertContains(t, content, "**Priority:** P2")
	assertContains(t, content, "**Size:** S")
	assertContains(t, content, "**Module:** scan")
	assertContains(t, content, "## Goal\n\nResolve TODO at `internal/scan/todo.go:4`.")
	assertContains(t, content, "## Acceptance Criteria\n\n- [ ] (to be filled)")
	assertContains(t, content, "## Attachments\n\n## Log")
	assertContains(t, content, "Created by `tb scan` from TODO comment")
	assertSectionsInOrder(t, content, []string{"## Goal", "## Acceptance Criteria", "## Attachments", "## Log"})

	assertContains(t, readFileForTest(t, sourcePath), "// TODO(TB-2): wire scan storage")

	nextID := strings.TrimSpace(readFileForTest(t, filepath.Join(boardDir, ".next-id")))
	if nextID != "3" {
		t.Fatalf(".next-id = %q, want 3", nextID)
	}

	boardContent := readFileForTest(t, filepath.Join(boardDir, "BOARD.md"))
	assertContains(t, boardContent, "| TB-2 | wire scan storage | tech-debt | P2 | S | scan |")

	var triageOut string
	withWorkingDirForTest(t, root, func() {
		triageOut = captureStdout(t, func() {
			cmdTriage(nil)
		})
	})
	assertContains(t, triageOut, filepath.Join("board", "backlog", "TB-2", folderTaskFileName))
	assertContains(t, triageOut, "auto-created by scan")
}

func TestScanForTodosReturnsWalkError(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	missingRoot := filepath.Join(boardDir, "missing")

	hits, err := scanForTodos(missingRoot, cfg.RootDir)
	if err == nil {
		t.Fatal("expected filepath.Walk error")
	}
	if len(hits) != 0 {
		t.Fatalf("hits = %v, want none on walk failure", hits)
	}
	if !strings.Contains(err.Error(), missingRoot) {
		t.Fatalf("scan error = %q, want missing root path", err)
	}
}
