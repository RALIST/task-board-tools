package agent

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// makeStubScript writes a /bin/sh script under tmp/<name> and returns the
// containing dir (so callers can prepend it to PATH).
func makeStubScript(t *testing.T, name, body string) string {
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

// withPATH prepends extraDir to PATH for the duration of the test.
func withPATH(t *testing.T, extraDir string) {
	t.Helper()
	orig := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", orig) })
	if err := os.Setenv("PATH", extraDir+string(os.PathListSeparator)+orig); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
}

func TestRunExternal_StreamsStdoutAndStderr(t *testing.T) {
	dir := makeStubScript(t, "fakebin", `
echo out-1
echo err-1 1>&2
echo out-2
echo err-2 1>&2
exit 0
`)
	withPATH(t, dir)

	var out, errb bytes.Buffer
	startedPID := 0
	res, err := runExternal(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Stdout:      &out,
		Stderr:      &errb,
		OnStarted:   func(pid, pgid int) { startedPID = pid },
	}, "fakebin", nil)
	if err != nil {
		t.Fatalf("runExternal: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code: %d", res.ExitCode)
	}
	if startedPID == 0 {
		t.Error("OnStarted was not invoked")
	}
	if !strings.Contains(out.String(), "out-1") || !strings.Contains(out.String(), "out-2") {
		t.Errorf("stdout missing lines: %q", out.String())
	}
	if !strings.Contains(errb.String(), "err-1") || !strings.Contains(errb.String(), "err-2") {
		t.Errorf("stderr missing lines: %q", errb.String())
	}
}

func TestRunExternal_NonZeroExit(t *testing.T) {
	dir := makeStubScript(t, "fakebin", `
echo about-to-fail
exit 7
	`)
	withPATH(t, dir)

	var out bytes.Buffer
	res, err := runExternal(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Stdout:      &out,
	}, "fakebin", nil)
	if err != nil {
		t.Fatalf("non-zero exit should not be an Err: %v", err)
	}
	if res.ExitCode != 7 {
		t.Errorf("exit code: %d, want 7", res.ExitCode)
	}
}

