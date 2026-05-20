package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"tools/tb-gui/internal/cli"
)

// fakeSwitcher captures Switch calls; satisfies the Switcher interface.
type fakeSwitcher struct {
	mu     sync.Mutex
	called []string
	err    error
	hook   func(boardDir string)
}

func (f *fakeSwitcher) Switch(boardDir string) error {
	if f.hook != nil {
		f.hook(boardDir)
	}
	f.mu.Lock()
	f.called = append(f.called, boardDir)
	f.mu.Unlock()
	return f.err
}

func (f *fakeSwitcher) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.called...)
}

// fixtureBoard creates a temp project with .tb.yaml + board/.next-id and the
// four status dirs. Returns the project root.
func fixtureBoard(t *testing.T, prefix string) string {
	t.Helper()
	root := t.TempDir()
	cfg := "board: board\n"
	if prefix != "" {
		cfg += "prefix: " + prefix + "\n"
	}
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	board := filepath.Join(root, "board")
	for _, d := range []string{"backlog", "in-progress", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(board, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(board, ".next-id"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// stubTbBinary writes a fake `tb` binary into a temp dir and returns its path.
func stubTbBinary(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("posix shell stub")
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "tb")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho '[]'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return stub
}

func stubTbBinaryWithMarker(t *testing.T, marker, logPath string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("posix shell stub")
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "tb")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%s\\n' >> %q\necho '[]'\n", marker, logPath)
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return stub
}

func TestReadBoardInfo_HappyPath(t *testing.T) {
	root := fixtureBoard(t, "TB")
	info, err := readBoardInfo(root)
	if err != nil {
		t.Fatalf("readBoardInfo: %v", err)
	}
	if info.ProjectRoot != root {
		t.Errorf("projectRoot: got %q", info.ProjectRoot)
	}
	if info.Prefix != "TB" {
		t.Errorf("prefix: got %q", info.Prefix)
	}
	if !filepath.IsAbs(info.BoardDir) {
		t.Errorf("boardDir not absolute: %q", info.BoardDir)
	}
	if info.WIPLimit != 2 {
		t.Errorf("wipLimit default: got %d", info.WIPLimit)
	}
}

func TestReadBoardInfo_DefaultsPrefix(t *testing.T) {
	root := fixtureBoard(t, "")
	info, err := readBoardInfo(root)
	if err != nil {
		t.Fatalf("readBoardInfo: %v", err)
	}
	if info.Prefix != "PR" {
		t.Errorf("default prefix should be PR, got %q", info.Prefix)
	}
}

func TestReadBoardInfo_NoTbYaml(t *testing.T) {
	root := t.TempDir()
	_, err := readBoardInfo(root)
	if !errors.Is(err, ErrNoTbYaml) {
		t.Fatalf("want ErrNoTbYaml, got %v", err)
	}
}

func TestOpenBoard_HappyPath(t *testing.T) {
	root := fixtureBoard(t, "TB")
	stub := stubTbBinary(t)
	recents := filepath.Join(t.TempDir(), "recent.json")

	board := NewBoardService()
	sw := &fakeSwitcher{}
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		Watcher:     sw,
		CLIPath:     stub,
		RecentsPath: recents,
	})

	if err := svc.OpenBoard(context.Background(), root); err != nil {
		t.Fatalf("OpenBoard: %v", err)
	}

	if got := svc.GetProjectRoot(); got != root {
		t.Errorf("GetProjectRoot: got %q want %q", got, root)
	}
	info, err := svc.GetBoardInfo()
	if err != nil {
		t.Fatalf("GetBoardInfo: %v", err)
	}
	if info.Prefix != "TB" {
		t.Errorf("GetBoardInfo.Prefix = %q", info.Prefix)
	}

	// Watcher should have been pointed at the resolved board dir.
	calls := sw.calls()
	if len(calls) != 1 || calls[0] != info.BoardDir {
		t.Errorf("watcher.Switch calls: %v want [%s]", calls, info.BoardDir)
	}

	// BoardService should now have a client.
	if board.snapshot() == nil {
		t.Error("BoardService client not set after OpenBoard")
	}

	// recent.json should exist with one entry.
	recentList, err := svc.ListRecentBoards()
	if err != nil {
		t.Fatalf("ListRecentBoards: %v", err)
	}
	if len(recentList) != 1 || recentList[0].ProjectRoot != root {
		t.Errorf("recent list: %+v", recentList)
	}
}

