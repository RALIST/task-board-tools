package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseTaskFileReadsReviewRef verifies the ReviewRef metadata line is
// parsed into the Task model. Mirrors the Branch parsing path.
func TestParseTaskFileReadsReviewRef(t *testing.T) {
	prevPrefix := cfg.Prefix
	cfg.Prefix = "TB"
	t.Cleanup(func() { cfg.Prefix = prevPrefix })

	body := strings.Join([]string{
		"# TB-1: ReviewRef sample",
		"",
		"**Type:** feature",
		"**Priority:** P1",
		"**Size:** M",
		"**Module:** cli",
		"**ReviewRef:** branch: feat/x",
		"**Branch:** feat/x",
		"",
		"## Goal",
		"",
		"Body content.",
		"",
	}, "\n")

	path := filepath.Join(t.TempDir(), "TB-1.md")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	tk, err := parseTaskFile(path)
	if err != nil {
		t.Fatalf("parseTaskFile: %v", err)
	}
	if tk.ReviewRef != "branch: feat/x" {
		t.Fatalf("ReviewRef = %q, want %q", tk.ReviewRef, "branch: feat/x")
	}
}

// TestMarshalTaskEmitsReviewRef confirms the JSON wire shape exposes
// reviewRef as a top-level camelCase key. Empty/placeholder values must
// normalize to "" so consumers branch on a single missing sentinel.
func TestMarshalTaskEmitsReviewRef(t *testing.T) {
	cases := []struct {
		name string
		in   Task
		want string
	}{
		{name: "non-empty value passes through", in: Task{ReviewRef: "feat/x"}, want: "feat/x"},
		{name: "em-dash placeholder normalizes to empty", in: Task{ReviewRef: "—"}, want: ""},
		{name: "whitespace placeholder normalizes to empty", in: Task{ReviewRef: "   "}, want: ""},
		{name: "absent stays empty", in: Task{}, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := marshalTask(tc.in)
			if out.ReviewRef != tc.want {
				t.Fatalf("ReviewRef = %q, want %q", out.ReviewRef, tc.want)
			}
		})
	}
}

// TestEditReviewRefSetsAndClears covers the `--review-ref` flag round-trip:
// set inserts the metadata line, "none" clears it, whitespace-only is
// rejected by the flag validator (mirrors --title).
func TestEditReviewRefSetsAndClears(t *testing.T) {
	initial := strings.Join([]string{
		"# TB-1: Existing Task",
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Body.",
		"",
		"## Log",
		"",
		"- 2026-05-19: Created",
		"",
	}, "\n")

	t.Run("sets a value and logs reviewref label", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		out := captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--review-ref", "feat/x"})
		})
		assertContains(t, out, "reviewref=feat/x")

		data, err := os.ReadFile(taskPath)
		if err != nil {
			t.Fatalf("read task: %v", err)
		}
		content := string(data)
		assertContains(t, content, "**ReviewRef:** feat/x")
		// Placement should be in the metadata header, not body.
		if strings.Index(content, "**ReviewRef:**") > strings.Index(content, "## Goal") {
			t.Fatalf("ReviewRef leaked into the body:\n%s", content)
		}
	})

	t.Run("clears the field with --review-ref none", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		seeded := strings.Replace(initial, "**Branch:** -\n", "**ReviewRef:** old-ref\n**Branch:** -\n", 1)
		if err := os.WriteFile(taskPath, []byte(seeded), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		out := captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--review-ref", "none"})
		})
		assertContains(t, out, "reviewref=none")

		data, err := os.ReadFile(taskPath)
		if err != nil {
			t.Fatalf("read task: %v", err)
		}
		content := string(data)
		assertNotContains(t, metadataHeader(content), "**ReviewRef:**")
	})

	t.Run("does not disturb unrelated metadata or body", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--review-ref", "PR #42"})
		})
		data, err := os.ReadFile(taskPath)
		if err != nil {
			t.Fatalf("read task: %v", err)
		}
		content := string(data)
		assertContains(t, content, "**Type:** bug")
		assertContains(t, content, "**Module:** cli")
		assertContains(t, content, "**Branch:** -")
		assertContains(t, content, "## Goal\n\nBody.")
		assertContains(t, content, "- 2026-05-19: Created")
	})
}

