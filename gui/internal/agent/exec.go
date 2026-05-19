package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// runExternal is the shared spawn/stream/wait pipeline that both
// ClaudeRunner and CodexRunner sit on top of. Centralising the process
// machinery here keeps the per-runner files focused on argv construction.
//
// Contract (matches TB-44 acceptance):
//   - cwd = RunInput.ProjectRoot
//   - env = whitelist + caller-passed extras
//   - SysProcAttr.Setpgid = true so SIGKILL on the pgid cascades to children
//   - cmd.Start succeeds → OnStarted(pid, pgid) fires synchronously, BEFORE
//     any output is forwarded
//   - stdout/stderr scanned line-by-line, forwarded to RunInput writers
//   - ctx.Done → SIGTERM (Runner does NOT escalate to SIGKILL; that's TB-48
//     reaching into the process group via the activeRun cancel path)
//   - RunInput.Timeout reached → unattended-timeout path: SIGTERM, 5s grace,
//     SIGKILL on the pgid; returns ErrTimeout
//   - direct parent process wait return → RunResult{ExitCode, Err}; Err
//     non-nil only for the reasons documented on the Runner type (spawn
//     failure, IO, timeout, ctx cancellation).
//   - direct parent process exit is authoritative: descendants that inherit
//     stdout/stderr get postExitPipeDrainGrace to close the pipes naturally
//     before the runner closes its read ends and records terminal state.
const postExitPipeDrainGrace = 1 * time.Second

func runExternal(ctx context.Context, in RunInput, binary string, args []string) (RunResult, error) {
	if in.ProjectRoot == "" {
		return RunResult{ExitCode: -1, Err: errors.New("runExternal: empty ProjectRoot")}, errors.New("runExternal: empty ProjectRoot")
	}
	if _, err := exec.LookPath(binary); err != nil {
		return RunResult{ExitCode: -1, Err: ErrBinaryNotFound}, ErrBinaryNotFound
	}

	cmd := exec.Command(binary, args...)
	cmd.Dir = in.ProjectRoot
	cmd.Env = buildEnv(in.Env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return RunResult{ExitCode: -1, Err: err}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return RunResult{ExitCode: -1, Err: err}, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		// Convert os/exec's wrapped not-found into our sentinel so the
		// caller can branch on errors.Is(err, ErrBinaryNotFound).
		var pe *exec.Error
		if errors.As(err, &pe) || strings.Contains(err.Error(), "executable file not found") {
			return RunResult{ExitCode: -1, Err: ErrBinaryNotFound}, ErrBinaryNotFound
		}
		return RunResult{ExitCode: -1, Err: err}, fmt.Errorf("start: %w", err)
	}

	pid := cmd.Process.Pid
	// pgid equals pid only when Setpgid actually took effect. Verify via
	// the kernel rather than assuming, so a future regression (or a
	// platform where Setpgid silently fails) can't make us issue a
	// `kill -<pgid>` against an unrelated process group.
	pgid, gErr := syscall.Getpgid(pid)
	if gErr != nil || pgid != pid {
		// Conservative: zero out pgid so killers fall back to the leader
		// signal only. Better to leave a stranded grandchild than to
		// SIGKILL the wrong group.
		pgid = 0
	}

	if in.OnStarted != nil {
		in.OnStarted(pid, pgid)
	}

	// Cancellation goroutines:
	//
	//   - ctxCancel watches ctx.Done() and delivers ONE SIGTERM. It does
	//     not escalate; AgentService.CancelRun owns the SIGKILL on pgid.
	//
	//   - deadline (only when Timeout > 0) implements the unattended-run
	//     escalation: SIGTERM → wait 5s → SIGKILL on the pgid.
	//
	// They cooperate via a processDone channel so neither lingers past the
	// direct parent process exit.
	processDone := make(chan struct{})
	var timedOut atomic_bool

	go func() {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Signal(syscall.SIGTERM)
		case <-processDone:
		}
	}()

	if in.Timeout > 0 {
		go func() {
			timer := time.NewTimer(in.Timeout)
			defer timer.Stop()

			select {
			case <-timer.C:
				timedOut.set(true)
				_ = cmd.Process.Signal(syscall.SIGTERM)

				killTimer := time.NewTimer(5 * time.Second)
				defer killTimer.Stop()
				select {
				case <-killTimer.C:
					// pgid==0 means Setpgid didn't take; fall back to
					// the leader signal only rather than risk killing
					// the wrong process group.
					killProcessGroupOrLeader(pgid, cmd.Process, syscall.SIGKILL)
				case <-processDone:
				}
			case <-processDone:
			}
		}()
	}

	// Stream stdout and stderr concurrently. Each scanner gets a generous
	// max-line buffer so long agent output (long tool-use payloads, JSON
	// blobs) doesn't break scanning.
	var streamWG sync.WaitGroup
	streamWG.Add(2)
	var streamErr atomic_error
	go func() {
		defer streamWG.Done()
		if err := streamLines(stdoutPipe, in.Stdout); err != nil && !errors.Is(err, os.ErrClosed) {
			streamErr.set(fmt.Errorf("stdout: %w", err))
		}
	}()
	go func() {
		defer streamWG.Done()
		if err := streamLines(stderrPipe, in.Stderr); err != nil && !errors.Is(err, os.ErrClosed) {
			streamErr.set(fmt.Errorf("stderr: %w", err))
		}
	}()

	streamDone := make(chan struct{})
	go func() {
		streamWG.Wait()
		close(streamDone)
	}()

	waitState, waitErr := cmd.Process.Wait()
	if waitState != nil {
		cmd.ProcessState = waitState
	}
	close(processDone)
	waitForStreamsAfterParentExit(streamDone, timedOut.get(), pgid, cmd.Process, stdoutPipe, stderrPipe)

	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	// Map waitErr / ctx.Err() / timeout to our sentinels.
	switch {
	case timedOut.get():
		return RunResult{ExitCode: -1, Err: ErrTimeout}, ErrTimeout
	case ctx.Err() != nil:
		return RunResult{ExitCode: -1, Err: ctx.Err()}, ctx.Err()
	case streamErr.get() != nil:
		return RunResult{ExitCode: exitCode, Err: streamErr.get()}, streamErr.get()
	case waitErr != nil:
		return RunResult{ExitCode: exitCode, Err: waitErr}, waitErr
	case exitCode != 0:
		// Exit-code-only failure (non-zero exit or signal termination).
		// Surface as RunResult, NOT as an error — a failed agent run is not
		// a Runner bug.
		return RunResult{ExitCode: exitCode, Err: nil}, nil
	default:
		return RunResult{ExitCode: exitCode, Err: nil}, nil
	}
}

