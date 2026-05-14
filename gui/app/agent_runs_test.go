package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"tools/tb-gui/internal/agent"
)

// writeJSONLFixture creates `<boardDir>/.agent-state/<taskID>.jsonl` with
// the given events serialised one per line and returns the boardDir.
func writeJSONLFixture(t *testing.T, taskID string, events []agent.Event) string {
	t.Helper()
	boardDir := t.TempDir()
	stateDir := filepath.Join(boardDir, ".agent-state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(filepath.Join(stateDir, taskID+".jsonl"))
	if err != nil {
		t.Fatalf("create jsonl: %v", err)
	}
	defer f.Close()
	for _, ev := range events {
		b, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return boardDir
}

func writeFolderJSONLFixture(t *testing.T, taskID string, events []agent.Event) string {
	t.Helper()
	boardDir := t.TempDir()
	taskDir := filepath.Join(boardDir, "backlog", taskID)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatalf("mkdir folder task: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "TASK.md"), []byte("# "+taskID+": Folder task\n"), 0o644); err != nil {
		t.Fatalf("write TASK.md: %v", err)
	}
	f, err := os.Create(filepath.Join(taskDir, ".agent-state.jsonl"))
	if err != nil {
		t.Fatalf("create folder jsonl: %v", err)
	}
	defer f.Close()
	for _, ev := range events {
		b, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return boardDir
}

func newSvcForRuns(t *testing.T, boardDir string) *AgentService {
	t.Helper()
	stub := makeStub(t, `:`)
	board := NewBoardService()
	board.setClient(newClient(t, stub))
	board.setBoardDir(boardDir)
	return NewAgentService(AgentServiceOptions{Board: board})
}

func TestListRuns_ThreeRunsSortedDesc(t *testing.T) {
	events := []agent.Event{
		// Oldest run.
		{TS: "2026-05-13T10:00:00Z", RunID: "r_aaaa1111", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: "implement"},
		{TS: "2026-05-13T10:00:01Z", RunID: "r_aaaa1111", TaskID: "TB-1", Event: agent.EvStarted, PID: 100},
		{TS: "2026-05-13T10:01:00Z", RunID: "r_aaaa1111", TaskID: "TB-1", Event: agent.EvFinished, Status: agent.StatusSuccess, ExitCode: 0},
		// Mid run (interleaved with newest).
		{TS: "2026-05-13T11:00:00Z", RunID: "r_bbbb2222", TaskID: "TB-1", Event: agent.EvQueued, Agent: "codex", Mode: "implement"},
		{TS: "2026-05-13T11:00:01Z", RunID: "r_bbbb2222", TaskID: "TB-1", Event: agent.EvStarted, PID: 200},
		// Newest run (started later).
		{TS: "2026-05-13T12:00:00Z", RunID: "r_cccc3333", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: "implement"},
		{TS: "2026-05-13T12:00:01Z", RunID: "r_cccc3333", TaskID: "TB-1", Event: agent.EvStarted, PID: 300},
		{TS: "2026-05-13T12:00:30Z", RunID: "r_cccc3333", TaskID: "TB-1", Event: agent.EvFinished, Status: agent.StatusFailed, ExitCode: 1},
		// Late `finished` for the mid run so the parser sees it after the newest.
		{TS: "2026-05-13T13:00:00Z", RunID: "r_bbbb2222", TaskID: "TB-1", Event: agent.EvFinished, Status: agent.StatusCancelled, ExitCode: -1},
	}
	dir := writeJSONLFixture(t, "TB-1", events)
	svc := newSvcForRuns(t, dir)

	runs, err := svc.ListRuns(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("got %d runs, want 3", len(runs))
	}

	if runs[0].RunID != "r_cccc3333" || runs[0].Status != "failed" || runs[0].ExitCode != 1 {
		t.Errorf("newest: %+v", runs[0])
	}
	if runs[1].RunID != "r_bbbb2222" || runs[1].Status != "cancelled" {
		t.Errorf("mid: %+v", runs[1])
	}
	if runs[2].RunID != "r_aaaa1111" || runs[2].Status != "success" {
		t.Errorf("oldest: %+v", runs[2])
	}

	// LogPath must be derivable without a frontend index.
	for _, r := range runs {
		if r.LogPath == "" {
			t.Errorf("run %s has empty LogPath", r.RunID)
		}
		if r.TaskID != "TB-1" {
			t.Errorf("run %s missing TaskID: %+v", r.RunID, r)
		}
	}
}

func TestListRuns_MissingFileReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	svc := newSvcForRuns(t, dir)
	runs, err := svc.ListRuns(context.Background(), "TB-99")
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if runs == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(runs) != 0 {
		t.Fatalf("got %d runs", len(runs))
	}
}

func TestListRunsAndGetRunLog_FolderTaskLocalArtifacts(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: "r_folder01", TaskID: "TB-2", Event: agent.EvQueued, Agent: "claude", Mode: "implement"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_folder01", TaskID: "TB-2", Event: agent.EvStarted, Agent: "claude", Mode: "implement", PID: 123},
		{TS: "2026-05-14T10:00:02Z", RunID: "r_folder01", TaskID: "TB-2", Event: agent.EvFinished, Agent: "claude", Mode: "implement", Status: agent.StatusSuccess, ExitCode: 0},
	}
	boardDir := writeFolderJSONLFixture(t, "TB-2", events)
	logDir := filepath.Join(boardDir, "backlog", "TB-2", ".agent-logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir log dir: %v", err)
	}
	content := "folder log\n"
	if err := os.WriteFile(filepath.Join(logDir, "r_folder01.log"), []byte(content), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	svc := newSvcForRuns(t, boardDir)
	runs, err := svc.ListRuns(context.Background(), "TB-2")
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("got %d runs, want 1: %+v", len(runs), runs)
	}
	wantLogPath := filepath.Join(logDir, "r_folder01.log")
	if runs[0].LogPath != wantLogPath {
		t.Fatalf("LogPath: got %s, want %s", runs[0].LogPath, wantLogPath)
	}
	got, err := svc.GetRunLog(context.Background(), "TB-2", "r_folder01")
	if err != nil {
		t.Fatalf("GetRunLog: %v", err)
	}
	if got != content {
		t.Fatalf("content: %q, want %q", got, content)
	}
	if _, err := os.Stat(filepath.Join(boardDir, ".agent-state", "TB-2.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("folder task should not read/create board-root state, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, ".agent-logs", "TB-2")); !os.IsNotExist(err) {
		t.Fatalf("folder task should not read/create board-root logs, err=%v", err)
	}
}

