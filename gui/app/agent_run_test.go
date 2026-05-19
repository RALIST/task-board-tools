package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

// stubRunner is a configurable Runner that emits whatever lines / exit code
// the test asked for. The OnStarted callback is invoked synchronously so we
// hit the same code paths the real runners do.
type stubRunner struct {
	name        string
	stdoutLines []string
	stderrLines []string
	exitCode    int
	runErr      error
	startedOnce bool
	startedDone chan struct{}
	// fireSessionID, if non-empty, is delivered to in.OnSessionID after
	// OnStarted but before stdout streaming. Simulates the Codex
	// translator's mid-stream session-id capture without depending on
	// real codex --json output.
	fireSessionID string
	lastInput     agent.RunInput
	mu            sync.Mutex
}

func (s *stubRunner) Name() string { return s.name }

func (s *stubRunner) Run(ctx context.Context, in agent.RunInput) (agent.RunResult, error) {
	s.mu.Lock()
	s.lastInput = in
	s.mu.Unlock()
	if in.OnStarted != nil {
		s.mu.Lock()
		s.startedOnce = true
		s.mu.Unlock()
		in.OnStarted(99999, 99999)
		if s.startedDone != nil {
			close(s.startedDone)
		}
	}
	if s.fireSessionID != "" && in.OnSessionID != nil {
		in.OnSessionID(s.fireSessionID)
	}
	for _, ln := range s.stdoutLines {
		_, _ = in.Stdout.Write([]byte(ln + "\n"))
	}
	for _, ln := range s.stderrLines {
		_, _ = in.Stderr.Write([]byte(ln + "\n"))
	}
	return agent.RunResult{ExitCode: s.exitCode, Err: s.runErr}, s.runErr
}

func (s *stubRunner) input() agent.RunInput {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastInput
}

// realTbBoardForRun builds a real `tb` board with one assigned task in
// backlog. Returns the service ready to RunAgent against. The runner factory
// is swapped to a stub so the agent process never actually spawns.
func realTbBoardForRun(t *testing.T, agentName string, stub *stubRunner) (*AgentService, string) {
	return realTbBoardForRunWithOptions(t, agentName, stub, nil)
}

func realTbFolderBoardForRun(t *testing.T, agentName string, stub *stubRunner) (*AgentService, string) {
	return realTbBoardForRunWithStorage(t, agentName, stub, nil, true)
}

