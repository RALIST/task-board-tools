package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMoveFolderTaskRenamesWholeDirectoryAndPreservesArtifacts(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	srcDir := writeMoveFolderTask(t, boardDir, "backlog", "TB-1", "Folder Move")

	var renameCalls [][2]string
	restore := stubTaskRename(func(src, dst string) error {
		renameCalls = append(renameCalls, [2]string{src, dst})
		return os.Rename(src, dst)
	})
	defer restore()

	result, err := moveTaskOnBoard(boardDir, "TB-1", "in-progress", "Started — moved to in-progress")
	if err != nil {
		t.Fatalf("moveTaskOnBoard: %v", err)
	}

	destDir := filepath.Join(boardDir, "in-progress", "TB-1")
	if result.SrcStatus != "backlog" || result.TargetStatus != "in-progress" {
		t.Fatalf("move result = %+v, want backlog -> in-progress", result)
	}
	if len(renameCalls) != 1 {
		t.Fatalf("task rename calls = %d, want 1 (%v)", len(renameCalls), renameCalls)
	}
	if renameCalls[0] != [2]string{srcDir, destDir} {
		t.Fatalf("rename call = %v, want %v -> %v", renameCalls[0], srcDir, destDir)
	}

	assertMissing(t, srcDir)
	assertMissing(t, filepath.Join(boardDir, "backlog", "TB-1.md"))
	assertExists(t, filepath.Join(destDir, folderTaskFileName))
	assertExists(t, filepath.Join(destDir, "attachments", "evidence.txt"))
	assertExists(t, filepath.Join(destDir, ".agent-state.jsonl"))
	assertExists(t, filepath.Join(destDir, ".agent-logs", "r_1.log"))

	content := readFileForTest(t, filepath.Join(destDir, folderTaskFileName))
	assertContains(t, content, "Started — moved to in-progress")
	boardContent := readFileForTest(t, filepath.Join(boardDir, "BOARD.md"))
	assertContains(t, boardContent, "| TB-1 | Folder Move | P0 | cli | - |")
	assertSingleTaskRepresentation(t, boardDir, "TB-1", filepath.Join("in-progress", "TB-1", folderTaskFileName))
}

func TestArchiveAndRestoreFolderTaskDirectory(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeMoveFolderTask(t, boardDir, "done", "TB-2", "Folder Archive")

	archiveResult, err := archiveTaskOnBoard(boardDir, "TB-2")
	if err != nil {
		t.Fatalf("archiveTaskOnBoard: %v", err)
	}
	if archiveResult.SrcStatus != "done" || archiveResult.TargetStatus != "archive" {
		t.Fatalf("archive result = %+v, want done -> archive", archiveResult)
	}

	archiveDir := filepath.Join(boardDir, "archive", "TB-2")
	assertMissing(t, filepath.Join(boardDir, "done", "TB-2"))
	assertExists(t, filepath.Join(archiveDir, folderTaskFileName))
	assertExists(t, filepath.Join(archiveDir, "attachments", "evidence.txt"))
	assertExists(t, filepath.Join(archiveDir, ".agent-state.jsonl"))
	assertExists(t, filepath.Join(archiveDir, ".agent-logs", "r_1.log"))
	assertContains(t, readFileForTest(t, filepath.Join(archiveDir, folderTaskFileName)), "Closed (archived from done)")
	assertNotContains(t, readFileForTest(t, filepath.Join(boardDir, "BOARD.md")), "Folder Archive")

	restoreResult, err := moveTaskOnBoard(boardDir, "TB-2", "backlog", "Moved to backlog")
	if err != nil {
		t.Fatalf("restore moveTaskOnBoard: %v", err)
	}
	if restoreResult.SrcStatus != "archive" || restoreResult.TargetStatus != "backlog" {
		t.Fatalf("restore result = %+v, want archive -> backlog", restoreResult)
	}

	restoredDir := filepath.Join(boardDir, "backlog", "TB-2")
	assertMissing(t, archiveDir)
	assertExists(t, filepath.Join(restoredDir, folderTaskFileName))
	assertExists(t, filepath.Join(restoredDir, "attachments", "evidence.txt"))
	assertExists(t, filepath.Join(restoredDir, ".agent-state.jsonl"))
	assertExists(t, filepath.Join(restoredDir, ".agent-logs", "r_1.log"))
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "BOARD.md")), "| TB-2 | Folder Archive | feature | P0 | M | cli |")
	assertSingleTaskRepresentation(t, boardDir, "TB-2", filepath.Join("backlog", "TB-2", folderTaskFileName))
}

