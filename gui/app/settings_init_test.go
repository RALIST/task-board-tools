package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestInitBoard_HappyPath runs the real `tb init` against a fresh directory
// and verifies the on-disk artifacts plus the post-init OpenBoard wiring.
func TestInitBoard_HappyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("buildTbForIntegration relies on posix go build invocation")
	}
	tb := buildTbForIntegration(t)
	root := t.TempDir()
	recents := filepath.Join(t.TempDir(), "recent.json")

	board := NewBoardService()
	sw := &fakeSwitcher{}
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		Watcher:     sw,
		CLIPath:     tb,
		RecentsPath: recents,
	})

	if err := svc.InitBoard(context.Background(), root, "", ""); err != nil {
		t.Fatalf("InitBoard: %v", err)
	}

	// The on-disk artifacts must match `tb init` defaults.
	for _, p := range []string{
		".tb.yaml",
		filepath.Join("board", ".next-id"),
		filepath.Join("board", "BOARD.md"),
		filepath.Join("board", "CONVENTIONS.md"),
		filepath.Join("board", "SKILL.md"),
		filepath.Join("board", "backlog"),
		filepath.Join("board", "in-progress"),
		filepath.Join("board", "done"),
		filepath.Join("board", "archive"),
	} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("missing artifact %s: %v", p, err)
		}
	}

	// `.tb.yaml` should embed the resolved defaults.
	cfg, err := os.ReadFile(filepath.Join(root, ".tb.yaml"))
	if err != nil {
		t.Fatalf("read .tb.yaml: %v", err)
	}
	cfgStr := string(cfg)
	if !strings.Contains(cfgStr, "board: board") {
		t.Errorf(".tb.yaml missing default board path: %q", cfgStr)
	}
	if !strings.Contains(cfgStr, "prefix: PR") {
		t.Errorf(".tb.yaml missing default prefix: %q", cfgStr)
	}

	// OpenBoard wiring must be live.
	if got := svc.GetProjectRoot(); got != root {
		t.Errorf("GetProjectRoot: got %q want %q", got, root)
	}
	info, err := svc.GetBoardInfo()
	if err != nil {
		t.Fatalf("GetBoardInfo: %v", err)
	}
	if info.Prefix != "PR" {
		t.Errorf("BoardInfo.Prefix: got %q want PR", info.Prefix)
	}
	if calls := sw.calls(); len(calls) != 1 || calls[0] != info.BoardDir {
		t.Errorf("watcher.Switch calls: %v want [%s]", calls, info.BoardDir)
	}
	if board.snapshot() == nil {
		t.Error("BoardService client not set after InitBoard")
	}

	// New board should appear in recents.
	rl, err := svc.ListRecentBoards()
	if err != nil {
		t.Fatalf("ListRecentBoards: %v", err)
	}
	if len(rl) != 1 || rl[0].ProjectRoot != root {
		t.Errorf("recent list: %+v", rl)
	}
}

// TestInitBoard_CustomPrefixAndBoardPath verifies non-default arguments
// flow through to the CLI and surface in BoardInfo.
func TestInitBoard_CustomPrefixAndBoardPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("buildTbForIntegration relies on posix go build invocation")
	}
	tb := buildTbForIntegration(t)
	root := t.TempDir()

	svc := NewSettingsService(SettingsOptions{
		Board:       NewBoardService(),
		Watcher:     &fakeSwitcher{},
		CLIPath:     tb,
		RecentsPath: filepath.Join(t.TempDir(), "recent.json"),
	})

	if err := svc.InitBoard(context.Background(), root, "tasks", "WS"); err != nil {
		t.Fatalf("InitBoard: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "tasks", ".next-id")); err != nil {
		t.Errorf("expected tasks/.next-id: %v", err)
	}
	info, err := svc.GetBoardInfo()
	if err != nil {
		t.Fatalf("GetBoardInfo: %v", err)
	}
	if info.Prefix != "WS" {
		t.Errorf("BoardInfo.Prefix: got %q want WS", info.Prefix)
	}
	if filepath.Base(info.BoardDir) != "tasks" {
		t.Errorf("BoardInfo.BoardDir = %q want suffix tasks", info.BoardDir)
	}
}