func realTbMixedBoardForRun(t *testing.T, agentName string, stub *stubRunner) (*AgentService, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only board flock")
	}
	tbBinary := buildTbForIntegration(t)

	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, d := range []string{"backlog", "in-progress", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(boardDir, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte("board: board\nprefix: TB\n"), 0o644); err != nil {
		t.Fatalf(".tb.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("3\n"), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}

	fileTaskBody := agentTaskBodyForTest("TB-1", agentName)
	if err := os.WriteFile(filepath.Join(boardDir, "backlog", "TB-1.md"), []byte(fileTaskBody), 0o644); err != nil {
		t.Fatalf("file task md: %v", err)
	}
	folderTaskDir := filepath.Join(boardDir, "backlog", "TB-2")
	if err := os.MkdirAll(folderTaskDir, 0o755); err != nil {
		t.Fatalf("folder task dir: %v", err)
	}
	folderTaskBody := agentTaskBodyForTest("TB-2", agentName)
	if err := os.WriteFile(filepath.Join(folderTaskDir, "TASK.md"), []byte(folderTaskBody), 0o644); err != nil {
		t.Fatalf("folder task md: %v", err)
	}

	c, err := cli.NewClient(cli.Options{BinaryPath: tbBinary, Cwd: root})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	board := NewBoardService()
	board.setClient(c)
	board.setBoardDir(boardDir)
	em := newRecordingEmitter()
	svc := NewAgentService(AgentServiceOptions{Board: board, Emitter: em})
	if stub != nil {
		svc.setRunnerFactory(func(name string) (agent.Runner, error) {
			return stub, nil
		})
	}
	t.Cleanup(func() {
		svc.mu.Lock()
		for id, ar := range svc.active {
			ar.Cancel()
			<-ar.Done
			_ = id
		}
		svc.mu.Unlock()
	})
	return svc, boardDir
}

func realTbBoardForRunWithOptions(t *testing.T, agentName string, stub *stubRunner, configure func(*AgentServiceOptions)) (*AgentService, string) {
	return realTbBoardForRunWithStorage(t, agentName, stub, configure, false)
}

func realTbBoardForRunWithStorage(t *testing.T, agentName string, stub *stubRunner, configure func(*AgentServiceOptions), folderForm bool) (*AgentService, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only board flock")
	}
	tbBinary := buildTbForIntegration(t)

	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, d := range []string{"backlog", "in-progress", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(boardDir, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte("board: board\nprefix: TB\n"), 0o644); err != nil {
		t.Fatalf(".tb.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("2\n"), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}
	taskBody := agentTaskBodyForTest("TB-1", agentName)
	if folderForm {
		taskDir := filepath.Join(boardDir, "backlog", "TB-1")
		if err := os.MkdirAll(taskDir, 0o755); err != nil {
			t.Fatalf("task dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(taskDir, "TASK.md"), []byte(taskBody), 0o644); err != nil {
			t.Fatalf("task md: %v", err)
		}
	} else {
		if err := os.WriteFile(filepath.Join(boardDir, "backlog", "TB-1.md"), []byte(taskBody), 0o644); err != nil {
			t.Fatalf("task md: %v", err)
		}
	}

	c, err := cli.NewClient(cli.Options{BinaryPath: tbBinary, Cwd: root})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	board := NewBoardService()
	board.setClient(c)
	board.setBoardDir(boardDir)
	em := newRecordingEmitter()
	opts := AgentServiceOptions{Board: board, Emitter: em}
	if configure != nil {
		configure(&opts)
	}
	svc := NewAgentService(opts)
	if stub != nil {
		svc.setRunnerFactory(func(name string) (agent.Runner, error) {
			return stub, nil
		})
	}
	t.Cleanup(func() {
		// Make sure no leaked goroutine writes to test-scoped state.
		svc.mu.Lock()
		for id, ar := range svc.active {
			ar.Cancel()
			<-ar.Done
			_ = id
		}
		svc.mu.Unlock()
	})
	return svc, boardDir
}

func writeAppStubScript(t *testing.T, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only stub")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return dir
}

func prependPATHForTest(t *testing.T, dir string) {
	t.Helper()
	orig := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", orig) })
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+orig); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
}

func killAppPIDFromFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	pid := atoi(strings.TrimSpace(string(data)))
	if pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func agentTaskBodyForTest(id, agentName string) string {
	body := strings.Replace(sampleTaskBody, "# TB-1: Sample title", "# "+id+": Sample title", 1)
	return strings.Replace(body, "**Branch:** —", "**Branch:** —\n**Agent:** "+agentName, 1)
}

func assertAppPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be missing, got err=%v", path, err)
	}
}

func waitForRunCompletion(t *testing.T, svc *AgentService, id string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		ar, ok := svc.active[id]
		svc.mu.Unlock()
		if !ok {
			return
		}
		select {
		case <-ar.Done:
			return
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatalf("RunAgent for %s did not complete within %v", id, timeout)
}

func readEvents(t *testing.T, boardDir, taskID string) []agent.Event {
	t.Helper()
	f, err := os.Open(agent.StatePath(boardDir, taskID))
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer f.Close()
	var out []agent.Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev agent.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("decode: %v line=%q", err, sc.Text())
		}
		out = append(out, ev)
	}
	return out
}

func TestRunAgent_HappyPath_Success(t *testing.T) {
	stub := &stubRunner{
		name: "claude",
		stdoutLines: []string{
			"line-1", "line-2", "line-3", "line-4", "line-5",
			"line-6", "line-7", "line-8", "line-9", "line-10",
		},
		stderrLines: []string{"warn-1", "warn-2"},
		exitCode:    0,
	}
	svc, boardDir := realTbBoardForRun(t, "claude", stub)

	runID, err := svc.RunAgent(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if !strings.HasPrefix(runID, "r_") || len(runID) != 10 {
		t.Errorf("malformed runID: %q", runID)
	}

	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	events := readEvents(t, boardDir, "TB-1")
	// queued + started + session + 10 stdout + 2 stderr + finished = 16
	// (TB-130: session event lands after started for Claude pre-alloc.)
	if len(events) != 16 {
		t.Fatalf("event count: %d; events=%+v", len(events), events)
	}
	if events[0].Event != agent.EvQueued {
		t.Errorf("first event not queued: %+v", events[0])
	}
	if events[1].Event != agent.EvStarted {
		t.Errorf("second event not started: %+v", events[1])
	}
	if events[2].Event != agent.EvSession || events[2].SessionID == "" {
		t.Errorf("third event not session with id: %+v", events[2])
	}
	last := events[len(events)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusSuccess || last.ExitCode != 0 {
		t.Errorf("final event: %+v", last)
	}

	// Log file content matches the 12 streamed lines (in order).
	logBytes, err := os.ReadFile(agent.LogPath(boardDir, "TB-1", runID))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	logText := string(logBytes)
	for _, ln := range stub.stdoutLines {
		if !strings.Contains(logText, ln) {
			t.Errorf("log missing line %q", ln)
		}
	}
	for _, ln := range stub.stderrLines {
		if !strings.Contains(logText, ln) {
			t.Errorf("log missing stderr line %q", ln)
		}
	}

	// AgentStatus on disk reads as success.
	taskBytes, _ := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
	if !strings.Contains(string(taskBytes), "**AgentStatus:** success") {
		t.Errorf("AgentStatus not success:\n%s", taskBytes)
	}
}

func TestRunAgent_ExternalRunnerWithInheritedPipesFinishes(t *testing.T) {
	childPIDFile := filepath.Join(t.TempDir(), "child.pid")
	dir := writeAppStubScript(t, "claude", `
( trap "" HUP; exec sleep 30 ) &
printf '%s\n' "$!" > `+childPIDFile+`
echo parent-done
exit 0
	`)
	prependPATHForTest(t, dir)
	t.Cleanup(func() { killAppPIDFromFile(t, childPIDFile) })

	svc, boardDir := realTbBoardForRun(t, "claude", nil)

	start := time.Now()
	runID, err := svc.RunAgent(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("RunAgent completed after %v; want bounded completion after parent exit", elapsed)
	}

	events := readEvents(t, boardDir, "TB-1")
	last := events[len(events)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusSuccess || last.ExitCode != 0 {
		t.Fatalf("final event: %+v, want finished{success}", last)
	}
	taskBytes, _ := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
	if !strings.Contains(string(taskBytes), "**AgentStatus:** success") {
		t.Fatalf("AgentStatus not success:\n%s", taskBytes)
	}
	logBytes, err := os.ReadFile(agent.LogPath(boardDir, "TB-1", runID))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logBytes), "parent-done") {
		t.Fatalf("log missing parent output:\n%s", logBytes)
	}
}

func TestRunAgent_FolderTaskUsesTaskLocalArtifacts(t *testing.T) {
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: []string{"folder-line"},
		exitCode:    0,
	}
	svc, boardDir := realTbFolderBoardForRun(t, "claude", stub)

	runID, err := svc.RunAgent(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	wantState := filepath.Join(boardDir, "backlog", "TB-1", ".agent-state.jsonl")
	if got := agent.StatePath(boardDir, "TB-1"); got != wantState {
		t.Fatalf("StatePath: got %s, want %s", got, wantState)
	}
	events := readEvents(t, boardDir, "TB-1")
	if len(events) == 0 || events[len(events)-1].Status != agent.StatusSuccess {
		t.Fatalf("events did not finish successfully: %+v", events)
	}

	wantLog := filepath.Join(boardDir, "backlog", "TB-1", ".agent-logs", runID+".log")
	if got := agent.LogPath(boardDir, "TB-1", runID); got != wantLog {
		t.Fatalf("LogPath: got %s, want %s", got, wantLog)
	}
	logBytes, err := os.ReadFile(wantLog)
	if err != nil {
		t.Fatalf("read folder log: %v", err)
	}
	if !strings.Contains(string(logBytes), "folder-line") {
		t.Fatalf("folder log missing runner output:\n%s", logBytes)
	}

	assertAppPathMissing(t, filepath.Join(boardDir, ".agent-state", "TB-1.jsonl"))
	assertAppPathMissing(t, filepath.Join(boardDir, ".agent-logs", "TB-1", runID+".log"))

	taskBytes, _ := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1", "TASK.md"))
	if !strings.Contains(string(taskBytes), "**AgentStatus:** success") {
		t.Errorf("AgentStatus not success:\n%s", taskBytes)
	}
}

func TestRunAgent_MixedBoardRunsListAndReadLogsInOwnLayouts(t *testing.T) {
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: []string{"mixed-output"},
		exitCode:    0,
	}
	svc, boardDir := realTbMixedBoardForRun(t, "claude", stub)

	fileRunID, err := svc.RunAgent(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("RunAgent file task: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	folderRunID, err := svc.RunAgent(context.Background(), "TB-2")
	if err != nil {
		t.Fatalf("RunAgent folder task: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-2", 5*time.Second)

	for _, tc := range []struct {
		name          string
		taskID        string
		runID         string
		wantStatePath string
		wantLogPath   string
		wrongState    string
		wrongLog      string
	}{
		{
			name:          "file",
			taskID:        "TB-1",
			runID:         fileRunID,
			wantStatePath: filepath.Join(boardDir, ".agent-state", "TB-1.jsonl"),
			wantLogPath:   filepath.Join(boardDir, ".agent-logs", "TB-1", fileRunID+".log"),
			wrongState:    filepath.Join(boardDir, "backlog", "TB-1", ".agent-state.jsonl"),
			wrongLog:      filepath.Join(boardDir, "backlog", "TB-1", ".agent-logs", fileRunID+".log"),
		},
		{
			name:          "folder",
			taskID:        "TB-2",
			runID:         folderRunID,
			wantStatePath: filepath.Join(boardDir, "backlog", "TB-2", ".agent-state.jsonl"),
			wantLogPath:   filepath.Join(boardDir, "backlog", "TB-2", ".agent-logs", folderRunID+".log"),
			wrongState:    filepath.Join(boardDir, ".agent-state", "TB-2.jsonl"),
			wrongLog:      filepath.Join(boardDir, ".agent-logs", "TB-2", folderRunID+".log"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := agent.StatePath(boardDir, tc.taskID); got != tc.wantStatePath {
				t.Fatalf("StatePath: got %s, want %s", got, tc.wantStatePath)
			}
			if got := agent.LogPath(boardDir, tc.taskID, tc.runID); got != tc.wantLogPath {
				t.Fatalf("LogPath: got %s, want %s", got, tc.wantLogPath)
			}

			runs, err := svc.ListRuns(context.Background(), tc.taskID)
			if err != nil {
				t.Fatalf("ListRuns: %v", err)
			}
			if len(runs) != 1 {
				t.Fatalf("got %d runs, want 1: %+v", len(runs), runs)
			}
			if runs[0].RunID != tc.runID || runs[0].Status != string(agent.StatusSuccess) || runs[0].LogPath != tc.wantLogPath {
				t.Fatalf("run summary: %+v", runs[0])
			}

			logText, err := svc.GetRunLog(context.Background(), tc.taskID, tc.runID)
			if err != nil {
				t.Fatalf("GetRunLog: %v", err)
			}
			if !strings.Contains(logText, "mixed-output") {
				t.Fatalf("log missing output:\n%s", logText)
			}
			assertAppPathMissing(t, tc.wrongState)
			assertAppPathMissing(t, tc.wrongLog)
		})
	}
}

func TestGroomTask_HappyPath_Success(t *testing.T) {
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: []string{"groomed"},
		exitCode:    0,
	}
	svc, boardDir := realTbBoardForRun(t, "claude", stub)

	runID, err := svc.GroomTask(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("GroomTask: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	in := stub.input()
	if in.Mode != agent.ModeGroom {
		t.Fatalf("runner mode: %s, want groom", in.Mode)
	}
	if !strings.Contains(in.Prompt, "tb edit TB-1 --goal -") {
		t.Fatalf("runner prompt did not use groom template:\n%s", in.Prompt)
	}
	if strings.Contains(in.Prompt, "Implement the task") {
		t.Fatalf("runner prompt still looks like implement template:\n%s", in.Prompt)
	}

	events := readEvents(t, boardDir, "TB-1")
	for _, ev := range events {
		switch ev.Event {
		case agent.EvQueued, agent.EvStarted, agent.EvFinished:
			if ev.RunID == runID && ev.Mode != agent.ModeGroom.String() {
				t.Fatalf("%s mode: %q, want groom; event=%+v", ev.Event, ev.Mode, ev)
			}
		}
	}
	last := events[len(events)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusSuccess {
		t.Fatalf("final event: %+v", last)
	}
}

// TestStartGroomWithTriageHash_PersistsHashOnQueuedAndFinished pins the
// TB-174 contract: an auto-groom run records its triage_hash on both the
// `queued` and `finished` JSONL events. LastGroomTriageHash uses the
// finished event for cross-restart dedupe, while the queued event lets
// the daemon's pickup path replay the hash through the activeRun.
func TestStartGroomWithTriageHash_PersistsHashOnQueuedAndFinished(t *testing.T) {
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: []string{"groomed"},
		exitCode:    0,
	}
	svc, boardDir := realTbBoardForRun(t, "claude", stub)

	const wantHash = "sha256-fake-fingerprint"
	runID, err := svc.StartGroomWithTriageHash(context.Background(), "TB-1", wantHash)
	if err != nil {
		t.Fatalf("StartGroomWithTriageHash: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	events := readEvents(t, boardDir, "TB-1")
	var sawQueuedHash, sawFinishedHash bool
	for _, ev := range events {
		if ev.RunID != runID {
			continue
		}
		switch ev.Event {
		case agent.EvQueued:
			if ev.TriageHash != wantHash {
				t.Fatalf("queued TriageHash: %q, want %q", ev.TriageHash, wantHash)
			}
			sawQueuedHash = true
		case agent.EvFinished:
			if ev.TriageHash != wantHash {
				t.Fatalf("finished TriageHash: %q, want %q", ev.TriageHash, wantHash)
			}
			sawFinishedHash = true
		}
	}
	if !sawQueuedHash || !sawFinishedHash {
		t.Fatalf("missing TriageHash events: queued=%v finished=%v", sawQueuedHash, sawFinishedHash)
	}

	// Cross-check the durable helper.
	gotHash, ok, err := agent.LastGroomTriageHash(boardDir, "TB-1")
	if err != nil {
		t.Fatalf("LastGroomTriageHash: %v", err)
	}
	if !ok || gotHash != wantHash {
		t.Errorf("LastGroomTriageHash: got (%q, %v), want (%q, true)", gotHash, ok, wantHash)
	}
}

// TestGroomTask_ManualOmitsTriageHash confirms the public GroomTask entry
// point (drawer) does not record a hash, so manual runs don't pollute the
// auto-groom dedupe state.
func TestGroomTask_ManualOmitsTriageHash(t *testing.T) {
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: []string{"manual groom"},
		exitCode:    0,
	}
	svc, boardDir := realTbBoardForRun(t, "claude", stub)

	runID, err := svc.GroomTask(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("GroomTask: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	for _, ev := range readEvents(t, boardDir, "TB-1") {
		if ev.RunID != runID {
			continue
		}
		if ev.TriageHash != "" {
			t.Fatalf("manual groom event %s carried unexpected TriageHash %q", ev.Event, ev.TriageHash)
		}
	}

	if _, ok, err := agent.LastGroomTriageHash(boardDir, "TB-1"); err != nil || ok {
		t.Errorf("LastGroomTriageHash after manual groom: ok=%v err=%v, want (false, nil)", ok, err)
	}
}

func TestReviewTask_HappyPath_Success(t *testing.T) {
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: []string{"reviewed"},
		exitCode:    0,
	}
	svc, boardDir := realTbBoardForRun(t, "claude", stub)

	runID, err := svc.ReviewTask(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("ReviewTask: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	in := stub.input()
	if in.Mode != agent.ModeReview {
		t.Fatalf("runner mode: %s, want review", in.Mode)
	}
	if !strings.Contains(in.Prompt, "tb review --findings TB-1") {
		t.Fatalf("runner prompt did not use review template:\n%s", in.Prompt)
	}
	if !strings.Contains(in.Prompt, "tb review --fail TB-1") {
		t.Fatalf("runner prompt missing failure handoff guidance:\n%s", in.Prompt)
	}
	if strings.Contains(in.Prompt, "Implement the task") {
		t.Fatalf("runner prompt still looks like implement template:\n%s", in.Prompt)
	}

	events := readEvents(t, boardDir, "TB-1")
	for _, ev := range events {
		switch ev.Event {
		case agent.EvQueued, agent.EvStarted, agent.EvFinished:
			if ev.RunID == runID && ev.Mode != agent.ModeReview.String() {
				t.Fatalf("%s mode: %q, want review; event=%+v", ev.Event, ev.Mode, ev)
			}
		}
	}
	last := events[len(events)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusSuccess {
		t.Fatalf("final event: %+v", last)
	}
}

// TestRunAgent_ClaudePreAllocatesSessionID is the TB-135 contract: a
// Claude run gets a UUIDv4 SessionID pre-allocated in runGoroutine,
// passes it through RunInput.SessionID to the runner, and persists the
// same id in the post-`started` session JSONL event.
func TestRunAgent_ClaudePreAllocatesSessionID(t *testing.T) {
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: []string{"hi"},
		exitCode:    0,
	}
	svc, boardDir := realTbBoardForRun(t, "claude", stub)

	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	in := stub.input()
	if in.SessionID == "" {
		t.Fatalf("RunInput.SessionID empty — pre-alloc didn't reach runner")
	}
	if !agentSessionIDRegex().MatchString(in.SessionID) {
		t.Fatalf("RunInput.SessionID %q is not a canonical UUIDv4", in.SessionID)
	}

	events := readEvents(t, boardDir, "TB-1")
	var sessionEvent *agent.Event
	for i := range events {
		if events[i].Event == agent.EvSession {
			sessionEvent = &events[i]
			break
		}
	}
	if sessionEvent == nil {
		t.Fatalf("no session event in JSONL: %+v", events)
	}
	if sessionEvent.SessionID != in.SessionID {
		t.Fatalf("session event id %q != RunInput.SessionID %q", sessionEvent.SessionID, in.SessionID)
	}
}

// TestRunAgent_CodexDoesNotPreAllocateSessionID confirms only Claude
// runs get a pre-allocated SessionID. Codex captures its id from the
// --json stream callback (TB-136), not from a daemon-side pre-alloc.
func TestRunAgent_CodexDoesNotPreAllocateSessionID(t *testing.T) {
	stub := &stubRunner{
		name:        "codex",
		stdoutLines: []string{"hi"},
		exitCode:    0,
	}
	svc, _ := realTbBoardForRun(t, "codex", stub)

	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	if got := stub.input().SessionID; got != "" {
		t.Fatalf("Codex run got pre-allocated SessionID %q; should be empty", got)
	}
}

// TestRunAgent_CodexOnSessionIDCallbackWritesSessionEvent is the TB-136
// contract: when the Codex translator hands a session id up via
// RunInput.OnSessionID, runGoroutine records it on activeRun and writes
// a matching `session` JSONL event with the live PID.
func TestRunAgent_CodexOnSessionIDCallbackWritesSessionEvent(t *testing.T) {
	stub := &stubRunner{
		name:          "codex",
		stdoutLines:   []string{"first", "second"},
		exitCode:      0,
		fireSessionID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
	}
	svc, boardDir := realTbBoardForRun(t, "codex", stub)

	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	events := readEvents(t, boardDir, "TB-1")
	var sessionEv *agent.Event
	for i := range events {
		if events[i].Event == agent.EvSession {
			sessionEv = &events[i]
			break
		}
	}
	if sessionEv == nil {
		t.Fatalf("no session event in JSONL: %+v", events)
	}
	if sessionEv.SessionID != stub.fireSessionID {
		t.Fatalf("session event id %q != fireSessionID %q", sessionEv.SessionID, stub.fireSessionID)
	}
	if sessionEv.PID == 0 {
		t.Fatalf("session event PID was zero — OnStarted should have populated ar.Pid before OnSessionID")
	}

	// The session event must appear AFTER started (recovery's PID
	// liveness check depends on `started` landing first).
	var startedIdx, sessionIdx int = -1, -1
	for i, ev := range events {
		if ev.Event == agent.EvStarted {
			startedIdx = i
		}
		if ev.Event == agent.EvSession {
			sessionIdx = i
			break
		}
	}
	if startedIdx == -1 || sessionIdx == -1 || sessionIdx < startedIdx {
		t.Fatalf("session must follow started; startedIdx=%d sessionIdx=%d events=%+v", startedIdx, sessionIdx, events)
	}
}

// TestRunAgent_NoSessionEventWhenSessionIDUnset locks the TB-133 gate
// via the Codex path, where SessionID stays empty until TB-136 wires
// the --json OnSessionID callback. A Codex run with a stub runner that
// never reports a session id must produce no EvSession JSONL event.
//
// The Claude positive case is covered by
// TestRunAgent_ClaudePreAllocatesSessionID (TB-135).
func TestRunAgent_NoSessionEventWhenSessionIDUnset(t *testing.T) {
	stub := &stubRunner{
		name:        "codex",
		stdoutLines: []string{"first"},
		exitCode:    0,
	}
	svc, boardDir := realTbBoardForRun(t, "codex", stub)

	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	events := readEvents(t, boardDir, "TB-1")
	for _, ev := range events {
		if ev.Event == agent.EvSession {
			t.Fatalf("session event must not appear without SessionID; events=%+v", events)
		}
	}
}

func TestRunAgent_TimeoutProviderIsReadPerRun(t *testing.T) {
	stub := &stubRunner{
		name:     "claude",
		exitCode: 0,
	}
	timeout := time.Minute
	svc, _ := realTbBoardForRunWithOptions(t, "claude", stub, func(opts *AgentServiceOptions) {
		opts.TimeoutProvider = func() time.Duration { return timeout }
	})

	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("first RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)
	if got := stub.input().Timeout; got != time.Minute {
		t.Fatalf("first timeout: got %v, want 1m", got)
	}

	timeout = 2 * time.Minute
	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("second RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)
	if got := stub.input().Timeout; got != 2*time.Minute {
		t.Fatalf("second timeout: got %v, want 2m", got)
	}
}

func TestRunQueuedAgentSync_GroomQueuedEventUsesGroomPrompt(t *testing.T) {
	stub := &stubRunner{
		name:     "claude",
		exitCode: 0,
	}
	svc, boardDir := realTbBoardForRun(t, "claude", stub)
	c := svc.board.snapshot()
	if c == nil {
		t.Fatal("missing cli client")
	}

	if err := agent.AppendEvent(boardDir, "TB-1", agent.Event{
		TS:     time.Now().UTC().Format(time.RFC3339),
		RunID:  "r_groom001",
		TaskID: "TB-1",
		Event:  agent.EvQueued,
		Agent:  "claude",
		Mode:   agent.ModeGroom.String(),
	}); err != nil {
		t.Fatalf("append queued: %v", err)
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "queued"}); err != nil {
		t.Fatalf("edit queued: %v", err)
	}

	status, err := svc.RunQueuedAgentSync(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("RunQueuedAgentSync: %v", err)
	}
	if status != "success" {
		t.Fatalf("status: %s, want success", status)
	}

	in := stub.input()
	if in.Mode != agent.ModeGroom {
		t.Fatalf("runner mode: %s, want groom", in.Mode)
	}
	if !strings.Contains(in.Prompt, "tb edit TB-1 --acceptance -") {
		t.Fatalf("runner prompt did not use groom template:\n%s", in.Prompt)
	}

	events := readEvents(t, boardDir, "TB-1")
	for _, ev := range events {
		switch ev.Event {
		case agent.EvQueued, agent.EvStarted, agent.EvFinished:
			if ev.RunID == "r_groom001" && ev.Mode != agent.ModeGroom.String() {
				t.Fatalf("%s mode: %q, want groom; event=%+v", ev.Event, ev.Mode, ev)
			}
		}
	}
}

func TestRunQueuedAgentSync_FolderQueuedEventUsesTaskLocalState(t *testing.T) {
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: []string{"queued-folder"},
		exitCode:    0,
	}
	svc, boardDir := realTbFolderBoardForRun(t, "claude", stub)
	c := svc.board.snapshot()
	if c == nil {
		t.Fatal("missing cli client")
	}

	if err := agent.AppendEvent(boardDir, "TB-1", agent.Event{
		TS:     time.Now().UTC().Format(time.RFC3339),
		RunID:  "r_folderq",
		TaskID: "TB-1",
		Event:  agent.EvQueued,
		Agent:  "claude",
		Mode:   agent.ModeImplement.String(),
	}); err != nil {
		t.Fatalf("append queued: %v", err)
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "queued"}); err != nil {
		t.Fatalf("edit queued: %v", err)
	}

	status, err := svc.RunQueuedAgentSync(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("RunQueuedAgentSync: %v", err)
	}
	if status != "success" {
		t.Fatalf("status: %s, want success", status)
	}

	events := readEvents(t, boardDir, "TB-1")
	var sawStarted, sawFinished bool
	for _, ev := range events {
		if ev.RunID != "r_folderq" {
			t.Fatalf("RunQueuedAgentSync should reuse folder-local queued run; got event %+v", ev)
		}
		switch ev.Event {
		case agent.EvStarted:
			sawStarted = true
		case agent.EvFinished:
			sawFinished = ev.Status == agent.StatusSuccess
		}
	}
	if !sawStarted || !sawFinished {
		t.Fatalf("missing started/finished events: %+v", events)
	}
	assertAppPathMissing(t, filepath.Join(boardDir, ".agent-state", "TB-1.jsonl"))
	assertAppPathMissing(t, filepath.Join(boardDir, ".agent-logs", "TB-1"))
}

func TestRunAgent_NonZeroExit_MapsToFailed(t *testing.T) {
	stub := &stubRunner{
		name:        "claude",
		stdoutLines: []string{"about-to-fail"},
		exitCode:    7,
	}
	svc, boardDir := realTbBoardForRun(t, "claude", stub)

	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	events := readEvents(t, boardDir, "TB-1")
	last := events[len(events)-1]
	if last.Status != agent.StatusFailed {
		t.Errorf("status: %s", last.Status)
	}
	if last.Reason != "non-zero exit" {
		t.Errorf("reason: %q", last.Reason)
	}
	if last.ExitCode != 7 {
		t.Errorf("exit_code: %d", last.ExitCode)
	}

	taskBytes, _ := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
	if !strings.Contains(string(taskBytes), "**AgentStatus:** failed") {
		t.Errorf("AgentStatus not failed:\n%s", taskBytes)
	}
}

func TestRunAgent_BinaryNotFound_NoStartedEvent(t *testing.T) {
	stub := &stubRunner{
		name:   "claude",
		runErr: agent.ErrBinaryNotFound,
	}
	// Override the stub so OnStarted is NOT called — emulating cmd.Start
	// failure.
	stub2 := &noStartRunner{name: "claude", err: agent.ErrBinaryNotFound}
	svc, boardDir := realTbBoardForRun(t, "claude", nil)
	svc.setRunnerFactory(func(name string) (agent.Runner, error) {
		return stub2, nil
	})

	_, _ = stub, stub2 // silence unused

	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	events := readEvents(t, boardDir, "TB-1")
	// No `started` event because OnStarted never fired.
	for _, ev := range events {
		if ev.Event == agent.EvStarted {
			t.Fatalf("started event leaked: %+v", events)
		}
	}
	last := events[len(events)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusFailed || last.Reason != "binary not found" {
		t.Errorf("final event: %+v", last)
	}
}

type noStartRunner struct {
	name string
	err  error
}

func (r *noStartRunner) Name() string { return r.name }
func (r *noStartRunner) Run(ctx context.Context, in agent.RunInput) (agent.RunResult, error) {
	return agent.RunResult{ExitCode: -1, Err: r.err}, r.err
}

func TestRunAgent_RejectsAlreadyRunning(t *testing.T) {
	// First run sits blocked until we release it.
	gate := make(chan struct{})
	stub := &blockingRunner{name: "claude", release: gate}
	svc, _ := realTbBoardForRun(t, "claude", nil)
	svc.setRunnerFactory(func(name string) (agent.Runner, error) {
		return stub, nil
	})

	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("first RunAgent: %v", err)
	}

	// Wait until activeRun is populated AND AgentStatus has been written
	// to running so the second RunAgent's GetTask sees the new status.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		_, busy := svc.active["TB-1"]
		svc.mu.Unlock()
		if busy {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Second call should error.
	if _, err := svc.RunAgent(context.Background(), "TB-1"); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("want ErrAlreadyRunning, got %v", err)
	}

	close(gate)
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)
}

type blockingRunner struct {
	name    string
	release chan struct{}
}

func (r *blockingRunner) Name() string { return r.name }
func (r *blockingRunner) Run(ctx context.Context, in agent.RunInput) (agent.RunResult, error) {
	if in.OnStarted != nil {
		in.OnStarted(54321, 54321)
	}
	select {
	case <-r.release:
		return agent.RunResult{ExitCode: 0}, nil
	case <-ctx.Done():
		return agent.RunResult{ExitCode: -1, Err: ctx.Err()}, ctx.Err()
	}
}

func TestRunAgent_NoAgent(t *testing.T) {
	svc, _ := realTbBoardForRun(t, "", nil)
	_, err := svc.RunAgent(context.Background(), "TB-1")
	if !errors.Is(err, ErrNoAgent) {
		t.Fatalf("want ErrNoAgent, got %v", err)
	}
}

func TestMapRunnerOutcome(t *testing.T) {
	cases := []struct {
		name       string
		res        agent.RunResult
		err        error
		wantStatus string
		wantReason string
		wantExit   int
	}{
		{"success", agent.RunResult{ExitCode: 0}, nil, "success", "", 0},
		{"non-zero exit", agent.RunResult{ExitCode: 1}, nil, "failed", "non-zero exit", 1},
		{"binary not found", agent.RunResult{ExitCode: -1, Err: agent.ErrBinaryNotFound}, agent.ErrBinaryNotFound, "failed", "binary not found", -1},
		{"timeout", agent.RunResult{ExitCode: -1, Err: agent.ErrTimeout}, agent.ErrTimeout, "failed", "timeout", -1},
		{"ctx canceled", agent.RunResult{ExitCode: -1, Err: context.Canceled}, context.Canceled, "failed", context.Canceled.Error(), -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, r, ec := mapRunnerOutcome(c.res, c.err)
			if s != c.wantStatus || r != c.wantReason || ec != c.wantExit {
				t.Errorf("got (%s/%s/%d), want (%s/%s/%d)",
					s, r, ec, c.wantStatus, c.wantReason, c.wantExit)
			}
		})
	}
}

// --- TB-138: ResumeAgent tests ---

// resumeFixture builds a board with a single interrupted task whose
// JSONL already includes a session event. Returns the wired service, a
// CLI client (for `tb show` post-conditions), the boardDir, and the
// ResumeCandidate fields the caller will assert against.
func resumeFixture(t *testing.T, agentName string, stub *stubRunner) (svc *AgentService, c *cli.Client, boardDir string, candidate ResumeCandidate) {
	t.Helper()
	svc, boardDir = realTbBoardForRun(t, agentName, stub)
	c = svc.board.snapshot()

	// Append synthetic queued/started/session/finished{interrupted}
	// events so resumableSessionID has something to find.
	candidate = ResumeCandidate{
		SessionID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		RunID:     "r_parent",
		Cwd:       "/tmp/board/worktrees/TB-1",
		Env:       map[string]string{"TB_BOARD_PATH": "/tmp/board"},
	}
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: candidate.RunID, TaskID: "TB-1", Event: agent.EvQueued, Agent: agentName},
		{TS: "2026-05-14T10:00:01Z", RunID: candidate.RunID, TaskID: "TB-1", Event: agent.EvStarted, Agent: agentName, PID: 4242},
		{TS: "2026-05-14T10:00:02Z", RunID: candidate.RunID, TaskID: "TB-1", Event: agent.EvSession,
			SessionID: candidate.SessionID,
			PID:       4242,
			Cwd:       candidate.Cwd,
			RunEnv:    candidate.Env,
		},
		{TS: "2026-05-14T10:00:03Z", RunID: candidate.RunID, TaskID: "TB-1", Event: agent.EvFinished,
			Status: agent.StatusInterrupted, ExitCode: -1, Reason: "interrupted by daemon restart"},
	}
	for _, ev := range events {
		if err := agent.AppendEvent(boardDir, "TB-1", ev); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}
	// Flip the task's AgentStatus to interrupted via the same CLI path
	// recovery uses — also confirms the validator (TB-131) accepts the
	// value.
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "interrupted"}); err != nil {
		t.Fatalf("seed interrupted status: %v", err)
	}
	return svc, c, boardDir, candidate
}

func TestResumeAgent_RejectsWhenStatusNotInterrupted(t *testing.T) {
	stub := &stubRunner{name: "claude", exitCode: 0}
	svc, _ := realTbBoardForRun(t, "claude", stub)

	_, err := svc.ResumeAgent(context.Background(), "TB-1")
	if !errors.Is(err, ErrCannotResume) {
		t.Fatalf("ResumeAgent: got %v, want ErrCannotResume", err)
	}
}

func TestResumeAgent_RejectsWhenNoSession(t *testing.T) {
	stub := &stubRunner{name: "claude", exitCode: 0}
	svc, _ := realTbBoardForRun(t, "claude", stub)
	c := svc.board.snapshot()

	// Mark interrupted but with NO JSONL session line.
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "interrupted"}); err != nil {
		t.Fatalf("seed interrupted: %v", err)
	}
	_, err := svc.ResumeAgent(context.Background(), "TB-1")
	if !errors.Is(err, ErrNotResumable) {
		t.Fatalf("ResumeAgent: got %v, want ErrNotResumable", err)
	}
}

