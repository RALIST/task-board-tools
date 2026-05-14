package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestListJSONSkipsMalformedTasksAndWarns(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeJSONWarningTask(t, boardDir, "backlog", "TB-1", "Valid Task")
	malformedPath := filepath.Join(boardDir, "backlog", "TB-2.md")
	writeFileForTest(t, malformedPath, "# TB-2\n\n**Priority:** P0\n")

	var stderr string
	out := captureStdout(t, func() {
		stderr = captureStderr(t, func() {
			cmdList([]string{"--json", "--status", "backlog"})
		})
	})

	var got []taskJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal ls JSON: %v\n%s", err, out)
	}
	assertTaskIDs(t, got, []string{"TB-1"})
	assertContains(t, stderr, "warning: skipping malformed task file")
	assertContains(t, stderr, malformedPath)
	assertNotContains(t, out, "TB-2")
}

func TestBoardJSONSkipsMalformedTasksAndWarns(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeJSONWarningTask(t, boardDir, "backlog", "TB-1", "Valid Backlog")
	writeJSONWarningTask(t, boardDir, "done", "TB-3", "Valid Done")
	malformedPath := filepath.Join(boardDir, "backlog", "TB-2.md")
	writeFileForTest(t, malformedPath, "# TB-2:\n\n**Priority:** P0\n")

	var stderr string
	out := captureStdout(t, func() {
		stderr = captureStderr(t, func() {
			cmdBoard([]string{"--json"})
		})
	})

	var got boardSnapshotJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal board JSON: %v\n%s", err, out)
	}
	assertTaskIDs(t, got.Backlog, []string{"TB-1"})
	assertTaskIDs(t, got.RecentlyDone, []string{"TB-3"})
	assertContains(t, stderr, "warning: skipping malformed task file")
	assertContains(t, stderr, malformedPath)
	assertNotContains(t, out, "TB-2")
}

func TestShowJSONMalformedTaskExitsWithParseError(t *testing.T) {
	if os.Getenv("TB_TEST_SHOW_JSON_PARSE_ERROR") == "1" {
		cfg = tbConfig{
			RootDir:        os.Getenv("TB_TEST_ROOT_DIR"),
			BoardDir:       os.Getenv("TB_TEST_BOARD_DIR"),
			Prefix:         "TB",
			WipLimit:       2,
			ScanExtensions: defaultScanExtensions(),
		}
		cmdShow([]string{"TB-1", "--json"})
		return
	}

	boardDir := newCommandTestBoard(t)
	malformedPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	writeFileForTest(t, malformedPath, "# TB-1:    \n\n**Priority:** P1\n")

	cmd := exec.Command(os.Args[0], "-test.run=^TestShowJSONMalformedTaskExitsWithParseError$")
	cmd.Env = append(os.Environ(),
		"TB_TEST_SHOW_JSON_PARSE_ERROR=1",
		"TB_TEST_ROOT_DIR="+filepath.Dir(boardDir),
		"TB_TEST_BOARD_DIR="+boardDir,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("cmdShow succeeded, want non-zero exit")
	}
	if stdout.Len() != 0 {
		t.Fatalf("show --json wrote stdout on parse error:\n%s", stdout.String())
	}
	assertContains(t, stderr.String(), "error: parse ")
	assertContains(t, stderr.String(), malformedPath)
	assertContains(t, stderr.String(), "malformed task header")
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	return string(out)
}

func writeJSONWarningTask(t *testing.T, boardDir, status, id, title string) {
	t.Helper()

	content := strings.Join([]string{
		"# " + id + ": " + title,
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Exercise JSON output.",
		"",
	}, "\n")
	writeFileForTest(t, filepath.Join(boardDir, status, id+".md"), content)
}

func writeFileForTest(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
