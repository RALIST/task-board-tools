package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate_HappyPath(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "Created board/backlog/TB-42.md"`)
	c, err := NewClient(Options{BinaryPath: stub})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, err := c.Create(context.Background(), CreateInput{Title: "Hello", Module: "core"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != "TB-42" {
		t.Fatalf("ID: got %q want TB-42", got.ID)
	}
	if got.Path != "board/backlog/TB-42.md" {
		t.Fatalf("Path: got %q", got.Path)
	}
}

func TestCreate_FolderFormPath(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "Created board/backlog/TB-123/TASK.md"`)
	c, err := NewClient(Options{BinaryPath: stub})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, err := c.Create(context.Background(), CreateInput{Title: "Hello", Module: "core"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID != "TB-123" {
		t.Fatalf("ID: got %q want TB-123", got.ID)
	}
	if got.Path != "board/backlog/TB-123/TASK.md" {
		t.Fatalf("Path: got %q", got.Path)
	}
}

// TestCreate_FolderFormPath_RejectsLeadingDashSegment proves the dash-prefix-
// only constraint on idDirRe — a path like "board/backlog/-7/TASK.md" must
// not be parsed as an ID. Without this negative case, a future relaxation of
// the regex (e.g. dropping the `[A-Za-z]` anchor) would silently start
// extracting "-7" as an ID.
func TestCreate_FolderFormPath_RejectsLeadingDashSegment(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "Created board/backlog/-7/TASK.md"`)
	c, err := NewClient(Options{BinaryPath: stub})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = c.Create(context.Background(), CreateInput{Title: "Hello", Module: "core"})
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindUnknown {
		t.Fatalf("want unknown error from unparseable folder path, got %v", err)
	}
}

func TestCreate_MissingTitle(t *testing.T) {
	c, _ := NewClient(Options{BinaryPath: writeStub(t, t.TempDir(), "tb", `echo "Created x"`)})
	_, err := c.Create(context.Background(), CreateInput{Title: ""})
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindValidation {
		t.Fatalf("want validation, got %v", err)
	}
}

func TestCreate_UnparseableStdout(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "boop"`)
	c, _ := NewClient(Options{BinaryPath: stub})
	_, err := c.Create(context.Background(), CreateInput{Title: "x"})
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindUnknown {
		t.Fatalf("want unknown, got %v", err)
	}
}

func TestCreate_BoardNotFoundIsClassified(t *testing.T) {
	// Real CLI stderr: "board not found — run `tb init` to create .tb.yaml"
	stub := writeStub(t, t.TempDir(), "tb", "printf 'board not found — run tb init to create .tb.yaml\\n' 1>&2; exit 1")
	c, _ := NewClient(Options{BinaryPath: stub})
	_, err := c.Create(context.Background(), CreateInput{Title: "x"})
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindBoardNotFound {
		t.Fatalf("want board-not-found, got %v", err)
	}
}

func TestRegenerate_BoardDirMissingNextID(t *testing.T) {
	// Real CLI stderr: "board directory /x does not contain .next-id"
	stub := writeStub(t, t.TempDir(), "tb", "printf 'board directory /x does not contain .next-id — run tb init\\n' 1>&2; exit 1")
	c, _ := NewClient(Options{BinaryPath: stub})
	err := c.Regenerate(context.Background())
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindBoardNotFound {
		t.Fatalf("want board-not-found, got %v (%v)", err, me)
	}
}

func TestEdit_HappyPath(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "Updated TB-1: priority=P0"`)
	c, _ := NewClient(Options{BinaryPath: stub})
	if err := c.Edit(context.Background(), "TB-1", EditInput{Priority: "P0"}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
}

func TestEdit_NoChanges(t *testing.T) {
	c, _ := NewClient(Options{BinaryPath: writeStub(t, t.TempDir(), "tb", `:`)})
	err := c.Edit(context.Background(), "TB-1", EditInput{})
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindValidation {
		t.Fatalf("want validation, got %v", err)
	}
}

func TestEdit_TitleForwarded(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	// The stub records all args on each invocation so we can assert that
	// `--title` was forwarded with the trimmed value.
	stub := writeStub(t, dir, "tb",
		`printf "%s\n" "$@" > `+logPath+`; echo "Updated TB-1: title=New Name"`)
	c, _ := NewClient(Options{BinaryPath: stub})

	if err := c.Edit(context.Background(), "TB-1", EditInput{Title: "  New Name  "}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) < 4 || lines[0] != "edit" || lines[1] != "TB-1" {
		t.Fatalf("unexpected args:\n%s", string(got))
	}
	// Last two args should be "--title" "New Name".
	if lines[len(lines)-2] != "--title" || lines[len(lines)-1] != "New Name" {
		t.Fatalf("expected trailing --title \"New Name\", got:\n%s", string(got))
	}
}

func TestEdit_WhitespaceTitleRejected(t *testing.T) {
	c, _ := NewClient(Options{BinaryPath: writeStub(t, t.TempDir(), "tb", `:`)})
	err := c.Edit(context.Background(), "TB-1", EditInput{Title: "   "})
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindValidation {
		t.Fatalf("want validation, got %v", err)
	}
}

func TestEdit_TitleAloneCountsAsChange(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "Updated TB-1: title=x"`)
	c, _ := NewClient(Options{BinaryPath: stub})
	// HasChanges must consider Title; otherwise EditInput{Title: "x"} would
	// be rejected with "no changes specified" before exec.
	if err := c.Edit(context.Background(), "TB-1", EditInput{Title: "x"}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
}

