package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

// realProcessRunner spawns an actual /bin/sh process so the SIGKILL-on-pgid
// path is exercised against the kernel, not a mock. It writes the PID of
// the *child* `sleep` it spawns into childPIDFile so the test can probe
// liveness after cancel.
type realProcessRunner struct {
	name         string
	childPIDFile string
}

func (r *realProcessRunner) Name() string { return r.name }

func (r *realProcessRunner) Run(ctx context.Context, in agent.RunInput) (agent.RunResult, error) {
	// Stub script: writes its own pid and the child's pid, then sleeps
	// forever. trap '' TERM so SIGTERM is ignored — the test verifies
	// the SIGKILL-on-pgid escalation reaches everything.
	script := `
trap '' TERM
sleep 60 &
CHILD=$!
echo "$$ $CHILD" > ` + r.childPIDFile + `
wait $CHILD
`
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return agent.RunResult{ExitCode: -1, Err: err}, err
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return agent.RunResult{ExitCode: -1, Err: err}, err
	}
	if in.OnStarted != nil {
		in.OnStarted(cmd.Process.Pid, cmd.Process.Pid)
	}
	// Drain stdout into the writer so the runner stays alive until the
	// process is killed.
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 && in.Stdout != nil {
				_, _ = in.Stdout.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return agent.RunResult{ExitCode: exitErr.ExitCode()}, nil
		}
		return agent.RunResult{ExitCode: -1, Err: err}, err
	}
	return agent.RunResult{ExitCode: 0}, nil
}

