package main

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssignCommandHappyPath(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskPath := writeAssignTestTask(t, boardDir, "backlog", "TB-1", nil)

	out := captureStdout(t, func() {
		cmdAssign([]string{"TB-1", "codex"})
	})
	assertContains(t, out, "Assigned TB-1 to agent=codex, agentstatus=queued")

	content := readAssignTestTask(t, taskPath)
	assertContains(t, content, "**Agent:** codex\n**AgentStatus:** queued\n**Branch:** -")
	assertContains(t, content, "- 2026-05-14: Created")
	if got := strings.Count(content, ": Assigned agent=codex, agentstatus=queued"); got != 1 {
		t.Fatalf("assigned log entry count = %d, want 1:\n%s", got, content)
	}

	boardContent, err := os.ReadFile(filepath.Join(boardDir, "BOARD.md"))
	if err != nil {
		t.Fatalf("read regenerated BOARD.md: %v", err)
	}
	assertContains(t, string(boardContent), "TB-1")
	assertNoAgentArtifacts(t, boardDir)
}

func TestAssignNormalizesIDAndFindsArchivedTask(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskPath := writeAssignTestTask(t, boardDir, "archive", "TB-30", nil)

	out := captureStdout(t, func() {
		cmdAssign([]string{"30", "claude"})
	})
	assertContains(t, out, "Assigned TB-30 to agent=claude, agentstatus=queued")

	content := readAssignTestTask(t, taskPath)
	assertContains(t, content, "**Agent:** claude\n**AgentStatus:** queued\n**Branch:** -")
	if got := strings.Count(content, ": Assigned agent=claude, agentstatus=queued"); got != 1 {
		t.Fatalf("assigned log entry count = %d, want 1:\n%s", got, content)
	}
}

func TestAssignCommandRejectsInvalidArgs(t *testing.T) {
	if os.Getenv("TB_TEST_ASSIGN_REJECTS_INVALID_ARGS") == "1" {
		cfg = tbConfig{
			RootDir:        t.TempDir(),
			BoardDir:       t.TempDir(),
			Prefix:         "TB",
			WipLimit:       2,
			ScanExtensions: defaultScanExtensions(),
		}
		rawArgs := os.Getenv("TB_TEST_ASSIGN_ARGS")
		var args []string
		if rawArgs != "" {
			args = strings.Split(rawArgs, "\x1f")
		}
		cmdAssign(args)
		return
	}

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing all args", args: nil, want: "task ID and agent are required"},
		{name: "missing agent", args: []string{"TB-1"}, want: "task ID and agent are required"},
		{name: "extra arg", args: []string{"TB-1", "codex", "extra"}, want: "too many arguments"},
		{name: "invalid agent", args: []string{"TB-1", "gpt"}, want: `invalid agent "gpt"`},
		{name: "none is not runnable", args: []string{"TB-1", "none"}, want: `invalid agent "none"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, err := runAssignInvalidArgsHelper(t, tc.args)
			if err == nil {
				t.Fatalf("cmdAssign(%v) succeeded, want non-zero exit", tc.args)
			}
			assertContains(t, stderr, tc.want)
			assertContains(t, stderr, "Usage: tb assign <ID> <agent>")
			if tc.args != nil && len(tc.args) == 2 && tc.args[1] == "none" {
				assertContains(t, stderr, "runnable agents")
			}
		})
	}
}

func TestAssignDoesNotChangeEditAgentSemantics(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskPath := writeAssignTestTask(t, boardDir, "backlog", "TB-1", []string{
		"**Agent:** codex",
		"**AgentStatus:** success",
	})

	captureStdout(t, func() {
		cmdEdit([]string{"TB-1", "-a", "claude"})
	})

	content := readAssignTestTask(t, taskPath)
	assertContains(t, content, "**Agent:** claude\n**AgentStatus:** success\n**Branch:** -")
	assertContains(t, content, ": Edited agent=claude")
	assertNotContains(t, content, ": Assigned ")
}

func TestUsageIncludesAssign(t *testing.T) {
	out := captureStdout(t, usage)
	assertContains(t, out, "tb assign <ID> <agent>")
	assertContains(t, out, "Assign claude|codex and queue for daemon pickup")
	assertContains(t, out, "AgentStatus=queued for daemon pickup")
}

func runAssignInvalidArgsHelper(t *testing.T, args []string) (string, error) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestAssignCommandRejectsInvalidArgs$")
	cmd.Env = append(os.Environ(),
		"TB_TEST_ASSIGN_REJECTS_INVALID_ARGS=1",
		"TB_TEST_ASSIGN_ARGS="+strings.Join(args, "\x1f"),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}

func writeAssignTestTask(t *testing.T, boardDir, status, id string, metadata []string) string {
	t.Helper()

	lines := []string{
		"# " + id + ": Assignable Task",
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** cli",
	}
	lines = append(lines, metadata...)
	lines = append(lines,
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Queue this task for a runnable agent.",
		"",
		"## Acceptance Criteria",
		"",
		"- [ ] Agent metadata is updated.",
		"",
		"## Log",
		"",
		"- 2026-05-14: Created",
		"",
	)

	path := filepath.Join(boardDir, status, id+".md")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("write assign test task: %v", err)
	}
	return path
}

func readAssignTestTask(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read assign test task: %v", err)
	}
	return string(data)
}

func assertNoAgentArtifacts(t *testing.T, boardDir string) {
	t.Helper()

	for _, name := range []string{".agent-state", ".agent-logs"} {
		path := filepath.Join(boardDir, name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s should not be created by tb assign", path)
		}
	}

	err := filepath.WalkDir(boardDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".jsonl") {
			t.Fatalf("tb assign should not write JSONL files, found %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan board for agent artifacts: %v", err)
	}
}