func TestMoveArchiveRestoreFileTaskStaysFileForm(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeMoveFileTask(t, boardDir, "backlog", "TB-3", "File Move")

	if _, err := moveTaskOnBoard(boardDir, "TB-3", "done", "Done"); err != nil {
		t.Fatalf("move file to done: %v", err)
	}
	assertExists(t, filepath.Join(boardDir, "done", "TB-3.md"))
	assertMissing(t, filepath.Join(boardDir, "done", "TB-3"))

	if _, err := archiveTaskOnBoard(boardDir, "TB-3"); err != nil {
		t.Fatalf("archive file: %v", err)
	}
	assertExists(t, filepath.Join(boardDir, "archive", "TB-3.md"))
	assertMissing(t, filepath.Join(boardDir, "archive", "TB-3"))
	assertNotContains(t, readFileForTest(t, filepath.Join(boardDir, "BOARD.md")), "File Move")

	if _, err := moveTaskOnBoard(boardDir, "TB-3", "backlog", "Moved to backlog"); err != nil {
		t.Fatalf("restore file: %v", err)
	}
	assertExists(t, filepath.Join(boardDir, "backlog", "TB-3.md"))
	assertMissing(t, filepath.Join(boardDir, "backlog", "TB-3"))
	assertContains(t, readFileForTest(t, filepath.Join(boardDir, "BOARD.md")), "| TB-3 | File Move | feature | P0 | M | cli |")
	assertSingleTaskRepresentation(t, boardDir, "TB-3", filepath.Join("backlog", "TB-3.md"))
}

func TestMoveForcedRenameFailuresLeaveSingleSourceTask(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		writeTask  func(t *testing.T, boardDir, status, id, title string) string
		wantSource string
	}{
		{
			name:       "folder",
			id:         "TB-4",
			writeTask:  writeMoveFolderTask,
			wantSource: filepath.Join("backlog", "TB-4", folderTaskFileName),
		},
		{
			name:       "file",
			id:         "TB-5",
			writeTask:  writeMoveFileTask,
			wantSource: filepath.Join("backlog", "TB-5.md"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			boardDir := newCommandTestBoard(t)
			tc.writeTask(t, boardDir, "backlog", tc.id, "Forced Failure")

			restore := stubTaskRename(func(src, dst string) error {
				return errors.New("forced rename failure")
			})
			defer restore()

			_, err := moveTaskOnBoard(boardDir, tc.id, "done", "Done")
			if err == nil {
				t.Fatal("moveTaskOnBoard succeeded, want forced rename error")
			}
			assertContains(t, err.Error(), "forced rename failure")
			assertSingleTaskRepresentation(t, boardDir, tc.id, tc.wantSource)
			assertMissing(t, filepath.Join(boardDir, "done", tc.id+".md"))
			assertMissing(t, filepath.Join(boardDir, "done", tc.id))
			if tc.name == "folder" {
				sourceDir := filepath.Join(boardDir, "backlog", tc.id)
				assertExists(t, filepath.Join(sourceDir, "attachments", "evidence.txt"))
				assertExists(t, filepath.Join(sourceDir, ".agent-state.jsonl"))
				assertExists(t, filepath.Join(sourceDir, ".agent-logs", "r_1.log"))
			}
		})
	}
}

