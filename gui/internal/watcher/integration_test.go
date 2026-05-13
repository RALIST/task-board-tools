package watcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestIntegration_TBMvFiresOneBoardReloaded drives the real `tb` binary
// against a temp project and asserts that a single `tb mv` produces exactly
// one `board:reloaded` event within 1s, even though the mutation under the
// hood writes both the moved task and a regenerated BOARD.md.
func TestIntegration_TBMvFiresOneBoardReloaded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix-only test")
	}
	tbBin, err := exec.LookPath("tb")
	if err != nil {
		// Allow fallback to the project's go build output.
		tbBin = "/tmp/tb"
		if _, err := os.Stat(tbBin); err != nil {
			t.Skipf("tb binary not available: %v", err)
		}
	}

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

	// Find the created ID — `tb create` allocates from .next-id.
	entries, err := os.ReadDir(filepath.Join(boardDir, "backlog"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no backlog entries after create")
	}
	id := entries[0].Name()[:len(entries[0].Name())-3] // strip .md

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
