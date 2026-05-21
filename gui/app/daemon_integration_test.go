package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
	"tools/tb-gui/internal/daemon"
)

// boardAdapter implements daemon.Board on top of BoardService.
type boardAdapter struct{ b *BoardService }

func (a *boardAdapter) ListActive(ctx context.Context) ([]daemon.AgentTask, error) {
	snap, err := a.b.LoadBoard(ctx)
	if err != nil {
		return nil, err
	}
	out := []daemon.AgentTask{}
	for _, bucket := range [][]Task{snap.Backlog, snap.Ready, snap.InProgress, snap.CodeReview, snap.Done} {
		for _, t := range bucket {
			out = append(out, daemon.AgentTask{
				ID:          t.ID,
				Agent:       t.Agent,
				AgentStatus: t.AgentStatus,
			})
		}
	}
	return out, nil
}

func (a *boardAdapter) GetTask(ctx context.Context, id string) (daemon.AgentTask, error) {
	d, err := a.b.GetTask(ctx, id)
	if err != nil {
		return daemon.AgentTask{}, err
	}
	return daemon.AgentTask{
		ID:          d.Metadata.ID,
		Agent:       d.Metadata.Agent,
		AgentStatus: d.Metadata.AgentStatus,
	}, nil
}

// daemonAgentAdapter implements daemon.Agent on top of AgentService.
type daemonAgentAdapter struct{ s *AgentService }

func (a *daemonAgentAdapter) RunQueuedAgentSync(ctx context.Context, id string) (string, error) {
	return a.s.RunQueuedAgentSync(ctx, id)
}
func (a *daemonAgentAdapter) HasActiveRun(id string) bool { return a.s.HasActiveRun(id) }

// daemonIntegrationFixture mirrors realTbBoardForRun but constructs the
// daemon + adapters so the test exercises the full pickup path.
func daemonIntegrationFixture(t *testing.T, runner agent.Runner) (*daemon.Daemon, *AgentService, string, *cli.Client) {
	return daemonIntegrationFixtureWithStorage(t, runner, false)
}

func daemonFolderIntegrationFixture(t *testing.T, runner agent.Runner) (*daemon.Daemon, *AgentService, string, *cli.Client) {
	return daemonIntegrationFixtureWithStorage(t, runner, true)
}

func daemonIntegrationFixtureWithStorage(t *testing.T, runner agent.Runner, folderForm bool) (*daemon.Daemon, *AgentService, string, *cli.Client) {
	return daemonIntegrationFixtureWithStorageAndOptions(t, runner, folderForm, nil)
}

func daemonIntegrationFixtureWithStorageAndOptions(t *testing.T, runner agent.Runner, folderForm bool, configure func(*daemon.Options)) (*daemon.Daemon, *AgentService, string, *cli.Client) {
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
	taskBody := strings.Replace(sampleTaskBody, "**Branch:** —", "**Branch:** —\n**Agent:** claude", 1)
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
	svc := NewAgentService(AgentServiceOptions{Board: board, Emitter: em})
	svc.setRunnerFactory(func(name string) (agent.Runner, error) {
		return runner, nil
	})

	rec := NewRecoveryService(board, svc, func(int, string) bool { return false }, slog.Default())

	opts := daemon.Options{
		Board:      &boardAdapter{b: board},
		Agent:      &daemonAgentAdapter{s: svc},
		Recovery:   rec,
		MaxWorkers: 1,
	}
	if configure != nil {
		configure(&opts)
	}
	d := daemon.New(opts)
	return d, svc, boardDir, c
}

// shutdownRunner blocks until ctx is cancelled, simulating an agent
// happily processing when the GUI suddenly closes.
type shutdownRunner struct {
	started chan struct{}
	mu      sync.Mutex
	once    sync.Once
}

