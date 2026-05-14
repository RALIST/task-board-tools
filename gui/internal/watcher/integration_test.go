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

func locateTBBinary(t *testing.T) string {
	t.Helper()
	tbBin, err := exec.LookPath("tb")
	if err == nil {
		return tbBin
	}
	// Fallback to the project's go build output.
	fallback := "/tmp/tb"
	if _, err := os.Stat(fallback); err != nil {
		t.Skipf("tb binary not available: %v", err)
	}
	return fallback
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