func TestResumeAgent_ClaudeHappyPath(t *testing.T) {
	stub := &stubRunner{name: "claude", stdoutLines: []string{"resumed"}, exitCode: 0}
	svc, _, boardDir, candidate := resumeFixture(t, "claude", stub)

	runID, err := svc.ResumeAgent(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("ResumeAgent: %v", err)
	}
	if runID == candidate.RunID {
		t.Fatalf("new run id equals parent (%s) — must be fresh", runID)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	in := stub.input()
	if in.Mode != agent.ModeResume {
		t.Errorf("RunInput.Mode = %q, want %q", in.Mode, agent.ModeResume)
	}
	if in.SessionID != candidate.SessionID {
		t.Errorf("RunInput.SessionID = %q, want %q (parent's id)", in.SessionID, candidate.SessionID)
	}
	if in.ProjectRoot != candidate.Cwd {
		t.Errorf("RunInput.ProjectRoot = %q, want persisted Cwd %q", in.ProjectRoot, candidate.Cwd)
	}
	if in.Prompt != agent.PromptResume {
		t.Errorf("RunInput.Prompt did not match PromptResume:\ngot %q\nwant %q", in.Prompt, agent.PromptResume)
	}
	var sawTBBoardPath bool
	for _, kv := range in.Env {
		if kv == "TB_BOARD_PATH=/tmp/board" {
			sawTBBoardPath = true
			break
		}
	}
	if !sawTBBoardPath {
		t.Errorf("RunInput.Env missing TB_BOARD_PATH=/tmp/board; got %v", in.Env)
	}

	// queued JSONL must carry resumed_from + resumed_from_run.
	events := readEvents(t, boardDir, "TB-1")
	var queued *agent.Event
	for i := range events {
		if events[i].RunID == runID && events[i].Event == agent.EvQueued {
			queued = &events[i]
			break
		}
	}
	if queued == nil {
		t.Fatalf("no queued event for resumed run %s", runID)
	}
	if queued.ResumedFrom != candidate.SessionID {
		t.Errorf("queued.ResumedFrom = %q, want %q", queued.ResumedFrom, candidate.SessionID)
	}
	if queued.ResumedFromRun != candidate.RunID {
		t.Errorf("queued.ResumedFromRun = %q, want %q", queued.ResumedFromRun, candidate.RunID)
	}
	if queued.Mode != agent.ModeResume.String() {
		t.Errorf("queued.Mode = %q, want %q", queued.Mode, agent.ModeResume)
	}
}

func TestResumeAgent_CodexHappyPath(t *testing.T) {
	stub := &stubRunner{name: "codex", stdoutLines: []string{"resumed"}, exitCode: 0}
	svc, _, boardDir, candidate := resumeFixture(t, "codex", stub)

	runID, err := svc.ResumeAgent(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("ResumeAgent: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	in := stub.input()
	if in.Mode != agent.ModeResume {
		t.Errorf("RunInput.Mode = %q, want %q", in.Mode, agent.ModeResume)
	}
	if in.SessionID != candidate.SessionID {
		t.Errorf("RunInput.SessionID = %q, want %q (parent's id)", in.SessionID, candidate.SessionID)
	}
	if in.ProjectRoot != candidate.Cwd {
		t.Errorf("RunInput.ProjectRoot = %q, want %q", in.ProjectRoot, candidate.Cwd)
	}
	if in.Prompt != agent.PromptResume {
		t.Errorf("RunInput.Prompt did not match PromptResume")
	}

	events := readEvents(t, boardDir, "TB-1")
	var queued *agent.Event
	for i := range events {
		if events[i].RunID == runID && events[i].Event == agent.EvQueued {
			queued = &events[i]
			break
		}
	}
	if queued == nil {
		t.Fatalf("no queued event for resumed Codex run")
	}
	if queued.ResumedFrom != candidate.SessionID || queued.ResumedFromRun != candidate.RunID {
		t.Errorf("queued resume linkage: from=%q run=%q; want %q / %q",
			queued.ResumedFrom, queued.ResumedFromRun, candidate.SessionID, candidate.RunID)
	}
}

// TestResumeCycle_KillRecoverResume drives the full TB-130 acceptance
// flow in one test: a fake-runner mid-flight kill (synthesised by
// leaving the JSONL stream open at `session`) -> RecoverStale flips the
// task to interrupted -> ResumeAgent spawns a fresh run with the
// expected resume args, cwd, env, and queued-event linkage. Both
// fake-runner contracts from the TB-130 acceptance criteria are
// exercised: killing with a session present -> interrupted; resume
// receives -r <uuid>, the persisted Cwd, the TB_ env, and the resume
// prompt body.
func TestResumeCycle_KillRecoverResume(t *testing.T) {
	stub := &stubRunner{name: "claude", stdoutLines: []string{"continued"}, exitCode: 0}
	svc, _ := realTbBoardForRun(t, "claude", stub)
	c := svc.board.snapshot()
	boardDir, err := svc.board.resolveBoardDir(context.Background())
	if err != nil {
		t.Fatalf("resolveBoardDir: %v", err)
	}

	// --- Simulated kill mid-flight ---
	// Synthesise the JSONL the original daemon would have left behind
	// before crashing: queued + started + session, NO finished. Mark
	// the task .md AgentStatus=running to mimic what `tb edit` wrote
	// during the original run.
	parentRunID := "r_parent"
	sessionID := "11111111-2222-4333-8444-555555555555"
	parentCwd := boardDir // realistic default when worktrees.enabled=false
	parentEnv := map[string]string{"TB_BOARD_PATH": boardDir}
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: parentRunID, TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: parentRunID, TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 4242},
		{TS: "2026-05-14T10:00:02Z", RunID: parentRunID, TaskID: "TB-1", Event: agent.EvSession,
			SessionID: sessionID, PID: 4242, Cwd: parentCwd, RunEnv: parentEnv,
		},
	}
	for _, ev := range events {
		if err := agent.AppendEvent(boardDir, "TB-1", ev); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "running"}); err != nil {
		t.Fatalf("seed running: %v", err)
	}

	// --- Recovery flips to interrupted ---
	rec := NewRecoveryService(svc.board, svc, func(int, string) bool { return false }, nil)
	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** interrupted") {
		t.Fatalf("AgentStatus did not become interrupted:\n%s", out)
	}

	// --- ResumeAgent spawns a fresh run with the resume context ---
	newRunID, err := svc.ResumeAgent(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("ResumeAgent: %v", err)
	}
	if newRunID == parentRunID {
		t.Fatalf("resumed run id must differ from parent; got %q", newRunID)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

	in := stub.input()
	if in.Mode != agent.ModeResume {
		t.Errorf("resumed RunInput.Mode = %q, want %q", in.Mode, agent.ModeResume)
	}
	if in.SessionID != sessionID {
		t.Errorf("resumed RunInput.SessionID = %q, want parent's %q", in.SessionID, sessionID)
	}
	if in.ProjectRoot != parentCwd {
		t.Errorf("resumed RunInput.ProjectRoot = %q, want parent's Cwd %q", in.ProjectRoot, parentCwd)
	}
	if in.Prompt != agent.PromptResume {
		t.Errorf("resumed RunInput.Prompt did not match PromptResume")
	}
	var sawBoardPath bool
	for _, kv := range in.Env {
		if kv == "TB_BOARD_PATH="+boardDir {
			sawBoardPath = true
			break
		}
	}
	if !sawBoardPath {
		t.Errorf("resumed env missing TB_BOARD_PATH=%s; got %v", boardDir, in.Env)
	}

	// The resumed run's queued event must carry both linkage fields.
	all := readEvents(t, boardDir, "TB-1")
	var queued *agent.Event
	for i := range all {
		if all[i].RunID == newRunID && all[i].Event == agent.EvQueued {
			queued = &all[i]
			break
		}
	}
	if queued == nil {
		t.Fatalf("no queued event for resumed run %s", newRunID)
	}
	if queued.ResumedFrom != sessionID {
		t.Errorf("queued.ResumedFrom = %q, want %q", queued.ResumedFrom, sessionID)
	}
	if queued.ResumedFromRun != parentRunID {
		t.Errorf("queued.ResumedFromRun = %q, want %q", queued.ResumedFromRun, parentRunID)
	}
	if queued.Mode != agent.ModeResume.String() {
		t.Errorf("queued.Mode = %q, want %q", queued.Mode, agent.ModeResume)
	}

	// And the resumed run produced a finished{success} record — i.e.
	// the runGoroutine pipeline ran to completion under the resume
	// args, not just got rejected by validation.
	var finished *agent.Event
	for i := range all {
		if all[i].RunID == newRunID && all[i].Event == agent.EvFinished {
			finished = &all[i]
			break
		}
	}
	if finished == nil || finished.Status != agent.StatusSuccess {
		t.Fatalf("resumed run did not reach finished{success}: %+v", finished)
	}
}

