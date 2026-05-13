package agent

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newBoardDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
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
		if got[i] != ev {
			t.Errorf("event %d mismatch:\n got %+v\nwant %+v", i, got[i], ev)
		}
	}
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
