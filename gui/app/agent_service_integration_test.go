package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tools/tb-gui/internal/cli"
)

// TestAssignAgent_RealTb_PersistsToTaskFile is the F4.1 on-disk truth: after
// AssignAgent(claude), `tb show <id>` reports `**Agent:** claude`; after
// AssignAgent(none), the field is cleared. Without this test the rest of M4
// could pass while the assignment was silently lost.
func TestAssignAgent_RealTb_PersistsToTaskFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX flock only")
	}
	tbBinary := buildTbForIntegration(t)

	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, d := range []string{"backlog", "in-progress", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(boardDir, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte("board: board\nprefix: TB\n"), 0o644); err != nil {
		t.Fatalf(".tb.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("2\n"), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}
	taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	if err := os.WriteFile(taskPath, []byte(sampleTaskBody), 0o644); err != nil {
		t.Fatalf("task md: %v", err)
	}

	c, err := cli.NewClient(cli.Options{BinaryPath: tbBinary, Cwd: root})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	board := NewBoardService()
	board.setClient(c)
	board.setBoardDir(boardDir)
	svc := NewAgentService(AgentServiceOptions{Board: board})

	// 1. Assign claude — `tb show` should report **Agent:** claude.
	if err := svc.AssignAgent(context.Background(), "TB-1", "claude"); err != nil {
		t.Fatalf("AssignAgent(claude): %v", err)
	}
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**Agent:** claude") {
		t.Fatalf("after AssignAgent(claude), `tb show` missing **Agent:** claude:\n%s", out)
	}

	// 2. Clear via none — field should drop out of the metadata block.
	if err := svc.AssignAgent(context.Background(), "TB-1", "none"); err != nil {
		t.Fatalf("AssignAgent(none): %v", err)
	}
	out, err = c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show after clear: %v", err)
	}
	if strings.Contains(string(out), "**Agent:** claude") {
		t.Fatalf("after AssignAgent(none), `tb show` still shows **Agent:** claude:\n%s", out)
	}
}
