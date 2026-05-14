package shell

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRecentBoardLabel(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "task-board-tools")
	want := "task-board-tools (" + root + ")"
	if got := recentBoardLabel(root); got != want {
		t.Fatalf("recentBoardLabel: got %q, want %q", got, want)
	}
	if got := recentBoardLabel(""); got != "" {
		t.Fatalf("recentBoardLabel(empty): got %q, want empty", got)
	}
}

func TestFindProjectFileChecksWorkingDirectoryParents(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("docs"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)

	got, ok := findProjectFile("README.md")
	if !ok {
		t.Fatal("findProjectFile did not find parent README.md")
	}
	if got != readme {
		t.Fatalf("findProjectFile: got %q, want %q", got, readme)
	}
}

func TestDecrementActiveRunsFloorsAtZero(t *testing.T) {
	var c Controller

	c.decrementActiveRuns()
	if got := c.activeRuns.Load(); got != 0 {
		t.Fatalf("activeRuns after zero decrement: got %d, want 0", got)
	}

	c.activeRuns.Store(2)
	c.decrementActiveRuns()
	if got := c.activeRuns.Load(); got != 1 {
		t.Fatalf("activeRuns after decrement: got %d, want 1", got)
	}
}

func TestTrayIconPNGDecodes(t *testing.T) {
	idle, err := trayIconPNG(false)
	if err != nil {
		t.Fatalf("idle trayIconPNG: %v", err)
	}
	running, err := trayIconPNG(true)
	if err != nil {
		t.Fatalf("running trayIconPNG: %v", err)
	}
	if bytes.Equal(idle, running) {
		t.Fatal("running tray icon should differ from idle")
	}

	img, err := png.Decode(bytes.NewReader(idle))
	if err != nil {
		t.Fatalf("decode idle tray icon: %v", err)
	}
	if got := img.Bounds().Size(); got.X != 44 || got.Y != 44 {
		t.Fatalf("tray icon size: got %dx%d, want 44x44", got.X, got.Y)
	}
}
