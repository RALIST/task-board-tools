package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// captureEmitter records every event the watcher emits, in order.
type captureEmitter struct {
	mu     sync.Mutex
	events []event
}

type event struct {
	Name string
	Data []any
}

func (c *captureEmitter) Emit(name string, data ...any) {
	c.mu.Lock()
	c.events = append(c.events, event{Name: name, Data: append([]any(nil), data...)})
	c.mu.Unlock()
}

func (c *captureEmitter) snapshot() []event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]event(nil), c.events...)
}

// waitFor returns true once predicate is satisfied or timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, pred func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pred() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return pred()
}

func makeBoard(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range statusDirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	return root
}

func startWatcher(t *testing.T, board string, em *captureEmitter) (context.CancelFunc, *Watcher) {
	t.Helper()
	w := New(em, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = w.Start(ctx, board) }()
	// Give Start a moment to attach. The first attach happens synchronously
	// inside Start before the event loop, so a short yield is enough.
	time.Sleep(50 * time.Millisecond)
	t.Cleanup(cancel)
	return cancel, w
}

func TestCreate_FiresBoardReloaded(t *testing.T) {
	board := makeBoard(t)
	em := &captureEmitter{}
	startWatcher(t, board, em)

	// Atomic-create-pattern: write a tmp file, then rename into place. The
	// rename triggers a Create event on the dest path.
	tmp := filepath.Join(board, "backlog", "TB-1.md.tmp")
	if err := os.WriteFile(tmp, []byte("# TB-1: hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(board, "backlog", "TB-1.md")); err != nil {
		t.Fatal(err)
	}

	got := waitFor(t, 1*time.Second, func() bool {
		for _, e := range em.snapshot() {
			if e.Name == "board:reloaded" {
				return true
			}
		}
		return false
	})
	if !got {
		t.Fatalf("no board:reloaded received: %+v", em.snapshot())
	}
}

func TestDebounce_CoalescesBurst(t *testing.T) {
	board := makeBoard(t)
	em := &captureEmitter{}
	startWatcher(t, board, em)

	// Simulate the "tb mv" burst: remove old file, create new file in another
	// dir, then BOARD.md regenerate (which is ignored). All within the 200ms
	// debounce window.
	src := filepath.Join(board, "backlog", "TB-2.md")
	if err := os.WriteFile(src, []byte("# TB-2"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Let the create settle but keep within the debounce window.
	time.Sleep(20 * time.Millisecond)

	dst := filepath.Join(board, "in-progress", "TB-2.md")
	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce window to elapse, then a bit more.
	time.Sleep(400 * time.Millisecond)

	count := 0
	for _, e := range em.snapshot() {
		if e.Name == "board:reloaded" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("debounce failed: got %d board:reloaded events, want 1; events=%+v", count, em.snapshot())
	}
}

func TestWrite_FiresTaskUpdated(t *testing.T) {
	board := makeBoard(t)
	// Create a task file before starting the watcher so the Write doesn't
	// race with the Create handler.
	taskPath := filepath.Join(board, "in-progress", "TB-3.md")
	if err := os.WriteFile(taskPath, []byte("# TB-3"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)

	// Now overwrite — should produce a Write event.
	if err := os.WriteFile(taskPath, []byte("# TB-3 v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := waitFor(t, 1*time.Second, func() bool {
		for _, e := range em.snapshot() {
			if e.Name == "task:updated:TB-3" {
				return true
			}
		}
		return false
	})
	if !got {
		t.Fatalf("no task:updated:TB-3 received: %+v", em.snapshot())
	}
}

func TestAtomicRename_FiresTaskUpdated(t *testing.T) {
	board := makeBoard(t)
	taskPath := filepath.Join(board, "in-progress", "TB-30.md")
	if err := os.WriteFile(taskPath, []byte("# TB-30"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)

	tmp := filepath.Join(board, "in-progress", ".TB-30.md.tmp.12345")
	if err := os.WriteFile(tmp, []byte("# TB-30 v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, taskPath); err != nil {
		t.Fatal(err)
	}

	got := waitFor(t, 1*time.Second, func() bool {
		for _, e := range em.snapshot() {
			if e.Name == "task:updated:TB-30" {
				return true
			}
		}
		return false
	})
	if !got {
		t.Fatalf("no task:updated:TB-30 received: %+v", em.snapshot())
	}
	time.Sleep(400 * time.Millisecond)
	if count := countEvents(em, "board:reloaded"); count != 0 {
		t.Fatalf("task file atomic rename produced %d board:reloaded events, want 0: %+v", count, em.snapshot())
	}
}

func TestIgnore_BOARDMdDoesNotFire(t *testing.T) {
	board := makeBoard(t)
	em := &captureEmitter{}
	startWatcher(t, board, em)

	// Writing BOARD.md inside a status dir would be unusual but the rule is
	// basename-based — confirm it's ignored anyway.
	if err := os.WriteFile(filepath.Join(board, "backlog", "BOARD.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)

	for _, e := range em.snapshot() {
		if e.Name == "board:reloaded" {
			t.Fatalf("BOARD.md leaked through ignore filter: %+v", em.snapshot())
		}
	}
}

func TestSwitch_RetargetsToNewBoard(t *testing.T) {
	boardA := makeBoard(t)
	boardB := makeBoard(t)

	em := &captureEmitter{}
	_, w := startWatcher(t, boardA, em)

	if err := w.Switch(boardB); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	// Settle.
	time.Sleep(50 * time.Millisecond)

	// An event in boardB should now register.
	if err := os.WriteFile(filepath.Join(boardB, "backlog", "TB-7.md"), []byte("# TB-7"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := waitFor(t, 1*time.Second, func() bool {
		for _, e := range em.snapshot() {
			if e.Name == "board:reloaded" {
				return true
			}
		}
		return false
	})
	if !got {
		t.Fatalf("Switch did not retarget: %+v", em.snapshot())
	}
}

func TestStart_NoStatusDirs_ReturnsError(t *testing.T) {
	empty := t.TempDir()
	em := &captureEmitter{}
	w := New(em, nil)
	err := w.Start(context.Background(), empty)
	if err == nil {
		t.Fatal("want error for board without status dirs")
	}
}

func TestTaskIDFromPath(t *testing.T) {
	cases := map[string]string{
		"/x/y/backlog/TB-1.md":      "TB-1",
		"/x/y/done/PR-99.md":        "PR-99",
		"/x/y/done/BOARD.md":        "", // BOARD.md is on the ignored list
		"/x/y/done/.next-id":        "",
		"/x/y/done/notes.txt":       "",
		"/x/y/done/sub/TB-1.md.tmp": "",
	}
	for in, want := range cases {
		if got := taskIDFromPath(in); got != want {
			t.Errorf("taskIDFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}