func TestListRuns_TolerantOfPartialTrailingLine(t *testing.T) {
	events := []agent.Event{
		{TS: "2026-05-13T10:00:00Z", RunID: "r_full", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: "implement"},
		{TS: "2026-05-13T10:00:01Z", RunID: "r_full", TaskID: "TB-1", Event: agent.EvStarted, PID: 100},
		{TS: "2026-05-13T10:00:30Z", RunID: "r_full", TaskID: "TB-1", Event: agent.EvFinished, Status: agent.StatusSuccess, ExitCode: 0},
	}
	dir := writeJSONLFixture(t, "TB-1", events)
	// Append a half-line that's not valid JSON.
	path := filepath.Join(dir, ".agent-state", "TB-1.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := f.Write([]byte(`{"ts":"2026-05-13T10:01:00Z","run_id":"r_partial","event":"stdout","line":"oo`)); err != nil {
		t.Fatalf("write half: %v", err)
	}
	f.Close()

	svc := newSvcForRuns(t, dir)
	runs, err := svc.ListRuns(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "r_full" {
		t.Fatalf("expected one well-formed run, got %+v", runs)
	}
}

func TestGetRunLog_RoundTrip(t *testing.T) {
	boardDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(boardDir, ".agent-logs", "TB-1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "hello\nworld\n"
	if err := os.WriteFile(filepath.Join(boardDir, ".agent-logs", "TB-1", "r_x.log"), []byte(content), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	svc := newSvcForRuns(t, boardDir)
	got, err := svc.GetRunLog(context.Background(), "TB-1", "r_x")
	if err != nil {
		t.Fatalf("GetRunLog: %v", err)
	}
	if got != content {
		t.Fatalf("content: %q, want %q", got, content)
	}
}

func TestGetRunLog_NotFound(t *testing.T) {
	svc := newSvcForRuns(t, t.TempDir())
	_, err := svc.GetRunLog(context.Background(), "TB-1", "r_missing")
	if !errors.Is(err, ErrRunLogNotFound) {
		t.Fatalf("want ErrRunLogNotFound, got %v", err)
	}
}
