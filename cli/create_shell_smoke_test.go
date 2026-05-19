package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	smokeBinOnce sync.Once
	smokeBinPath string
	smokeBinErr  error
)

// buildSmokeBinary compiles the tb CLI into a tempdir once per `go test` run.
// Returns the absolute path to the binary so subprocess smoke tests can exec it.
func buildSmokeBinary(t *testing.T) string {
	t.Helper()

	smokeBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "tb-smoke-bin-*")
		if err != nil {
			smokeBinErr = err
			return
		}
		bin := filepath.Join(dir, "tb")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", bin, ".")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			smokeBinErr = err
			return
		}
		smokeBinPath = bin
	})

	if smokeBinErr != nil {
		t.Fatalf("build tb binary: %v", smokeBinErr)
	}
	return smokeBinPath
}

// TestCreateShellSmoke_LiteralBackticksWithSafeQuoting drives the real `tb`
// binary through `/bin/sh -c` to verify the safe quoting recipe documented in
// `tb create --help`: single quotes around values containing Markdown command
// spans. The shell must NOT perform command substitution on the backticks,
// and `tb show` must round-trip the literal characters.
//
// This complements TestCreatePreservesLiteralBackticks, which proves argv-level
// preservation. The smoke test proves the documented end-to-end recipe.
func TestCreateShellSmoke_LiteralBackticksWithSafeQuoting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("smoke test relies on POSIX /bin/sh quoting")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("sh not on PATH: %v", err)
	}

	bin := buildSmokeBinary(t)

	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, status := range []string{"backlog", "in-progress", "code-review", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(boardDir, status), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", status, err)
		}
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("1\n"), 0644); err != nil {
		t.Fatalf("write .next-id: %v", err)
	}
	// Minimal .tb.yaml so tb discovers the board via the configured path.
	tbYAML := "board_path: board\nprefix: TB\n"
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte(tbYAML), 0644); err != nil {
		t.Fatalf("write .tb.yaml: %v", err)
	}

	// Safe quoting recipe: single quotes preserve backticks literally.
	// The double-quoted form (which the user reported as broken) would let
	// /bin/sh run `tb init` BEFORE tb's argv is constructed — losing the
	// literal text. The recipe below is the one we document in --help.
	createScript := strings.Join([]string{
		bin + " create " +
			"'Try to init board with `tb init` and check if command passes' " +
			"-m cli " +
			"-d 'Some description included command `tb --help`'",
	}, " && ")

	runSh := func(script string) (string, string, error) {
		cmd := exec.Command("sh", "-c", script)
		cmd.Dir = root
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}

	if _, stderr, err := runSh(createScript); err != nil {
		t.Fatalf("create script failed: %v\nstderr: %s", err, stderr)
	}

	showStdout, showStderr, err := runSh(bin + " show 1")
	if err != nil {
		t.Fatalf("show command failed: %v\nstderr: %s", err, showStderr)
	}

	if !strings.Contains(showStdout, "`tb init`") {
		t.Fatalf("show output missing literal `tb init`:\n%s", showStdout)
	}
	if !strings.Contains(showStdout, "`tb --help`") {
		t.Fatalf("show output missing literal `tb --help`:\n%s", showStdout)
	}
	if !strings.Contains(showStdout, "Try to init board with `tb init` and check if command passes") {
		t.Fatalf("show output missing literal title:\n%s", showStdout)
	}
	if !strings.Contains(showStdout, "Some description included command `tb --help`") {
		t.Fatalf("show output missing literal description:\n%s", showStdout)
	}
}
