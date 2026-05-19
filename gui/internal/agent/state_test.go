package agent

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func newBoardDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func writeFileFormTask(t *testing.T, boardDir, status, id string) string {
	t.Helper()
	statusDir := filepath.Join(boardDir, status)
	if err := os.MkdirAll(statusDir, 0o755); err != nil {
		t.Fatalf("mkdir file-form status dir: %v", err)
	}
	path := filepath.Join(statusDir, id+".md")
	if err := os.WriteFile(path, []byte("# "+id+": File task\n"), 0o644); err != nil {
		t.Fatalf("write file-form task: %v", err)
	}
	return path
}

func writeFolderFormTask(t *testing.T, boardDir, status, id string) string {
	t.Helper()
	taskDir := filepath.Join(boardDir, status, id)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir folder-form task dir: %v", err)
	}
	path := filepath.Join(taskDir, folderTaskFileName)
	if err := os.WriteFile(path, []byte("# "+id+": Folder task\n"), 0o644); err != nil {
		t.Fatalf("write folder-form task: %v", err)
	}
	return path
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, got err=%v", path, err)
	}
}

func readJSONL(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer f.Close()
	var events []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("decode line %q: %v", sc.Text(), err)
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanner: %v", err)
	}
	return events
}

func TestAppendEvent_RoundTripsAllShapes(t *testing.T) {
	dir := newBoardDir(t)
	events := []Event{
		{TS: "2026-05-13T10:00:00Z", RunID: "r_aaaa1111", Event: EvQueued, Agent: "claude", Mode: "implement"},
		{TS: "2026-05-13T10:00:05Z", RunID: "r_aaaa1111", Event: EvStarted, PID: 12345},
		{TS: "2026-05-13T10:00:10Z", RunID: "r_aaaa1111", Event: EvStdout, Line: "Hello"},
		{TS: "2026-05-13T10:00:11Z", RunID: "r_aaaa1111", Event: EvStderr, Line: "Warn: x"},
		{TS: "2026-05-13T10:02:30Z", RunID: "r_aaaa1111", Event: EvFinished, Status: StatusSuccess, ExitCode: 0},
	}
	for _, ev := range events {
		if err := AppendEvent(dir, "TB-1", ev); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}

	path := StatePath(dir, "TB-1")
	got := readJSONL(t, path)
	if len(got) != len(events) {
		t.Fatalf("got %d events, want %d", len(got), len(events))
	}
	for i, ev := range events {
		ev.TaskID = "TB-1" // writer fills it in
		// Event carries a map (RunEnv) which prevents direct struct equality.
		if !reflect.DeepEqual(got[i], ev) {
			t.Errorf("event %d mismatch:\n got %+v\nwant %+v", i, got[i], ev)
		}
	}
}

func TestAppendEvent_SessionEventRoundTrip(t *testing.T) {
	dir := newBoardDir(t)
	ev := Event{
		TS:        "2026-05-14T10:00:00Z",
		RunID:     "r_session1",
		Event:     EvSession,
		SessionID: "11111111-2222-3333-4444-555555555555",
		PID:       12345,
		Cwd:       "/tmp/board/worktrees/TB-1",
		RunEnv:    map[string]string{"TB_BOARD_PATH": "/tmp/board"},
	}
	if err := AppendEvent(dir, "TB-1", ev); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	path := StatePath(dir, "TB-1")
	got := readJSONL(t, path)
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	ev.TaskID = "TB-1"
	if !reflect.DeepEqual(got[0], ev) {
		t.Errorf("session event mismatch:\n got %+v\nwant %+v", got[0], ev)
	}
}