// TestRunQueuedAgentSync_ResumeRehydratesParentContext is the
// adversarial-review BLOCKER fix: if the GUI crashes after
// ResumeAgent appends the queued event (with resumed_from +
// resumed_from_run + mode:"resume") but BEFORE the goroutine spawns,
// the daemon's RunQueuedAgentSync replays the run from the JSONL.
// Without rehydration, the runner would launch as a fresh implement
// (no -r flag for Claude, no `resume <uuid>` for Codex) — resume
// silently turns into "start over". This test seeds the exact
// crash-before-spawn state and asserts the rehydrated RunInput.
func TestRunQueuedAgentSync_ResumeRehydratesParentContext(t *testing.T) {
	stub := &stubRunner{name: "claude", stdoutLines: []string{"resumed"}, exitCode: 0}
	svc, _ := realTbBoardForRun(t, "claude", stub)
	c := svc.board.snapshot()
	boardDir, err := svc.board.resolveBoardDir(context.Background())
	if err != nil {
		t.Fatalf("resolveBoardDir: %v", err)
	}

	// Parent run with a session event — the resume context source.
	parentRunID := "r_parent"
	parentSessionID := "11111111-2222-4333-8444-555555555555"
	parentCwd := boardDir
	parentEnv := map[string]string{"TB_BOARD_PATH": boardDir}
	parentEvents := []agent.Event{
		// Parent's queued event carries Mode: "groom" so the resume's
		// per-mode write (TB-237) can be asserted to land on GroomedBy
		// / GroomStatus after the daemon-replay rehydrates ParentMode
		// via runModeFor.
		{TS: "2026-05-14T10:00:00Z", RunID: parentRunID, TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude", Mode: agent.ModeGroom.String()},
		{TS: "2026-05-14T10:00:01Z", RunID: parentRunID, TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", Mode: agent.ModeGroom.String(), PID: 4242},
		{TS: "2026-05-14T10:00:02Z", RunID: parentRunID, TaskID: "TB-1", Event: agent.EvSession,
			SessionID: parentSessionID, PID: 4242, Cwd: parentCwd, RunEnv: parentEnv,
		},
		{TS: "2026-05-14T10:00:03Z", RunID: parentRunID, TaskID: "TB-1", Event: agent.EvFinished,
			Status: agent.StatusInterrupted, ExitCode: -1, Reason: "interrupted by daemon restart"},
	}
	for _, ev := range parentEvents {
		if err := agent.AppendEvent(boardDir, "TB-1", ev); err != nil {
			t.Fatalf("append parent: %v", err)
		}
	}

	// Resume queued event without subsequent started — exactly the
	// state ResumeAgent leaves behind if the goroutine never spawns.
	resumeRunID := "r_resume"
	resumeQueued := agent.Event{
		TS: "2026-05-14T11:00:00Z", RunID: resumeRunID, TaskID: "TB-1",
		Event: agent.EvQueued, Agent: "claude", Mode: agent.ModeResume.String(),
		ResumedFrom: parentSessionID, ResumedFromRun: parentRunID,
	}
	if err := agent.AppendEvent(boardDir, "TB-1", resumeQueued); err != nil {
		t.Fatalf("append resume queued: %v", err)
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "queued"}); err != nil {
		t.Fatalf("seed queued: %v", err)
	}

	// Daemon replay.
	finalStatus, err := svc.RunQueuedAgentSync(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("RunQueuedAgentSync: %v", err)
	}
	if finalStatus != "success" {
		t.Errorf("final status = %q, want success", finalStatus)
	}

	in := stub.input()
	if in.Mode != agent.ModeResume {
		t.Errorf("replayed RunInput.Mode = %q, want %q (mode was discarded)", in.Mode, agent.ModeResume)
	}
	if in.SessionID != parentSessionID {
		t.Errorf("replayed RunInput.SessionID = %q, want %q (parent session was not rehydrated)", in.SessionID, parentSessionID)
	}
	if in.ProjectRoot != parentCwd {
		t.Errorf("replayed RunInput.ProjectRoot = %q, want %q (parent cwd was not rehydrated)", in.ProjectRoot, parentCwd)
	}
	if in.Prompt != agent.PromptResume {
		t.Errorf("replayed RunInput.Prompt did not match PromptResume; resume decorator skipped on replay")
	}
	var sawBoardPath bool
	for _, kv := range in.Env {
		if kv == "TB_BOARD_PATH="+boardDir {
			sawBoardPath = true
			break
		}
	}
	if !sawBoardPath {
		t.Errorf("replayed env missing TB_BOARD_PATH=%s; got %v", boardDir, in.Env)
	}

	// TB-237: the daemon-replay branch must also write the per-mode pair
	// onto the parent action's slot. With the parent queued as ModeGroom
	// above, the replayed resume must land on GroomedBy / GroomStatus —
	// proving runModeFor → effectiveMode → applyPerModeAttribution flows
	// through RunQueuedAgentSync's resume path.
	taskBytes, err := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(taskBytes)
	if !strings.Contains(body, "**GroomedBy:** claude") {
		t.Errorf("replayed resume must update parent action's pair; missing **GroomedBy:** in:\n%s", body)
	}
	if !strings.Contains(body, "**GroomStatus:** success") {
		t.Errorf("replayed resume must update parent action's pair; missing **GroomStatus:** in:\n%s", body)
	}
	if strings.Contains(body, "**ImplementedBy:**") || strings.Contains(body, "**ReviewedBy:**") {
		t.Errorf("daemon-replay resume should not populate non-parent actions in:\n%s", body)
	}
}

// TestRunQueuedAgentSync_ResumeRejectsMissingParent guards the
// rehydration error path: a malformed queued resume event (no
// resumed_from / resumed_from_run, or a resumed_from_run that points
// at a non-existent run) must NOT silently degrade to a fresh implement
// — that was the exact failure the BLOCKER fix targets.
func TestRunQueuedAgentSync_ResumeRejectsMissingParent(t *testing.T) {
	stub := &stubRunner{name: "claude", exitCode: 0}
	svc, _ := realTbBoardForRun(t, "claude", stub)
	c := svc.board.snapshot()
	boardDir, err := svc.board.resolveBoardDir(context.Background())
	if err != nil {
		t.Fatalf("resolveBoardDir: %v", err)
	}

	// Resume queued event whose resumed_from_run points at a run that
	// has no session event in this JSONL.
	resumeRunID := "r_resume"
	bad := agent.Event{
		TS: "2026-05-14T11:00:00Z", RunID: resumeRunID, TaskID: "TB-1",
		Event: agent.EvQueued, Agent: "claude", Mode: agent.ModeResume.String(),
		ResumedFrom: "11111111-2222-4333-8444-555555555555", ResumedFromRun: "r_missing",
	}
	if err := agent.AppendEvent(boardDir, "TB-1", bad); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "queued"}); err != nil {
		t.Fatalf("seed queued: %v", err)
	}

	_, err = svc.RunQueuedAgentSync(context.Background(), "TB-1")
	if err == nil {
		t.Fatalf("RunQueuedAgentSync: expected error for missing parent run, got nil")
	}
	if !strings.Contains(err.Error(), "rehydrate") {
		t.Fatalf("error should mention rehydrate failure; got %v", err)
	}
}

