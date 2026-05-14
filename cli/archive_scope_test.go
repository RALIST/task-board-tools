package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type archiveScopeTask struct {
	status string
	id     string
	title  string
	tags   string
	parent string
}

func TestBoardViewsUseActiveScope(t *testing.T) {
	boardDir := newArchiveScopeBoard(t)

	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "backlog",
		id:     "TB-1",
		title:  "Active Epic",
		tags:   "epic",
	})
	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "done",
		id:     "TB-2",
		title:  "Done Child",
		parent: "TB-1",
	})
	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "archive",
		id:     "TB-3",
		title:  "Archived Child",
		parent: "TB-1",
	})
	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "archive",
		id:     "TB-4",
		title:  "Archived Epic",
		tags:   "epic",
	})
	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "done",
		id:     "TB-5",
		title:  "Recently Done",
	})
	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "archive",
		id:     "TB-6",
		title:  "Archived Closed",
	})

	content, err := buildBoardContent(boardDir)
	if err != nil {
		t.Fatalf("buildBoardContent: %v", err)
	}
	if !strings.Contains(content, "| TB-1 | Active Epic | 1/1 | backlog | cli |") {
		t.Fatalf("active epic progress should ignore archived child:\n%s", content)
	}
	assertNotContains(t, content, "TB-3")
	assertNotContains(t, content, "Archived Epic")
	if !strings.Contains(content, "| TB-5 | Recently Done | improvement | cli |") {
		t.Fatalf("recently done task missing:\n%s", content)
	}
	assertNotContains(t, content, "Archived Closed")

	snapshot, err := buildBoardSnapshot(boardDir)
	if err != nil {
		t.Fatalf("buildBoardSnapshot: %v", err)
	}
	assertTaskIDs(t, snapshot.Epics, []string{"TB-1"})
	assertTaskIDs(t, snapshot.ActiveEpics, []string{"TB-1"})
	assertTaskIDs(t, snapshot.FinishedEpics, nil)
	assertTaskIDs(t, snapshot.RecentlyDone, []string{"TB-5", "TB-2"})
	assertTaskIDs(t, snapshot.Backlog, []string{"TB-1"})
	assertTaskIDs(t, snapshot.InProgress, nil)
}

func TestEpicDefaultUsesActiveScopeAndStatusFlagCanOptIntoArchive(t *testing.T) {
	boardDir := newArchiveScopeBoard(t)
	prevBoardDir := cfg.BoardDir
	cfg.BoardDir = boardDir
	t.Cleanup(func() { cfg.BoardDir = prevBoardDir })

	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "backlog",
		id:     "TB-1",
		title:  "Active Epic",
		tags:   "epic",
	})
	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "done",
		id:     "TB-2",
		title:  "Done Child",
		parent: "TB-1",
	})
	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "archive",
		id:     "TB-3",
		title:  "Archived Child",
		parent: "TB-1",
	})
	writeArchiveScopeTask(t, boardDir, archiveScopeTask{
		status: "archive",
		id:     "TB-4",
		title:  "Archived Epic",
		tags:   "epic",
	})

	activeOut := captureStdout(t, func() {
		cmdEpic([]string{"TB-1"})
	})
	if !strings.Contains(activeOut, "Status: backlog | Progress: 1/1") {
		t.Fatalf("default epic output should count active children only:\n%s", activeOut)
	}
	assertNotContains(t, activeOut, "TB-3")

	allOut := captureStdout(t, func() {
		cmdEpic([]string{"TB-1", "--status", "all"})
	})
	if !strings.Contains(allOut, "Status: backlog | Progress: 1/2") {
		t.Fatalf("all-scope epic output should include archived child in total, but not done count:\n%s", allOut)
	}
	if !strings.Contains(allOut, "TB-3") || !strings.Contains(allOut, "[archive]") {
		t.Fatalf("all-scope epic output should show archived child explicitly:\n%s", allOut)
	}

	if _, err := findTaskInStatuses(boardDir, "TB-4", statusDirs); err == nil {
		t.Fatal("archived epic should not be found in the default active scope")
	}
	archiveDirs, err := resolveStatusFilter("archive")
	if err != nil {
		t.Fatalf("resolve archive status: %v", err)
	}
	if _, err := findTaskInStatuses(boardDir, "TB-4", archiveDirs); err != nil {
		t.Fatalf("archived epic should be found when archive scope is explicit: %v", err)
	}
}

func newArchiveScopeBoard(t *testing.T) string {
	t.Helper()

	prevPrefix := cfg.Prefix
	cfg.Prefix = "TB"
	t.Cleanup(func() { cfg.Prefix = prevPrefix })

	boardDir := t.TempDir()
	for _, status := range allStatusDirs {
		if err := os.MkdirAll(filepath.Join(boardDir, status), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", status, err)
		}
	}
	return boardDir
}

func writeArchiveScopeTask(t *testing.T, boardDir string, task archiveScopeTask) {
	t.Helper()

	tagsLine := task.tags
	parentLine := task.parent
	if tagsLine == "" {
		tagsLine = "archive-scope-test"
	}

	content := "# " + task.id + ": " + task.title + "\n\n" +
		"**Type:** improvement\n" +
		"**Priority:** P2\n" +
		"**Size:** S\n" +
		"**Module:** cli\n" +
		"**Tags:** " + tagsLine + "\n" +
		"**Branch:** test\n"
	if parentLine != "" {
		content += "**Parent:** " + parentLine + "\n"
	}
	content += "\n## Goal\n\nTest fixture.\n"

	path := filepath.Join(boardDir, task.status, task.id+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertTaskIDs(t *testing.T, tasks []taskJSON, want []string) {
	t.Helper()

	if len(tasks) != len(want) {
		t.Fatalf("task IDs = %v, want %v", taskIDs(tasks), want)
	}
	for i, task := range tasks {
		if task.ID != want[i] {
			t.Fatalf("task IDs = %v, want %v", taskIDs(tasks), want)
		}
	}
}

func taskIDs(tasks []taskJSON) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func assertNotContains(t *testing.T, text, needle string) {
	t.Helper()
	if strings.Contains(text, needle) {
		t.Fatalf("unexpected %q in:\n%s", needle, text)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return string(out)
}
