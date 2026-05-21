package watcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// findFirstTaskID returns the ID of the first task it finds under
// statusDir, supporting both folder form (<ID>/TASK.md) and legacy file
// form (<ID>.md).
func findFirstTaskID(t *testing.T, statusDir string) string {
	t.Helper()
	entries, err := os.ReadDir(statusDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			return name
		}
		if filepath.Ext(name) == ".md" {
			return name[:len(name)-len(".md")]
		}
	}
	t.Fatalf("no task entries under %s: %+v", statusDir, entries)
	return ""
}

// locateTBBinary builds the CLI from source into t.TempDir() so the test
// always runs against the current source tree, not whatever stale `tb`
// happens to be on PATH or at /tmp/tb. Falls back to PATH lookup only if the
// repo's cli/ module is unreachable.
func locateTBBinary(t *testing.T) string {
	t.Helper()

	// Walk up from this test file's dir to find the repo root (the dir that
	// contains both `go.work` and `cli/`).
	cwd, err := os.Getwd()
	if err != nil {
		t.Skipf("getwd: %v", err)
	}
	root := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(root, "go.work")); err == nil {
			if _, err := os.Stat(filepath.Join(root, "cli")); err == nil {
				break
			}
		}
		parent := filepath.Dir(root)
		if parent == root {
			root = ""
			break
		}
		root = parent
	}
	if root == "" {
		if tbBin, err := exec.LookPath("tb"); err == nil {
			return tbBin
		}
		t.Skip("tb binary not available and could not locate source tree")
	}

	out := filepath.Join(t.TempDir(), "tb")
	cmd := exec.Command("go", "build", "-o", out, "./cli")
	cmd.Dir = root
	if buildOut, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("go build ./cli failed: %v\n%s", err, buildOut)
	}
	return out
}

// TestIntegration_TBMvFiresOneBoardReloaded drives the real `tb` binary
// against a temp project and asserts that a single `tb mv` produces exactly
// one `board:reloaded` event within 1s, even though the mutation under the
// hood writes both the moved task and a regenerated BOARD.md.
func TestIntegration_TBMvFiresOneBoardReloaded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix-only test")
	}
	tbBin := locateTBBinary(t)

	project := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(tbBin, args...)
		cmd.Dir = project
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("tb %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("create", "Watcher integration test")

	boardDir := filepath.Join(project, "board")
	em := &captureEmitter{}
	startWatcher(t, boardDir, em)

	// Find the created ID. `tb create` defaults to folder form
	// (board/backlog/<ID>/TASK.md), but `--legacy-file` would produce
	// board/backlog/<ID>.md — handle both.
	id := findFirstTaskID(t, filepath.Join(boardDir, "backlog"))

	run("mv", id, "ip")

	// Wait for debounce window + buffer.
	time.Sleep(400 * time.Millisecond)

	count := 0
	for _, e := range em.snapshot() {
		if e.Name == "board:reloaded" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("tb mv produced %d board:reloaded events (want 1): %+v", count, em.snapshot())
	}
}

// TestIntegration_TBAttachFiresOneBoardReloaded drives the real `tb attach`
// path: attaching one file to a folder-form task should debounce into one
// board:reloaded event despite the multiple intermediate temp files and
// renames the CLI uses for atomicity.
func TestIntegration_TBAttachFiresOneBoardReloaded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix-only test")
	}
	tbBin := locateTBBinary(t)

	project := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(tbBin, args...)
		cmd.Dir = project
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("tb %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	// `tb create` defaults to folder form (TB-97), so the new task lives at
	// board/backlog/<ID>/TASK.md and exercises the folder-task watch paths.
	run("create", "Attach integration test")

	boardDir := filepath.Join(project, "board")
	taskID := findFirstTaskID(t, filepath.Join(boardDir, "backlog"))

	// Prepare a real file to attach. Outside the project dir so `tb attach`
	// has a non-trivial copy path.
	attachSource := filepath.Join(t.TempDir(), "evidence.txt")
	if err := os.WriteFile(attachSource, []byte("evidence"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, boardDir, em)

	run("attach", taskID, attachSource)

	time.Sleep(400 * time.Millisecond)

	count := 0
	for _, e := range em.snapshot() {
		if e.Name == "board:reloaded" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("tb attach produced %d board:reloaded events (want 1): %+v", count, em.snapshot())
	}
}

func TestIntegration_TBEditFolderTaskFiresTaskUpdated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix-only test")
	}
	tbBin := locateTBBinary(t)

	project := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(tbBin, args...)
		cmd.Dir = project
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("tb %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("create", "Edit integration test")

	boardDir := filepath.Join(project, "board")
	taskID := findFirstTaskID(t, filepath.Join(boardDir, "backlog"))

	em := &captureEmitter{}
	startWatcher(t, boardDir, em)

	run("edit", taskID, "--title", "Edited by watcher integration")

	got := waitFor(t, 1*time.Second, func() bool {
		return countEvents(em, "task:updated:"+taskID) > 0
	})
	if !got {
		t.Fatalf("tb edit produced no task:updated:%s event: %+v", taskID, em.snapshot())
	}
}
