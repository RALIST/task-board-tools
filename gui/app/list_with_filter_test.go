package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListWithFilter_NoClient mirrors LoadBoard's contract — without a
// project root the service returns ErrNoBoard.
func TestListWithFilter_NoClient(t *testing.T) {
	svc := NewBoardService()
	_, err := svc.ListWithFilter(context.Background(), "ready", AutoImplementFilter{})
	if !errors.Is(err, ErrNoBoard) {
		t.Fatalf("want ErrNoBoard, got %v", err)
	}
}

// TestListWithFilter_ArgsRoundTrip seeds a shell stub that echoes its
// args into a log file, runs ListWithFilter with a multi-value filter,
// and asserts the CLI was invoked with the expected flag shapes. This
// is the contract that lets us delete gui/internal/automation/query:
// the GUI no longer matches in-process; it relies on `tb ls` to filter.
func TestListWithFilter_ArgsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	stub := makeStub(t, `printf '%s\n' "$@" > `+logPath+`
echo '[]'`)

	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	filter := AutoImplementFilter{
		Search:     "router",
		Types:      []string{"bug", "improvement"},
		Priorities: []string{"P0", "P1"},
		Modules:    []string{"gui"},
		Sizes:      []string{"S", "M"},
		Tags:       []string{"macos", "window"},
		Agents:     []string{"claude", "codex"},
		Parents:    []string{"TB-1", "TB-2"},
	}

	tasks, err := svc.ListWithFilter(context.Background(), "ready", filter)
	if err != nil {
		t.Fatalf("ListWithFilter: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("empty stub: want 0 tasks, got %d", len(tasks))
	}

	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read args log: %v", err)
	}
	got := string(logged)
	wantContains := []string{
		"ls", "--json", "--status", "ready",
		"-T", "bug,improvement",
		"-p", "P0,P1",
		"-m", "gui",
		"-s", "S,M",
		"-t", "macos,window",
		"--agent", "claude,codex",
		"--parent", "TB-1,TB-2",
		"--search", "router",
	}
	for _, w := range wantContains {
		if !strings.Contains(got, w+"\n") {
			t.Errorf("missing arg %q in:\n%s", w, got)
		}
	}
}

// TestListWithFilter_EmptyFilterSendsOnlyStatus pins that an empty
// filter degenerates to plain `tb ls --json --status ready`. This is
// the same byte-shape as LoadBoard's call so the JSON contract stays
// stable.
func TestListWithFilter_EmptyFilterSendsOnlyStatus(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	stub := makeStub(t, `printf '%s\n' "$@" > `+logPath+`
echo '[]'`)

	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	if _, err := svc.ListWithFilter(context.Background(), "ready", AutoImplementFilter{}); err != nil {
		t.Fatalf("ListWithFilter: %v", err)
	}
	logged, _ := os.ReadFile(logPath)
	got := string(logged)
	want := "ls\n--json\n--status\nready\n"
	if got != want {
		t.Fatalf("unexpected CLI args:\n--got--\n%s\n--want--\n%s", got, want)
	}
}

// TestListWithFilter_DefaultStatus pins the empty-status fallback to
// "ready" since the coordinator always wants the ready pool.
func TestListWithFilter_DefaultStatus(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	stub := makeStub(t, `printf '%s\n' "$@" > `+logPath+`
echo '[]'`)

	svc := NewBoardService()
	svc.setClient(newClient(t, stub))

	if _, err := svc.ListWithFilter(context.Background(), "", AutoImplementFilter{}); err != nil {
		t.Fatalf("ListWithFilter: %v", err)
	}
	logged, _ := os.ReadFile(logPath)
	if !strings.Contains(string(logged), "--status\nready\n") {
		t.Fatalf("expected default status=ready: %s", logged)
	}
}
