package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tools/tb-gui/internal/cli"
)

func makeStub(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell stub")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "tb")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return path
}

func newClient(t *testing.T, stubPath string) *cli.Client {
	t.Helper()
	c, err := cli.NewClient(cli.Options{BinaryPath: stubPath})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestLoadBoard_NoClient(t *testing.T) {
	svc := NewBoardService()
	_, err := svc.LoadBoard(context.Background())
	if !errors.Is(err, ErrNoBoard) {
		t.Fatalf("want ErrNoBoard, got %v", err)
	}
}

func TestLoadBoard_BucketsByStatus(t *testing.T) {
	stub := makeStub(t, `cat <<JSON
[
  {"id":"TB-1","title":"A","status":"backlog","tags":[]},
  {"id":"TB-2","title":"B","status":"in-progress","tags":[]},
  {"id":"TB-3","title":"C","status":"done","tags":[]},
  {"id":"TB-4","title":"D","status":"backlog","tags":[]},
  {"id":"TB-5","title":"E","status":"archive","tags":[]}
]
JSON`)

	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	snap, err := svc.LoadBoard(context.Background())
	if err != nil {
		t.Fatalf("LoadBoard: %v", err)
	}
	if got := len(snap.Backlog); got != 2 {
		t.Fatalf("backlog: got %d want 2", got)
	}
	if got := len(snap.InProgress); got != 1 || snap.InProgress[0].ID != "TB-2" {
		t.Fatalf("in-progress mis-bucketed: %+v", snap.InProgress)
	}
	if got := len(snap.Done); got != 1 || snap.Done[0].ID != "TB-3" {
		t.Fatalf("done mis-bucketed: %+v", snap.Done)
	}
	// Archive must be dropped — the snapshot is active-only.
	for _, t := range append(append(snap.Backlog, snap.InProgress...), snap.Done...) {
		if t.Status == "archive" {
			panic("archive leaked into active snapshot: " + t.ID)
		}
	}
}

func TestLoadBoard_EmptyArray(t *testing.T) {
	stub := makeStub(t, `echo "[]"`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	snap, err := svc.LoadBoard(context.Background())
	if err != nil {
		t.Fatalf("LoadBoard: %v", err)
	}
	// Slices must be non-nil so the JSON encoder emits [] not null on the
	// wire (matches the CLI's same invariant).
	if snap.Backlog == nil || snap.InProgress == nil || snap.Done == nil || snap.Archive == nil {
		t.Fatal("empty board should expose non-nil slices")
	}
	if len(snap.Backlog)+len(snap.InProgress)+len(snap.Done)+len(snap.Archive) != 0 {
		t.Fatal("empty board should have zero tasks")
	}
}

func TestGetTask_NotFound(t *testing.T) {
	stub := makeStub(t, "echo 'error: task TB-999 not found in any directory (backlog, in-progress, done, archive). Verify the ID with `tb ls --status all`' 1>&2; exit 1")
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	_, err := svc.GetTask(context.Background(), "TB-999")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetTask_HappyPath(t *testing.T) {
	stub := makeStub(t, `cat <<JSON
{"metadata":{"id":"TB-1","title":"Hello","status":"backlog","tags":["x"]},"body":"# Body"}
JSON`)

	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	d, err := svc.GetTask(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if d.Metadata.ID != "TB-1" || d.Metadata.Title != "Hello" || d.Body != "# Body" {
		t.Fatalf("decode mismatch: %+v", d)
	}
}

func TestLoadBoardWithMode_AllIncludesArchive(t *testing.T) {
	// Stub asserts the --status arg comes through as `all` and emits archive entries.
	stub := makeStub(t, `
if [ "$4" = "all" ]; then
  cat <<JSON
[
  {"id":"TB-1","title":"A","status":"backlog","tags":[]},
  {"id":"TB-5","title":"E","status":"archive","tags":[]}
]
JSON
else
  echo '[{"id":"TB-1","title":"A","status":"backlog","tags":[]}]'
fi
`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	snap, err := svc.LoadBoardWithMode(context.Background(), "all")
	if err != nil {
		t.Fatalf("LoadBoardWithMode(all): %v", err)
	}
	if len(snap.Archive) != 1 || snap.Archive[0].ID != "TB-5" {
		t.Fatalf("archive bucket: %+v", snap.Archive)
	}
}

func TestLoadBoardWithMode_DefaultsToActive(t *testing.T) {
	stub := makeStub(t, `echo '[{"id":"TB-1","title":"A","status":"backlog","tags":[]}]'`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	snap, err := svc.LoadBoardWithMode(context.Background(), "garbage")
	if err != nil {
		t.Fatalf("LoadBoardWithMode: %v", err)
	}
	if len(snap.Backlog) != 1 {
		t.Fatalf("expected 1 backlog task, got %d", len(snap.Backlog))
	}
}

func TestTriage_ReturnsReasonMapAndCaches(t *testing.T) {
	jsonPath := filepath.Join(t.TempDir(), "triage.json")
	if err := os.WriteFile(jsonPath, []byte(`[{"id":"TB-1","title":"A","reasons":["no goal","no module"]}]`), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}
	stub := makeStub(t, fmt.Sprintf(`cat %q`, jsonPath))
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	first, err := svc.Triage(context.Background())
	if err != nil {
		t.Fatalf("Triage first: %v", err)
	}
	if got := first["TB-1"]; len(got) != 2 || got[0] != "no goal" || got[1] != "no module" {
		t.Fatalf("triage map: %#v", first)
	}

	if err := os.WriteFile(jsonPath, []byte(`[{"id":"TB-2","title":"B","reasons":["no acceptance criteria"]}]`), 0o644); err != nil {
		t.Fatalf("rewrite json: %v", err)
	}
	second, err := svc.Triage(context.Background())
	if err != nil {
		t.Fatalf("Triage cached: %v", err)
	}
	if _, ok := second["TB-2"]; ok {
		t.Fatalf("cache was not used: %#v", second)
	}
}

func TestTriage_InvalidJSONFromZeroExitReturnsEmptyMap(t *testing.T) {
	stub := makeStub(t, `
if [ "$1" = "triage" ] && [ "$2" = "--json" ]; then
  echo "Found 1 task needing grooming"
  exit 0
fi
echo "unexpected args: $*" 1>&2
exit 1
`)
	var logs bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	got, err := svc.Triage(context.Background())
	if err != nil {
		t.Fatalf("Triage should resolve legacy human output, got error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("legacy human output should resolve to an empty triage map: %#v", got)
	}

	logText := logs.String()
	if !strings.Contains(logText, stub) {
		t.Fatalf("diagnostic should include active tb binary path %q; log=%q", stub, logText)
	}
	if !strings.Contains(logText, "selected CLI must support triage --json") {
		t.Fatalf("diagnostic should tell users the selected CLI must support triage --json; log=%q", logText)
	}
}

func TestBoardWatcherSink_InvalidatesTriageWithoutRefreshing(t *testing.T) {
	jsonPath := filepath.Join(t.TempDir(), "triage.json")
	if err := os.WriteFile(jsonPath, []byte(`[{"id":"TB-1","reasons":["no goal"]}]`), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}
	callsPath := filepath.Join(t.TempDir(), "calls.log")
	stub := makeStub(t, fmt.Sprintf(`
printf 'triage\n' >> %q
cat %q
`, callsPath, jsonPath))
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))
	sink := NewBoardWatcherSink(svc)

	initial, err := svc.Triage(context.Background())
	if err != nil {
		t.Fatalf("Triage initial: %v", err)
	}
	if !containsTriageID(initial, "TB-1") {
		t.Fatalf("initial triage missing TB-1: %#v", initial)
	}
	assertTriageCalls(t, callsPath, 1)

	if err := os.WriteFile(jsonPath, []byte(`[]`), 0o644); err != nil {
		t.Fatalf("rewrite json: %v", err)
	}
	sink.Emit("task:updated:TB-1", "TB-1")
	assertTriageCalls(t, callsPath, 1)

	updated, err := svc.Triage(context.Background())
	if err != nil {
		t.Fatalf("Triage after task update: %v", err)
	}
	if containsTriageID(updated, "TB-1") {
		t.Fatalf("task update did not invalidate cache: %#v", updated)
	}
	assertTriageCalls(t, callsPath, 2)

	if err := os.WriteFile(jsonPath, []byte(`[{"id":"TB-2","reasons":["no module"]}]`), 0o644); err != nil {
		t.Fatalf("rewrite json: %v", err)
	}
	sink.Emit("board:reloaded")
	assertTriageCalls(t, callsPath, 2)

	reloaded, err := svc.Triage(context.Background())
	if err != nil {
		t.Fatalf("Triage after board reload: %v", err)
	}
	if !containsTriageID(reloaded, "TB-2") {
		t.Fatalf("board reload did not invalidate cache: %#v", reloaded)
	}
	assertTriageCalls(t, callsPath, 3)
}

func containsTriageID(m map[string][]string, id string) bool {
	_, ok := m[id]
	return ok
}

func assertTriageCalls(t *testing.T, path string, want int) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read calls: %v", err)
	}
	if got := strings.Count(string(raw), "triage\n"); got != want {
		t.Fatalf("triage calls: got %d want %d; log=%q", got, want, raw)
	}
}

func TestCreateTask_HappyPath(t *testing.T) {
	stub := makeStub(t, `echo "Created board/backlog/TB-42.md"`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	res, err := svc.CreateTask(context.Background(), CreateTaskInput{Title: "Hello"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if res.ID != "TB-42" {
		t.Fatalf("ID: %q", res.ID)
	}
}

func TestCreateTask_NoBoard(t *testing.T) {
	svc := NewBoardService()
	_, err := svc.CreateTask(context.Background(), CreateTaskInput{Title: "x"})
	if !errors.Is(err, ErrNoBoard) {
		t.Fatalf("want ErrNoBoard, got %v", err)
	}
}

func TestEditTask_HappyPath(t *testing.T) {
	stub := makeStub(t, `echo "Updated TB-1"`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))
	if err := svc.EditTask(context.Background(), "TB-1", EditTaskInput{Priority: "P0"}); err != nil {
		t.Fatalf("EditTask: %v", err)
	}
}

func TestMoveTask_HappyPath(t *testing.T) {
	stub := makeStub(t, `echo "Moved TB-1"`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))
	if err := svc.MoveTask(context.Background(), "TB-1", "done"); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
}

func TestCloseTask_HappyPath(t *testing.T) {
	stub := makeStub(t, `echo "Closed TB-1"`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))
	if err := svc.CloseTask(context.Background(), "TB-1"); err != nil {
		t.Fatalf("CloseTask: %v", err)
	}
}

func TestRegenerate_HappyPath(t *testing.T) {
	stub := makeStub(t, `echo "Regenerated"`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))
	if err := svc.Regenerate(context.Background()); err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
}

func Test_setClient_NilClearsBoard(t *testing.T) {
	stub := makeStub(t, `echo "[]"`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))
	if _, err := svc.LoadBoard(context.Background()); err != nil {
		t.Fatalf("LoadBoard ok: %v", err)
	}
	svc.setClient(nil)
	if _, err := svc.LoadBoard(context.Background()); !errors.Is(err, ErrNoBoard) {
		t.Fatalf("want ErrNoBoard after clear, got %v", err)
	}
}