func TestEdit_TaskNotFound(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "error: task TB-9 not found in any directory (backlog, in-progress, done, archive)" 1>&2; exit 1`)
	c, _ := NewClient(Options{BinaryPath: stub})
	err := c.Edit(context.Background(), "TB-9", EditInput{Priority: "P0"})
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindTaskNotFound {
		t.Fatalf("want task-not-found, got %v", err)
	}
}

func TestEdit_InvalidPriority(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "error: invalid priority \"P9\" — use P0, P1, P2, or P3" 1>&2; exit 1`)
	c, _ := NewClient(Options{BinaryPath: stub})
	err := c.Edit(context.Background(), "TB-1", EditInput{Priority: "P9"})
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindValidation {
		t.Fatalf("want validation, got %v", err)
	}
}

func TestMove_HappyPath(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "Moved TB-1 from backlog to in-progress"`)
	c, _ := NewClient(Options{BinaryPath: stub})
	if err := c.Move(context.Background(), "TB-1", "in-progress"); err != nil {
		t.Fatalf("Move: %v", err)
	}
}

func TestMove_MissingStatus(t *testing.T) {
	c, _ := NewClient(Options{BinaryPath: writeStub(t, t.TempDir(), "tb", `:`)})
	err := c.Move(context.Background(), "TB-1", "")
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindValidation {
		t.Fatalf("want validation, got %v", err)
	}
}

func TestMove_TaskNotFound(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "error: task TB-9 not found in any directory" 1>&2; exit 1`)
	c, _ := NewClient(Options{BinaryPath: stub})
	err := c.Move(context.Background(), "TB-9", "done")
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindTaskNotFound {
		t.Fatalf("want task-not-found, got %v", err)
	}
}

func TestClose_HappyPath(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "Closed TB-1 (archived from done)"`)
	c, _ := NewClient(Options{BinaryPath: stub})
	if err := c.Close(context.Background(), "TB-1"); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestClose_TaskNotFound(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "error: task TB-9 not found in any directory" 1>&2; exit 1`)
	c, _ := NewClient(Options{BinaryPath: stub})
	err := c.Close(context.Background(), "TB-9")
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindTaskNotFound {
		t.Fatalf("want task-not-found, got %v", err)
	}
}

func TestRegenerate_HappyPath(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", `echo "Regenerated BOARD.md"`)
	c, _ := NewClient(Options{BinaryPath: stub})
	if err := c.Regenerate(context.Background()); err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
}

func TestRegenerate_BoardNotFound(t *testing.T) {
	stub := writeStub(t, t.TempDir(), "tb", "printf 'board not found — run tb init to create .tb.yaml\\n' 1>&2; exit 1")
	c, _ := NewClient(Options{BinaryPath: stub})
	err := c.Regenerate(context.Background())
	var me *MutationError
	if !errors.As(err, &me) || me.Kind != ErrKindBoardNotFound {
		t.Fatalf("want board-not-found, got %v", err)
	}
}

func TestIDFromPath(t *testing.T) {
	cases := map[string]string{
		// Legacy file-form
		"board/backlog/TB-42.md":         "TB-42",
		"TB-1.md":                        "TB-1",
		"/abs/path/board/done/PR-100.md": "PR-100",
		"./relative/in-progress/WS-7.md": "WS-7",
		// Folder-form (TB-97: default layout)
		"board/backlog/TB-123/TASK.md":             "TB-123",
		"/abs/path/board/in-progress/PR-7/TASK.md": "PR-7",
		"done/TB-42/TASK.md":                       "TB-42",
		// Negatives
		"weird name.md":              "",
		"no-extension":               "",
		"backlog/TB-42":              "",
		"board/backlog/junk/TASK.md": "",
	}
	for in, want := range cases {
		if got := idFromPath(in); got != want {
			t.Errorf("idFromPath(%q): got %q want %q", in, got, want)
		}
	}
}

func TestMutationError_Message(t *testing.T) {
	e := &MutationError{Kind: ErrKindTaskNotFound, Op: "mv"}
	if !strings.Contains(e.Error(), "task not found") {
		t.Fatalf("unexpected: %q", e.Error())
	}
}
