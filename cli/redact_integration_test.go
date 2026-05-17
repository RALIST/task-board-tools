package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAPISecret is a clearly-fabricated credential pattern used to assert
// that redaction reaches the task markdown AND the regenerated BOARD.md
// without ever committing or paging a real token.
const fakeAPISecret = "sk-fake-NOT-A-REAL-KEY-1234567890"

func TestCreateDescriptionRedactsSecrets(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	captureStdout(t, func() {
		cmdCreate([]string{
			"Sample task",
			"-m", "cli",
			"-d", "Try OPENAI_API_KEY=" + fakeAPISecret + " in the env.",
		})
	})

	// newCommandTestBoard seeds .next-id=2, so the created task is TB-2.
	taskPath := filepath.Join(boardDir, "backlog", "TB-2", "TASK.md")
	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(data)
	if strings.Contains(body, fakeAPISecret) {
		t.Fatalf("task body leaked raw secret:\n%s", body)
	}
	if !strings.Contains(body, "[REDACTED]") {
		t.Fatalf("task body missing [REDACTED] marker:\n%s", body)
	}

	boardData, err := os.ReadFile(filepath.Join(boardDir, "BOARD.md"))
	if err != nil {
		t.Fatalf("read BOARD.md: %v", err)
	}
	if strings.Contains(string(boardData), fakeAPISecret) {
		t.Fatalf("BOARD.md leaked raw secret:\n%s", boardData)
	}
}

func TestEditGoalFromStdinRedactsSecrets(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	initial := strings.Join([]string{
		"# TB-1: Sample",
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"old goal",
		"",
		"## Log",
		"",
		"- 2026-05-17: Created",
		"",
	}, "\n")
	if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	stdinBody := "Updated goal with leak: GITHUB_TOKEN=" + fakeAPISecret + "\n"
	withStdin(t, stdinBody, func() {
		captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--goal", "-"})
		})
	})

	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(data)
	if strings.Contains(body, fakeAPISecret) {
		t.Fatalf("task body leaked raw secret:\n%s", body)
	}
	if !strings.Contains(body, "GITHUB_TOKEN=[REDACTED]") {
		t.Fatalf("task body missing redacted GITHUB_TOKEN marker:\n%s", body)
	}

	boardData, err := os.ReadFile(filepath.Join(boardDir, "BOARD.md"))
	if err != nil {
		t.Fatalf("read BOARD.md: %v", err)
	}
	if strings.Contains(string(boardData), fakeAPISecret) {
		t.Fatalf("BOARD.md leaked raw secret:\n%s", boardData)
	}
}

func TestEditAcceptanceFromStdinRedactsSecrets(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	initial := strings.Join([]string{
		"# TB-1: Sample",
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"goal body",
		"",
		"## Acceptance Criteria",
		"",
		"- [ ] old",
		"",
		"## Log",
		"",
		"- 2026-05-17: Created",
		"",
	}, "\n")
	if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	stdinBody := "- [ ] do not log Bearer " + fakeAPISecret + "\n"
	withStdin(t, stdinBody, func() {
		captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--acceptance", "-"})
		})
	})

	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(data)
	if strings.Contains(body, fakeAPISecret) {
		t.Fatalf("task body leaked raw secret:\n%s", body)
	}
	if !strings.Contains(body, "Bearer [REDACTED]") {
		t.Fatalf("task body missing redacted Bearer marker:\n%s", body)
	}
}

func TestCreateRedactsSecretsInTitleModuleTags(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	captureStdout(t, func() {
		cmdCreate([]string{
			"OPENAI_API_KEY=" + fakeAPISecret + " in title",
			"-m", "core OPENAI_API_KEY=" + fakeAPISecret,
			"-t", "ops,GITHUB_TOKEN=" + fakeAPISecret + ",alpha",
		})
	})

	taskPath := filepath.Join(boardDir, "backlog", "TB-2", "TASK.md")
	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(data)
	if strings.Contains(body, fakeAPISecret) {
		t.Fatalf("task body leaked raw secret via title/module/tags:\n%s", body)
	}
	if !strings.Contains(body, "[REDACTED]") {
		t.Fatalf("task body missing [REDACTED] marker:\n%s", body)
	}

	boardData, err := os.ReadFile(filepath.Join(boardDir, "BOARD.md"))
	if err != nil {
		t.Fatalf("read BOARD.md: %v", err)
	}
	if strings.Contains(string(boardData), fakeAPISecret) {
		t.Fatalf("BOARD.md leaked raw secret via title/module/tags:\n%s", boardData)
	}
}

func TestEditRedactsSecretsInTitleModuleTags(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	initial := strings.Join([]string{
		"# TB-1: Sample",
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** core",
		"**Tags:** initial",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"goal",
		"",
		"## Log",
		"",
		"- 2026-05-17: Created",
		"",
	}, "\n")
	if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	captureStdout(t, func() {
		cmdEdit([]string{
			"TB-1",
			"--title", "Renamed OPENAI_API_KEY=" + fakeAPISecret,
			"-m", "newmod OPENAI_API_KEY=" + fakeAPISecret,
			"-t", "alpha,GITHUB_TOKEN=" + fakeAPISecret + ",beta",
		})
	})

	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	body := string(data)
	if strings.Contains(body, fakeAPISecret) {
		t.Fatalf("task body leaked raw secret after edit:\n%s", body)
	}
	if !strings.Contains(body, "[REDACTED]") {
		t.Fatalf("task body missing [REDACTED] marker after edit:\n%s", body)
	}

	boardData, _ := os.ReadFile(filepath.Join(boardDir, "BOARD.md"))
	if strings.Contains(string(boardData), fakeAPISecret) {
		t.Fatalf("BOARD.md leaked raw secret after edit:\n%s", boardData)
	}
}

func TestAppendLogEntryRedactsUserSuppliedValues(t *testing.T) {
	// Direct unit test for the redaction wrapping in appendLogEntry. A
	// caller that interpolates user-provided text (e.g. an `Edited tags=`
	// label that happened to contain a token-like value) must end up with
	// a sanitized log line.
	in := "no log section yet"
	got := appendLogEntry(in, "- 2026-05-17: Edited tags=OPENAI_API_KEY="+fakeAPISecret+"\n")
	if strings.Contains(got, fakeAPISecret) {
		t.Fatalf("appendLogEntry leaked raw secret in entry:\n%s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("appendLogEntry missing [REDACTED] in entry:\n%s", got)
	}
}