func TestRunExternal_ReturnsWhenChildInheritsOutputPipes(t *testing.T) {
	childPIDFile := filepath.Join(t.TempDir(), "child.pid")
	dir := makeStubScript(t, "fakebin", `
( trap "" HUP; exec sleep 30 ) &
printf '%s\n' "$!" > `+childPIDFile+`
echo parent-done
exit 0
	`)
	withPATH(t, dir)
	t.Cleanup(func() { killPIDFromFile(t, childPIDFile) })

	var out bytes.Buffer
	type runResult struct {
		res RunResult
		err error
	}
	resCh := make(chan runResult, 1)
	start := time.Now()
	go func() {
		res, err := runExternal(context.Background(), RunInput{
			ProjectRoot: t.TempDir(),
			Stdout:      &out,
		}, "fakebin", nil)
		resCh <- runResult{res: res, err: err}
	}()

	select {
	case got := <-resCh:
		if got.err != nil {
			t.Fatalf("runExternal: %v", got.err)
		}
		if got.res.ExitCode != 0 {
			t.Fatalf("exit code: got %d, want 0", got.res.ExitCode)
		}
		if !strings.Contains(out.String(), "parent-done") {
			t.Fatalf("stdout missing parent output: %q", out.String())
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("runExternal returned after %v; want bounded completion after parent exit", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runExternal blocked waiting for stdout/stderr EOF from inherited child pipes")
	}
}

func TestRunExternal_BinaryNotFound(t *testing.T) {
	// Empty PATH so the lookup fails.
	t.Setenv("PATH", t.TempDir())
	res, err := runExternal(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
	}, "nope-not-installed-anywhere", nil)
	if !errors.Is(err, ErrBinaryNotFound) {
		t.Fatalf("want ErrBinaryNotFound, got %v", err)
	}
	if res.ExitCode != -1 {
		t.Errorf("exit code: %d", res.ExitCode)
	}
}

func TestRunExternal_ContextCancelDeliversSIGTERM(t *testing.T) {
	// Use `exec sleep` so the shell process replaces itself with sleep —
	// this way SIGTERM delivered to the leader reaches the actual blocking
	// syscall and we don't have to worry about sh ignoring/proxying signals.
	dir := makeStubScript(t, "fakebin", `
echo started
exec sleep 30
`)
	withPATH(t, dir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startedCh := make(chan struct{}, 1)
	var out bytes.Buffer
	stdoutWrapper := &flushWriter{wrote: startedCh, buf: &out}

	resCh := make(chan struct {
		res RunResult
		err error
	}, 1)
	go func() {
		res, err := runExternal(ctx, RunInput{
			ProjectRoot: t.TempDir(),
			Stdout:      stdoutWrapper,
		}, "fakebin", nil)
		resCh <- struct {
			res RunResult
			err error
		}{res, err}
	}()

	select {
	case <-startedCh:
	case <-time.After(3 * time.Second):
		t.Fatal("stub never wrote 'started' — runner not spawning?")
	}

	cancel()

	select {
	case got := <-resCh:
		if !errors.Is(got.err, context.Canceled) {
			t.Errorf("want context.Canceled, got %v", got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not return within 5s after cancel")
	}
}

func TestRunExternal_TimeoutEscalatesToSIGKILL(t *testing.T) {
	// Stub script writes its own PID to a file, then sleeps forever — so
	// we can verify the process group really died.
	pidFile := filepath.Join(t.TempDir(), "pid")
	dir := makeStubScript(t, "fakebin", `
echo $$ > `+pidFile+`
echo started
# Ignore SIGTERM so the runner must escalate to SIGKILL.
trap '' TERM
sleep 30
`)
	withPATH(t, dir)

	startedCh := make(chan struct{}, 1)
	var out bytes.Buffer
	stdoutWrapper := &flushWriter{wrote: startedCh, buf: &out}

	start := time.Now()
	res, err := runExternal(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Stdout:      stdoutWrapper,
		Timeout:     500 * time.Millisecond, // short timeout
	}, "fakebin", nil)
	elapsed := time.Since(start)

	select {
	case <-startedCh:
	default:
		// The stub may have been killed before draining the pipe — that's
		// fine, the SIGKILL escalation is the only thing we're checking.
	}

	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	if res.ExitCode != -1 {
		t.Errorf("exit code: %d", res.ExitCode)
	}
	if elapsed > 8*time.Second {
		t.Errorf("escalation took too long: %v", elapsed)
	}

	// Verify the stub's PID is no longer alive (process group kill should
	// have reached it). syscall.Kill(pid, 0) probes liveness.
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Logf("pid file: %v (stub may not have flushed)", err)
		return
	}
	pid := atoi(strings.TrimSpace(string(data)))
	if pid <= 0 {
		t.Logf("invalid pid: %q", data)
		return
	}
	// Give the OS a beat to reap.
	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		t.Errorf("stub PID %d still alive after timeout — escalation didn't reach pgid", pid)
	}
}

func TestRunExternal_OnStartedFiresBeforeFirstLine(t *testing.T) {
	dir := makeStubScript(t, "fakebin", `
echo line-1
echo line-2
`)
	withPATH(t, dir)

	var onStartedAt time.Time
	var firstLineAt time.Time
	var mu sync.Mutex
	stdoutWrapper := &timestampedWriter{
		onWrite: func() {
			mu.Lock()
			if firstLineAt.IsZero() {
				firstLineAt = time.Now()
			}
			mu.Unlock()
		},
	}

	_, err := runExternal(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Stdout:      stdoutWrapper,
		OnStarted: func(pid, pgid int) {
			mu.Lock()
			onStartedAt = time.Now()
			mu.Unlock()
		},
	}, "fakebin", nil)
	if err != nil {
		t.Fatalf("runExternal: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if onStartedAt.IsZero() {
		t.Fatal("OnStarted never fired")
	}
	if firstLineAt.IsZero() {
		t.Fatal("stdout never written")
	}
	if !onStartedAt.Before(firstLineAt) {
		t.Errorf("OnStarted (%v) should fire before first line (%v)", onStartedAt, firstLineAt)
	}
}

func TestRunExternal_EnvWhitelist(t *testing.T) {
	dir := makeStubScript(t, "fakebin", `
echo HOME=$HOME
echo PATH=$PATH
echo SHOULD_NOT_LEAK=$SHOULD_NOT_LEAK
echo CUSTOM=$CUSTOM
`)
	withPATH(t, dir)
	t.Setenv("SHOULD_NOT_LEAK", "secret123")

	var out bytes.Buffer
	res, err := runExternal(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Stdout:      &out,
		Env:         []string{"CUSTOM=hello"},
	}, "fakebin", nil)
	if err != nil {
		t.Fatalf("runExternal: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit: %d", res.ExitCode)
	}
	s := out.String()
	if strings.Contains(s, "secret123") {
		t.Errorf("non-whitelisted env leaked:\n%s", s)
	}
	if !strings.Contains(s, "CUSTOM=hello") {
		t.Errorf("caller-passed env missing:\n%s", s)
	}
}