// TestReviewRefShowJSONEmitsField confirms `tb show --json` surfaces the
// reviewRef field through emitShowJSON.
func TestReviewRefShowJSONEmitsField(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	body := strings.Join([]string{
		"# TB-1: ReviewRef JSON",
		"",
		"**Type:** feature",
		"**Priority:** P1",
		"**Size:** M",
		"**Module:** cli",
		"**ReviewRef:** feat/x@deadbeef",
		"**Branch:** feat/x",
		"",
		"## Goal",
		"",
		"Body.",
		"",
	}, "\n")
	path := filepath.Join(boardDir, "backlog", "TB-1.md")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	stdout := captureStdout(t, func() {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read task: %v", err)
		}
		if err := emitShowJSON(path, data); err != nil {
			t.Fatalf("emitShowJSON: %v", err)
		}
	})

	var payload struct {
		Metadata struct {
			ReviewRef string `json:"reviewRef"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode emitShowJSON output: %v\nstdout: %s", err, stdout)
	}
	if payload.Metadata.ReviewRef != "feat/x@deadbeef" {
		t.Fatalf("reviewRef = %q, want %q", payload.Metadata.ReviewRef, "feat/x@deadbeef")
	}
}

// TestMoveToCodeReviewRequiresReviewRef pins down the validation gate added
// by TB-235. Each subcase is asserted in isolation: the no-op when source is
// already code-review, the alias coverage (cr / review), the negative path
// (no ReviewRef → error, source untouched), and the success path with a
// valid ReviewRef.
func TestMoveToCodeReviewRequiresReviewRef(t *testing.T) {
	baseNoRef := strings.Join([]string{
		"# TB-1: Sample",
		"",
		"**Type:** feature",
		"**Priority:** P1",
		"**Size:** M",
		"**Module:** cli",
		"**Branch:** feat/x",
		"",
		"## Goal",
		"",
		"Body.",
		"",
		"## Log",
		"",
		"- 2026-05-19: Created",
		"",
	}, "\n")
	withRef := strings.Replace(baseNoRef, "**Branch:** feat/x\n", "**ReviewRef:** feat/x\n**Branch:** feat/x\n", 1)

	t.Run("rejects move without ReviewRef from in-progress", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		srcPath := filepath.Join(boardDir, "in-progress", "TB-1.md")
		if err := os.WriteFile(srcPath, []byte(baseNoRef), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}
		preStat, err := os.Stat(srcPath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}

		_, err = moveTaskOnBoard(boardDir, "TB-1", "code-review", "Moved to code-review")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "ReviewRef") {
			t.Fatalf("expected error to mention ReviewRef, got: %v", err)
		}
		if !strings.Contains(err.Error(), "tb edit TB-1 --review-ref") {
			t.Fatalf("expected actionable error naming `tb edit TB-1 --review-ref ...`, got: %v", err)
		}

		if _, err := os.Stat(srcPath); err != nil {
			t.Fatalf("source file should still exist: %v", err)
		}
		if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); !os.IsNotExist(err) {
			t.Fatalf("destination should NOT exist after rejected move, got err=%v", err)
		}
		postStat, err := os.Stat(srcPath)
		if err != nil {
			t.Fatalf("stat after: %v", err)
		}
		if !preStat.ModTime().Equal(postStat.ModTime()) {
			t.Fatalf("rejected move should not modify source mtime: %v -> %v", preStat.ModTime(), postStat.ModTime())
		}
		data, _ := os.ReadFile(srcPath)
		if string(data) != baseNoRef {
			t.Fatalf("rejected move altered source content:\n%s", string(data))
		}
	})

	t.Run("rejects move when ReviewRef is em-dash placeholder", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		seeded := strings.Replace(baseNoRef, "**Branch:** feat/x\n", "**ReviewRef:** —\n**Branch:** feat/x\n", 1)
		srcPath := filepath.Join(boardDir, "in-progress", "TB-1.md")
		if err := os.WriteFile(srcPath, []byte(seeded), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		_, err := moveTaskOnBoard(boardDir, "TB-1", "code-review", "Moved to code-review")
		if err == nil {
			t.Fatalf("expected error for placeholder ReviewRef")
		}
		if !strings.Contains(err.Error(), "ReviewRef") {
			t.Fatalf("expected ReviewRef error, got: %v", err)
		}
	})

	t.Run("succeeds when ReviewRef is set", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		srcPath := filepath.Join(boardDir, "in-progress", "TB-1.md")
		if err := os.WriteFile(srcPath, []byte(withRef), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		if _, err := moveTaskOnBoard(boardDir, "TB-1", "code-review", "Moved to code-review"); err != nil {
			t.Fatalf("moveTaskOnBoard: %v", err)
		}
		if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); err != nil {
			t.Fatalf("task should be in code-review/: %v", err)
		}
		if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
			t.Fatalf("source file should be removed: %v", err)
		}
	})

	for _, alias := range []string{"code-review", "cr", "review"} {
		alias := alias
		t.Run("alias "+alias+" rejects missing ReviewRef", func(t *testing.T) {
			boardDir := newCommandTestBoard(t)
			srcPath := filepath.Join(boardDir, "in-progress", "TB-1.md")
			if err := os.WriteFile(srcPath, []byte(baseNoRef), 0644); err != nil {
				t.Fatalf("write task: %v", err)
			}
			resolved, err := resolveStatus(alias)
			if err != nil {
				t.Fatalf("resolveStatus(%q): %v", alias, err)
			}
			if _, err := moveTaskOnBoard(boardDir, "TB-1", resolved, "Moved to "+resolved); err == nil {
				t.Fatalf("alias %s: expected error, got nil", alias)
			} else if !strings.Contains(err.Error(), "ReviewRef") {
				t.Fatalf("alias %s: expected ReviewRef error, got: %v", alias, err)
			}
		})
	}

	t.Run("noop when already in code-review", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		srcPath := filepath.Join(boardDir, "code-review", "TB-1.md")
		// No ReviewRef present — already-in-code-review path must not
		// invoke the validation gate (only entry transitions do).
		if err := os.WriteFile(srcPath, []byte(baseNoRef), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		result, err := moveTaskOnBoard(boardDir, "TB-1", "code-review", "Moved to code-review")
		if err != nil {
			t.Fatalf("noop move should not error, got: %v", err)
		}
		if !result.Noop {
			t.Fatalf("expected Noop=true, got %+v", result)
		}
	})

	t.Run("moves between other statuses keep working without ReviewRef", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		srcPath := filepath.Join(boardDir, "in-progress", "TB-1.md")
		if err := os.WriteFile(srcPath, []byte(baseNoRef), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}
		if _, err := moveTaskOnBoard(boardDir, "TB-1", "done", "Done"); err != nil {
			t.Fatalf("moving to done without ReviewRef should still work: %v", err)
		}
	})
}

// TestReviewSubmitRequiresReviewRef ensures `tb review --submit` enforces
// the same gate as direct `tb mv` to code-review.
func TestReviewSubmitRequiresReviewRef(t *testing.T) {
	noRefBody := strings.Join([]string{
		"# TB-1: Sample",
		"",
		"**Type:** feature",
		"**Priority:** P1",
		"**Size:** M",
		"**Module:** cli",
		"**Branch:** feat/x",
		"",
		"## Goal",
		"",
		"Do the thing.",
		"",
		"## Log",
		"",
		"- 2026-05-13: Created",
		"",
	}, "\n")

	t.Run("rejects in-progress without ReviewRef", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		srcPath := filepath.Join(boardDir, "in-progress", "TB-1.md")
		if err := os.WriteFile(srcPath, []byte(noRefBody), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}
		preStat, err := os.Stat(srcPath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}

		_, err = reviewSubmit("TB-1")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "ReviewRef") {
			t.Fatalf("expected ReviewRef error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "tb edit TB-1 --review-ref") {
			t.Fatalf("expected actionable error naming `tb edit TB-1 --review-ref ...`, got: %v", err)
		}
		// Source must remain unchanged: file present, dest absent, no Log/tag edits.
		if _, err := os.Stat(srcPath); err != nil {
			t.Fatalf("source should remain: %v", err)
		}
		if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); !os.IsNotExist(err) {
			t.Fatalf("destination should not exist after rejected submit, got err=%v", err)
		}
		postStat, err := os.Stat(srcPath)
		if err != nil {
			t.Fatalf("stat after: %v", err)
		}
		if !preStat.ModTime().Equal(postStat.ModTime()) {
			t.Fatalf("rejected submit should not touch source mtime: %v -> %v", preStat.ModTime(), postStat.ModTime())
		}
	})

	t.Run("rejects review-failed backlog without ReviewRef and preserves tag", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		seeded := strings.Replace(noRefBody,
			"**Module:** cli\n",
			"**Module:** cli\n**Tags:** code-review,review-failed\n", 1)
		srcPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		if err := os.WriteFile(srcPath, []byte(seeded), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		_, err := reviewSubmit("TB-1")
		if err == nil {
			t.Fatalf("expected error for missing ReviewRef")
		}
		if !strings.Contains(err.Error(), "ReviewRef") {
			t.Fatalf("expected ReviewRef error, got: %v", err)
		}
		// review-failed tag and source file untouched.
		data, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			t.Fatalf("read source: %v", readErr)
		}
		content := string(data)
		if !strings.Contains(content, "**Tags:** code-review,review-failed") {
			t.Fatalf("tags should remain after rejected resubmit:\n%s", content)
		}
		// No "Cleared review-failed marker" log entry was appended.
		if strings.Contains(content, "Cleared review-failed marker") {
			t.Fatalf("rejected submit must not clear review-failed marker:\n%s", content)
		}
	})

	t.Run("succeeds with ReviewRef set", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		seeded := strings.Replace(noRefBody,
			"**Branch:** feat/x\n",
			"**ReviewRef:** feat/x\n**Branch:** feat/x\n", 1)
		srcPath := filepath.Join(boardDir, "in-progress", "TB-1.md")
		if err := os.WriteFile(srcPath, []byte(seeded), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		captureStderr(t, func() {
			msg, err := reviewSubmit("TB-1")
			if err != nil {
				t.Fatalf("reviewSubmit: %v", err)
			}
			if !strings.Contains(msg, "Submitted TB-1 from in-progress to code-review") {
				t.Fatalf("unexpected submit msg: %q", msg)
			}
		})
		if _, err := os.Stat(filepath.Join(boardDir, "code-review", "TB-1.md")); err != nil {
			t.Fatalf("task should be in code-review/: %v", err)
		}
	})
}