func TestCancelRun_KillsProcessAndWritesCancelledRecord(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only signals")
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
	if err := os.WriteFile(filepath.Join(boardDir, "backlog", "TB-1.md"), []byte(taskBody), 0o644); err != nil {
		t.Fatalf("task md: %v", err)
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

	pidFile := filepath.Join(t.TempDir(), "pids")
	runner := &realProcessRunner{name: "claude", childPIDFile: pidFile}
	svc.setRunnerFactory(func(name string) (agent.Runner, error) { return runner, nil })

	runID, err := svc.RunAgent(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}

	// Wait for the script to write its pids.
	var parentPID, childPID int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil && len(data) > 0 {
			parts := strings.Fields(string(data))
			if len(parts) == 2 {
				parentPID = atoi(strings.TrimSpace(parts[0]))
				childPID = atoi(strings.TrimSpace(parts[1]))
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if parentPID == 0 || childPID == 0 {
		t.Fatalf("pids not written; got %q", readFileSafe(pidFile))
	}

	// Now cancel. Expect both pids to die.
	start := time.Now()
	if err := svc.CancelRun(context.Background(), "TB-1"); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 8*time.Second {
		t.Errorf("CancelRun took too long: %v", elapsed)
	}

	// Give the kernel a beat to reap.
	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(parentPID, 0); err == nil {
		t.Errorf("parent pid %d still alive after CancelRun", parentPID)
	}
	if err := syscall.Kill(childPID, 0); err == nil {
		t.Errorf("child pid %d still alive after CancelRun — pgid kill didn't cascade", childPID)
	}

	// JSONL: exactly one `finished` line for this run_id, with cancelled status.
	events := readEvents(t, boardDir, "TB-1")
	finished := 0
	for _, ev := range events {
		if ev.RunID == runID && ev.Event == agent.EvFinished {
			finished++
			if ev.Status != agent.StatusCancelled {
				t.Errorf("finished event status: %s", ev.Status)
			}
			if ev.Reason != "user cancelled" {
				t.Errorf("finished event reason: %q", ev.Reason)
			}
		}
	}
	if finished != 1 {
		t.Errorf("got %d finished events for run %s, want 1 (TB-47 post-run handler should have been suppressed)", finished, runID)
	}

	// AgentStatus on disk reads as cancelled.
	taskBytes, _ := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
	if !strings.Contains(string(taskBytes), "**AgentStatus:** cancelled") {
		t.Errorf("AgentStatus not cancelled:\n%s", taskBytes)
	}

	// Idempotent: second cancel returns ErrNotRunning.
	if err := svc.CancelRun(context.Background(), "TB-1"); !errors.Is(err, ErrNotRunning) {
		t.Errorf("second cancel: want ErrNotRunning, got %v", err)
	}

	// Wails sink saw run-finished{cancelled}.
	sawFinished := false
	for _, ev := range em.snapshot() {
		if ev.Name == "agent:run-finished" {
			sawFinished = true
			if payload, ok := ev.Payload[0].(map[string]any); ok {
				if payload["status"] != string(agent.StatusCancelled) {
					t.Errorf("Wails finished payload status: %v", payload["status"])
				}
			}
		}
	}
	if !sawFinished {
		t.Error("Wails did not see agent:run-finished")
	}

	// Simulate restart: new AgentService instance against same board. Its
	// startup logic must not flip AgentStatus away from `cancelled`. M4
	// doesn't have explicit startup logic yet (that's M5's daemon recovery),
	// but the carve-out lives in the design: a fresh service reading the
	// task should report cancelled.
	fresh := NewAgentService(AgentServiceOptions{Board: board, Emitter: em})
	freshDetail, err := board.GetTask(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("fresh GetTask: %v", err)
	}
	if freshDetail.Metadata.AgentStatus != "cancelled" {
		t.Errorf("fresh service reads AgentStatus: %s, want cancelled", freshDetail.Metadata.AgentStatus)
	}
	_ = fresh
}

func TestCancelRun_NotRunning(t *testing.T) {
	svc, _ := realTbBoardForRun(t, "", nil)
	err := svc.CancelRun(context.Background(), "TB-1")
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("want ErrNotRunning, got %v", err)
	}
}

// TestCancelRun_BeforeOnStarted exercises the race where CancelRun fires
// after RunAgent's synchronous return but before the runner's OnStarted
// callback has populated Pid/Pgid. The fix in agent_run.go skips writing
// `running` when wasCancelled() is true at OnStarted time, so the final
// AgentStatus on disk must be `cancelled`, not `running`.
func TestCancelRun_BeforeOnStarted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only signals")
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
	if err := os.WriteFile(filepath.Join(boardDir, "backlog", "TB-1.md"), []byte(taskBody), 0o644); err != nil {
		t.Fatalf("task md: %v", err)
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

	// Slow-start runner: holds the OnStarted callback until the test
	// releases it, giving us a guaranteed window to fire CancelRun before
	// Pid/Pgid are written.
	gate := make(chan struct{})
	stub := &slowStartRunner{name: "claude", release: gate}
	svc.setRunnerFactory(func(name string) (agent.Runner, error) { return stub, nil })

	if _, err := svc.RunAgent(context.Background(), "TB-1"); err != nil {
		t.Fatalf("RunAgent: %v", err)
	}

	// Cancel BEFORE OnStarted fires. At this point Pid/Pgid are still 0
	// in activeRun.
	if err := svc.CancelRun(context.Background(), "TB-1"); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}

	// Release the runner so any lingering goroutine work completes.
	close(gate)
	time.Sleep(200 * time.Millisecond)

	// AgentStatus on disk must be `cancelled`, not `running` (the bug
	// the fix prevents).
	taskBytes, _ := os.ReadFile(filepath.Join(boardDir, "backlog", "TB-1.md"))
	if !strings.Contains(string(taskBytes), "**AgentStatus:** cancelled") {
		t.Fatalf("AgentStatus not cancelled after cancel-before-OnStarted:\n%s", taskBytes)
	}
	if strings.Contains(string(taskBytes), "**AgentStatus:** running") {
		t.Fatalf("AgentStatus left as running — running write raced cancel:\n%s", taskBytes)
	}
}

// slowStartRunner blocks inside OnStarted until release is closed. Used by
// TestCancelRun_BeforeOnStarted to deterministically reproduce the
// cancel-before-OnStarted window.
type slowStartRunner struct {
	name    string
	release chan struct{}
}

func (r *slowStartRunner) Name() string { return r.name }
func (r *slowStartRunner) Run(ctx context.Context, in agent.RunInput) (agent.RunResult, error) {
	// Wait for the test to give us permission to "start" the process.
	select {
	case <-r.release:
	case <-ctx.Done():
		return agent.RunResult{ExitCode: -1, Err: ctx.Err()}, ctx.Err()
	}
	if in.OnStarted != nil {
		in.OnStarted(99999, 99999)
	}
	return agent.RunResult{ExitCode: 0}, nil
}

func readFileSafe(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}