func TestMoveRefusesDestinationCollisionsWithoutDataLoss(t *testing.T) {
	cases := []struct {
		name      string
		source    func(t *testing.T, boardDir, status, id, title string) string
		dest      func(t *testing.T, boardDir, status, id, title string) string
		destCheck string
	}{
		{
			name:      "folder source to destination file",
			source:    writeMoveFolderTask,
			dest:      writeMoveFileTask,
			destCheck: filepath.Join("done", "TB-6.md"),
		},
		{
			name:      "file source to destination folder",
			source:    writeMoveFileTask,
			dest:      writeMoveFolderTask,
			destCheck: filepath.Join("done", "TB-6", folderTaskFileName),
		},
		{
			name:      "folder source to destination folder",
			source:    writeMoveFolderTask,
			dest:      writeMoveFolderTask,
			destCheck: filepath.Join("done", "TB-6", folderTaskFileName),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			boardDir := newCommandTestBoard(t)
			tc.source(t, boardDir, "backlog", "TB-6", "Source Task")
			tc.dest(t, boardDir, "done", "TB-6", "Destination Task")

			_, err := moveTaskOnBoard(boardDir, "TB-6", "done", "Done")
			if err == nil {
				t.Fatal("moveTaskOnBoard succeeded, want destination collision")
			}
			assertContains(t, err.Error(), "destination done already contains task TB-6")
			assertContains(t, err.Error(), "refusing to overwrite or merge")

			assertExists(t, filepath.Join(boardDir, tc.destCheck))
			if strings.Contains(tc.destCheck, folderTaskFileName) {
				assertContains(t, readFileForTest(t, filepath.Join(boardDir, tc.destCheck)), "Destination Task")
			} else {
				assertContains(t, readFileForTest(t, filepath.Join(boardDir, tc.destCheck)), "Destination Task")
			}
			if _, err := resolveTaskRef(boardDir, "TB-6", []string{"backlog"}); err != nil {
				t.Fatalf("source task lost after collision: %v", err)
			}
		})
	}
}

func stubTaskRename(fn func(src, dst string) error) func() {
	prev := renameTaskPath
	renameTaskPath = fn
	return func() { renameTaskPath = prev }
}

func writeMoveFileTask(t *testing.T, boardDir, status, id, title string) string {
	t.Helper()

	path := filepath.Join(boardDir, status, id+".md")
	writeFileForTest(t, path, moveTaskMarkdown(id, title))
	return path
}

func writeMoveFolderTask(t *testing.T, boardDir, status, id, title string) string {
	t.Helper()

	taskDir := filepath.Join(boardDir, status, id)
	if err := os.MkdirAll(filepath.Join(taskDir, "attachments"), 0755); err != nil {
		t.Fatalf("mkdir attachments: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(taskDir, ".agent-logs"), 0755); err != nil {
		t.Fatalf("mkdir agent logs: %v", err)
	}
	writeFileForTest(t, filepath.Join(taskDir, "attachments", "evidence.txt"), "attachment\n")
	writeFileForTest(t, filepath.Join(taskDir, ".agent-state.jsonl"), `{"event":"queued"}`+"\n")
	writeFileForTest(t, filepath.Join(taskDir, ".agent-logs", "r_1.log"), "log\n")
	writeFileForTest(t, filepath.Join(taskDir, folderTaskFileName), moveTaskMarkdown(id, title))
	return taskDir
}

func moveTaskMarkdown(id, title string) string {
	return fmt.Sprintf(`# %s: %s

**Type:** feature
**Priority:** P0
**Size:** M
**Module:** cli
**Branch:** -

## Goal

Move fixture.

## Acceptance Criteria

- [ ] Move works.

## Log

- 2026-05-14: Created
`, id, title)
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Fatalf("expected %s to be missing", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertSingleTaskRepresentation(t *testing.T, boardDir, id, wantRel string) {
	t.Helper()

	var got []string
	for _, status := range allStatusDirs {
		filePath := filepath.Join(boardDir, status, id+".md")
		if _, err := os.Lstat(filePath); err == nil {
			got = append(got, filepath.Join(status, id+".md"))
		}
		folderMarkdown := filepath.Join(boardDir, status, id, folderTaskFileName)
		if _, err := os.Lstat(folderMarkdown); err == nil {
			got = append(got, filepath.Join(status, id, folderTaskFileName))
		}
	}
	if len(got) != 1 || filepath.Clean(got[0]) != filepath.Clean(wantRel) {
		t.Fatalf("task representations for %s = %v, want [%s]", id, got, wantRel)
	}
}
