package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readyGroomedTask is a fully-groomed task body: priority, module, non-
// placeholder goal and acceptance criteria. Suitable for promotion to ready
// without triage complaints.
const readyGroomedTask = `# TB-1: Groomed Task

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** core
**Branch:** —

## Goal

Implement the canonical pull mechanic.

## Acceptance Criteria

- [ ] Promotes only from backlog.
- [ ] Fails when the task is not groomed.

## Log

- 2026-05-19: Created
`

// readyUngroomedTask is missing priority and has placeholder goal/AC — a
// realistic "fresh capture" that the triage gate should reject.
const readyUngroomedTask = `# TB-1: Ungroomed Task

**Type:** bug
**Size:** M
**Branch:** —

## Goal

(to be filled)

## Acceptance Criteria

- [ ] (to be filled)

## Log

- 2026-05-19: Created
`

func writeReadyTestTask(t *testing.T, boardDir, status, id, body string) string {
	t.Helper()
	path := filepath.Join(boardDir, status, id+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	body = strings.Replace(body, "TB-1:", id+":", 1)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}
	return path
}

func TestPromoteToReadyHappyPath(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReadyTestTask(t, boardDir, "backlog", "TB-1", readyGroomedTask)

	msg, err := promoteToReady("TB-1")
	if err != nil {
		t.Fatalf("promoteToReady: %v", err)
	}
	if !strings.Contains(msg, "Moved TB-1 from backlog to ready") {
		t.Fatalf("unexpected msg: %q", msg)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "ready", "TB-1.md")); err != nil {
		t.Fatalf("task should be in ready/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "backlog", "TB-1.md")); !os.IsNotExist(err) {
		t.Fatalf("source file should be removed, got err=%v", err)
	}
	data, err := os.ReadFile(filepath.Join(boardDir, "ready", "TB-1.md"))
	if err != nil {
		t.Fatalf("read promoted task: %v", err)
	}
	if !strings.Contains(string(data), "Committed — moved to ready") {
		t.Fatalf("expected commit log entry, got:\n%s", data)
	}
}

