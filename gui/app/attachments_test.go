package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// fakeOpener records every Open call so tests can assert without launching
// processes.
type fakeOpener struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (f *fakeOpener) Open(ctx context.Context, path string) error {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, path)
	return f.err
}

func (f *fakeOpener) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func makeFolderTask(t *testing.T, board, status, id string, attachments map[string]string) string {
	t.Helper()
	taskDir := filepath.Join(board, status, id)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "TASK.md"), []byte("# "+id+": demo\n"), 0o644); err != nil {
		t.Fatalf("write TASK.md: %v", err)
	}
	for name, content := range attachments {
		if err := os.WriteFile(filepath.Join(taskDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write attachment %s: %v", name, err)
		}
	}
	return taskDir
}

func makeLegacyAttachment(t *testing.T, taskDir, name, content string) {
	t.Helper()
	ad := filepath.Join(taskDir, "attachments")
	if err := os.MkdirAll(ad, 0o755); err != nil {
		t.Fatalf("mkdir legacy attachments: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ad, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write legacy attachment %s: %v", name, err)
	}
}

func makeBoardFS(t *testing.T) string {
	t.Helper()
	board := t.TempDir()
	for _, d := range []string{"backlog", "in-progress", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(board, d), 0o755); err != nil {
			t.Fatalf("mkdir status %s: %v", d, err)
		}
	}
	return board
}

func TestListAttachments_NoBoard(t *testing.T) {
	svc := NewBoardService()
	if _, err := svc.ListAttachments(context.Background(), "TB-1"); !errors.Is(err, ErrNoBoard) {
		t.Fatalf("want ErrNoBoard, got %v", err)
	}
}

func TestListAttachments_FolderTask_SortedAndSized(t *testing.T) {
	board := makeBoardFS(t)
	makeFolderTask(t, board, "backlog", "TB-1", map[string]string{
		"zeta.txt":  "zzz",
		"alpha.log": "hello world",
	})

	svc := NewBoardService()
	svc.setBoardDir(board)

	list, err := svc.ListAttachments(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 attachments, got %d", len(list))
	}
	if list[0].Name != "alpha.log" || list[1].Name != "zeta.txt" {
		t.Fatalf("not sorted: %+v", list)
	}
	if list[0].Size != int64(len("hello world")) || list[1].Size != int64(len("zzz")) {
		t.Fatalf("sizes wrong: %+v", list)
	}
}

