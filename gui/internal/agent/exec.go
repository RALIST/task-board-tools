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
//   - cmd.Wait return → RunResult{ExitCode, Err}; Err non-nil only for the
//     reasons documented on the Runner type (spawn failure, IO, timeout,
//     ctx cancellation).
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
	// They cooperate via a done channel so neither lingers past Wait.
	done := make(chan struct{})
	var timedOut atomic_bool

	go func() {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Signal(syscall.SIGTERM)
		case <-done:
		}
	}()

	if in.Timeout > 0 {
		go func() {
			select {
			case <-time.After(in.Timeout):
				timedOut.set(true)
				_ = cmd.Process.Signal(syscall.SIGTERM)
				select {
				case <-time.After(5 * time.Second):
					// pgid==0 means Setpgid didn't take; fall back to
					// the leader signal only rather than risk killing
					// the wrong process group.
					if pgid > 0 {
						_ = syscall.Kill(-pgid, syscall.SIGKILL)
					} else {
						_ = cmd.Process.Signal(syscall.SIGKILL)
					}
				case <-done:
				}
			case <-done:
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

	streamWG.Wait()
	waitErr := cmd.Wait()
	close(done)

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
		// Exit-code-only failure (non-zero exit). Surface as RunResult,
		// NOT as an error — non-zero exit is a failed run, not a Runner
		// bug. The "err" return value is nil so the caller doesn't think
		// the Runner itself broke.
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return RunResult{ExitCode: exitCode, Err: nil}, nil
		}
		return RunResult{ExitCode: exitCode, Err: waitErr}, waitErr
	default:
		return RunResult{ExitCode: exitCode, Err: nil}, nil
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
	// 1MiB max line — long enough for agent JSON tool-use blobs, short
	// enough that a runaway stream can't OOM the process.
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
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