// TestResumeCycle_KillBeforeSessionStaysLost locks the negative
// contract: when the mid-flight kill happens BEFORE a session id was
// captured, recovery falls through to `lost` (resume isn't possible
// without a session id, so widening to `interrupted` would just hide
// the dead end). This is the regression gate that prevents future
// refactors from making every dead-PID run resumable or calling it an
// agent failure.
func TestResumeCycle_KillBeforeSessionStaysLost(t *testing.T) {
	stub := &stubRunner{name: "claude", exitCode: 0}
	svc, _ := realTbBoardForRun(t, "claude", stub)
	c := svc.board.snapshot()
	boardDir, err := svc.board.resolveBoardDir(context.Background())
	if err != nil {
		t.Fatalf("resolveBoardDir: %v", err)
	}

	// queued + started, NO session event (kill landed too early).
	events := []agent.Event{
		{TS: "2026-05-14T10:00:00Z", RunID: "r_nofs", TaskID: "TB-1", Event: agent.EvQueued, Agent: "claude"},
		{TS: "2026-05-14T10:00:01Z", RunID: "r_nofs", TaskID: "TB-1", Event: agent.EvStarted, Agent: "claude", PID: 4242},
	}
	for _, ev := range events {
		if err := agent.AppendEvent(boardDir, "TB-1", ev); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "running"}); err != nil {
		t.Fatalf("seed running: %v", err)
	}

	rec := NewRecoveryService(svc.board, svc, func(int, string) bool { return false }, nil)
	if err := rec.RecoverStale(context.Background(), boardDir); err != nil {
		t.Fatalf("RecoverStale: %v", err)
	}

	out, _ := c.Run(context.Background(), "show", "TB-1")
	if !strings.Contains(string(out), "**AgentStatus:** lost") {
		t.Fatalf("AgentStatus must be lost (no SessionID to resume):\n%s", out)
	}

	// ResumeAgent must reject with ErrCannotResume.
	if _, err := svc.ResumeAgent(context.Background(), "TB-1"); !errors.Is(err, ErrCannotResume) {
		t.Fatalf("ResumeAgent on lost task: got %v, want ErrCannotResume", err)
	}
}

