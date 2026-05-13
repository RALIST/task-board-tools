package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomic_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.md")
	want := []byte("# PR-1: hello\n")

	if err := writeFileAtomic(path, want, 0644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("perm = %o, want 0644", info.Mode().Perm())
	}

	// No stray temp files should remain in the directory.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("temp file leaked: %s", e.Name())
		}
	}
}

func TestWriteFileAtomic_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.md")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := writeFileAtomic(path, []byte("new"), 0644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("got %q, want %q", got, "new")
	}
}

func TestWriteFileAtomic_TargetDirMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope", "task.md")
	err := writeFileAtomic(path, []byte("x"), 0644)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	// And no temp file leaked in the parent dir either.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("temp file leaked: %s", e.Name())
		}
	}
}

func TestWriteFileAtomic_RenameCollisionOK(t *testing.T) {
	// Concurrent-ish writes to the same target. We can't easily force a
	// rename failure on a normal filesystem, but we can verify that two
	// sequential writes both succeed and the second wins — which is the
	// observable guarantee we need.
	dir := t.TempDir()
	path := filepath.Join(dir, "task.md")
	if err := writeFileAtomic(path, []byte("first"), 0644); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	if err := writeFileAtomic(path, []byte("second"), 0644); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "second" {
		t.Errorf("got %q, want %q", got, "second")
	}
}
