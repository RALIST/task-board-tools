package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestScanForTodosReturnsWalkError(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	missingRoot := filepath.Join(boardDir, "missing")

	hits, err := scanForTodos(missingRoot, cfg.RootDir)
	if err == nil {
		t.Fatal("expected filepath.Walk error")
	}
	if len(hits) != 0 {
		t.Fatalf("hits = %v, want none on walk failure", hits)
	}
	if !strings.Contains(err.Error(), missingRoot) {
		t.Fatalf("scan error = %q, want missing root path", err)
	}
}
