package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFolderTaskCreate_FiresOneBoardReloaded simulates `tb create` for a
// folder-form task: rename a fully-populated staging dir into place under a
// status dir. The watcher should debounce the burst into exactly one
// board:reloaded event.
func TestFolderTaskCreate_FiresOneBoardReloaded(t *testing.T) {
	board := makeBoard(t)
	em := &captureEmitter{}
	startWatcher(t, board, em)

	// Mimic the CLI's atomic publish pattern: build the task folder in a
	// hidden staging dir, then rename it into its final position.
	staging := filepath.Join(board, "backlog", ".TB-1.staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "TASK.md"), []byte("# TB-1: demo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staging, filepath.Join(board, "backlog", "TB-1")); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	count := countEvents(em, "board:reloaded")
	if count != 1 {
		t.Fatalf("folder-task create produced %d board:reloaded events, want 1: %+v", count, em.snapshot())
	}
}

// TestAttachmentAdd_FiresOneBoardReloaded simulates `tb attach`: writes a
// staging temp file under the attachments dir, renames it into place, then
// rewrites TASK.md (atomic temp+rename). The whole burst should produce a
// single board:reloaded event.
func TestAttachmentAdd_FiresOneBoardReloaded(t *testing.T) {
	board := makeBoard(t)
	taskDir := filepath.Join(board, "backlog", "TB-1")
	if err := os.MkdirAll(filepath.Join(taskDir, "attachments"), 0o755); err != nil {
		t.Fatal(err)
	}
	taskFile := filepath.Join(taskDir, "TASK.md")
	if err := os.WriteFile(taskFile, []byte("# TB-1\n\n## Attachments\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)
	// Settle: addExistingFolderTaskWatches above already attached at start.
	time.Sleep(50 * time.Millisecond)

	// Stage the attachment (.tmp file ignored), then publish via rename.
	tmpAttach := filepath.Join(taskDir, "attachments", ".note.txt.tmp")
	if err := os.WriteFile(tmpAttach, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmpAttach, filepath.Join(taskDir, "attachments", "note.txt")); err != nil {
		t.Fatal(err)
	}
	// Atomic TASK.md rewrite (CLI uses writeFileAtomic).
	taskTmp := taskFile + ".tmp.X"
	if err := os.WriteFile(taskTmp, []byte("# TB-1\n\n## Attachments\n- attachments/note.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(taskTmp, taskFile); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	count := countEvents(em, "board:reloaded")
	if count != 1 {
		t.Fatalf("attachment add produced %d board:reloaded events, want 1: %+v", count, em.snapshot())
	}
}

// TestAttachmentRemove_FiresOneBoardReloaded mirrors the add test but
// removes one file then rewrites TASK.md.
func TestAttachmentRemove_FiresOneBoardReloaded(t *testing.T) {
	board := makeBoard(t)
	taskDir := filepath.Join(board, "backlog", "TB-2")
	if err := os.MkdirAll(filepath.Join(taskDir, "attachments"), 0o755); err != nil {
		t.Fatal(err)
	}
	taskFile := filepath.Join(taskDir, "TASK.md")
	if err := os.WriteFile(taskFile, []byte("# TB-2\n\n## Attachments\n- attachments/x.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	attach := filepath.Join(taskDir, "attachments", "x.txt")
	if err := os.WriteFile(attach, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)
	time.Sleep(50 * time.Millisecond)

	if err := os.Remove(attach); err != nil {
		t.Fatal(err)
	}
	taskTmp := taskFile + ".tmp.X"
	if err := os.WriteFile(taskTmp, []byte("# TB-2\n\n## Attachments\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(taskTmp, taskFile); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	count := countEvents(em, "board:reloaded")
	if count != 1 {
		t.Fatalf("attachment remove produced %d board:reloaded events, want 1: %+v", count, em.snapshot())
	}
}

// TestFolderTaskMove_FiresOneBoardReloaded simulates `tb mv`: rename the
// task folder from one status dir into another. Expect exactly one
// board:reloaded event.
func TestFolderTaskMove_FiresOneBoardReloaded(t *testing.T) {
	board := makeBoard(t)
	src := filepath.Join(board, "backlog", "TB-3")
	if err := os.MkdirAll(filepath.Join(src, "attachments"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "TASK.md"), []byte("# TB-3"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "attachments", "note.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)
	time.Sleep(50 * time.Millisecond)

	dst := filepath.Join(board, "in-progress", "TB-3")
	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	count := countEvents(em, "board:reloaded")
	if count != 1 {
		t.Fatalf("folder-task move produced %d board:reloaded events, want 1: %+v", count, em.snapshot())
	}
}

// TestTmpFiles_NoEventStorm asserts the watcher swallows .tmp/.tmp.* burst
// from atomic-write staging — those are pure noise to the UI.
func TestTmpFiles_NoEventStorm(t *testing.T) {
	board := makeBoard(t)
	em := &captureEmitter{}
	startWatcher(t, board, em)

	// Write and remove several .tmp files in quick succession. None should
	// reach the emitter.
	for range 5 {
		p := filepath.Join(board, "backlog", "TB-99.md.tmp.X")
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(400 * time.Millisecond)

	if count := countEvents(em, "board:reloaded"); count != 0 {
		t.Fatalf("tmp files leaked through ignore filter: %d board:reloaded events: %+v", count, em.snapshot())
	}
}

func countEvents(em *captureEmitter, name string) int {
	n := 0
	for _, e := range em.snapshot() {
		if e.Name == name {
			n++
		}
	}
	return n
}
