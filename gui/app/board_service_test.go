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

	"tools/tb-gui/internal/agent"
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

func TestLoadBoard_PreservesDoneCompletionOrderFromCLI(t *testing.T) {
	stub := makeStub(t, `
if [ "$1" = "board" ]; then
  echo '{}'
  exit 0
fi
cat <<JSON
[
  {"id":"TB-10","title":"Backlog P0","status":"backlog","priority":"P0","tags":[]},
  {"id":"TB-20","title":"Backlog P2","status":"backlog","priority":"P2","tags":[]},
  {"id":"TB-40","title":"In Progress P0","status":"in-progress","priority":"P0","tags":[]},
  {"id":"TB-50","title":"In Progress P2","status":"in-progress","priority":"P2","tags":[]},
  {"id":"TB-31","title":"Newer done P2","status":"done","priority":"P2","completedAt":"2026-05-12","tags":[]},
  {"id":"TB-30","title":"Older done P0","status":"done","priority":"P0","completedAt":"2026-05-01","tags":[]},
  {"id":"TB-99","title":"Archived closed","status":"archive","priority":"P0","completedAt":"2026-05-13","tags":[]}
]
JSON
`)

	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	snap, err := svc.LoadBoard(context.Background())
	if err != nil {
		t.Fatalf("LoadBoard: %v", err)
	}
	if got := taskIDsFromAppTasks(snap.Backlog); strings.Join(got, ",") != "TB-10,TB-20" {
		t.Fatalf("backlog order = %v, want [TB-10 TB-20]", got)
	}
	if got := taskIDsFromAppTasks(snap.InProgress); strings.Join(got, ",") != "TB-40,TB-50" {
		t.Fatalf("in-progress order = %v, want [TB-40 TB-50]", got)
	}
	if got := taskIDsFromAppTasks(snap.Done); strings.Join(got, ",") != "TB-31,TB-30" {
		t.Fatalf("done order = %v, want newer completion before older higher priority", got)
	}
	if len(snap.Archive) != 0 {
		t.Fatalf("active snapshot should not expose archive tasks: %+v", snap.Archive)
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

func TestGetTask_AgentResumableReflectsLatestSession(t *testing.T) {
	cases := []struct {
		name            string
		folderForm      bool
		agentStatus     string
		writeSession    bool
		wantResumable   bool
		wantStateSuffix string
	}{
		{
			name:            "file form interrupted with captured session",
			agentStatus:     "interrupted",
			writeSession:    true,
			wantResumable:   true,
			wantStateSuffix: ".agent-state/TB-1.jsonl",
		},
		{
			name:            "file form interrupted without captured session",
			agentStatus:     "interrupted",
			wantResumable:   false,
			wantStateSuffix: ".agent-state/TB-1.jsonl",
		},
		{
			name:            "folder form interrupted with captured session",
			folderForm:      true,
			agentStatus:     "interrupted",
			writeSession:    true,
			wantResumable:   true,
			wantStateSuffix: "backlog/TB-1/.agent-state.jsonl",
		},
		{
			name:            "folder form interrupted without captured session",
			folderForm:      true,
			agentStatus:     "interrupted",
			wantResumable:   false,
			wantStateSuffix: "backlog/TB-1/.agent-state.jsonl",
		},
		{
			name:            "success with captured session",
			agentStatus:     "success",
			writeSession:    true,
			wantResumable:   true,
			wantStateSuffix: ".agent-state/TB-1.jsonl",
		},
		{
			name:            "failed with captured session",
			agentStatus:     "failed",
			writeSession:    true,
			wantResumable:   true,
			wantStateSuffix: ".agent-state/TB-1.jsonl",
		},
		{
			name:            "cancelled with captured session",
			agentStatus:     "cancelled",
			writeSession:    true,
			wantResumable:   true,
			wantStateSuffix: ".agent-state/TB-1.jsonl",
		},
		{
			name:            "lost with captured session",
			agentStatus:     "lost",
			writeSession:    true,
			wantResumable:   true,
			wantStateSuffix: ".agent-state/TB-1.jsonl",
		},
		{
			name:            "queued with captured session is not resumable",
			agentStatus:     "queued",
			writeSession:    true,
			wantResumable:   false,
			wantStateSuffix: ".agent-state/TB-1.jsonl",
		},
		{
			name:            "running with captured session is not resumable",
			agentStatus:     "running",
			writeSession:    true,
			wantResumable:   false,
			wantStateSuffix: ".agent-state/TB-1.jsonl",
		},
		{
			name:            "needs-user with captured session is not resumable",
			agentStatus:     "needs-user",
			writeSession:    true,
			wantResumable:   false,
			wantStateSuffix: ".agent-state/TB-1.jsonl",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, boardDir := realTbBoardForRunWithStorage(t, "claude", nil, nil, tc.folderForm)
			c := svc.board.snapshot()
			if tc.writeSession {
				appendGetTaskSessionEvent(t, boardDir)
			}
			if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: tc.agentStatus}); err != nil {
				t.Fatalf("edit AgentStatus: %v", err)
			}

			detail, err := svc.board.GetTask(context.Background(), "TB-1")
			if err != nil {
				t.Fatalf("GetTask: %v", err)
			}
			if detail.Metadata.AgentResumable != tc.wantResumable {
				t.Fatalf("AgentResumable = %v, want %v", detail.Metadata.AgentResumable, tc.wantResumable)
			}

			statePath := agent.StatePath(boardDir, "TB-1")
			if !strings.HasSuffix(filepath.ToSlash(statePath), tc.wantStateSuffix) {
				t.Fatalf("StatePath = %s, want suffix %s", filepath.ToSlash(statePath), tc.wantStateSuffix)
			}
		})
	}
}