func (r *shutdownRunner) Name() string { return "claude" }
func (r *shutdownRunner) Run(ctx context.Context, in agent.RunInput) (agent.RunResult, error) {
	if in.OnStarted != nil {
		in.OnStarted(99998, 99998)
	}
	r.mu.Lock()
	r.once.Do(func() { close(r.started) })
	r.mu.Unlock()
	<-ctx.Done()
	return agent.RunResult{ExitCode: -1, Err: ctx.Err()}, ctx.Err()
}

// successRunner finishes immediately with exit 0. The OnStarted
// callback fires so the daemon's runner ctx watcher gets the pid/pgid
// (even though they're synthetic).
type successRunner struct {
	stdoutLines []string
}

func (r *successRunner) Name() string { return "claude" }
func (r *successRunner) Run(ctx context.Context, in agent.RunInput) (agent.RunResult, error) {
	if in.OnStarted != nil {
		in.OnStarted(99997, 99997)
	}
	for _, ln := range r.stdoutLines {
		_, _ = in.Stdout.Write([]byte(ln + "\n"))
	}
	return agent.RunResult{ExitCode: 0}, nil
}

// TestDaemon_F51_CLIQueuesTriggerRun is the F5.1 end-to-end acceptance:
// from the terminal, `tb edit X --agent-status queued` (no JSONL
// AppendEvent) triggers the daemon to pick the task up via the watcher
// sink → board:reloaded → rescan → enqueue → run. The daemon synthesises
// the queued JSONL event itself since the CLI doesn't write one.
func TestDaemon_F51_CLIQueuesTriggerRun(t *testing.T) {
	runner := &successRunner{stdoutLines: []string{"hello"}}
	d, _, boardDir, c := daemonIntegrationFixture(t, runner)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := d.Activate(ctx, boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	// Simulate the F5.1 scenario: CLI flips AgentStatus to queued. No
	// JSONL AppendEvent — the CLI doesn't know about the JSONL schema.
	if err := c.Edit(ctx, "TB-1", cli.EditInput{AgentStatus: "queued"}); err != nil {
		t.Fatalf("edit queued: %v", err)
	}

	// Enqueue manually because the watcher tee isn't wired in the
	// test fixture. (TB-58's own tests cover the watcher path.) The
	// production flow goes through the watcher sink instead.
	if ok, err := d.Enqueue("TB-1"); err != nil || !ok {
		t.Fatalf("enqueue: ok=%v err=%v", ok, err)
	}

	// Wait for the run to complete (terminal status → activeRun
	// released from the daemon's active set).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !d.IsActive("TB-1") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if d.IsActive("TB-1") {
		t.Fatalf("daemon never finished the run")
	}

	// JSONL now contains queued + started + stdout + finished{success}.
	events := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events (queued/started/stdout/finished); got %d: %+v",
			len(events), events)
	}
	if events[0].Event != agent.EvQueued {
		t.Errorf("first event: %s, want queued", events[0].Event)
	}
	if events[0].Agent != "claude" {
		t.Errorf("queued agent: %q, want claude", events[0].Agent)
	}
	last := events[len(events)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusSuccess {
		t.Errorf("last event: %+v, want finished{success}", last)
	}

	// AgentStatus on disk now reads success.
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** success") {
		t.Fatalf("AgentStatus not success:\n%s", out)
	}
}