func TestClaudeRunner_Name(t *testing.T) {
	if NewClaudeRunner().Name() != "claude" {
		t.Fatal("name mismatch")
	}
}

func TestCodexRunner_Name(t *testing.T) {
	if NewCodexRunner().Name() != "codex" {
		t.Fatal("name mismatch")
	}
}

func TestClaudeRunner_PassesPromptViaDashP(t *testing.T) {
	// Capture argv by writing it to a file; we can't fake exec.LookPath
	// easily so we substitute the "claude" name itself via a stub on PATH.
	argFile := filepath.Join(t.TempDir(), "argv")
	dir := makeStubScript(t, "claude", `
printf '%s\n' "$@" > `+argFile+`
echo ok
`)
	withPATH(t, dir)

	var out bytes.Buffer
	r := NewClaudeRunner()
	res, err := r.Run(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Prompt:      "do the thing",
		Stdout:      &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit: %d", res.ExitCode)
	}
	data, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("argv file: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	want := []string{"-p", "do the thing", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"}
	if len(args) != len(want) {
		t.Fatalf("argv length: got %d want %d (%#v)", len(args), len(want), args)
	}
	for i, w := range want {
		if args[i] != w {
			t.Fatalf("argv[%d]: got %q want %q (full=%#v)", i, args[i], w, args)
		}
	}
}