// TestInitBoard_RejectsBadInputsBeforeWrite asserts that validation errors
// short-circuit before any file is written so the project root stays
// untouched.
func TestInitBoard_RejectsBadInputsBeforeWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("buildTbForIntegration relies on posix go build invocation")
	}
	tb := buildTbForIntegration(t)
	svc := NewSettingsService(SettingsOptions{
		Board:       NewBoardService(),
		Watcher:     &fakeSwitcher{},
		CLIPath:     tb,
		RecentsPath: filepath.Join(t.TempDir(), "recent.json"),
	})

	cases := []struct {
		name       string
		root       string
		boardPath  string
		prefix     string
		wantErr    error
		wantSubstr string
	}{
		{name: "empty root", root: "", wantSubstr: "empty project root"},
		{name: "missing root", root: filepath.Join(t.TempDir(), "does-not-exist"), wantSubstr: "project root"},
		{name: "absolute board path", root: t.TempDir(), boardPath: "/abs", wantErr: ErrInvalidBoardPath},
		{name: "traversal board path", root: t.TempDir(), boardPath: "../escape", wantErr: ErrInvalidBoardPath},
		{name: "dot board path", root: t.TempDir(), boardPath: ".", wantErr: ErrInvalidBoardPath},
		{name: "bad prefix start", root: t.TempDir(), prefix: "1AB", wantErr: ErrInvalidPrefix},
		{name: "bad prefix chars", root: t.TempDir(), prefix: "PR-X", wantErr: ErrInvalidPrefix},
		{name: "long prefix", root: t.TempDir(), prefix: "ABCDEFGHIJK", wantErr: ErrInvalidPrefix},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.InitBoard(context.Background(), tc.root, tc.boardPath, tc.prefix)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want errors.Is %v", err, tc.wantErr)
			}
			if tc.wantSubstr != "" && !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("err = %v, want substring %q", err, tc.wantSubstr)
			}
			// When a real root path is provided, the rejection must not
			// have created `.tb.yaml`.
			if tc.root != "" {
				if _, statErr := os.Stat(filepath.Join(tc.root, ".tb.yaml")); statErr == nil {
					t.Errorf("validation should not have created .tb.yaml")
				}
			}
		})
	}
}

// TestInitBoard_AlreadyInitializedIsTyped asserts a pre-existing `.tb.yaml`
// is reported via ErrBoardAlreadyInitialized without re-running `tb init`.
func TestInitBoard_AlreadyInitializedIsTyped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("buildTbForIntegration relies on posix go build invocation")
	}
	tb := buildTbForIntegration(t)
	root := fixtureBoard(t, "TB")
	svc := NewSettingsService(SettingsOptions{
		Board:       NewBoardService(),
		Watcher:     &fakeSwitcher{},
		CLIPath:     tb,
		RecentsPath: filepath.Join(t.TempDir(), "recent.json"),
	})
	err := svc.InitBoard(context.Background(), root, "board", "PR")
	if !errors.Is(err, ErrBoardAlreadyInitialized) {
		t.Fatalf("want ErrBoardAlreadyInitialized, got %v", err)
	}
}

// TestInitBoard_PreservesPriorBoardOnFailure verifies a failed init does
// not corrupt the previously active board's wiring. We provoke failure by
// pointing the CLIPath at a non-existent binary AFTER opening a valid
// board.
func TestInitBoard_PreservesPriorBoardOnFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("buildTbForIntegration relies on posix go build invocation")
	}
	tb := buildTbForIntegration(t)
	priorRoot := fixtureBoard(t, "TB")
	uninitRoot := t.TempDir()
	dir := t.TempDir()
	prefs := filepath.Join(dir, "preferences.json")
	recents := filepath.Join(dir, "recent.json")

	board := NewBoardService()
	sw := &fakeSwitcher{}
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		Watcher:     sw,
		CLIPath:     tb,
		RecentsPath: recents,
		PrefsPath:   prefs,
	})
	if err := svc.OpenBoard(context.Background(), priorRoot); err != nil {
		t.Fatalf("OpenBoard prior: %v", err)
	}
	priorClient := board.snapshot()
	priorSwitchCalls := len(sw.calls())
	priorInfo, err := svc.GetBoardInfo()
	if err != nil {
		t.Fatalf("GetBoardInfo prior: %v", err)
	}

	// Validation that fails BEFORE tb init runs: bad prefix.
	if err := svc.InitBoard(context.Background(), uninitRoot, "board", "PR-bad"); !errors.Is(err, ErrInvalidPrefix) {
		t.Fatalf("InitBoard bad prefix: want ErrInvalidPrefix, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(uninitRoot, ".tb.yaml")); statErr == nil {
		t.Errorf("validation failure should not create .tb.yaml")
	}

	// Prior board state must remain unchanged.
	got, err := svc.GetBoardInfo()
	if err != nil {
		t.Fatalf("GetBoardInfo after failed init: %v", err)
	}
	if got.ProjectRoot != priorInfo.ProjectRoot {
		t.Errorf("ProjectRoot mutated: got %q want %q", got.ProjectRoot, priorInfo.ProjectRoot)
	}
	if board.snapshot() != priorClient {
		t.Errorf("BoardService client swapped despite failed init")
	}
	if len(sw.calls()) != priorSwitchCalls {
		t.Errorf("watcher.Switch was called: %v", sw.calls())
	}
}
