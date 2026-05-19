package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reviewBaseTask is the seed task body used across review tests. It carries
// a ReviewRef so reviewSubmit's TB-235 gate is satisfied; tests that
// specifically exercise the missing-ref rejection live in
// review_ref_test.go and seed their own bodies without it.
const reviewBaseTask = `# TB-1: Sample Task

**Type:** feature
**Priority:** P1
**Size:** M
**Module:** cli
**ReviewRef:** feat/x
**Branch:** feat/x

## Goal

Do the thing.

## Acceptance Criteria

- [ ] AC1

## Log

- 2026-05-13: Created
`

func writeReviewTask(t *testing.T, boardDir, status, body string) string {
	t.Helper()
	path := filepath.Join(boardDir, status, "TB-1.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}
	return path
}

func readReviewTask(t *testing.T, boardDir, status string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(boardDir, status, "TB-1.md"))
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	return string(data)
}

func TestReviewSubmitFromInProgressEmitsWarningWhenTargetMissing(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "in-progress", reviewBaseTask)

	stderr := captureStderr(t, func() {
		msg, err := reviewSubmit("TB-1")
		if err != nil {
			t.Fatalf("reviewSubmit: %v", err)
		}
		if !strings.Contains(msg, "Submitted TB-1 from in-progress to code-review") {
			t.Fatalf("unexpected submit msg: %q", msg)
		}
	})

	if !strings.Contains(stderr, "no ## Review Target section") {
		t.Fatalf("expected missing-target warning on stderr, got: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); err != nil {
		t.Fatalf("task should be in code-review/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "in-progress", "TB-1.md")); !os.IsNotExist(err) {
		t.Fatalf("source file should be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "BOARD.md")); err != nil {
		t.Fatalf("BOARD.md should be regenerated: %v", err)
	}
}

func TestReviewSubmitSilentWhenTargetPresent(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	body := strings.Replace(reviewBaseTask,
		"## Log\n",
		"## Review Target\n\nbranch: feat/x\n\n## Log\n", 1)
	writeReviewTask(t, boardDir, "in-progress", body)

	stderr := captureStderr(t, func() {
		if _, err := reviewSubmit("TB-1"); err != nil {
			t.Fatalf("reviewSubmit: %v", err)
		}
	})

	if strings.Contains(stderr, "no ## Review Target") {
		t.Fatalf("did not expect missing-target warning, got: %s", stderr)
	}
	content := readReviewTask(t, boardDir, "code-review")
	if !strings.Contains(content, "Submitted to code-review") {
		t.Fatalf("expected submit log entry, got:\n%s", content)
	}
}

func TestReviewSubmitFromBacklogRejected(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "backlog", reviewBaseTask)

	_, err := reviewSubmit("TB-1")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "in-progress") {
		t.Fatalf("expected error about in-progress, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "backlog", "TB-1.md")); err != nil {
		t.Fatalf("task should still be in backlog: %v", err)
	}
}

func TestReviewSubmitClearsReviewFailedTagOnResubmit(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	body := strings.Replace(reviewBaseTask,
		"**Module:** cli\n",
		"**Module:** cli\n**Tags:** code-review,review-failed\n", 1)
	body = strings.Replace(body,
		"## Log\n",
		"## Review Target\n\nbranch: feat/x\n\n## Log\n", 1)
	writeReviewTask(t, boardDir, "backlog", body)

	captureStderr(t, func() {
		if _, err := reviewSubmit("TB-1"); err != nil {
			t.Fatalf("reviewSubmit: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); err != nil {
		t.Fatalf("task should be in code-review/: %v", err)
	}
	content := readReviewTask(t, boardDir, "code-review")
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "**Tags:**") && strings.Contains(line, "review-failed") {
			t.Fatalf("review-failed tag should be cleared from Tags line, got: %q\nfull:\n%s", line, content)
		}
	}
	if !strings.Contains(content, "Cleared review-failed marker on resubmit") {
		t.Fatalf("expected log entry for tag clear, got:\n%s", content)
	}
}

func TestReviewWriteTargetCreatesSection(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "in-progress", reviewBaseTask)

	withStdin(t, "branch: feat/x\nPR: https://example.com/pr/42\n", func() {
		if _, err := reviewWriteSection("TB-1", "-", "## Review Target", "review-target", "review target"); err != nil {
			t.Fatalf("reviewWriteSection: %v", err)
		}
	})

	content := readReviewTask(t, boardDir, "in-progress")
	if !strings.Contains(content, "## Review Target\n\nbranch: feat/x\nPR: https://example.com/pr/42") {
		t.Fatalf("expected review target section, got:\n%s", content)
	}
	if !strings.Contains(content, "Edited review-target") {
		t.Fatalf("expected log entry, got:\n%s", content)
	}
}

func TestReviewWriteNotesReplacesExisting(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	body := strings.Replace(reviewBaseTask,
		"## Log\n",
		"## Reviewer Notes\n\nOld notes.\n\n## Log\n", 1)
	writeReviewTask(t, boardDir, "code-review", body)

	withStdin(t, "Focus on regenerateBoard behavior.\n", func() {
		if _, err := reviewWriteSection("TB-1", "-", "## Reviewer Notes", "reviewer-notes", "reviewer notes"); err != nil {
			t.Fatalf("reviewWriteSection: %v", err)
		}
	})

	content := readReviewTask(t, boardDir, "code-review")
	if strings.Contains(content, "Old notes.") {
		t.Fatalf("expected old notes to be replaced, got:\n%s", content)
	}
	if !strings.Contains(content, "Focus on regenerateBoard behavior.") {
		t.Fatalf("expected new notes content, got:\n%s", content)
	}
}

func TestReviewWriteFindingsRejectsEmptyStdin(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

	var err error
	withStdin(t, "   \n\n", func() {
		_, err = reviewWriteSection("TB-1", "-", "## Review Findings", "review-findings", "review findings")
	})
	if err == nil {
		t.Fatalf("expected error from empty stdin")
	}
	if !strings.Contains(err.Error(), "empty after trimming") {
		t.Fatalf("expected empty content error, got: %v", err)
	}
}

func TestReviewFailMovesToBacklogWithMarker(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

	withStdin(t, "- Found a regression in moveTask.\n", func() {
		if _, err := reviewFail("TB-1", "-"); err != nil {
			t.Fatalf("reviewFail: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(boardDir, "backlog", "TB-1.md")); err != nil {
		t.Fatalf("task should be in backlog/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); !os.IsNotExist(err) {
		t.Fatalf("source file should be removed, got err=%v", err)
	}
	content := readReviewTask(t, boardDir, "backlog")
	if !strings.Contains(content, "**Tags:** review-failed") {
		t.Fatalf("expected review-failed tag, got:\n%s", content)
	}
	if !strings.Contains(content, "## Review Findings\n\n- Found a regression in moveTask.") {
		t.Fatalf("expected findings section, got:\n%s", content)
	}
	if !strings.Contains(content, "Failed code review") {
		t.Fatalf("expected fail log entry, got:\n%s", content)
	}
}

func TestReviewFailRejectsNonCodeReview(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "in-progress", reviewBaseTask)

	var err error
	withStdin(t, "blocking issue\n", func() {
		_, err = reviewFail("TB-1", "-")
	})
	if err == nil {
		t.Fatalf("expected error from non-code-review state")
	}
	if !strings.Contains(err.Error(), "only accepts tasks in code-review") {
		t.Fatalf("expected code-review-only error, got: %v", err)
	}
}

func TestStatusAliasesCodeReview(t *testing.T) {
	for _, alias := range []string{"code-review", "cr", "review"} {
		t.Run(alias, func(t *testing.T) {
			status, err := resolveStatus(alias)
			if err != nil {
				t.Fatalf("resolveStatus(%q): %v", alias, err)
			}
			if status != "code-review" {
				t.Fatalf("resolveStatus(%q) = %q, want code-review", alias, status)
			}

			dirs, err := resolveStatusFilter(alias)
			if err != nil {
				t.Fatalf("resolveStatusFilter(%q): %v", alias, err)
			}
			if len(dirs) != 1 || dirs[0] != "code-review" {
				t.Fatalf("resolveStatusFilter(%q) = %v, want [code-review]", alias, dirs)
			}
		})
	}
}

func TestActiveStatusFilterIncludesCodeReview(t *testing.T) {
	dirs, err := resolveStatusFilter("active")
	if err != nil {
		t.Fatalf("resolveStatusFilter(active): %v", err)
	}
	found := false
	for _, d := range dirs {
		if d == "code-review" {
			found = true
		}
	}
	if !found {
		t.Fatalf("active filter should include code-review, got: %v", dirs)
	}
}

func TestBoardJSONIncludesCodeReviewBucket(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

	snap, err := buildBoardSnapshot(boardDir)
	if err != nil {
		t.Fatalf("buildBoardSnapshot: %v", err)
	}
	if len(snap.CodeReview) != 1 {
		t.Fatalf("expected 1 task in CodeReview bucket, got %d", len(snap.CodeReview))
	}
	if snap.CodeReview[0].ID != "TB-1" {
		t.Fatalf("CodeReview[0].ID = %q, want TB-1", snap.CodeReview[0].ID)
	}
}
