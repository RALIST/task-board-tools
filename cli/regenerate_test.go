package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRegenerateBoardLockedWaitsForBoardLock(t *testing.T) {
	if os.Getenv("TB_TEST_HOLD_REGENERATE_LOCK") == "1" {
		holdRegenerateLockForTest(t)
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("flock-based locking is not portable on Windows")
	}

	prevPrefix := cfg.Prefix
	cfg.Prefix = "TB"
	defer func() { cfg.Prefix = prevPrefix }()

	boardDir := t.TempDir()
	for _, dir := range statusDirs {
		if err := os.MkdirAll(filepath.Join(boardDir, dir), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("2\n"), 0644); err != nil {
		t.Fatalf("seed .next-id: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "BOARD.md"), []byte("# Board\n\nold\n"), 0644); err != nil {
		t.Fatalf("seed BOARD.md: %v", err)
	}

	controlDir := t.TempDir()
	readyPath := filepath.Join(controlDir, "ready")
	releasePath := filepath.Join(controlDir, "release")
	taskTitle := "Generated After Lock"

	cmd := exec.Command(os.Args[0], "-test.run=TestRegenerateBoardLockedWaitsForBoardLock")
	cmd.Env = append(os.Environ(),
		"TB_TEST_HOLD_REGENERATE_LOCK=1",
		"TB_TEST_BOARD_DIR="+boardDir,
		"TB_TEST_LOCK_READY="+readyPath,
		"TB_TEST_RELEASE_LOCK="+releasePath,
		"TB_TEST_TASK_TITLE="+taskTitle,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start lock holder: %v", err)
	}
	defer func() {
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	waitForFile(t, readyPath)

	done := make(chan error, 1)
	go func() {
		done <- regenerateBoardLocked(boardDir)
	}()

	select {
	case err := <-done:
		t.Fatalf("regenerateBoardLocked returned before lock release: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	if err := os.WriteFile(releasePath, []byte("release\n"), 0644); err != nil {
		t.Fatalf("release lock holder: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("lock holder failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("regenerateBoardLocked: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("regenerateBoardLocked did not finish after lock release")
	}

	data, err := os.ReadFile(filepath.Join(boardDir, "BOARD.md"))
	if err != nil {
		t.Fatalf("read BOARD.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "TB-1") || !strings.Contains(content, taskTitle) {
		t.Fatalf("BOARD.md did not include task written before lock release:\n%s", content)
	}
}

func holdRegenerateLockForTest(t *testing.T) {
	t.Helper()

	boardDir := os.Getenv("TB_TEST_BOARD_DIR")
	readyPath := os.Getenv("TB_TEST_LOCK_READY")
	releasePath := os.Getenv("TB_TEST_RELEASE_LOCK")
	taskTitle := os.Getenv("TB_TEST_TASK_TITLE")
	if boardDir == "" || readyPath == "" || releasePath == "" || taskTitle == "" {
		t.Fatal("missing lock-holder test environment")
	}

	lock, err := lockBoard(boardDir)
	if err != nil {
		t.Fatalf("lockBoard: %v", err)
	}
	defer lock.unlock()

	if err := os.WriteFile(readyPath, []byte("ready\n"), 0644); err != nil {
		t.Fatalf("write ready file: %v", err)
	}
	waitForFile(t, releasePath)

	taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	content := fmt.Sprintf(`# TB-1: %s

**Type:** bug
**Priority:** P2
**Size:** S
**Module:** cli
**Branch:** -

## Goal

Created by a locked writer.
`, taskTitle)
	if err := writeFileAtomic(taskPath, []byte(content), 0644); err != nil {
		t.Fatalf("write locked task: %v", err)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}