func TestOpenBoard_UsesPersistedCLIPath(t *testing.T) {
	root := fixtureBoard(t, "TB")
	dir := t.TempDir()
	prefs := filepath.Join(dir, "preferences.json")
	logPath := filepath.Join(dir, "stub.log")
	stub := stubTbBinaryWithMarker(t, "persisted", logPath)

	board := NewBoardService()
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		RecentsPath: filepath.Join(dir, "recent.json"),
		PrefsPath:   prefs,
	})
	if err := svc.SetCLIPath(stub); err != nil {
		t.Fatalf("SetCLIPath: %v", err)
	}
	if err := svc.OpenBoard(context.Background(), root); err != nil {
		t.Fatalf("OpenBoard: %v", err)
	}
	if _, err := board.LoadBoard(context.Background()); err != nil {
		t.Fatalf("LoadBoard: %v", err)
	}

	// OpenBoard runs `tb ls --json --status active` to validate the
	// candidate board before committing the switch (TB-208). LoadBoard
	// itself fires two invocations under canonical kanban: `tb ls --json
	// --status active` for the buckets plus `tb board --json` for the
	// WIP metadata. Total: 1 (validate) + 2 (load) = 3.
	if got := readMarkerLog(t, logPath); got != "persisted\npersisted\npersisted\n" {
		t.Fatalf("stub log: got %q, want persisted marker three times (validate + LoadBoard ls + LoadBoard board)", got)
	}
}

func TestSetCLIPath_ReloadsActiveBoardClient(t *testing.T) {
	root := fixtureBoard(t, "TB")
	dir := t.TempDir()
	prefs := filepath.Join(dir, "preferences.json")
	logPath := filepath.Join(dir, "stub.log")
	first := stubTbBinaryWithMarker(t, "first", logPath)
	second := stubTbBinaryWithMarker(t, "second", logPath)

	board := NewBoardService()
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		RecentsPath: filepath.Join(dir, "recent.json"),
		PrefsPath:   prefs,
	})
	if err := svc.SetCLIPath(first); err != nil {
		t.Fatalf("SetCLIPath(first): %v", err)
	}
	if err := svc.OpenBoard(context.Background(), root); err != nil {
		t.Fatalf("OpenBoard: %v", err)
	}
	if _, err := board.LoadBoard(context.Background()); err != nil {
		t.Fatalf("LoadBoard first: %v", err)
	}

	if err := svc.SetCLIPath(second); err != nil {
		t.Fatalf("SetCLIPath(second): %v", err)
	}
	if _, err := board.LoadBoard(context.Background()); err != nil {
		t.Fatalf("LoadBoard second: %v", err)
	}

	// "first" is logged three times: OpenBoard's candidate-board validate
	// (TB-208), then LoadBoard's two invocations (ls --json + board
	// --json for WIP metadata). SetCLIPath swaps in the "second" binary,
	// which records the next LoadBoard's two invocations.
	if got := readMarkerLog(t, logPath); got != "first\nfirst\nfirst\nsecond\nsecond\n" {
		t.Fatalf("stub log: got %q, want first validate+load then second", got)
	}
}

func TestSetCLIPath_BadPathDoesNotPersistOrSwapActiveClient(t *testing.T) {
	root := fixtureBoard(t, "TB")
	dir := t.TempDir()
	prefs := filepath.Join(dir, "preferences.json")
	logPath := filepath.Join(dir, "stub.log")
	stub := stubTbBinaryWithMarker(t, "valid", logPath)

	board := NewBoardService()
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		RecentsPath: filepath.Join(dir, "recent.json"),
		PrefsPath:   prefs,
	})
	if err := svc.SetCLIPath(stub); err != nil {
		t.Fatalf("SetCLIPath(stub): %v", err)
	}
	if err := svc.OpenBoard(context.Background(), root); err != nil {
		t.Fatalf("OpenBoard: %v", err)
	}
	if _, err := board.LoadBoard(context.Background()); err != nil {
		t.Fatalf("LoadBoard before bad path: %v", err)
	}

	badPath := filepath.Join(dir, "missing-tb")
	err := svc.SetCLIPath(badPath)
	if !errors.Is(err, cli.ErrBinaryNotFound) {
		t.Fatalf("want ErrBinaryNotFound, got %v", err)
	}
	if got := svc.GetCLIPath(); got != stub {
		t.Fatalf("bad path should not be persisted: got %q, want %q", got, stub)
	}
	if _, err := board.LoadBoard(context.Background()); err != nil {
		t.Fatalf("LoadBoard after bad path: %v", err)
	}
	// Five "valid" entries: OpenBoard's TB-208 candidate validate (1),
	// the pre-bad-path LoadBoard's ls + board --json (2), and the
	// post-bad-path LoadBoard's ls + board --json (2). SetCLIPath
	// rejected the missing binary so the active client stays valid.
	if got := readMarkerLog(t, logPath); got != "valid\nvalid\nvalid\nvalid\nvalid\n" {
		t.Fatalf("stub log: got %q, want active client to remain on valid binary", got)
	}
}

