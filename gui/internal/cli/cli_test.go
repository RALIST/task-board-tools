package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeStub creates an executable shell script in dir and returns its path.
// Caller is responsible for cleaning up dir.
func writeStub(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs require a POSIX shell")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return path
}

func TestNewClient_BinaryNotFound(t *testing.T) {
	if _, err := NewClient(Options{BinaryPath: "/nonexistent/no-such-tb-binary-xyz"}); !errors.Is(err, ErrBinaryNotFound) {
		t.Fatalf("want ErrBinaryNotFound, got %v", err)
	}
}

func TestRun_HappyPath(t *testing.T) {
	dir := t.TempDir()
	stub := writeStub(t, dir, "tb", `echo '{"hello":"world"}'`)

	c, err := NewClient(Options{BinaryPath: stub})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	out, err := c.Run(context.Background(), "ls", "--json")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(string(out), `"hello":"world"`) {
		t.Fatalf("unexpected stdout: %s", out)
	}
}

func TestRunJSON_DecodesStdout(t *testing.T) {
	dir := t.TempDir()
	stub := writeStub(t, dir, "tb", `echo '[{"id":"PR-1","title":"Test"}]'`)

	c, err := NewClient(Options{BinaryPath: stub})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	var got []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := c.RunJSON(context.Background(), &got, "ls", "--json"); err != nil {
		t.Fatalf("RunJSON: %v", err)
	}
	if len(got) != 1 || got[0].ID != "PR-1" || got[0].Title != "Test" {
		t.Fatalf("unexpected decoded: %+v", got)
	}
}

func TestRunJSON_EmptyStdoutIsError(t *testing.T) {
	dir := t.TempDir()
	stub := writeStub(t, dir, "tb", `exit 0`)

	c, err := NewClient(Options{BinaryPath: stub})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	var got []any
	err = c.RunJSON(context.Background(), &got, "ls", "--json")
	if err == nil || !strings.Contains(err.Error(), "empty stdout") {
		t.Fatalf("want empty-stdout error, got %v", err)
	}
}

func TestRun_NonZeroExitReturnsExitError(t *testing.T) {
	dir := t.TempDir()
	stub := writeStub(t, dir, "tb", `echo "boom" 1>&2; exit 7`)

	c, err := NewClient(Options{BinaryPath: stub})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = c.Run(context.Background(), "fail")
	var exit *ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("want *ExitError, got %T %v", err, err)
	}
	if exit.Code != 7 {
		t.Fatalf("exit code: want 7, got %d", exit.Code)
	}
	if !strings.Contains(exit.Stderr, "boom") {
		t.Fatalf("stderr not captured: %q", exit.Stderr)
	}
}

func TestRun_ContextCancellationKillsProcess(t *testing.T) {
	dir := t.TempDir()
	// Sleeps for 10s — we cancel after 50ms; must return promptly.
	stub := writeStub(t, dir, "tb", `sleep 10`)

	c, err := NewClient(Options{BinaryPath: stub})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = c.Run(ctx, "slow")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want context error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("want context error, got %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("process not killed: elapsed=%v", elapsed)
	}
}

func TestRun_CwdIsRespected(t *testing.T) {
	stubDir := t.TempDir()
	workDir := t.TempDir()
	stub := writeStub(t, stubDir, "tb", `pwd`)

	c, err := NewClient(Options{BinaryPath: stub, Cwd: workDir})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	out, err := c.Run(context.Background(), "pwd")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := strings.TrimSpace(string(out))
	// macOS prefixes /private to TMPDIR; tolerate that.
	want := strings.TrimPrefix(workDir, "/private")
	if got != workDir && got != want && !strings.HasSuffix(got, want) {
		t.Fatalf("cwd not respected: got %q want %q", got, workDir)
	}
}

func TestExitError_String(t *testing.T) {
	e := &ExitError{Args: []string{"ls", "--json"}, Code: 2, Stderr: "bad arg"}
	if !strings.Contains(e.Error(), "exit 2") || !strings.Contains(e.Error(), "bad arg") {
		t.Fatalf("ExitError.Error() = %q", e.Error())
	}
}