// TestClaudeRunner_ResumeUsesDashR locks the TB-138 argv: in
// ModeResume + non-empty SessionID, the runner emits `-r <uuid>`
// instead of `--session-id <uuid>`. The two flags are mutually
// exclusive for Claude — passing both would be ambiguous.
func TestClaudeRunner_ResumeUsesDashR(t *testing.T) {
	argFile := filepath.Join(t.TempDir(), "argv")
	dir := makeStubScript(t, "claude", `
printf '%s\n' "$@" > `+argFile+`
echo ok
`)
	withPATH(t, dir)

	var out bytes.Buffer
	r := NewClaudeRunner()
	uuid := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	_, err := r.Run(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Prompt:      "continue",
		Mode:        ModeResume,
		SessionID:   uuid,
		Stdout:      &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("argv file: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	want := []string{"-p", "continue", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions", "-r", uuid}
	if len(args) != len(want) {
		t.Fatalf("argv length: got %d want %d (%#v)", len(args), len(want), args)
	}
	for i, w := range want {
		if args[i] != w {
			t.Fatalf("argv[%d]: got %q want %q (full=%#v)", i, args[i], w, args)
		}
	}
	for _, a := range args {
		if a == "--session-id" {
			t.Fatalf("--session-id must not appear in resume args (mutually exclusive with -r): %#v", args)
		}
	}
}

// TestClaudeRunner_AppendsSessionIDFlag is the TB-135 contract for the
// Claude argv: when RunInput.SessionID is non-empty, `--session-id
// <uuid>` is appended to the args; empty SessionID leaves args
// unchanged.
func TestClaudeRunner_AppendsSessionIDFlag(t *testing.T) {
	argFile := filepath.Join(t.TempDir(), "argv")
	dir := makeStubScript(t, "claude", `
printf '%s\n' "$@" > `+argFile+`
echo ok
`)
	withPATH(t, dir)

	var out bytes.Buffer
	r := NewClaudeRunner()
	uuid := "11111111-2222-4333-8444-555555555555"
	_, err := r.Run(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Prompt:      "do the thing",
		SessionID:   uuid,
		Stdout:      &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("argv file: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	want := []string{"-p", "do the thing", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions", "--session-id", uuid}
	if len(args) != len(want) {
		t.Fatalf("argv length: got %d want %d (%#v)", len(args), len(want), args)
	}
	for i, w := range want {
		if args[i] != w {
			t.Fatalf("argv[%d]: got %q want %q (full=%#v)", i, args[i], w, args)
		}
	}
}

// TestCodexRunner_ResumeArgs is the TB-139 contract: in ModeResume +
// non-empty SessionID, args become `exec --json resume <uuid>
// <prompt>` — codex's documented resume invocation form. Fresh runs
// keep `exec --json <prompt>` from TB-134.
func TestCodexRunner_ResumeArgs(t *testing.T) {
	argFile := filepath.Join(t.TempDir(), "argv")
	dir := makeStubScript(t, "codex", `
printf '%s\n' "$@" > `+argFile+`
echo ok
`)
	withPATH(t, dir)

	var out bytes.Buffer
	r := NewCodexRunner()
	uuid := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	_, err := r.Run(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Prompt:      "continue",
		Mode:        ModeResume,
		SessionID:   uuid,
		Stdout:      &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("argv file: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	want := []string{"exec", "--json", "resume", uuid, "continue"}
	if len(args) != len(want) {
		t.Fatalf("argv length: got %d want %d (%#v)", len(args), len(want), args)
	}
	for i, w := range want {
		if args[i] != w {
			t.Fatalf("argv[%d]: got %q want %q (full=%#v)", i, args[i], w, args)
		}
	}
}

func TestCodexRunner_PassesPromptPositionally(t *testing.T) {
	argFile := filepath.Join(t.TempDir(), "argv")
	dir := makeStubScript(t, "codex", `
printf '%s\n' "$@" > `+argFile+`
echo ok
`)
	withPATH(t, dir)

	var out bytes.Buffer
	r := NewCodexRunner()
	res, err := r.Run(context.Background(), RunInput{
		ProjectRoot: t.TempDir(),
		Prompt:      "do the thing",
		Stdout:      &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit: %d", res.ExitCode)
	}
	data, err := os.ReadFile(argFile)
	if err != nil {
		t.Fatalf("argv file: %v", err)
	}
	args := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	// TB-130: codex exec switched to --json so we can parse session_id
	// from the structured stream (codex doesn't accept a pre-allocated
	// session id like Claude does).
	if len(args) != 3 || args[0] != "exec" || args[1] != "--json" || args[2] != "do the thing" {
		t.Fatalf("argv: %#v", args)
	}
}

// --- helpers ---

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

func killPIDFromFile(t *testing.T, path string) {
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

type flushWriter struct {
	wrote chan<- struct{}
	buf   *bytes.Buffer
	once  sync.Once
}

func (w *flushWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		select {
		case w.wrote <- struct{}{}:
		default:
		}
	})
	return w.buf.Write(p)
}

type timestampedWriter struct {
	onWrite func()
}

func (w *timestampedWriter) Write(p []byte) (int, error) {
	if w.onWrite != nil {
		w.onWrite()
	}
	return len(p), nil
}
