package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
	lastInput   agent.RunInput
	mu          sync.Mutex
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
	// queued + started + 10 stdout + 2 stderr + finished = 15
	if len(events) != 15 {
		t.Fatalf("event count: %d; events=%+v", len(events), events)
	}
	if events[0].Event != agent.EvQueued {
		t.Errorf("first event not queued: %+v", events[0])
	}
	if events[1].Event != agent.EvStarted {
		t.Errorf("second event not started: %+v", events[1])
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

// silence unused
var _ io.Writer = (*lineSink)(nil)