func TestListAttachments_MixedRootLegacyAndReservedFiles(t *testing.T) {
	board := makeBoardFS(t)
	taskDir := makeFolderTask(t, board, "backlog", "TB-10", map[string]string{
		"root.txt": "root",
	})
	makeLegacyAttachment(t, taskDir, "legacy.txt", "legacy")
	if err := os.WriteFile(filepath.Join(taskDir, ".agent-state.jsonl"), []byte("state"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(taskDir, ".agent-logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, ".hidden.tmp"), []byte("tmp"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(taskDir, "notes-dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := NewBoardService()
	svc.setBoardDir(board)

	list, err := svc.ListAttachments(context.Background(), "TB-10")
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 attachments, got %d: %+v", len(list), list)
	}
	got := []string{list[0].Name, list[1].Name}
	want := []string{"attachments/legacy.txt", "root.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("attachment names = %v, want %v", got, want)
	}
}

func TestListAttachments_FolderTaskNoAttachmentsDir_EmptySlice(t *testing.T) {
	board := makeBoardFS(t)
	makeFolderTask(t, board, "backlog", "TB-2", nil)

	svc := NewBoardService()
	svc.setBoardDir(board)

	list, err := svc.ListAttachments(context.Background(), "TB-2")
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("want empty non-nil slice, got %v", list)
	}
}

func TestListAttachments_LegacyFileTask_EmptySlice(t *testing.T) {
	board := makeBoardFS(t)
	if err := os.WriteFile(filepath.Join(board, "backlog", "TB-3.md"), []byte("# TB-3: legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewBoardService()
	svc.setBoardDir(board)

	list, err := svc.ListAttachments(context.Background(), "TB-3")
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("legacy task should report no attachments, got %+v (err=%v)", list, err)
	}
}

func TestListAttachments_UnknownTask_NotFound(t *testing.T) {
	board := makeBoardFS(t)

	svc := NewBoardService()
	svc.setBoardDir(board)

	_, err := svc.ListAttachments(context.Background(), "TB-99")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestOpenAttachment_LaunchesViaOpener(t *testing.T) {
	board := makeBoardFS(t)
	makeFolderTask(t, board, "in-progress", "TB-4", map[string]string{"notes.txt": "hi"})

	opener := &fakeOpener{}
	svc := NewBoardService()
	svc.setBoardDir(board)
	svc.openFile = opener

	if err := svc.OpenAttachment(context.Background(), "TB-4", "notes.txt"); err != nil {
		t.Fatalf("OpenAttachment: %v", err)
	}
	calls := opener.snapshot()
	if len(calls) != 1 {
		t.Fatalf("want 1 open call, got %d (%v)", len(calls), calls)
	}
	want := filepath.Join(board, "in-progress", "TB-4", "notes.txt")
	if calls[0] != want {
		t.Fatalf("opened wrong path: got %q want %q", calls[0], want)
	}
}

func TestOpenAttachment_LaunchesLegacyAttachmentViaOpener(t *testing.T) {
	board := makeBoardFS(t)
	taskDir := makeFolderTask(t, board, "in-progress", "TB-14", nil)
	makeLegacyAttachment(t, taskDir, "legacy.txt", "hi")

	opener := &fakeOpener{}
	svc := NewBoardService()
	svc.setBoardDir(board)
	svc.openFile = opener

	if err := svc.OpenAttachment(context.Background(), "TB-14", "attachments/legacy.txt"); err != nil {
		t.Fatalf("OpenAttachment: %v", err)
	}
	calls := opener.snapshot()
	if len(calls) != 1 {
		t.Fatalf("want 1 open call, got %d (%v)", len(calls), calls)
	}
	want := filepath.Join(board, "in-progress", "TB-14", "attachments", "legacy.txt")
	if calls[0] != want {
		t.Fatalf("opened wrong path: got %q want %q", calls[0], want)
	}
}

func TestOpenAttachment_RejectsTraversal(t *testing.T) {
	board := makeBoardFS(t)
	makeFolderTask(t, board, "backlog", "TB-5", map[string]string{"x.txt": "hi"})

	opener := &fakeOpener{}
	svc := NewBoardService()
	svc.setBoardDir(board)
	svc.openFile = opener

	bad := []string{"../escape", "../../etc/passwd", "/etc/passwd", "sub/dir.txt", "..\\windows", ""}
	for _, name := range bad {
		if err := svc.OpenAttachment(context.Background(), "TB-5", name); err == nil {
			t.Fatalf("OpenAttachment(%q) should have failed", name)
		}
	}
	if calls := opener.snapshot(); len(calls) != 0 {
		t.Fatalf("opener should not be invoked for invalid names; got %v", calls)
	}
}

func TestOpenAttachment_RejectsRootSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	board := makeBoardFS(t)
	taskDir := makeFolderTask(t, board, "backlog", "TB-6", nil)

	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(taskDir, "evil.txt")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	opener := &fakeOpener{}
	svc := NewBoardService()
	svc.setBoardDir(board)
	svc.openFile = opener

	err := svc.OpenAttachment(context.Background(), "TB-6", "evil.txt")
	if err == nil {
		t.Fatalf("symlink escape should be rejected")
	}
	msg := err.Error()
	if !strings.Contains(msg, "symlink") && !strings.Contains(msg, "outside") {
		t.Fatalf("want symlink/outside rejection, got %v", err)
	}
	if calls := opener.snapshot(); len(calls) != 0 {
		t.Fatalf("opener should not be invoked for symlink; got %v", calls)
	}
}

func TestOpenAttachment_RejectsLegacySymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	board := makeBoardFS(t)
	taskDir := makeFolderTask(t, board, "backlog", "TB-16", nil)
	attachmentsDir := filepath.Join(taskDir, "attachments")
	if err := os.MkdirAll(attachmentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(attachmentsDir, "evil.txt")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	opener := &fakeOpener{}
	svc := NewBoardService()
	svc.setBoardDir(board)
	svc.openFile = opener

	err := svc.OpenAttachment(context.Background(), "TB-16", "attachments/evil.txt")
	if err == nil {
		t.Fatalf("symlink escape should be rejected")
	}
	if calls := opener.snapshot(); len(calls) != 0 {
		t.Fatalf("opener should not be invoked for symlink; got %v", calls)
	}
}

func TestOpenAttachment_RejectsReservedNames(t *testing.T) {
	board := makeBoardFS(t)
	taskDir := makeFolderTask(t, board, "backlog", "TB-17", nil)
	if err := os.WriteFile(filepath.Join(taskDir, ".agent-state.jsonl"), []byte("state"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(taskDir, ".agent-logs"), 0o755); err != nil {
		t.Fatal(err)
	}

	opener := &fakeOpener{}
	svc := NewBoardService()
	svc.setBoardDir(board)
	svc.openFile = opener

	for _, name := range []string{"TASK.md", ".agent-state.jsonl", ".agent-logs", "attachments"} {
		err := svc.OpenAttachment(context.Background(), "TB-17", name)
		if err == nil {
			t.Fatalf("OpenAttachment(%q) succeeded, want reserved-name error", name)
		}
		if !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("OpenAttachment(%q) error = %v, want reserved-name error", name, err)
		}
	}
	if calls := opener.snapshot(); len(calls) != 0 {
		t.Fatalf("opener should not be invoked for reserved names; got %v", calls)
	}
}

func TestAddAttachments_DelegatesToCLI(t *testing.T) {
	board := makeBoardFS(t)
	makeFolderTask(t, board, "backlog", "TB-7", nil)

	calls := filepath.Join(t.TempDir(), "calls.log")
	stub := makeStub(t, "echo \"$@\" >> "+calls+`
echo "Attached 1 file(s) to TB-7: src.txt"`)

	svc := NewBoardService()
	svc.setClient(newClient(t, stub))
	svc.setBoardDir(board)

	src := filepath.Join(t.TempDir(), "src.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := svc.AddAttachments(context.Background(), "TB-7", []string{src}); err != nil {
		t.Fatalf("AddAttachments: %v", err)
	}

	got, err := os.ReadFile(calls)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	if !strings.Contains(string(got), "attach TB-7 -- "+src) {
		t.Fatalf("attach not invoked with expected args, got: %q", string(got))
	}
}

func TestRemoveAttachments_DelegatesToCLI(t *testing.T) {
	board := makeBoardFS(t)
	makeFolderTask(t, board, "backlog", "TB-8", map[string]string{"x.txt": "hi"})

	calls := filepath.Join(t.TempDir(), "calls.log")
	stub := makeStub(t, "echo \"$@\" >> "+calls+`
echo "Removed attachment from TB-8: x.txt"`)

	svc := NewBoardService()
	svc.setClient(newClient(t, stub))
	svc.setBoardDir(board)

	if err := svc.RemoveAttachments(context.Background(), "TB-8", []string{"x.txt"}); err != nil {
		t.Fatalf("RemoveAttachments: %v", err)
	}
	got, err := os.ReadFile(calls)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	if !strings.Contains(string(got), "attach --rm TB-8 -- x.txt") {
		t.Fatalf("remove not invoked with expected args, got: %q", string(got))
	}
}

func TestAddAttachments_PropagatesNoBoardError(t *testing.T) {
	svc := NewBoardService()
	if err := svc.AddAttachments(context.Background(), "TB-1", []string{"x"}); !errors.Is(err, ErrNoBoard) {
		t.Fatalf("want ErrNoBoard, got %v", err)
	}
}

func TestRemoveAttachments_PropagatesNoBoardError(t *testing.T) {
	svc := NewBoardService()
	if err := svc.RemoveAttachments(context.Background(), "TB-1", []string{"x"}); !errors.Is(err, ErrNoBoard) {
		t.Fatalf("want ErrNoBoard, got %v", err)
	}
}