func appendGetTaskSessionEvent(t *testing.T, boardDir string) {
	t.Helper()
	events := []agent.Event{
		{
			TS:     "2026-05-19T10:00:00Z",
			RunID:  "r_get_task",
			TaskID: "TB-1",
			Event:  agent.EvQueued,
			Agent:  "claude",
			Mode:   agent.ModeImplement.String(),
		},
		{
			TS:     "2026-05-19T10:00:01Z",
			RunID:  "r_get_task",
			TaskID: "TB-1",
			Event:  agent.EvStarted,
			Agent:  "claude",
			Mode:   agent.ModeImplement.String(),
			PID:    12345,
		},
		{
			TS:        "2026-05-19T10:00:02Z",
			RunID:     "r_get_task",
			TaskID:    "TB-1",
			Event:     agent.EvSession,
			Agent:     "claude",
			Mode:      agent.ModeImplement.String(),
			SessionID: "session-get-task",
		},
	}
	for _, ev := range events {
		if err := agent.AppendEvent(boardDir, "TB-1", ev); err != nil {
			t.Fatalf("append event: %v", err)
		}
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

func TestLoadBoardWithMode_CoalescesConcurrentSameBoardLoads(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "calls.log")
	stub := makeStub(t, fmt.Sprintf(`
printf '%%s\n' "$*" >> %q
sleep 0.1
case "$1" in
  board) echo '{}' ;;
  *) echo '[{"id":"TB-1","title":"A","status":"backlog","tags":[]}]' ;;
esac
`, logPath))
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	start := make(chan struct{})
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := svc.LoadBoardWithMode(context.Background(), "active")
			errs <- err
		}()
	}
	close(start)

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("LoadBoardWithMode: %v", err)
		}
	}

	gotBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	if got := string(gotBytes); got != "ls --json --status active\nboard --json\n" {
		t.Fatalf("stub calls:\n%s", got)
	}
}

func TestLoadBoard_DuplicateCanonicalPathsAreActionable(t *testing.T) {
	first := filepath.Join(t.TempDir(), "board", "backlog", "WS-1486.md")
	second := filepath.Join(t.TempDir(), "board", "done", "WS-1486.md")
	stub := makeStub(t, fmt.Sprintf(`
echo 'error: task WS-1486 resolves to multiple canonical markdown paths in requested status scope: %s and %s' 1>&2
exit 1
`, first, second))
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	_, err := svc.LoadBoard(context.Background())
	if err == nil {
		t.Fatal("LoadBoard should fail")
	}
	msg := err.Error()
	for _, want := range []string{
		"cannot load active board",
		"WS-1486",
		"backlog: " + first,
		"done: " + second,
		"Move or remove one duplicate task file",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
	for _, unwanted := range []string{"tb [ls", "Binding call failed"} {
		if strings.Contains(msg, unwanted) {
			t.Fatalf("error %q should not contain %q", msg, unwanted)
		}
	}
}

func TestLoadBoard_NonDuplicateCLIFailurePreservesExitError(t *testing.T) {
	stub := makeStub(t, `
echo 'error: cannot read status directory /tmp/nope: permission denied' 1>&2
exit 2
`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	_, err := svc.LoadBoard(context.Background())
	var exit *cli.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("want original ExitError, got %T: %v", err, err)
	}
	if exit.Code != 2 {
		t.Fatalf("exit code: got %d want 2", exit.Code)
	}
	if !strings.Contains(err.Error(), "tb [ls --json --status active]: exit 2") {
		t.Fatalf("non-duplicate failure should keep CLI error shape, got %q", err)
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

func taskIDsFromAppTasks(tasks []Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
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

// TestEditTask_ReviewRefForwarded confirms BoardService.EditTask threads the
// ReviewRef field through cli.EditInput and on to `tb edit --review-ref` so
// the GUI's TaskDrawer autosave path can persist the new field (TB-235).
func TestEditTask_ReviewRefForwarded(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	stubPath := filepath.Join(dir, "tb")
	body := `printf "%s\n" "$@" > ` + logPath + `; echo "Updated TB-1: reviewref=feat/x"`
	if err := os.WriteFile(stubPath, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	svc := NewBoardService()
	svc.setClient(newClient(t, stubPath))
	if err := svc.EditTask(context.Background(), "TB-1", EditTaskInput{ReviewRef: "feat/x"}); err != nil {
		t.Fatalf("EditTask: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 4 || lines[0] != "edit" || lines[1] != "TB-1" {
		t.Fatalf("unexpected args:\n%s", string(data))
	}
	if lines[len(lines)-2] != "--review-ref" || lines[len(lines)-1] != "feat/x" {
		t.Fatalf("expected trailing --review-ref feat/x, got:\n%s", string(data))
	}
}

// TestLoadBoard_PropagatesReviewRef confirms the BoardSnapshot Task struct
// exposes reviewRef so the frontend can display and gate UI on it.
func TestLoadBoard_PropagatesReviewRef(t *testing.T) {
	stub := makeStub(t, `cat <<'JSON'
[
  {
    "id": "TB-7",
    "title": "Has ReviewRef",
    "status": "code-review",
    "reviewRef": "feat/x"
  },
  {
    "id": "TB-8",
    "title": "No ReviewRef",
    "status": "backlog"
  }
]
JSON`)
	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	snap, err := svc.LoadBoard(context.Background())
	if err != nil {
		t.Fatalf("LoadBoard: %v", err)
	}
	if len(snap.CodeReview) != 1 || snap.CodeReview[0].ReviewRef != "feat/x" {
		t.Fatalf("code-review reviewRef = %+v", snap.CodeReview)
	}
	if len(snap.Backlog) != 1 || snap.Backlog[0].ReviewRef != "" {
		t.Fatalf("backlog reviewRef expected empty, got %+v", snap.Backlog)
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
