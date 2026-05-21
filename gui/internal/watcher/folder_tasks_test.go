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

// TestFolderTaskPromotion_EmitsTaskUpdatedAfterRename simulates the file→
// folder promotion publish: a staging dir with TASK.md already inside is
// renamed into the status dir. The Create event for the new task dir fires
// AFTER TASK.md is already in place, so an immediately-following write to
// TASK.md could be lost in the gap between the Create and the watcher's
// fsw.Add. The watcher defends against that by synthesising a
// task:updated:<id> emission when the just-watched task dir already has a
// TASK.md.
func TestFolderTaskPromotion_EmitsTaskUpdatedAfterRename(t *testing.T) {
	board := makeBoard(t)
	em := &captureEmitter{}
	startWatcher(t, board, em)

	staging := filepath.Join(board, "backlog", ".TB-7.promote.staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "TASK.md"), []byte("# TB-7: promoted"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staging, filepath.Join(board, "backlog", "TB-7")); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	if count := countEvents(em, "task:updated:TB-7"); count < 1 {
		t.Fatalf("promotion produced %d task:updated:TB-7 events, want >=1: %+v", count, em.snapshot())
	}
}

func TestFolderTaskMarkdownAtomicRename_FiresTaskUpdated(t *testing.T) {
	board := makeBoard(t)
	taskDir := filepath.Join(board, "in-progress", "TB-8")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	taskFile := filepath.Join(taskDir, "TASK.md")
	if err := os.WriteFile(taskFile, []byte("# TB-8: old"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)

	tmp := filepath.Join(taskDir, ".TASK.md.tmp.12345")
	if err := os.WriteFile(tmp, []byte("# TB-8: fresh"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, taskFile); err != nil {
		t.Fatal(err)
	}

	got := waitFor(t, 1*time.Second, func() bool {
		return countEvents(em, "task:updated:TB-8") > 0
	})
	if !got {
		t.Fatalf("no task:updated:TB-8 received after TASK.md rename: %+v", em.snapshot())
	}
	time.Sleep(400 * time.Millisecond)
	if count := countEvents(em, "board:reloaded"); count != 0 {
		t.Fatalf("TASK.md rename produced %d board:reloaded events, want 0: %+v", count, em.snapshot())
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

// TestTaskRootAttachmentAdd_FiresOneBoardReloaded covers the TB-224 storage
// contract: new attachments are regular files in the task directory itself.
func TestTaskRootAttachmentAdd_FiresOneBoardReloaded(t *testing.T) {
	board := makeBoard(t)
	taskDir := filepath.Join(board, "backlog", "TB-11")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	taskFile := filepath.Join(taskDir, "TASK.md")
	if err := os.WriteFile(taskFile, []byte("# TB-11\n\n## Attachments\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)
	time.Sleep(50 * time.Millisecond)

	tmpAttach := filepath.Join(taskDir, ".note.txt.tmp")
	if err := os.WriteFile(tmpAttach, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmpAttach, filepath.Join(taskDir, "note.txt")); err != nil {
		t.Fatal(err)
	}
	taskTmp := taskFile + ".tmp.X"
	if err := os.WriteFile(taskTmp, []byte("# TB-11\n\n## Attachments\n- note.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(taskTmp, taskFile); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	count := countEvents(em, "board:reloaded")
	if count != 1 {
		t.Fatalf("task-root attachment add produced %d board:reloaded events, want 1: %+v", count, em.snapshot())
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

func TestTaskRootAttachmentRemove_FiresOneBoardReloaded(t *testing.T) {
	board := makeBoard(t)
	taskDir := filepath.Join(board, "backlog", "TB-12")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	taskFile := filepath.Join(taskDir, "TASK.md")
	if err := os.WriteFile(taskFile, []byte("# TB-12\n\n## Attachments\n- x.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	attach := filepath.Join(taskDir, "x.txt")
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
	if err := os.WriteFile(taskTmp, []byte("# TB-12\n\n## Attachments\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(taskTmp, taskFile); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	count := countEvents(em, "board:reloaded")
	if count != 1 {
		t.Fatalf("task-root attachment remove produced %d board:reloaded events, want 1: %+v", count, em.snapshot())
	}
}

func TestTaskRootAttachmentRename_FiresOneBoardReloaded(t *testing.T) {
	board := makeBoard(t)
	taskDir := filepath.Join(board, "backlog", "TB-13")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "TASK.md"), []byte("# TB-13\n\n## Attachments\n- draft.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(taskDir, "draft.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)
	time.Sleep(50 * time.Millisecond)

	if err := os.Rename(src, filepath.Join(taskDir, "final.txt")); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	count := countEvents(em, "board:reloaded")
	if count != 1 {
		t.Fatalf("task-root attachment rename produced %d board:reloaded events, want 1: %+v", count, em.snapshot())
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

// TestRealCLITempAndStagingPatterns_AreIgnored exercises the actual on-disk
// names the CLI uses for atomic writes (`.<base>.tmp.<pid>.<token>`, see
// cli/atomicfs.go) and for promotion/attach staging (`.<ID>.promote.<pid>.<token>/`
// and `.attach.<pid>.<token>/`, see cli/attach.go makeHiddenWorkDir). The
// shorter `.tmp.X` names used by other tests rely on the same dot-prefix
// branch in isIgnored; without these tests, a future watcher change that
// loosens dot-prefix handling could let the real CLI patterns through.
func TestRealCLITempAndStagingPatterns_AreIgnored(t *testing.T) {
	board := makeBoard(t)
	taskDir := filepath.Join(board, "backlog", "TB-1")
	if err := os.MkdirAll(filepath.Join(taskDir, "attachments"), 0o755); err != nil {
		t.Fatal(err)
	}
	taskFile := filepath.Join(taskDir, "TASK.md")
	if err := os.WriteFile(taskFile, []byte("# TB-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)
	time.Sleep(50 * time.Millisecond)

	// CLI atomic-write temp pattern: .<base>.tmp.<pid>.<hex>
	cliAtomicTmp := filepath.Join(taskDir, ".TASK.md.tmp.12345.deadbeefcafe")
	if err := os.WriteFile(cliAtomicTmp, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cliAtomicTmp); err != nil {
		t.Fatal(err)
	}

	// CLI promotion staging: .<ID>.promote.<pid>.<hex>/
	promoteStaging := filepath.Join(board, "backlog", ".TB-1.promote.12345.deadbeefcafe")
	if err := os.MkdirAll(promoteStaging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(promoteStaging); err != nil {
		t.Fatal(err)
	}

	// CLI attach staging: .attach.<pid>.<hex>/
	attachStaging := filepath.Join(taskDir, ".attach.12345.deadbeefcafe")
	if err := os.MkdirAll(attachStaging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(attachStaging); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	if count := countEvents(em, "board:reloaded"); count != 0 {
		t.Fatalf("real CLI temp/staging patterns leaked through ignore filter: %d board:reloaded events: %+v", count, em.snapshot())
	}
	if count := countEvents(em, "task:updated:TB-1"); count != 0 {
		t.Fatalf("real CLI temp/staging patterns triggered task:updated: %d events: %+v", count, em.snapshot())
	}
}

func TestTaskRootInternalFiles_AreIgnored(t *testing.T) {
	board := makeBoard(t)
	taskDir := filepath.Join(board, "backlog", "TB-21")
	if err := os.MkdirAll(filepath.Join(taskDir, ".agent-logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "TASK.md"), []byte("# TB-21\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	em := &captureEmitter{}
	startWatcher(t, board, em)
	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(taskDir, ".agent-state.jsonl"), []byte("state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, ".agent-logs", "r_123.log"), []byte("log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, ".attach.12345.deadbeef", "ignored"), nil, 0o644); err == nil {
		t.Fatal("unexpectedly wrote inside missing staging dir")
	}
	staging := filepath.Join(taskDir, ".attach.12345.deadbeef")
	if err := os.Mkdir(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "candidate.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(staging); err != nil {
		t.Fatal(err)
	}

	time.Sleep(400 * time.Millisecond)

	if count := countEvents(em, "board:reloaded"); count != 0 {
		t.Fatalf("internal files leaked through ignore filter: %d board:reloaded events: %+v", count, em.snapshot())
	}
	if count := countEvents(em, "task:updated:TB-21"); count != 0 {
		t.Fatalf("internal files triggered task update: %d events: %+v", count, em.snapshot())
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
