package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestPidAlive_NegativeOrZero(t *testing.T) {
	if pidAlive(0, "claude") {
		t.Errorf("pid=0 should be dead")
	}
	if pidAlive(-1, "claude") {
		t.Errorf("pid=-1 should be dead")
	}
}

func TestPidAlive_DeadPID(t *testing.T) {
	// Spawn `true` and reap it so we have a confidently-dead pid.
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true not on PATH")
	}
	cmd := exec.Command(truePath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait()
	// Give the kernel a moment to fully clean up the entry.
	time.Sleep(50 * time.Millisecond)

	if pidAlive(pid, "anything") {
		t.Errorf("reaped pid %d reported alive", pid)
	}
}

// helperBinaryNamed compiles a tiny Go program at $TMPDIR/<name> that
// blocks on a signal, so we can spawn it under a chosen basename and
// test the comm-match path without depending on system binaries.
func helperBinaryNamed(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(`package main
import (
	"os"
	"os/signal"
	"syscall"
)
func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	<-c
}
`), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	out := filepath.Join(dir, name)
	build := exec.Command("go", "build", "-o", out, src)
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build helper: %v", err)
	}
	return out
}

func TestPidAlive_CommMatchesExact(t *testing.T) {
	bin := helperBinaryNamed(t, "claude")
	cmd := exec.Command(bin)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_, _ = cmd.Process.Wait()
	})
	// `ps` may take a moment to see the process.
	time.Sleep(100 * time.Millisecond)

	if !pidAlive(cmd.Process.Pid, "claude") {
		t.Errorf("alive process with comm=claude not reported alive")
	}
	if pidAlive(cmd.Process.Pid, "claude2") {
		t.Errorf("claude2 should NOT match claude")
	}
	if pidAlive(cmd.Process.Pid, "claude-bin") {
		t.Errorf("claude-bin should NOT match claude")
	}
}

func TestPidAlive_ArgsFallback(t *testing.T) {
	// Simulate a node shebang invocation: the process is `node` but its
	// argv[1] is a path whose basename matches `claude`.
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; skipping shebang fallback test")
	}
	// A no-op JS that listens for SIGTERM.
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "claude")
	if err := os.WriteFile(wrapper, []byte("setInterval(()=>{}, 1000);\n"), 0o644); err != nil {
		t.Fatalf("write js: %v", err)
	}
	cmd := exec.Command(node, wrapper)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_, _ = cmd.Process.Wait()
	})
	time.Sleep(100 * time.Millisecond)

	if !pidAlive(pid, "claude") {
		t.Errorf("pid %d (node %s) should match via args fallback", pid, wrapper)
	}
	if pidAlive(pid, "codex") {
		t.Errorf("pid %d (node %s) should not match unrelated agent codex", pid, wrapper)
	}
}

// TestPidAlive_RaceWith_Reaping is a sanity sweep — many concurrent
// pidAlive calls on the same live pid.
func TestPidAlive_Concurrent(t *testing.T) {
	bin := helperBinaryNamed(t, "claude")
	cmd := exec.Command(bin)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_, _ = cmd.Process.Wait()
	})
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pidAlive(cmd.Process.Pid, "claude")
		}()
	}
	wg.Wait()
	_ = strconv.Itoa(0) // keep strconv import alive in case other helpers shrink
}