func TestDaemon_CLIQueuesFolderTaskUsesTaskLocalArtifacts(t *testing.T) {
	runner := &successRunner{stdoutLines: []string{"folder hello"}}
	d, _, boardDir, c := daemonFolderIntegrationFixture(t, runner)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := d.Activate(ctx, boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if err := c.Edit(ctx, "TB-1", cli.EditInput{AgentStatus: "queued"}); err != nil {
		t.Fatalf("edit queued: %v", err)
	}
	if ok, err := d.Enqueue("TB-1"); err != nil || !ok {
		t.Fatalf("enqueue: ok=%v err=%v", ok, err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !d.IsActive("TB-1") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if d.IsActive("TB-1") {
		t.Fatalf("daemon never finished the folder-task run")
	}

	statePath := filepath.Join(boardDir, "backlog", "TB-1", ".agent-state.jsonl")
	if got := agent.StatePath(boardDir, "TB-1"); got != statePath {
		t.Fatalf("StatePath: got %s, want %s", got, statePath)
	}
	events := readJSONL(t, statePath)
	if len(events) < 4 {
		t.Fatalf("expected queued/started/stdout/finished events; got %d: %+v", len(events), events)
	}
	last := events[len(events)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusSuccess {
		t.Fatalf("last event: %+v, want finished{success}", last)
	}
	logPath := filepath.Join(boardDir, "backlog", "TB-1", ".agent-logs", last.RunID+".log")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("folder log missing at %s: %v", logPath, err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, ".agent-state", "TB-1.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("folder daemon run should not create board-root state, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, ".agent-logs", "TB-1")); !os.IsNotExist(err) {
		t.Fatalf("folder daemon run should not create board-root logs, err=%v", err)
	}
}

func TestDaemonPeriodicRecovery_ReconcilesStaleRunningWithoutRestart(t *testing.T) {
	runner := &successRunner{}
	d, svc, boardDir, c := daemonIntegrationFixtureWithStorageAndOptions(t, runner, false, func(opts *daemon.Options) {
		opts.PeriodicRecoveryInterval = 20 * time.Millisecond
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := d.Activate(ctx, boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if err := agent.AppendEvent(boardDir, "TB-1", agent.Event{
		TS:     time.Now().UTC().Format(time.RFC3339),
		RunID:  "r_periodic",
		TaskID: "TB-1",
		Event:  agent.EvQueued,
		Agent:  "claude",
		Mode:   agent.ModeImplement.String(),
	}); err != nil {
		t.Fatalf("append queued: %v", err)
	}
	if err := agent.AppendEvent(boardDir, "TB-1", agent.Event{
		TS:     time.Now().UTC().Format(time.RFC3339),
		RunID:  "r_periodic",
		TaskID: "TB-1",
		Event:  agent.EvStarted,
		Agent:  "claude",
		Mode:   agent.ModeImplement.String(),
		PID:    99999,
	}); err != nil {
		t.Fatalf("append started: %v", err)
	}
	if err := c.Edit(ctx, "TB-1", cli.EditInput{AgentStatus: "running"}); err != nil {
		t.Fatalf("edit running: %v", err)
	}

	waitForCondition(t, 3*time.Second, func() bool {
		out, err := c.Run(context.Background(), "show", "TB-1")
		return err == nil && strings.Contains(string(out), "**AgentStatus:** lost")
	})

	events := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	last := events[len(events)-1]
	if last.Event != agent.EvFinished || last.Status != agent.StatusLost {
		t.Fatalf("last event: %+v, want finished{lost}", last)
	}
	if last.RunID != "r_periodic" {
		t.Fatalf("finished run id: got %q, want r_periodic", last.RunID)
	}

	foundFinishedEmit := false
	for _, ev := range svc.emitter.(*recordingEmitter).snapshot() {
		if ev.Name == "agent:run-finished" {
			foundFinishedEmit = true
			break
		}
	}
	if !foundFinishedEmit {
		t.Fatalf("periodic recovery did not emit agent:run-finished")
	}
}

// TestDaemonShutdown_FlushesCancelledJSONL is the F5.4 acceptance test:
// start a run via the daemon, close the daemon mid-run, assert the
// JSONL ends with finished{cancelled, reason: "shutdown"} and the
// AgentStatus on disk is `cancelled`.
func TestDaemonShutdown_FlushesCancelledJSONL(t *testing.T) {
	runner := &shutdownRunner{started: make(chan struct{})}
	d, _, boardDir, c := daemonIntegrationFixture(t, runner)

	// Activate the daemon (recovery + scan).
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := d.Activate(ctx, boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	// Flip TB-1 to queued via the CLI.
	if err := c.Edit(ctx, "TB-1", cli.EditInput{AgentStatus: "queued"}); err != nil {
		t.Fatalf("edit queued: %v", err)
	}
	// Append a JSONL queued event so RunQueuedAgentSync can find an open run.
	if err := agent.AppendEvent(boardDir, "TB-1", agent.Event{
		TS:     time.Now().UTC().Format(time.RFC3339),
		RunID:  "r_test0001",
		TaskID: "TB-1",
		Event:  agent.EvQueued,
		Agent:  "claude",
		Mode:   "implement",
	}); err != nil {
		t.Fatalf("append queued: %v", err)
	}

	// Enqueue manually since the watcher tee isn't wired in this test.
	if ok, err := d.Enqueue("TB-1"); err != nil || !ok {
		t.Fatalf("enqueue: ok=%v err=%v", ok, err)
	}

	// Wait for the runner to enter Run (the started channel closes once).
	select {
	case <-runner.started:
	case <-time.After(5 * time.Second):
		t.Fatalf("runner never started")
	}

	// Close the daemon — should propagate ctx cancellation to the
	// runner and end with finished{cancelled, reason: "shutdown"}.
	start := time.Now()
	_ = d.Close()
	elapsed := time.Since(start)
	if elapsed > 7*time.Second {
		t.Errorf("Close took too long: %v", elapsed)
	}

	// Inspect the JSONL for the finished{cancelled} record.
	events := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if len(events) == 0 {
		t.Fatalf("no JSONL events")
	}
	last := events[len(events)-1]
	if last.Event != agent.EvFinished {
		t.Errorf("last event not finished: %+v", last)
	}
	if last.Status != agent.StatusCancelled {
		t.Errorf("last status: %s, want cancelled", last.Status)
	}
	if last.Reason != "shutdown" {
		t.Errorf("last reason: %q, want shutdown", last.Reason)
	}

	// AgentStatus on disk reads cancelled.
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Fatalf("AgentStatus not cancelled:\n%s", out)
	}
}

func TestDaemonShutdown_ConcurrentCloseWritesCancelledOnce(t *testing.T) {
	runner := &shutdownRunner{started: make(chan struct{})}
	d, _, boardDir, c := daemonIntegrationFixture(t, runner)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := d.Activate(ctx, boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := c.Edit(ctx, "TB-1", cli.EditInput{AgentStatus: "queued"}); err != nil {
		t.Fatalf("edit queued: %v", err)
	}
	if err := agent.AppendEvent(boardDir, "TB-1", agent.Event{
		TS:     time.Now().UTC().Format(time.RFC3339),
		RunID:  "r_shutdown_once",
		TaskID: "TB-1",
		Event:  agent.EvQueued,
		Agent:  "claude",
		Mode:   "implement",
	}); err != nil {
		t.Fatalf("append queued: %v", err)
	}
	if ok, err := d.Enqueue("TB-1"); err != nil || !ok {
		t.Fatalf("enqueue: ok=%v err=%v", ok, err)
	}
	select {
	case <-runner.started:
	case <-time.After(5 * time.Second):
		t.Fatalf("runner never started")
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- d.Close()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	finished := 0
	for _, ev := range readJSONL(t, agent.StatePath(boardDir, "TB-1")) {
		if ev.RunID != "r_shutdown_once" || ev.Event != agent.EvFinished {
			continue
		}
		finished++
		if ev.Status != agent.StatusCancelled {
			t.Fatalf("finished status = %s, want cancelled", ev.Status)
		}
		if ev.Reason != "shutdown" {
			t.Fatalf("finished reason = %q, want shutdown", ev.Reason)
		}
	}
	if finished != 1 {
		t.Fatalf("finished event count = %d, want exactly 1", finished)
	}
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Fatalf("AgentStatus not cancelled:\n%s", out)
	}
}

func TestDaemonBoardSwitch_FlushesCancelledJSONL(t *testing.T) {
	runner := &shutdownRunner{started: make(chan struct{})}
	d, _, boardDir, c := daemonIntegrationFixture(t, runner)
	t.Cleanup(func() { _ = d.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := d.Activate(ctx, boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := c.Edit(ctx, "TB-1", cli.EditInput{AgentStatus: "queued"}); err != nil {
		t.Fatalf("edit queued: %v", err)
	}
	if err := agent.AppendEvent(boardDir, "TB-1", agent.Event{
		TS:     time.Now().UTC().Format(time.RFC3339),
		RunID:  "r_switch0001",
		TaskID: "TB-1",
		Event:  agent.EvQueued,
		Agent:  "claude",
		Mode:   "implement",
	}); err != nil {
		t.Fatalf("append queued: %v", err)
	}
	if ok, err := d.Enqueue("TB-1"); err != nil || !ok {
		t.Fatalf("enqueue: ok=%v err=%v", ok, err)
	}
	select {
	case <-runner.started:
	case <-time.After(5 * time.Second):
		t.Fatalf("runner never started")
	}

	if err := d.Deactivate(); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	events := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
	if len(events) == 0 {
		t.Fatalf("no JSONL events")
	}
	last := events[len(events)-1]
	if last.Event != agent.EvFinished {
		t.Errorf("last event not finished: %+v", last)
	}
	if last.Status != agent.StatusCancelled {
		t.Errorf("last status: %s, want cancelled", last.Status)
	}
	if last.Reason != "board switch" {
		t.Errorf("last reason: %q, want board switch", last.Reason)
	}
	out, err := c.Run(context.Background(), "show", "TB-1")
	if err != nil {
		t.Fatalf("tb show: %v", err)
	}
	if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
		t.Fatalf("AgentStatus not cancelled:\n%s", out)
	}
}

func TestAgentServiceBoardSwitchCancelsAutomationStartedRuns(t *testing.T) {
	tests := []struct {
		name  string
		start func(*AgentService) (string, error)
		mode  agent.Mode
	}{
		{
			name: "auto-groom",
			start: func(s *AgentService) (string, error) {
				return s.StartGroomWithTriageHashAs(context.Background(), "TB-1", "triage-hash", agent.InitiatorAutoGroom)
			},
			mode: agent.ModeGroom,
		},
		{
			name: "auto-implement",
			start: func(s *AgentService) (string, error) {
				return s.RunAgentAs(context.Background(), "TB-1", agent.InitiatorAutoImplement)
			},
			mode: agent.ModeImplement,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &shutdownRunner{started: make(chan struct{})}
			_, svc, boardDir, c := daemonIntegrationFixture(t, runner)
			runID, err := tt.start(svc)
			if err != nil {
				t.Fatalf("start automation run: %v", err)
			}
			select {
			case <-runner.started:
			case <-time.After(5 * time.Second):
				t.Fatalf("runner never started")
			}

			if err := svc.CancelRunsForCurrentBoard(context.Background(), "board switch"); err != nil {
				t.Fatalf("CancelRunsForCurrentBoard: %v", err)
			}

			events := readJSONL(t, agent.StatePath(boardDir, "TB-1"))
			last := events[len(events)-1]
			if last.Event != agent.EvFinished {
				t.Fatalf("last event = %+v, want finished", last)
			}
			if last.RunID != runID {
				t.Fatalf("last run = %q, want %q", last.RunID, runID)
			}
			if last.Mode != tt.mode.String() {
				t.Fatalf("last mode = %q, want %q", last.Mode, tt.mode.String())
			}
			if last.Status != agent.StatusCancelled {
				t.Fatalf("last status = %s, want cancelled", last.Status)
			}
			if last.Reason != "board switch" {
				t.Fatalf("last reason = %q, want board switch", last.Reason)
			}
			out, err := c.Run(context.Background(), "show", "TB-1")
			if err != nil {
				t.Fatalf("tb show: %v", err)
			}
			if !strings.Contains(string(out), "**AgentStatus:** cancelled") {
				t.Fatalf("AgentStatus not cancelled:\n%s", out)
			}
		})
	}
}