// TestStartAgentRunRejectsNeedsUser ensures RunAgent and GroomTask
// refuse to start a fresh run on a task that is currently parked in
// `needs-user` (TB-182). Resolution requires `tb edit --agent-status
// none` first.
func TestStartAgentRunRejectsNeedsUser(t *testing.T) {
	stub := &stubRunner{name: "claude", exitCode: 0}
	svc, _ := realTbBoardForRun(t, "claude", stub)

	c := svc.board.snapshot()
	if c == nil {
		t.Fatalf("no CLI client on BoardService")
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "needs-user"}); err != nil {
		t.Fatalf("seed needs-user: %v", err)
	}

	if _, err := svc.RunAgent(context.Background(), "TB-1"); !errors.Is(err, ErrNeedsUserAttention) {
		t.Fatalf("RunAgent on needs-user: got %v, want ErrNeedsUserAttention", err)
	}
	if _, err := svc.GroomTask(context.Background(), "TB-1"); !errors.Is(err, ErrNeedsUserAttention) {
		t.Fatalf("GroomTask on needs-user: got %v, want ErrNeedsUserAttention", err)
	}
	if _, err := svc.ResumeAgent(context.Background(), "TB-1"); !errors.Is(err, ErrNeedsUserAttention) {
		t.Fatalf("ResumeAgent on needs-user: got %v, want ErrNeedsUserAttention", err)
	}

	// Clearing the status with --agent-status none re-enables manual run.
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "none"}); err != nil {
		t.Fatalf("clear needs-user: %v", err)
	}
	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("RunAgent after clear: %v", err)
	}
	waitForRunCompletion(t, svc, "TB-1", 5*time.Second)
}