func TestOpenBoard_NoTbYamlIsTyped(t *testing.T) {
	notABoard := t.TempDir()
	svc := NewSettingsService(SettingsOptions{
		CLIPath:     stubTbBinary(t),
		RecentsPath: filepath.Join(t.TempDir(), "recent.json"),
	})
	err := svc.OpenBoard(context.Background(), notABoard)
	if !errors.Is(err, ErrNoTbYaml) {
		t.Fatalf("want ErrNoTbYaml, got %v", err)
	}
	if svc.GetProjectRoot() != "" {
		t.Error("failed OpenBoard should not change active project root")
	}
}

func TestRecentBoards_DedupAndCap(t *testing.T) {
	recents := filepath.Join(t.TempDir(), "recent.json")
	stub := stubTbBinary(t)

	svc := NewSettingsService(SettingsOptions{
		CLIPath:     stub,
		RecentsPath: recents,
	})

	// Open root A twice and root B once. Expect dedup + B-before-A-second on
	// LastOpened sort.
	rootA := fixtureBoard(t, "TB")
	rootB := fixtureBoard(t, "PR")

	mustOpen := func(p string) {
		t.Helper()
		if err := svc.OpenBoard(context.Background(), p); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond) // ensure distinct LastOpened
	}
	mustOpen(rootA)
	mustOpen(rootB)
	mustOpen(rootA)

	list, err := svc.ListRecentBoards()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("dedup failed: %+v", list)
	}
	if list[0].ProjectRoot != rootA {
		t.Errorf("most recent should be rootA: %+v", list)
	}
	if list[1].ProjectRoot != rootB {
		t.Errorf("second should be rootB: %+v", list)
	}
}

func TestRecentBoards_DeadEntriesFilteredOnList(t *testing.T) {
	recents := filepath.Join(t.TempDir(), "recent.json")
	// Pre-seed with a project that doesn't exist on disk.
	preload := []RecentBoard{
		{ProjectRoot: "/this/does/not/exist", Prefix: "X", LastOpened: time.Now()},
	}
	b, _ := json.Marshal(preload)
	if err := os.MkdirAll(filepath.Dir(recents), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recents, b, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewSettingsService(SettingsOptions{
		CLIPath:     stubTbBinary(t),
		RecentsPath: recents,
	})
	list, err := svc.ListRecentBoards()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("dead entries should be filtered: %+v", list)
	}
}

func TestRecentBoards_CorruptFileIsTolerated(t *testing.T) {
	recents := filepath.Join(t.TempDir(), "recent.json")
	if err := os.MkdirAll(filepath.Dir(recents), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recents, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewSettingsService(SettingsOptions{
		CLIPath:     stubTbBinary(t),
		RecentsPath: recents,
	})
	list, err := svc.ListRecentBoards()
	if err != nil {
		t.Fatalf("corrupt file should not error: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("want empty list, got %+v", list)
	}
}

func TestOpenBoard_WatcherFailureDoesNotCommit(t *testing.T) {
	root := fixtureBoard(t, "TB")
	stub := stubTbBinary(t)
	recents := filepath.Join(t.TempDir(), "recent.json")

	sw := &fakeSwitcher{err: errors.New("watcher boom")}
	board := NewBoardService()
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		Watcher:     sw,
		CLIPath:     stub,
		RecentsPath: recents,
	})

	err := svc.OpenBoard(context.Background(), root)
	if err == nil || !errors.Is(err, sw.err) {
		t.Fatalf("want wrapped watcher err, got %v", err)
	}
	if svc.GetProjectRoot() != "" {
		t.Error("OpenBoard must not mutate state on watcher failure")
	}
	if board.snapshot() != nil {
		t.Error("BoardService client must not be set on watcher failure")
	}
}

