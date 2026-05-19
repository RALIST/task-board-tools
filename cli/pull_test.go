package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pullP1Task returns a groomed task body with a configurable priority. The
// {{ID}}/{{PRIORITY}} placeholders are substituted by callers so multiple
// fixtures can be created without ad-hoc string surgery.
const pullTaskTemplate = `# {{ID}}: Pull Candidate

**Type:** feature
**Priority:** {{PRIORITY}}
**Size:** M
**Module:** core
**Branch:** —

## Goal

Be pullable.

## Acceptance Criteria

- [ ] Picked in priority order.

## Log

- 2026-05-19: Created
`

func writePullTask(t *testing.T, boardDir, status, id, priority string) {
	t.Helper()
	body := strings.NewReplacer("{{ID}}", id, "{{PRIORITY}}", priority).Replace(pullTaskTemplate)
	writeReadyTestTask(t, boardDir, status, id, body)
}

func TestPullPicksHighestPriorityOldest(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	// Older P2 first, newer P0 second — pull should prefer the P0.
	writePullTask(t, boardDir, "ready", "TB-1", "P2")
	writePullTask(t, boardDir, "ready", "TB-2", "P0")
	writePullTask(t, boardDir, "ready", "TB-3", "P0")

	msg, err := pullReadyTask("")
	if err != nil {
		t.Fatalf("pullReadyTask: %v", err)
	}
	if !strings.Contains(msg, "Pulled TB-2") {
		t.Fatalf("expected P0 oldest task TB-2 to be pulled, msg: %q", msg)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "in-progress", "TB-2.md")); err != nil {
		t.Fatalf("TB-2 should be in in-progress/: %v", err)
	}
}

func TestPullEmptyReadyIsNoop(t *testing.T) {
	newCommandTestBoard(t)
	stderr := captureStderr(t, func() {
		msg, err := pullReadyTask("")
		if err != nil {
			t.Fatalf("pullReadyTask: %v", err)
		}
		if msg != "" {
			t.Fatalf("expected empty msg, got %q", msg)
		}
	})
	if !strings.Contains(stderr, "ready column is empty") {
		t.Fatalf("expected empty-ready notice, got stderr: %s", stderr)
	}
}

func TestPullExplicitIDRequiresReadySource(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writePullTask(t, boardDir, "backlog", "TB-1", "P1")

	if _, err := pullReadyTask("TB-1"); err == nil {
		t.Fatal("expected error pulling from non-ready source")
	} else if !strings.Contains(err.Error(), "only accepts tasks in ready") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPullExplicitIDSuccessLogsEntry(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writePullTask(t, boardDir, "ready", "TB-1", "P2")

	msg, err := pullReadyTask("TB-1")
	if err != nil {
		t.Fatalf("pullReadyTask: %v", err)
	}
	if !strings.Contains(msg, "Pulled TB-1 from ready to in-progress") {
		t.Fatalf("unexpected msg: %q", msg)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "in-progress", "TB-1.md")); err != nil {
		t.Fatalf("TB-1 should be in in-progress/: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(boardDir, "in-progress", "TB-1.md"))
	if err != nil {
		t.Fatalf("read pulled task: %v", err)
	}
	if !strings.Contains(string(data), "Pulled into in-progress") {
		t.Fatalf("expected pull log entry, got:\n%s", data)
	}
}

func TestPullStrictWipBlocksWhenInProgressFull(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	prevLimits := cfg.WipLimits
	prevEnforcement := cfg.WipEnforcement
	cfg.WipLimits = map[string]int{"in-progress": 1}
	cfg.WipEnforcement = "strict"
	t.Cleanup(func() {
		cfg.WipLimits = prevLimits
		cfg.WipEnforcement = prevEnforcement
	})

	writePullTask(t, boardDir, "in-progress", "TB-1", "P1")
	writePullTask(t, boardDir, "ready", "TB-2", "P0")

	if _, err := pullReadyTask(""); err == nil {
		t.Fatal("expected strict WIP error")
	} else if !strings.Contains(err.Error(), "WIP limit reached for in-progress") {
		t.Fatalf("expected WIP error, got: %v", err)
	}
}