// needsUserSettingRunner is a stub runner that flips the task's
// AgentStatus to `needs-user` via the CLI client mid-run, before
// returning a successful exit code. It mirrors what an in-process agent
// would do via `tb edit --agent-status needs-user --user-attention -`.
type needsUserSettingRunner struct {
	stub     stubRunner
	cli      *cli.Client
	id       string
	exitCode int
}

func (r *needsUserSettingRunner) Name() string { return r.stub.Name() }

func (r *needsUserSettingRunner) Run(ctx context.Context, in agent.RunInput) (agent.RunResult, error) {
	if in.OnStarted != nil {
		in.OnStarted(99999, 99999)
	}
	if err := r.cli.Edit(context.Background(), r.id, cli.EditInput{AgentStatus: "needs-user"}); err != nil {
		return agent.RunResult{}, err
	}
	return agent.RunResult{ExitCode: r.exitCode}, nil
}

// TestPostRunPreservesNeedsUser locks the TB-182 carve-out: when the
// running agent set AgentStatus=needs-user mid-run, the postRun
// terminal-status write must NOT overwrite it with the exit-mapped
// status. The JSONL `finished` line still records the exit outcome so
// run history is intact.
func TestPostRunPreservesNeedsUser(t *testing.T) {
	cases := []struct {
		name          string
		exitCode      int
		expectStatus  agent.Status
		expectAgentMD string // expected substring in task .md AgentStatus line
	}{
		{
			name:          "success exit preserves needs-user",
			exitCode:      0,
			expectStatus:  agent.StatusSuccess,
			expectAgentMD: "**AgentStatus:** needs-user",
		},
		{
			name:          "failed exit preserves needs-user",
			exitCode:      1,
			expectStatus:  agent.StatusFailed,
			expectAgentMD: "**AgentStatus:** needs-user",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, boardDir := realTbBoardForRun(t, "claude", &stubRunner{name: "claude"})
			c := svc.board.snapshot()
			if c == nil {
				t.Fatalf("no CLI client on BoardService")
			}

			custom := &needsUserSettingRunner{
				stub:     stubRunner{name: "claude"},
				cli:      c,
				id:       "TB-1",
				exitCode: tc.exitCode,
			}
			svc.setRunnerFactory(func(name string) (agent.Runner, error) { return custom, nil })

			if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
				t.Fatalf("RunAgent: %v", err)
			}
			waitForRunCompletion(t, svc, "TB-1", 5*time.Second)

			taskBytes, err := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
			if err != nil {
				t.Fatalf("read task: %v", err)
			}
			body := string(taskBytes)
			if !strings.Contains(body, tc.expectAgentMD) {
				t.Fatalf("needs-user was overwritten; task body:\n%s", body)
			}

			// The JSONL `finished` event still captures the real exit outcome so
			// run history is intact.
			events := readEvents(t, boardDir, "TB-1")
			last := events[len(events)-1]
			if last.Event != agent.EvFinished {
				t.Fatalf("last event not finished: %+v", last)
			}
			if last.Status != tc.expectStatus {
				t.Fatalf("finished event status: got %q, want %q", last.Status, tc.expectStatus)
			}
		})
	}
}

// TestCancelOverridesNeedsUser ensures the TB-182 needs-user carve-out
// is scoped to exit-mapped statuses only: an explicit user-initiated
// cancel (which writes `cancelled`) wins over a needs-user marker the
// agent set mid-run. Same precedent for daemon shutdown's cancel write.
func TestCancelOverridesNeedsUser(t *testing.T) {
	svc, boardDir := realTbBoardForRun(t, "claude", &stubRunner{name: "claude"})
	c := svc.board.snapshot()
	if c == nil {
		t.Fatalf("no CLI client on BoardService")
	}

	// Seed needs-user as if an agent already wrote it.
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "needs-user"}); err != nil {
		t.Fatalf("seed needs-user: %v", err)
	}

	// Manually construct an activeRun and route it through recordTerminal
	// with status=cancelled — this is the same code path finishCancelled
	// would exercise from the UI Cancel button or daemon shutdown.
	ar := &activeRun{
		RunID:  agent.GenerateRunID(),
		TaskID: "TB-1",
		Agent:  "claude",
		Mode:   agent.ModeImplement.String(),
		Done:   make(chan struct{}),
	}
	svc.recordTerminal(c, ar, boardDir, agent.StatusCancelled, "user cancelled", -1)

	taskBytes, err := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(taskBytes)
	if !strings.Contains(body, "**AgentStatus:** cancelled") {
		t.Fatalf("cancel must win over needs-user; got body:\n%s", body)
	}
}

// TestRecordTerminalPerModeAttribution locks the TB-237 contract: the
// terminal-status write that updates AgentStatus must ALSO update the
// per-mode pair matching the run's mode. The legacy AgentStatus / Agent
// fields stay populated for back-compat with auto-implement / auto-groom
// / daemon pickup.
func TestRecordTerminalPerModeAttribution(t *testing.T) {
	cases := []struct {
		name           string
		runMode        agent.Mode
		runAgent       string
		runStatus      agent.Status
		wantByLine     string // **<ByField>:** <agent>
		wantStatusLine string // **<StatusField>:** <status>
		notByLine1     string
		notByLine2     string
	}{
		{
			name:           "groom success populates GroomedBy/GroomStatus only",
			runMode:        agent.ModeGroom,
			runAgent:       "claude",
			runStatus:      agent.StatusSuccess,
			wantByLine:     "**GroomedBy:** claude",
			wantStatusLine: "**GroomStatus:** success",
			notByLine1:     "**ImplementedBy:**",
			notByLine2:     "**ReviewedBy:**",
		},
		{
			name:           "implement failed populates ImplementedBy/ImplementStatus only",
			runMode:        agent.ModeImplement,
			runAgent:       "codex",
			runStatus:      agent.StatusFailed,
			wantByLine:     "**ImplementedBy:** codex",
			wantStatusLine: "**ImplementStatus:** failed",
			notByLine1:     "**GroomedBy:**",
			notByLine2:     "**ReviewedBy:**",
		},
		{
			name:           "review success populates ReviewedBy/ReviewStatus only",
			runMode:        agent.ModeReview,
			runAgent:       "claude",
			runStatus:      agent.StatusSuccess,
			wantByLine:     "**ReviewedBy:** claude",
			wantStatusLine: "**ReviewStatus:** success",
			notByLine1:     "**GroomedBy:**",
			notByLine2:     "**ImplementedBy:**",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, boardDir := realTbBoardForRun(t, "claude", &stubRunner{name: "claude"})
			c := svc.board.snapshot()
			if c == nil {
				t.Fatalf("no CLI client on BoardService")
			}

			ar := &activeRun{
				RunID:  agent.GenerateRunID(),
				TaskID: "TB-1",
				Agent:  tc.runAgent,
				Mode:   tc.runMode.String(),
				Done:   make(chan struct{}),
			}
			svc.recordTerminal(c, ar, boardDir, tc.runStatus, "", 0)

			taskBytes, err := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
			if err != nil {
				t.Fatalf("read task: %v", err)
			}
			body := string(taskBytes)
			if !strings.Contains(body, tc.wantByLine) {
				t.Errorf("missing %q in task body:\n%s", tc.wantByLine, body)
			}
			if !strings.Contains(body, tc.wantStatusLine) {
				t.Errorf("missing %q in task body:\n%s", tc.wantStatusLine, body)
			}
			// Other modes' pairs stay absent — no placeholder rows.
			if strings.Contains(body, tc.notByLine1) {
				t.Errorf("unexpected %q in task body:\n%s", tc.notByLine1, body)
			}
			if strings.Contains(body, tc.notByLine2) {
				t.Errorf("unexpected %q in task body:\n%s", tc.notByLine2, body)
			}
			// Legacy AgentStatus continues to reflect the most recent run.
			if !strings.Contains(body, "**AgentStatus:** "+string(tc.runStatus)) {
				t.Errorf("missing legacy AgentStatus=%q in task body:\n%s", tc.runStatus, body)
			}
		})
	}
}