func TestOpenBoard_WatcherFailureAfterExistingBoardDoesNotDeactivate(t *testing.T) {
	rootA := fixtureBoard(t, "TA")
	rootB := fixtureBoard(t, "TB")
	stub := stubTbBinary(t)
	recents := filepath.Join(t.TempDir(), "recent.json")

	sw := &fakeSwitcher{}
	board := NewBoardService()
	activator := &fakeOpenBoardActivator{}
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		Watcher:     sw,
		CLIPath:     stub,
		RecentsPath: recents,
		Activator:   activator,
	})
	if err := svc.OpenBoard(context.Background(), rootA); err != nil {
		t.Fatalf("OpenBoard A: %v", err)
	}
	activator.mu.Lock()
	activator.calls = nil
	activator.mu.Unlock()
	sw.err = errors.New("watcher boom")

	err := svc.OpenBoard(context.Background(), rootB)
	if err == nil || !errors.Is(err, sw.err) {
		t.Fatalf("want wrapped watcher err, got %v", err)
	}
	if got := svc.GetProjectRoot(); got != rootA {
		t.Fatalf("failed switch changed project root: got %q want %q", got, rootA)
	}
	if calls := activator.callsSnapshot(); len(calls) != 0 {
		t.Fatalf("failed watcher switch deactivated/reactivated current board: %v", calls)
	}
	if board.snapshot() == nil {
		t.Fatalf("existing BoardService client was cleared")
	}
}

type fakeOpenBoardActivator struct {
	mu             sync.Mutex
	calls          []string
	deactivateHook func()
}

func (f *fakeOpenBoardActivator) Activate(ctx context.Context, boardDir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "activate:"+boardDir)
	return nil
}

func (f *fakeOpenBoardActivator) Deactivate() error {
	if f.deactivateHook != nil {
		f.deactivateHook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "deactivate")
	return nil
}

func (f *fakeOpenBoardActivator) SetPeriodicRecoveryEnabled(enabled bool) {}

func (f *fakeOpenBoardActivator) callsSnapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func TestOpenBoard_DeactivatesBeforeRebindingBoardClient(t *testing.T) {
	rootA := fixtureBoard(t, "TA")
	rootB := fixtureBoard(t, "TB")
	stub := stubTbBinary(t)
	recents := filepath.Join(t.TempDir(), "recent.json")

	board := NewBoardService()
	activator := &fakeOpenBoardActivator{}
	sw := &fakeSwitcher{}
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		Watcher:     sw,
		CLIPath:     stub,
		RecentsPath: recents,
		Activator:   activator,
	})

	if err := svc.OpenBoard(context.Background(), rootA); err != nil {
		t.Fatalf("OpenBoard A: %v", err)
	}
	activator.mu.Lock()
	activator.calls = nil
	activator.mu.Unlock()
	switchSeenBeforeDeactivate := false
	sw.hook = func(boardDir string) {
		calls := activator.callsSnapshot()
		switchSeenBeforeDeactivate = len(calls) == 0
		if board.snapshot() == nil {
			t.Errorf("existing board client disappeared before watcher switch")
		}
	}
	deactivateSawOldClient := false
	activator.deactivateHook = func() {
		if got := svc.GetProjectRoot(); got != rootA {
			t.Errorf("deactivate saw project root %q, want old root %q", got, rootA)
		}
		if c := board.snapshot(); c != nil && c.Cwd() == rootA {
			deactivateSawOldClient = true
		}
	}

	if err := svc.OpenBoard(context.Background(), rootB); err != nil {
		t.Fatalf("OpenBoard B: %v", err)
	}
	if !switchSeenBeforeDeactivate {
		t.Fatalf("watcher did not switch before deactivation; calls=%v", activator.callsSnapshot())
	}
	if !deactivateSawOldClient {
		t.Fatalf("deactivate did not run while BoardService still pointed at old root")
	}
}

func readMarkerLog(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read marker log: %v", err)
	}
	return string(b)
}
