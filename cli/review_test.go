package main

import (
	"bytes"
	"os"
	"os/exec"
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

func TestReviewFailMovesToReadyWithMarker(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

	withStdin(t, "- Found a regression in moveTask.\n", func() {
		if _, err := reviewFail("TB-1", "-"); err != nil {
			t.Fatalf("reviewFail: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(boardDir, "ready", "TB-1.md")); err != nil {
		t.Fatalf("task should be in ready/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); !os.IsNotExist(err) {
		t.Fatalf("source file should be removed, got err=%v", err)
	}
	content := readReviewTask(t, boardDir, "ready")
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

func TestReviewPassMovesToDoneWithFindings(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

	withStdin(t, "- No blocking findings.\n", func() {
		if _, err := reviewPass("TB-1", "-"); err != nil {
			t.Fatalf("reviewPass: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(boardDir, "done", "TB-1.md")); err != nil {
		t.Fatalf("task should be in done/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); !os.IsNotExist(err) {
		t.Fatalf("source file should be removed, got err=%v", err)
	}
	content := readReviewTask(t, boardDir, "done")
	if !strings.Contains(content, "## Review Findings\n\n- No blocking findings.") {
		t.Fatalf("expected findings section, got:\n%s", content)
	}
	if !strings.Contains(content, "Passed code review") {
		t.Fatalf("expected pass log entry, got:\n%s", content)
	}
}

func TestReviewPassCommandAcceptsIDThenFile(t *testing.T) {
	if os.Getenv("TB_TEST_REVIEW_PASS_COMMAND") == "1" {
		boardDir := newCommandTestBoard(t)
		writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

		inputPath := filepath.Join(t.TempDir(), "findings.md")
		if err := os.WriteFile(inputPath, []byte("- No blocking findings.\n"), 0644); err != nil {
			t.Fatalf("write findings: %v", err)
		}

		captureStdout(t, func() {
			cmdReview([]string{"--pass", "TB-1", inputPath})
		})

		content := readReviewTask(t, boardDir, "done")
		if !strings.Contains(content, "## Review Findings\n\n- No blocking findings.") {
			t.Fatalf("expected findings section, got:\n%s", content)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestReviewPassCommandAcceptsIDThenFile$", "-test.v")
	cmd.Env = append(os.Environ(), "TB_TEST_REVIEW_PASS_COMMAND=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("child should pass; err=%v\nstderr:\n%s", err, stderr.String())
	}
}

func TestReviewPassMovesFolderTaskToDone(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeMoveFolderTask(t, boardDir, "code-review", "TB-1", "Folder Review")

	withStdin(t, "- No blocking findings.\n", func() {
		if _, err := reviewPass("TB-1", "-"); err != nil {
			t.Fatalf("reviewPass: %v", err)
		}
	})

	donePath := filepath.Join(boardDir, "done", "TB-1", folderTaskFileName)
	if _, err := os.Stat(donePath); err != nil {
		t.Fatalf("folder task should be in done/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1", folderTaskFileName)); !os.IsNotExist(err) {
		t.Fatalf("source folder task should be removed, got err=%v", err)
	}
	content, err := os.ReadFile(donePath)
	if err != nil {
		t.Fatalf("read done folder task: %v", err)
	}
	if !strings.Contains(string(content), "## Review Findings\n\n- No blocking findings.") {
		t.Fatalf("expected findings section, got:\n%s", string(content))
	}
	assertSingleTaskRepresentation(t, boardDir, "TB-1", filepath.Join("done", "TB-1", folderTaskFileName))
}

func TestReviewPassRejectsNonCodeReview(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "ready", reviewBaseTask)

	var err error
	withStdin(t, "- No blocking findings.\n", func() {
		_, err = reviewPass("TB-1", "-")
	})
	if err == nil || !strings.Contains(err.Error(), "only accepts tasks in code-review") {
		t.Fatalf("reviewPass error = %v, want code-review-only error", err)
	}
	if _, statErr := os.Stat(filepath.Join(boardDir, "ready", "TB-1.md")); statErr != nil {
		t.Fatalf("task should remain ready: %v", statErr)
	}
}

func TestReviewPassRejectsEmptyFindings(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeReviewTask(t, boardDir, "code-review", reviewBaseTask)

	var err error
	withStdin(t, "\n  \n", func() {
		_, err = reviewPass("TB-1", "-")
	})
	if err == nil || !strings.Contains(err.Error(), "empty after trimming") {
		t.Fatalf("reviewPass error = %v, want empty-content error", err)
	}
	if _, statErr := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); statErr != nil {
		t.Fatalf("task should remain code-review: %v", statErr)
	}
}

// TB-268: a failed review must leave the generic AgentStatus blank so
// auto-implement's retry-pickup predicate sees the rework-ready task as
// eligible. Per-mode attribution (e.g. ImplementStatus) is preserved.
func TestReviewFailClearsAgentStatusForRetry(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	body := strings.Replace(
		reviewBaseTask,
		"**Module:** cli\n",
		"**Module:** cli\n**Agent:** claude\n**AgentStatus:** success\n**ImplementedBy:** claude\n**ImplementStatus:** success\n",
		1,
	)
	writeReviewTask(t, boardDir, "code-review", body)

	withStdin(t, "- Found a regression.\n", func() {
		if _, err := reviewFail("TB-1", "-"); err != nil {
			t.Fatalf("reviewFail: %v", err)
		}
	})

	content := readReviewTask(t, boardDir, "ready")
	if strings.Contains(content, "**AgentStatus:**") {
		t.Fatalf("expected AgentStatus cleared after review --fail, got:\n%s", content)
	}
	if !strings.Contains(content, "**ImplementStatus:** success") {
		t.Fatalf("expected ImplementStatus preserved, got:\n%s", content)
	}
	if !strings.Contains(content, "**ImplementedBy:** claude") {
		t.Fatalf("expected ImplementedBy preserved, got:\n%s", content)
	}
	if !strings.Contains(content, "**Agent:** claude") {
		t.Fatalf("expected Agent field preserved (only AgentStatus is cleared), got:\n%s", content)
	}
	if !strings.Contains(content, "**Tags:** review-failed") {
		t.Fatalf("expected review-failed tag, got:\n%s", content)
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

func TestStatusAliasesReady(t *testing.T) {
	for _, alias := range []string{"ready", "r"} {
		t.Run(alias, func(t *testing.T) {
			status, err := resolveStatus(alias)
			if err != nil {
				t.Fatalf("resolveStatus(%q): %v", alias, err)
			}
			if status != "ready" {
				t.Fatalf("resolveStatus(%q) = %q, want ready", alias, status)
			}

			dirs, err := resolveStatusFilter(alias)
			if err != nil {
				t.Fatalf("resolveStatusFilter(%q): %v", alias, err)
			}
			if len(dirs) != 1 || dirs[0] != "ready" {
				t.Fatalf("resolveStatusFilter(%q) = %v, want [ready]", alias, dirs)
			}
		})
	}
}

func TestActiveStatusFilterIncludesReady(t *testing.T) {
	dirs, err := resolveStatusFilter("active")
	if err != nil {
		t.Fatalf("resolveStatusFilter(active): %v", err)
	}
	found := false
	for _, d := range dirs {
		if d == "ready" {
			found = true
		}
	}
	if !found {
		t.Fatalf("active filter should include ready, got: %v", dirs)
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

func TestBoardJSONExposesWipLimitsAndCounts(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	prevLimits := cfg.WipLimits
	prevEnforcement := cfg.WipEnforcement
	cfg.WipLimits = map[string]int{"ready": 3, "in-progress": 2}
	cfg.WipEnforcement = "strict"
	t.Cleanup(func() {
		cfg.WipLimits = prevLimits
		cfg.WipEnforcement = prevEnforcement
	})

	writeReviewTask(t, boardDir, "in-progress", reviewBaseTask)

	snap, err := buildBoardSnapshot(boardDir)
	if err != nil {
		t.Fatalf("buildBoardSnapshot: %v", err)
	}
	if got := snap.WipLimits["ready"]; got != 3 {
		t.Fatalf("WipLimits[ready] = %d, want 3", got)
	}
	if got := snap.WipLimits["in-progress"]; got != 2 {
		t.Fatalf("WipLimits[in-progress] = %d, want 2", got)
	}
	if _, ok := snap.WipLimits["code-review"]; ok {
		t.Fatalf("WipLimits[code-review] should be absent (no limit configured), got %d", snap.WipLimits["code-review"])
	}
	if snap.WipCounts["in-progress"] != 1 {
		t.Fatalf("WipCounts[in-progress] = %d, want 1", snap.WipCounts["in-progress"])
	}
	if snap.WipCounts["ready"] != 0 {
		t.Fatalf("WipCounts[ready] = %d, want 0", snap.WipCounts["ready"])
	}
	if snap.WipEnforcement != "strict" {
		t.Fatalf("WipEnforcement = %q, want strict", snap.WipEnforcement)
	}
}

func TestParseWipLimitsHonoursExplicitZeroAsDisabled(t *testing.T) {
	limits, legacy := parseWipLimits(map[string]string{
		"wip_limit_in_progress": "0",
		"wip_limit_ready":       "5",
	})
	if got := limits["in-progress"]; got != 0 {
		t.Fatalf("explicit zero in-progress should be preserved as 0, got %d", got)
	}
	if legacy != 0 {
		t.Fatalf("legacy mirror should be 0 (disabled), got %d", legacy)
	}
	prev := cfg.WipLimits
	cfg.WipLimits = limits
	t.Cleanup(func() { cfg.WipLimits = prev })
	if _, ok := cfg.wipLimitFor("in-progress"); ok {
		t.Fatalf("wipLimitFor should return ok=false when limit is 0")
	}
	if n, ok := cfg.wipLimitFor("ready"); !ok || n != 5 {
		t.Fatalf("wipLimitFor(ready) = (%d, %v), want (5, true)", n, ok)
	}
}