// TestRecordTerminalPreservesBlankAgentStatusOnReviewFailHandoff locks
// the TB-268 carve-out: when a review-mode run finishes with status=success
// and the task is now in ready with the `review-failed` tag (the post-state
// after `tb review --fail`), recordTerminal must leave the generic
// AgentStatus blank. The per-mode pair (ReviewedBy / ReviewStatus) is
// still written so review attribution survives.
func TestRecordTerminalPreservesBlankAgentStatusOnReviewFailHandoff(t *testing.T) {
	svc, boardDir := realTbBoardForRun(t, "claude", &stubRunner{name: "claude"})
	c := svc.board.snapshot()
	if c == nil {
		t.Fatalf("no CLI client on BoardService")
	}

	// Simulate the post-state of `tb review --fail`: task is in ready/
	// with the `review-failed` tag and a blank generic AgentStatus.
	if err := c.Move(context.Background(), "TB-1", "ready"); err != nil {
		t.Fatalf("move to ready: %v", err)
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{Tags: "review-failed"}); err != nil {
		t.Fatalf("set review-failed tag: %v", err)
	}

	ar := &activeRun{
		RunID:  agent.GenerateRunID(),
		TaskID: "TB-1",
		Agent:  "claude",
		Mode:   agent.ModeReview.String(),
		Done:   make(chan struct{}),
	}
	svc.recordTerminal(c, ar, boardDir, agent.StatusSuccess, "", 0)

	taskBytes, err := os.ReadFile(filepath.Join(boardDir, "ready", "TB-1.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(taskBytes)
	if strings.Contains(body, "**AgentStatus:**") {
		t.Fatalf("AgentStatus should stay blank after review-fail handoff; got body:\n%s", body)
	}
	if !strings.Contains(body, "**ReviewedBy:** claude") {
		t.Errorf("expected per-mode ReviewedBy preserved; got body:\n%s", body)
	}
	if !strings.Contains(body, "**ReviewStatus:** success") {
		t.Errorf("expected per-mode ReviewStatus preserved; got body:\n%s", body)
	}
	if !strings.Contains(body, "**Tags:** review-failed") {
		t.Errorf("review-failed tag should be preserved; got body:\n%s", body)
	}

	// JSONL still captures the real exit outcome so run history is intact.
	events := readEvents(t, boardDir, "TB-1")
	last := events[len(events)-1]
	if last.Event != agent.EvFinished {
		t.Fatalf("last event not finished: %+v", last)
	}
	if last.Status != agent.StatusSuccess {
		t.Fatalf("finished event status: got %q, want success", last.Status)
	}
}

// TestRecordTerminalClearsLingeringAgentStatusOnReviewFailHandoff covers
// the alternate-path case Codex flagged: a successful review-mode run
// ends with the task in ready + review-failed, but the legacy AgentStatus
// on disk is still `success` from a prior implement run (e.g. the agent
// reached the state without going through `tb review --fail`, or the file
// was edited externally between the CLI clear and the GUI write). The
// carve-out must actively clear the cursor so auto-implement sees the
// task as retry-eligible.
func TestRecordTerminalClearsLingeringAgentStatusOnReviewFailHandoff(t *testing.T) {
	svc, boardDir := realTbBoardForRun(t, "claude", &stubRunner{name: "claude"})
	c := svc.board.snapshot()
	if c == nil {
		t.Fatalf("no CLI client on BoardService")
	}

	if err := c.Move(context.Background(), "TB-1", "ready"); err != nil {
		t.Fatalf("move to ready: %v", err)
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{
		Tags:        "review-failed",
		AgentStatus: "success", // stale from a prior implement run
	}); err != nil {
		t.Fatalf("set tags + stale AgentStatus: %v", err)
	}

	ar := &activeRun{
		RunID:  agent.GenerateRunID(),
		TaskID: "TB-1",
		Agent:  "claude",
		Mode:   agent.ModeReview.String(),
		Done:   make(chan struct{}),
	}
	svc.recordTerminal(c, ar, boardDir, agent.StatusSuccess, "", 0)

	taskBytes, err := os.ReadFile(filepath.Join(boardDir, "ready", "TB-1.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(taskBytes)
	if strings.Contains(body, "**AgentStatus:**") {
		t.Fatalf("stale AgentStatus must be cleared by carve-out; got body:\n%s", body)
	}
	if !strings.Contains(body, "**ReviewedBy:** claude") {
		t.Errorf("expected per-mode ReviewedBy; got body:\n%s", body)
	}
}

// TestRecordTerminalNeedsUserBeatsReviewFailHandoff guards the carve-out
// precedence: if the agent set AgentStatus=needs-user, that wins over the
// review-fail handoff carve-out (needs-user is an explicit human-action
// request and is never overwritten by automation).
func TestRecordTerminalNeedsUserBeatsReviewFailHandoff(t *testing.T) {
	svc, boardDir := realTbBoardForRun(t, "claude", &stubRunner{name: "claude"})
	c := svc.board.snapshot()
	if c == nil {
		t.Fatalf("no CLI client on BoardService")
	}

	if err := c.Move(context.Background(), "TB-1", "ready"); err != nil {
		t.Fatalf("move to ready: %v", err)
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{
		Tags:        "review-failed",
		AgentStatus: "needs-user",
	}); err != nil {
		t.Fatalf("seed needs-user: %v", err)
	}

	ar := &activeRun{
		RunID:  agent.GenerateRunID(),
		TaskID: "TB-1",
		Agent:  "claude",
		Mode:   agent.ModeReview.String(),
		Done:   make(chan struct{}),
	}
	svc.recordTerminal(c, ar, boardDir, agent.StatusSuccess, "", 0)

	taskBytes, err := os.ReadFile(filepath.Join(boardDir, "ready", "TB-1.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(taskBytes)
	if !strings.Contains(body, "**AgentStatus:** needs-user") {
		t.Fatalf("needs-user must win over review-fail carve-out; got body:\n%s", body)
	}
}

// TestRecordTerminalReviewSuccessWithoutFailHandoffWritesAgentStatus
// guards the opposite path of TB-268: a review-mode run that ended
// without `tb review --fail` (no review-failed tag) still writes the
// generic AgentStatus normally. The carve-out is scoped, not blanket.
func TestRecordTerminalReviewSuccessWithoutFailHandoffWritesAgentStatus(t *testing.T) {
	svc, boardDir := realTbBoardForRun(t, "claude", &stubRunner{name: "claude"})
	c := svc.board.snapshot()
	if c == nil {
		t.Fatalf("no CLI client on BoardService")
	}

	ar := &activeRun{
		RunID:  agent.GenerateRunID(),
		TaskID: "TB-1",
		Agent:  "claude",
		Mode:   agent.ModeReview.String(),
		Done:   make(chan struct{}),
	}
	svc.recordTerminal(c, ar, boardDir, agent.StatusSuccess, "", 0)

	taskBytes, err := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(taskBytes)
	if !strings.Contains(body, "**AgentStatus:** success") {
		t.Fatalf("expected AgentStatus=success on non-fail review handoff; got body:\n%s", body)
	}
	if !strings.Contains(body, "**ReviewedBy:** claude") {
		t.Errorf("expected per-mode ReviewedBy; got body:\n%s", body)
	}
}

// TestRecordTerminalResumeUsesParentMode locks the TB-237 invariant that
// a resume run updates the originating action's per-mode pair, never a
// fourth "resume" slot.
func TestRecordTerminalResumeUsesParentMode(t *testing.T) {
	svc, boardDir := realTbBoardForRun(t, "claude", &stubRunner{name: "claude"})
	c := svc.board.snapshot()
	if c == nil {
		t.Fatalf("no CLI client on BoardService")
	}

	ar := &activeRun{
		RunID:      agent.GenerateRunID(),
		TaskID:     "TB-1",
		Agent:      "claude",
		Mode:       agent.ModeResume.String(),
		ParentMode: agent.ModeGroom.String(),
		Done:       make(chan struct{}),
	}
	svc.recordTerminal(c, ar, boardDir, agent.StatusSuccess, "", 0)

	taskBytes, err := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(taskBytes)
	if !strings.Contains(body, "**GroomedBy:** claude") {
		t.Errorf("resume must update parent action's pair; missing **GroomedBy:** in:\n%s", body)
	}
	if !strings.Contains(body, "**GroomStatus:** success") {
		t.Errorf("resume must update parent action's pair; missing **GroomStatus:** in:\n%s", body)
	}
	if strings.Contains(body, "**ImplementedBy:**") || strings.Contains(body, "**ReviewedBy:**") {
		t.Errorf("resume should not populate non-parent actions in:\n%s", body)
	}
}

// silence unused
var _ io.Writer = (*lineSink)(nil)

// agentSessionIDRegex returns the regex that locks the UUIDv4 shape we
// hand to `claude --session-id`. The CLI rejects anything that doesn't
// match this, so the test ensures GenerateSessionID stays compliant.
func agentSessionIDRegex() *regexp.Regexp {
	return regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
}
