package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"tools/tb-gui/internal/cli"
)

const sampleTaskBody = `# TB-1: Sample title

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** core
**Tags:** quick-win
**Branch:** —

## Goal

Old goal text.

## Acceptance Criteria

- [ ] one
- [ ] two

## Log

- 2026-05-13: Created
`

// writeBoardFixture sets up a board layout with one task at <board>/<status>/<id>.md.
func writeBoardFixture(t *testing.T, status, id, body string) (boardDir, taskPath string) {
	t.Helper()
	boardDir = t.TempDir()
	for _, d := range []string{"backlog", "in-progress", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(boardDir, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	taskPath = filepath.Join(boardDir, status, id+".md")
	if err := os.WriteFile(taskPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return boardDir, taskPath
}

// writeBoardFolderFixture sets up a folder-form task at <board>/<status>/<id>/TASK.md.
func writeBoardFolderFixture(t *testing.T, status, id, body string) (boardDir, taskPath string) {
	t.Helper()
	boardDir = t.TempDir()
	for _, d := range []string{"backlog", "in-progress", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(boardDir, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	taskDir := filepath.Join(boardDir, status, id)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir task folder: %v", err)
	}
	taskPath = filepath.Join(taskDir, "TASK.md")
	if err := os.WriteFile(taskPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return boardDir, taskPath
}

func newServiceForBody(t *testing.T, boardDir string) *BoardService {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX flock only")
	}
	stub := makeStub(t, `:`) // regenerate stub: no-op success
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))
	svc.setBoardDir(boardDir)
	return svc
}

func TestEditTaskBody_CleanEdit(t *testing.T) {
	boardDir, taskPath := writeBoardFixture(t, "backlog", "TB-1", sampleTaskBody)
	svc := newServiceForBody(t, boardDir)

	newBody := strings.Replace(sampleTaskBody, "Old goal text.", "Fresh goal text.", 1)
	if err := svc.EditTaskBody(context.Background(), "TB-1", newBody); err != nil {
		t.Fatalf("EditTaskBody: %v", err)
	}

	got, _ := os.ReadFile(taskPath)
	if !strings.Contains(string(got), "Fresh goal text.") {
		t.Fatalf("body not updated: %s", got)
	}
	if !strings.Contains(string(got), ": Edited body via GUI") {
		t.Fatalf("log entry missing: %s", got)
	}
}

func TestEditTaskBody_RejectsHeaderMutation(t *testing.T) {
	boardDir, _ := writeBoardFixture(t, "backlog", "TB-1", sampleTaskBody)
	svc := newServiceForBody(t, boardDir)

	tampered := strings.Replace(sampleTaskBody, "# TB-1: Sample title", "# TB-1: New title", 1)
	err := svc.EditTaskBody(context.Background(), "TB-1", tampered)
	if !errors.Is(err, ErrHeaderMutation) {
		t.Fatalf("want ErrHeaderMutation, got %v", err)
	}
}

func TestEditTaskBody_RejectsMetadataMutation(t *testing.T) {
	boardDir, _ := writeBoardFixture(t, "backlog", "TB-1", sampleTaskBody)
	svc := newServiceForBody(t, boardDir)

	tampered := strings.Replace(sampleTaskBody, "**Priority:** P1", "**Priority:** P0", 1)
	err := svc.EditTaskBody(context.Background(), "TB-1", tampered)
	if !errors.Is(err, ErrHeaderMutation) {
		t.Fatalf("want ErrHeaderMutation, got %v", err)
	}
}

func TestEditTaskBody_AppendsLogEntry(t *testing.T) {
	boardDir, taskPath := writeBoardFixture(t, "in-progress", "TB-7", sampleTaskBody)
	svc := newServiceForBody(t, boardDir)

	newBody := strings.Replace(sampleTaskBody, "Old goal text.", "Updated body.\n\nMore detail.", 1)
	if err := svc.EditTaskBody(context.Background(), "TB-7", newBody); err != nil {
		t.Fatalf("EditTaskBody: %v", err)
	}

	got, _ := os.ReadFile(taskPath)
	today := time.Now().Format("2006-01-02")
	want := "- " + today + ": Edited body via GUI"
	if !strings.Contains(string(got), want) {
		t.Fatalf("log entry %q not found:\n%s", want, got)
	}
}

func TestEditTaskBody_NoBoard(t *testing.T) {
	svc := NewBoardService()
	err := svc.EditTaskBody(context.Background(), "TB-1", "anything")
	if !errors.Is(err, ErrNoBoard) {
		t.Fatalf("want ErrNoBoard, got %v", err)
	}
}

func TestEditTaskBody_FolderForm(t *testing.T) {
	boardDir, taskPath := writeBoardFolderFixture(t, "in-progress", "TB-124", sampleTaskBody)
	svc := newServiceForBody(t, boardDir)

	newBody := strings.Replace(sampleTaskBody, "Old goal text.", "Folder-form body edit.", 1)
	if err := svc.EditTaskBody(context.Background(), "TB-124", newBody); err != nil {
		t.Fatalf("EditTaskBody: %v", err)
	}

	got, _ := os.ReadFile(taskPath)
	if !strings.Contains(string(got), "Folder-form body edit.") {
		t.Fatalf("body not updated: %s", got)
	}
	if !strings.Contains(string(got), ": Edited body via GUI") {
		t.Fatalf("log entry missing: %s", got)
	}
}

func TestEditTaskBody_FolderFormPreferredOverFileForm(t *testing.T) {
	boardDir, folderPath := writeBoardFolderFixture(t, "in-progress", "TB-124", sampleTaskBody)
	siblingFile := filepath.Join(boardDir, "in-progress", "TB-124.md")
	if err := os.WriteFile(siblingFile, []byte("# TB-124: stale legacy\n\n**Type:** bug\n\n## Goal\n\nstale\n"), 0o644); err != nil {
		t.Fatalf("write sibling: %v", err)
	}
	svc := newServiceForBody(t, boardDir)

	newBody := strings.Replace(sampleTaskBody, "Old goal text.", "Picked folder form.", 1)
	if err := svc.EditTaskBody(context.Background(), "TB-124", newBody); err != nil {
		t.Fatalf("EditTaskBody: %v", err)
	}
	got, _ := os.ReadFile(folderPath)
	if !strings.Contains(string(got), "Picked folder form.") {
		t.Fatalf("folder-form body not updated: %s", got)
	}
	stale, _ := os.ReadFile(siblingFile)
	if strings.Contains(string(stale), "Picked folder form.") {
		t.Fatalf("legacy file form was written to; should have been left alone:\n%s", stale)
	}
}

func TestEditTaskBody_FolderFormBareNumber(t *testing.T) {
	boardDir, taskPath := writeBoardFolderFixture(t, "in-progress", "TB-124", sampleTaskBody)
	svc := newServiceForBody(t, boardDir)

	newBody := strings.Replace(sampleTaskBody, "Old goal text.", "Bare-id folder edit.", 1)
	if err := svc.EditTaskBody(context.Background(), "124", newBody); err != nil {
		t.Fatalf("EditTaskBody bare id: %v", err)
	}
	got, _ := os.ReadFile(taskPath)
	if !strings.Contains(string(got), "Bare-id folder edit.") {
		t.Fatalf("body not updated: %s", got)
	}
}

func TestEditTaskBody_TaskNotFound(t *testing.T) {
	boardDir, _ := writeBoardFixture(t, "backlog", "TB-1", sampleTaskBody)
	svc := newServiceForBody(t, boardDir)
	err := svc.EditTaskBody(context.Background(), "TB-99", "anything")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestEditTaskBody_AtomicRename(t *testing.T) {
	// After a successful edit, no `.tmp` siblings should remain next to the task.
	boardDir, taskPath := writeBoardFixture(t, "backlog", "TB-1", sampleTaskBody)
	svc := newServiceForBody(t, boardDir)

	newBody := strings.Replace(sampleTaskBody, "Old goal text.", "New.", 1)
	if err := svc.EditTaskBody(context.Background(), "TB-1", newBody); err != nil {
		t.Fatalf("EditTaskBody: %v", err)
	}

	siblings, _ := os.ReadDir(filepath.Dir(taskPath))
	for _, s := range siblings {
		if strings.Contains(s.Name(), ".tmp.") {
			t.Fatalf("leftover temp file: %s", s.Name())
		}
	}
}

func TestAppendBodyEditLog_NoLogSection(t *testing.T) {
	src := `# TB-1: x

**Type:** bug

## Goal

hi
`
	out := appendBodyEditLog(src, time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(out, "## Log") || !strings.Contains(out, "2026-05-13: Edited body via GUI") {
		t.Fatalf("missing log section:\n%s", out)
	}
}

// --- locking integration: prove we hold .board.lock under POSIX flock ---

func TestEditTaskBody_HoldsBoardLock_VsRealTbBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX flock only")
	}
	tbBinary := buildTbForIntegration(t)

	// Build a real board with a .tb.yaml so `tb edit` finds the config.
	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	if err := os.MkdirAll(filepath.Join(boardDir, "backlog"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte("board: board\nprefix: TB\n"), 0o644); err != nil {
		t.Fatalf("write .tb.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("2\n"), 0o644); err != nil {
		t.Fatalf("write .next-id: %v", err)
	}
	taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	if err := os.WriteFile(taskPath, []byte(sampleTaskBody), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	c, err := cli.NewClient(cli.Options{BinaryPath: tbBinary, Cwd: root})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	svc := NewBoardService()
	svc.setClient(c)
	svc.setBoardDir(boardDir)

	// Acquire the lock manually outside EditTaskBody so we can prove a real
	// `tb edit` child process blocks. (EditTaskBody acquires/releases too
	// fast to race deterministically.)
	lock, err := lockBoard(boardDir)
	if err != nil {
		t.Fatalf("lockBoard: %v", err)
	}
	releaseTriggered := make(chan struct{})

	// Run `tb edit` as a child; it should block on flock until we unlock.
	editDone := make(chan error, 1)
	go func() {
		err := c.Edit(context.Background(), "TB-1", cli.EditInput{Priority: "P0"})
		editDone <- err
	}()

	// Give the child enough time to actually request the lock.
	select {
	case err := <-editDone:
		lock.unlock()
		t.Fatalf("tb edit returned %v before we released the lock — flock not actually held", err)
	case <-time.After(300 * time.Millisecond):
		// Expected: child still blocked.
	}

	close(releaseTriggered)
	lock.unlock()

	select {
	case err := <-editDone:
		if err != nil {
			t.Fatalf("tb edit after unlock: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tb edit never completed after lock release")
	}

	// End-to-end EditTaskBody round-trip against the live `tb` binary —
	// proves the protected prefix (header + metadata block) survives a body
	// edit byte-for-byte. AC: TB-37 "title line and metadata block are
	// byte-identical to before the edit; verified end-to-end against the
	// live `tb` binary".
	beforeBytes, _ := os.ReadFile(taskPath)
	beforeHeader := protectedPrefix(string(beforeBytes))

	mutated := strings.Replace(string(beforeBytes), "Old goal text.", "Locked-write OK.", 1)
	if err := svc.EditTaskBody(context.Background(), "TB-1", mutated); err != nil {
		t.Fatalf("EditTaskBody after lock release: %v", err)
	}
	afterBytes, _ := os.ReadFile(taskPath)
	if !strings.Contains(string(afterBytes), "Locked-write OK.") {
		t.Fatalf("EditTaskBody body change missing:\n%s", afterBytes)
	}
	afterHeader := protectedPrefix(string(afterBytes))
	if beforeHeader != afterHeader {
		t.Fatalf("protected prefix changed across EditTaskBody:\n--- before ---\n%s\n--- after ---\n%s", beforeHeader, afterHeader)
	}

	// Also verify the live `tb show --json` reports the same metadata it had
	// before — this is the real-binary read-back the AC asks for.
	var detail TaskDetail
	if err := c.RunJSON(context.Background(), &detail, "show", "TB-1", "--json"); err != nil {
		t.Fatalf("tb show --json: %v", err)
	}
	if detail.Metadata.Title != "Sample title" {
		t.Fatalf("title mutated: %q", detail.Metadata.Title)
	}
	if detail.Metadata.Priority != "P0" {
		t.Fatalf("priority mutated: %q (want P0)", detail.Metadata.Priority)
	}
}

// buildTbForIntegration builds the CLI binary into a temp dir and returns its
// path. Mirrors gui/internal/watcher/integration_test.go's approach.
func buildTbForIntegration(t *testing.T) string {
	t.Helper()
	cliDir, err := filepath.Abs("../../cli")
	if err != nil {
		t.Fatalf("abs cli dir: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "tb")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = cliDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build cli: %v\n%s", err, out)
	}
	return bin
}