func waitForStreamsAfterParentExit(streamDone <-chan struct{}, timedOut bool, pgid int, process *os.Process, pipes ...io.Closer) {
	select {
	case <-streamDone:
		closePipes(pipes...)
		return
	case <-time.After(postExitPipeDrainGrace):
		if timedOut {
			killProcessGroupOrLeader(pgid, process, syscall.SIGKILL)
		}
		closePipes(pipes...)
		<-streamDone
	}
}

func closePipes(pipes ...io.Closer) {
	for _, pipe := range pipes {
		if pipe != nil {
			_ = pipe.Close()
		}
	}
}

func killProcessGroupOrLeader(pgid int, process *os.Process, sig syscall.Signal) {
	if pgid > 0 {
		_ = syscall.Kill(-pgid, sig)
		return
	}
	if process != nil {
		_ = process.Signal(sig)
	}
}

// streamLines reads `src` line-by-line and forwards each line followed by a
// newline to `dst`. Returns when EOF is reached or dst returns an error. A
// nil dst is treated as io.Discard so callers can pass nil to suppress one
// of the two streams.
func streamLines(src io.Reader, dst io.Writer) error {
	if dst == nil {
		dst = io.Discard
	}
	sc := bufio.NewScanner(src)
	// 16 MiB max line — a single Claude stream-json `tool_result` can carry
	// a full file or a long Bash output in one line. 1 MiB used to truncate
	// the scan mid-run. 16 MiB still bounds memory if a stream goes rogue.
	sc.Buffer(make([]byte, 0, 64*1024), 1<<24)
	for sc.Scan() {
		line := append(sc.Bytes(), '\n')
		if _, err := dst.Write(line); err != nil {
			return err
		}
	}
	return sc.Err()
}

// envWhitelist is the minimum set of env vars we always forward to the
// agent. Ambient secrets like OPENAI_API_KEY are intentionally excluded —
// callers who want them set them explicitly via RunInput.Env.
var envWhitelist = []string{
	"HOME", "PATH", "USER", "LANG", "TERM",
}

func buildEnv(extra []string) []string {
	env := make([]string, 0, len(envWhitelist)+len(extra)+8)
	for _, k := range envWhitelist {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	// LC_* is a prefix family — forward every LC_* the parent has.
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "LC_") {
			env = append(env, kv)
		}
	}
	env = append(env, extra...)
	return env
}

// --- tiny lock-free helpers (avoid pulling sync/atomic for two scalars) ---

type atomic_bool struct {
	mu sync.Mutex
	v  bool
}

func (a *atomic_bool) set(v bool) { a.mu.Lock(); a.v = v; a.mu.Unlock() }
func (a *atomic_bool) get() bool  { a.mu.Lock(); defer a.mu.Unlock(); return a.v }

type atomic_error struct {
	mu sync.Mutex
	v  error
}

func (a *atomic_error) set(err error) { a.mu.Lock(); a.v = err; a.mu.Unlock() }
func (a *atomic_error) get() error    { a.mu.Lock(); defer a.mu.Unlock(); return a.v }