func TestFilterTBEnv_KeepsTBPrefixedKeysOnly(t *testing.T) {
	in := []string{
		"TB_BOARD_PATH=/tmp/board",
		"ANTHROPIC_API_KEY=sk-ant-secret",
		"OPENAI_API_KEY=sk-oai-secret",
		"PATH=/usr/local/bin",
		"TB_WORKTREE=/tmp/wt",
		"HOME=/home/u",
		"NOT_AN_ENV", // no `=`, must be skipped
		"=VALUEONLY", // empty key, must be skipped
	}
	got := FilterTBEnv(in)
	want := map[string]string{
		"TB_BOARD_PATH": "/tmp/board",
		"TB_WORKTREE":   "/tmp/wt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterTBEnv mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestFilterTBEnv_NoMatchingKeysReturnsNil(t *testing.T) {
	got := FilterTBEnv([]string{"HOME=/home/u", "PATH=/usr/bin"})
	if got != nil {
		t.Fatalf("FilterTBEnv: expected nil for no matches, got %v", got)
	}
}

// TestGenerateSessionID_CanonicalUUIDv4 locks the format claude expects.
// `claude --session-id` is strict — a malformed UUID is rejected
// silently, which would break resume in a way fake-runner tests can't
// detect. The regex enforces version 4 (third group starts with `4`)
// and the canonical variant (fourth group starts with [89ab]).
func TestGenerateSessionID_CanonicalUUIDv4(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := make(map[string]bool, 64)
	for i := 0; i < 64; i++ {
		id := GenerateSessionID()
		if !re.MatchString(id) {
			t.Fatalf("GenerateSessionID returned non-canonical UUIDv4: %q", id)
		}
		if seen[id] {
			t.Fatalf("GenerateSessionID returned duplicate %q in 64 calls — entropy is broken", id)
		}
		seen[id] = true
	}
}

func TestResolveArtifactPaths_FileAndFolderForms(t *testing.T) {
	dir := newBoardDir(t)
	writeFileFormTask(t, dir, "backlog", "TB-1")
	folderTaskPath := writeFolderFormTask(t, dir, "in-progress", "TB-2")

	filePaths, err := ResolveArtifactPaths(dir, "TB-1")
	if err != nil {
		t.Fatalf("ResolveArtifactPaths file: %v", err)
	}
	if filePaths.Layout != ArtifactLayoutFile {
		t.Fatalf("file layout: got %s, want %s", filePaths.Layout, ArtifactLayoutFile)
	}
	if want := filepath.Join(dir, ".agent-state", "TB-1.jsonl"); filePaths.StatePath != want {
		t.Fatalf("file state path: got %s, want %s", filePaths.StatePath, want)
	}
	if want := filepath.Join(dir, ".agent-logs", "TB-1", "r_file.log"); filePaths.LogPath("r_file") != want {
		t.Fatalf("file log path: got %s, want %s", filePaths.LogPath("r_file"), want)
	}

	folderPaths, err := ResolveArtifactPaths(dir, "TB-2")
	if err != nil {
		t.Fatalf("ResolveArtifactPaths folder: %v", err)
	}
	if folderPaths.Layout != ArtifactLayoutFolder {
		t.Fatalf("folder layout: got %s, want %s", folderPaths.Layout, ArtifactLayoutFolder)
	}
	if want := filepath.Dir(folderTaskPath); folderPaths.TaskDir != want {
		t.Fatalf("folder task dir: got %s, want %s", folderPaths.TaskDir, want)
	}
	if want := filepath.Join(dir, "in-progress", "TB-2", ".agent-state.jsonl"); folderPaths.StatePath != want {
		t.Fatalf("folder state path: got %s, want %s", folderPaths.StatePath, want)
	}
	if want := filepath.Join(dir, "in-progress", "TB-2", ".agent-logs", "r_folder.log"); folderPaths.LogPath("r_folder") != want {
		t.Fatalf("folder log path: got %s, want %s", folderPaths.LogPath("r_folder"), want)
	}
}

func TestResolveArtifactPaths_DualFormPrefersFolderLayout(t *testing.T) {
	dir := newBoardDir(t)
	writeFileFormTask(t, dir, "backlog", "TB-1")
	writeFolderFormTask(t, dir, "backlog", "TB-1")

	paths, err := ResolveArtifactPaths(dir, "TB-1")
	if err != nil {
		t.Fatalf("ResolveArtifactPaths: %v", err)
	}
	if paths.Layout != ArtifactLayoutFolder {
		t.Fatalf("layout: got %s, want %s", paths.Layout, ArtifactLayoutFolder)
	}
	if want := filepath.Join(dir, "backlog", "TB-1", ".agent-state.jsonl"); paths.StatePath != want {
		t.Fatalf("state path: got %s, want %s", paths.StatePath, want)
	}
}

func TestAppendEventAndLogWriter_MixedFormsDoNotCrossWrite(t *testing.T) {
	dir := newBoardDir(t)
	writeFileFormTask(t, dir, "backlog", "TB-1")
	writeFolderFormTask(t, dir, "backlog", "TB-2")

	if err := AppendEvent(dir, "TB-1", Event{TS: "2026-05-14T10:00:00Z", RunID: "r_file", Event: EvQueued}); err != nil {
		t.Fatalf("AppendEvent file: %v", err)
	}
	if err := AppendEvent(dir, "TB-2", Event{TS: "2026-05-14T10:00:00Z", RunID: "r_folder", Event: EvQueued}); err != nil {
		t.Fatalf("AppendEvent folder: %v", err)
	}

	fileLog, err := NewLogWriter(dir, "TB-1", "r_file")
	if err != nil {
		t.Fatalf("NewLogWriter file: %v", err)
	}
	if _, err := fileLog.Write([]byte("file line\n")); err != nil {
		t.Fatalf("write file log: %v", err)
	}
	if err := fileLog.Close(); err != nil {
		t.Fatalf("close file log: %v", err)
	}

	folderLog, err := NewLogWriter(dir, "TB-2", "r_folder")
	if err != nil {
		t.Fatalf("NewLogWriter folder: %v", err)
	}
	if _, err := folderLog.Write([]byte("folder line\n")); err != nil {
		t.Fatalf("write folder log: %v", err)
	}
	if err := folderLog.Close(); err != nil {
		t.Fatalf("close folder log: %v", err)
	}

	assertPathExists(t, filepath.Join(dir, ".agent-state", "TB-1.jsonl"))
	assertPathExists(t, filepath.Join(dir, ".agent-logs", "TB-1", "r_file.log"))
	assertPathMissing(t, filepath.Join(dir, "backlog", "TB-1", ".agent-state.jsonl"))
	assertPathMissing(t, filepath.Join(dir, "backlog", "TB-1", ".agent-logs", "r_file.log"))

	assertPathExists(t, filepath.Join(dir, "backlog", "TB-2", ".agent-state.jsonl"))
	assertPathExists(t, filepath.Join(dir, "backlog", "TB-2", ".agent-logs", "r_folder.log"))
	assertPathMissing(t, filepath.Join(dir, ".agent-state", "TB-2.jsonl"))
	assertPathMissing(t, filepath.Join(dir, ".agent-logs", "TB-2", "r_folder.log"))
}

func TestAppendEvent_MissingBoardDirReturnsTypedError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	err := AppendEvent(missing, "TB-1", Event{Event: EvQueued, RunID: "r_x"})
	if !errors.Is(err, ErrBoardDirMissing) {
		t.Fatalf("want ErrBoardDirMissing, got %v", err)
	}
	// And no file was created.
	if _, err := os.Stat(StatePath(missing, "TB-1")); !os.IsNotExist(err) {
		t.Errorf("expected no file, got %v", err)
	}
}

