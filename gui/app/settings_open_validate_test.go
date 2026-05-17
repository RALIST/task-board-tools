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

// TestOpenBoard_RejectsCandidateWithDuplicateTask covers TB-208: switching
// from a valid open board to a candidate whose active scope contains the
// same task ID in two status directories must fail with an actionable
// error and leave the previous board fully intact — no watcher swap,
// no BoardService re-bind, no recents update, and no board:opened /
// board:reloaded emit.
func TestOpenBoard_RejectsCandidateWithDuplicateTask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shell stub")
	}
	tb := buildTbForIntegration(t)

	rootValid := fixtureBoard(t, "TB")
	rootDup := fixtureBoardWithDuplicateTask(t, "TB", "TB-9")
	recents := filepath.Join(t.TempDir(), "recent.json")

	board := NewBoardService()
	sw := &fakeSwitcher{}
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		Watcher:     sw,
		CLIPath:     tb,
		RecentsPath: recents,
	})

	// Open the valid board first so there is a committed state to defend.
	if err := svc.OpenBoard(context.Background(), rootValid); err != nil {
		t.Fatalf("first OpenBoard valid: %v", err)
	}
	validInfo, err := svc.GetBoardInfo()
	if err != nil {
		t.Fatalf("GetBoardInfo: %v", err)
	}
	validClient := board.snapshot()
	if validClient == nil {
		t.Fatalf("BoardService client was not set after valid open")
	}
	validSwitchCount := len(sw.calls())
	validRecents, err := svc.ListRecentBoards()
	if err != nil {
		t.Fatalf("ListRecentBoards: %v", err)
	}
	validRecentsLen := len(validRecents)

	// Attempt to switch to the duplicate-task board.
	err = svc.OpenBoard(context.Background(), rootDup)
	if err == nil {
		t.Fatalf("OpenBoard duplicate: want error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "cannot load active board") {
		t.Errorf("error missing actionable prefix: %v", err)
	}
	if !strings.Contains(msg, "TB-9") {
		t.Errorf("error missing duplicate task ID: %v", err)
	}
	if !strings.Contains(msg, "backlog") || !strings.Contains(msg, "done") {
		t.Errorf("error missing both conflicting statuses: %v", err)
	}
	if strings.Contains(strings.ToLower(msg), "binding call failed") {
		t.Errorf("error must not surface raw Wails binding text: %v", err)
	}

	// Previous board state must be unchanged.
	got, err := svc.GetBoardInfo()
	if err != nil {
		t.Fatalf("GetBoardInfo after failed switch: %v", err)
	}
	if got.ProjectRoot != validInfo.ProjectRoot {
		t.Errorf("BoardInfo.ProjectRoot mutated: got %q want %q", got.ProjectRoot, validInfo.ProjectRoot)
	}
	if got.BoardDir != validInfo.BoardDir {
		t.Errorf("BoardInfo.BoardDir mutated: got %q want %q", got.BoardDir, validInfo.BoardDir)
	}
	if board.snapshot() != validClient {
		t.Errorf("BoardService client was swapped despite failed switch")
	}
	if got := len(sw.calls()); got != validSwitchCount {
		t.Errorf("watcher.Switch called %d times; want unchanged at %d", got, validSwitchCount)
	}
	currentRecents, err := svc.ListRecentBoards()
	if err != nil {
		t.Fatalf("ListRecentBoards after failed switch: %v", err)
	}
	if len(currentRecents) != validRecentsLen {
		t.Errorf("recent list mutated: got %d entries want %d", len(currentRecents), validRecentsLen)
	}
	for _, r := range currentRecents {
		if r.ProjectRoot == rootDup {
			t.Errorf("failed candidate board leaked into recents: %+v", r)
		}
	}
}

// TestOpenBoard_RecoverableAfterFailedSwitch verifies the same OpenBoard
// instance can still open another valid board after rejecting an invalid
// candidate — no daemon reset required.
func TestOpenBoard_RecoverableAfterFailedSwitch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shell stub")
	}
	tb := buildTbForIntegration(t)

	rootInvalid := fixtureBoardWithDuplicateTask(t, "TB", "TB-9")
	rootValid := fixtureBoard(t, "TB")
	recents := filepath.Join(t.TempDir(), "recent.json")

	board := NewBoardService()
	sw := &fakeSwitcher{}
	svc := NewSettingsService(SettingsOptions{
		Board:       board,
		Watcher:     sw,
		CLIPath:     tb,
		RecentsPath: recents,
	})

	// No prior valid board open. The failed open must leave the picker
	// state usable instead of stranding the app.
	if err := svc.OpenBoard(context.Background(), rootInvalid); err == nil {
		t.Fatalf("OpenBoard invalid: want error, got nil")
	}
	if board.snapshot() != nil {
		t.Errorf("BoardService client must remain nil after failed first open")
	}
	if calls := sw.calls(); len(calls) != 0 {
		t.Errorf("watcher.Switch should not have been called: %v", calls)
	}

	// A subsequent open of a valid board must succeed.
	if err := svc.OpenBoard(context.Background(), rootValid); err != nil {
		t.Fatalf("OpenBoard valid after failed: %v", err)
	}
	if board.snapshot() == nil {
		t.Errorf("BoardService client not set after recovery open")
	}
}

// fixtureBoardWithDuplicateTask builds a board with one task ID present in
// both backlog/ and done/, which is the invariant violation BoardService
// reports as "task X resolves to multiple canonical markdown paths".
func fixtureBoardWithDuplicateTask(t *testing.T, prefix, taskID string) string {
	t.Helper()
	root := fixtureBoard(t, prefix)
	board := filepath.Join(root, "board")
	body := "# " + taskID + ": Dup\n\n**Type:** bug\n**Priority:** P2\n**Size:** M\n**Module:** core\n**Branch:** -\n\n## Goal\n\ngoal\n\n## Log\n\n- 2026-05-17: Created\n"
	if err := os.WriteFile(filepath.Join(board, "backlog", taskID+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("seed backlog dup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(board, "done", taskID+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("seed done dup: %v", err)
	}
	return root
}

// guard against accidental import drift — keep `errors` referenced even when
// future edits remove the only call site.
var _ = errors.New