func TestPromoteToReadyRejectsUngroomed(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReadyTestTask(t, boardDir, "backlog", "TB-1", readyUngroomedTask)

	if _, err := promoteToReady("TB-1"); err == nil {
		t.Fatal("expected error from ungroomed task")
	} else if !strings.Contains(err.Error(), "needs grooming") {
		t.Fatalf("expected grooming error, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "backlog", "TB-1.md")); err != nil {
		t.Fatalf("task should remain in backlog/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "ready", "TB-1.md")); !os.IsNotExist(err) {
		t.Fatalf("ready/ should be empty, got err=%v", err)
	}
}

func TestPromoteToReadyRejectsNonBacklogSource(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReadyTestTask(t, boardDir, "in-progress", "TB-1", readyGroomedTask)

	if _, err := promoteToReady("TB-1"); err == nil {
		t.Fatal("expected error from in-progress source")
	} else if !strings.Contains(err.Error(), "only promotes from backlog") {
		t.Fatalf("expected source error, got: %v", err)
	}
}

func TestPromoteToReadyNoopWhenAlreadyInReady(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReadyTestTask(t, boardDir, "ready", "TB-1", readyGroomedTask)

	stderr := captureStderr(t, func() {
		msg, err := promoteToReady("TB-1")
		if err != nil {
			t.Fatalf("promoteToReady: %v", err)
		}
		if msg != "" {
			t.Fatalf("expected empty msg, got %q", msg)
		}
	})
	if !strings.Contains(stderr, "already in ready") {
		t.Fatalf("expected already-in-ready notice, got stderr: %s", stderr)
	}
}

// readyGroomedTaskWithAgentState mirrors readyGroomedTask but with the
// generic AgentStatus carrying a leftover terminal status from a prior
// run (auto-groom typically leaves `success` here) and per-mode
// attribution populated. Used to exercise the TB-NNN clear-on-ready
// behaviour added in this file's promoteToReady.
const readyGroomedTaskWithAgentState = `# TB-1: Groomed Task

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** core
**Agent:** codex
**AgentStatus:** success
**GroomedBy:** codex
**GroomStatus:** success
**Branch:** —

## Goal

Implement the canonical pull mechanic.

## Acceptance Criteria

- [ ] Promotes only from backlog.
- [ ] Fails when the task is not groomed.

## Log

- 2026-05-19: Created
- 2026-05-19: Edited agentstatus=success, groomed-by=codex, groom-status=success
`

func TestPromoteToReadyClearsAgentStatusForFreshPickup(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReadyTestTask(t, boardDir, "backlog", "TB-1", readyGroomedTaskWithAgentState)

	if _, err := promoteToReady("TB-1"); err != nil {
		t.Fatalf("promoteToReady: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(boardDir, "ready", "TB-1.md"))
	if err != nil {
		t.Fatalf("read promoted task: %v", err)
	}
	body := string(data)
	for _, banned := range []string{"**AgentStatus:**"} {
		if strings.Contains(body, banned) {
			t.Errorf("expected %q to be cleared, got:\n%s", banned, body)
		}
	}
	// Per-mode attribution must be preserved so groom history stays
	// intact (the whole point of TB-237's per-mode fields).
	for _, required := range []string{"**GroomedBy:** codex", "**GroomStatus:** success", "**Agent:** codex"} {
		if !strings.Contains(body, required) {
			t.Errorf("expected %q preserved, got:\n%s", required, body)
		}
	}
}

func TestPromoteToReadyNoopOnAlreadyBlankAgentStatus(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReadyTestTask(t, boardDir, "backlog", "TB-1", readyGroomedTask)

	if _, err := promoteToReady("TB-1"); err != nil {
		t.Fatalf("promoteToReady: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(boardDir, "ready", "TB-1.md"))
	if err != nil {
		t.Fatalf("read promoted task: %v", err)
	}
	if strings.Contains(string(data), "**AgentStatus:**") {
		t.Errorf("unexpected AgentStatus line on a task that never had one:\n%s", data)
	}
}

func TestPromoteToReadyHonoursStrictWipLimit(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	prevLimits := cfg.WipLimits
	prevEnforcement := cfg.WipEnforcement
	cfg.WipLimits = map[string]int{"ready": 1, "in-progress": 2}
	cfg.WipEnforcement = "strict"
	t.Cleanup(func() {
		cfg.WipLimits = prevLimits
		cfg.WipEnforcement = prevEnforcement
	})

	// One task already in ready saturates the limit.
	writeReadyTestTask(t, boardDir, "ready", "TB-1", readyGroomedTask)
	writeReadyTestTask(t, boardDir, "backlog", "TB-2", readyGroomedTask)

	if _, err := promoteToReady("TB-2"); err == nil {
		t.Fatal("expected strict WIP error")
	} else if !strings.Contains(err.Error(), "WIP limit reached for ready") {
		t.Fatalf("expected WIP error, got: %v", err)
	}
}

func TestPromoteToReadyWarnWipLimitAllowsManualPromotion(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	prevLimits := cfg.WipLimits
	prevEnforcement := cfg.WipEnforcement
	cfg.WipLimits = map[string]int{"ready": 1}
	cfg.WipEnforcement = "warn"
	t.Cleanup(func() {
		cfg.WipLimits = prevLimits
		cfg.WipEnforcement = prevEnforcement
	})

	writeReadyTestTask(t, boardDir, "ready", "TB-1", readyGroomedTask)
	writeReadyTestTask(t, boardDir, "backlog", "TB-2", readyGroomedTask)

	stderr := captureStderr(t, func() {
		if _, err := promoteToReady("TB-2"); err != nil {
			t.Fatalf("promoteToReady warn mode should allow promotion: %v", err)
		}
	})
	if !strings.Contains(stderr, "WIP limit reached for ready") {
		t.Fatalf("expected WIP warning, got stderr: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "ready", "TB-2.md")); err != nil {
		t.Fatalf("warn mode should promote into ready: %v", err)
	}
}

func TestPromoteToReadyStrictWipOverrideBlocksWarnModePromotion(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	prevLimits := cfg.WipLimits
	prevEnforcement := cfg.WipEnforcement
	cfg.WipLimits = map[string]int{"ready": 1}
	cfg.WipEnforcement = "warn"
	t.Cleanup(func() {
		cfg.WipLimits = prevLimits
		cfg.WipEnforcement = prevEnforcement
	})

	writeReadyTestTask(t, boardDir, "ready", "TB-1", readyGroomedTask)
	writeReadyTestTask(t, boardDir, "backlog", "TB-2", readyGroomedTask)

	if _, err := promoteToReadyWithOptions("TB-2", readyOptions{strictWIP: true}); err == nil {
		t.Fatalf("strict WIP override should block promotion")
	}
	if _, err := os.Stat(filepath.Join(boardDir, "backlog", "TB-2.md")); err != nil {
		t.Fatalf("strict WIP override should leave task in backlog: %v", err)
	}
}

func TestEnforceWipLimitReportsCollectTasksError(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	prevLimits := cfg.WipLimits
	prevEnforcement := cfg.WipEnforcement
	cfg.WipLimits = map[string]int{"ready": 1}
	cfg.WipEnforcement = "warn"
	t.Cleanup(func() {
		cfg.WipLimits = prevLimits
		cfg.WipEnforcement = prevEnforcement
	})

	readyDir := filepath.Join(boardDir, "ready")
	if err := os.Remove(readyDir); err != nil {
		t.Fatalf("remove ready dir: %v", err)
	}
	if err := os.WriteFile(readyDir, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("write ready file: %v", err)
	}

	err := enforceWipLimit("ready", boardDir)
	if err == nil {
		t.Fatal("expected collectTasks error")
	}
	if !strings.Contains(err.Error(), "cannot check WIP limit for ready") {
		t.Fatalf("WIP error = %q, want collection context", err)
	}
}