func TestAppendEvent_ConcurrentAppendsProduceWellFormedLines(t *testing.T) {
	dir := newBoardDir(t)
	const goroutines = 8
	const perGoroutine = 50

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				ev := Event{
					TS:    "2026-05-13T10:00:00Z",
					RunID: fmt.Sprintf("r_%08x", g),
					Event: EvStdout,
					Line:  strings.Repeat("x", 200) + fmt.Sprintf(" g=%d i=%d", g, i),
				}
				if err := AppendEvent(dir, "TB-1", ev); err != nil {
					t.Errorf("AppendEvent: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	events := readJSONL(t, StatePath(dir, "TB-1"))
	if len(events) != goroutines*perGoroutine {
		t.Fatalf("got %d events, want %d", len(events), goroutines*perGoroutine)
	}
	for i, ev := range events {
		if ev.TaskID != "TB-1" || ev.Event != EvStdout {
			t.Errorf("event %d malformed: %+v", i, ev)
		}
	}
}

func TestNewLogWriter_CreatesPerRunFile(t *testing.T) {
	dir := newBoardDir(t)

	w1, err := NewLogWriter(dir, "TB-1", "r_aaaa")
	if err != nil {
		t.Fatalf("NewLogWriter: %v", err)
	}
	if _, err := w1.Write([]byte("line one\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	w2, err := NewLogWriter(dir, "TB-1", "r_bbbb")
	if err != nil {
		t.Fatalf("NewLogWriter run 2: %v", err)
	}
	if _, err := w2.Write([]byte("line two\n")); err != nil {
		t.Fatalf("write run 2: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("close run 2: %v", err)
	}

	one, err := os.ReadFile(LogPath(dir, "TB-1", "r_aaaa"))
	if err != nil {
		t.Fatalf("read run 1: %v", err)
	}
	two, err := os.ReadFile(LogPath(dir, "TB-1", "r_bbbb"))
	if err != nil {
		t.Fatalf("read run 2: %v", err)
	}
	if string(one) != "line one\n" || string(two) != "line two\n" {
		t.Errorf("log files crossed wires: %q / %q", one, two)
	}
}

func TestNewLogWriter_MissingBoardDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	_, err := NewLogWriter(missing, "TB-1", "r_x")
	if !errors.Is(err, ErrBoardDirMissing) {
		t.Fatalf("want ErrBoardDirMissing, got %v", err)
	}
}

func TestGenerateRunID_ShapeAndUniqueness(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		id := GenerateRunID()
		if !strings.HasPrefix(id, "r_") || len(id) != 10 {
			t.Fatalf("malformed run id: %q", id)
		}
		hexPart := id[2:]
		for _, c := range hexPart {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Fatalf("non-hex char in id %q", id)
			}
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate run id %q at i=%d", id, i)
		}
		seen[id] = struct{}{}
	}
}
