package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditBodySections(t *testing.T) {
	cases := []struct {
		name        string
		initial     string
		goalInput   string
		acceptInput string
		stdinInput  string
		args        func(goalPath, acceptPath string) []string
		assert      func(t *testing.T, content string)
	}{
		{
			name: "replaces goal and acceptance from files with metadata in one log entry",
			initial: strings.Join([]string{
				"# TB-1: Existing Task",
				"",
				"**Type:** bug",
				"**Priority:** P2",
				"**Size:** M",
				"**Module:** old",
				"**Branch:** -",
				"",
				"## Goal",
				"",
				"Old goal.",
				"",
				"## Context",
				"",
				"Keep this context exactly.",
				"",
				"## Acceptance Criteria",
				"",
				"- [ ] Old acceptance.",
				"",
				"## Log",
				"",
				"- 2026-05-13: Created",
				"",
			}, "\n"),
			goalInput:   "\n\nShip scriptable edits.\nWith multiline detail.\n\n",
			acceptInput: "\n- [ ] Goal can be replaced\n- [ ] Acceptance can be replaced\n\n",
			args: func(goalPath, acceptPath string) []string {
				return []string{"TB-1", "--goal", goalPath, "--acceptance", acceptPath, "-m", "cli"}
			},
			assert: func(t *testing.T, content string) {
				t.Helper()
				assertContains(t, content, "**Module:** cli")
				assertContains(t, content, "## Goal\n\nShip scriptable edits.\nWith multiline detail.\n\n## Context")
				assertContains(t, content, "## Acceptance Criteria\n\n- [ ] Goal can be replaced\n- [ ] Acceptance can be replaced\n\n## Log")
				assertContains(t, content, "Keep this context exactly.")
				assertContains(t, content, "- 2026-05-13: Created")
				assertContains(t, content, ": Edited module=cli, goal, acceptance")
				assertNotContains(t, content, "Old goal.")
				assertNotContains(t, content, "Old acceptance.")
				if strings.Count(content, ": Edited ") != 1 {
					t.Fatalf("expected one combined edit log entry, got content:\n%s", content)
				}
			},
		},
		{
			name: "inserts missing acceptance from stdin before log",
			initial: strings.Join([]string{
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
				"Existing goal.",
				"",
				"## Log",
				"",
				"- 2026-05-13: Created",
				"",
			}, "\n"),
			stdinInput: "\n\n- [ ] Read body from stdin\n- [ ] Preserve log\n\n",
			args: func(_, _ string) []string {
				return []string{"TB-1", "--acceptance", "-"}
			},
			assert: func(t *testing.T, content string) {
				t.Helper()
				assertContains(t, content, "## Goal\n\nExisting goal.\n\n## Acceptance Criteria\n\n- [ ] Read body from stdin\n- [ ] Preserve log\n\n## Log")
				assertContains(t, content, "- 2026-05-13: Created")
				assertContains(t, content, ": Edited acceptance")
				if strings.Count(content, "## Log") != 1 {
					t.Fatalf("log section should be preserved exactly once:\n%s", content)
				}
			},
		},
		{
			name: "inserts missing goal before context",
			initial: strings.Join([]string{
				"# TB-1: Existing Task",
				"",
				"**Type:** bug",
				"**Priority:** P2",
				"**Size:** M",
				"**Module:** cli",
				"**Branch:** -",
				"",
				"## Context",
				"",
				"Do not rewrite this.",
				"",
				"## Acceptance Criteria",
				"",
				"- [ ] Existing acceptance.",
				"",
				"## Log",
				"",
				"- 2026-05-13: Created",
				"",
			}, "\n"),
			goalInput: "\n\nInserted goal.\n\n",
			args: func(goalPath, _ string) []string {
				return []string{"TB-1", "--goal", goalPath}
			},
			assert: func(t *testing.T, content string) {
				t.Helper()
				assertContains(t, content, "**Branch:** -\n\n## Goal\n\nInserted goal.\n\n## Context")
				assertContains(t, content, "Do not rewrite this.")
				assertContains(t, content, "- [ ] Existing acceptance.")
				assertContains(t, content, ": Edited goal")
			},
		},
		{
			name: "strips leading section heading from supplied body",
			initial: strings.Join([]string{
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
				"Old goal.",
				"",
				"## Log",
				"",
				"- 2026-05-13: Created",
				"",
			}, "\n"),
			goalInput: "\n## Goal\n\nNew goal without duplicate heading.\n\n",
			args: func(goalPath, _ string) []string {
				return []string{"TB-1", "--goal", goalPath}
			},
			assert: func(t *testing.T, content string) {
				t.Helper()
				if strings.Count(content, "## Goal") != 1 {
					t.Fatalf("goal heading should not be duplicated:\n%s", content)
				}
				assertContains(t, content, "## Goal\n\nNew goal without duplicate heading.\n\n## Log")
			},
		},
		{
			name: "preserves fenced task headings inside edited section",
			initial: strings.Join([]string{
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
				"```md",
				"## Log",
				"not a real task section",
				"```",
				"",
				"Actual goal tail.",
				"",
				"## Log",
				"",
				"- 2026-05-13: Created",
				"",
			}, "\n"),
			goalInput: "\nReplacement goal.\n\n",
			args: func(goalPath, _ string) []string {
				return []string{"TB-1", "--goal", goalPath}
			},
			assert: func(t *testing.T, content string) {
				t.Helper()
				assertContains(t, content, "## Goal\n\nReplacement goal.\n\n## Log\n\n- 2026-05-13: Created")
				assertNotContains(t, content, "not a real task section")
			},
		},
		{
			name: "preserves non-task markdown headings inside edited section",
			initial: strings.Join([]string{
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
				"Old goal.",
				"",
				"## Acceptance Criteria",
				"",
				"- [ ] Keep acceptance.",
				"",
				"## Log",
				"",
				"- 2026-05-13: Created",
				"",
			}, "\n"),
			goalInput: "\nNew goal.\n\n## Example\n\nThis heading is content, not a task section.\n\n",
			args: func(goalPath, _ string) []string {
				return []string{"TB-1", "--goal", goalPath}
			},
			assert: func(t *testing.T, content string) {
				t.Helper()
				assertContains(t, content, "## Goal\n\nNew goal.\n\n## Example\n\nThis heading is content, not a task section.\n\n## Acceptance Criteria")
				assertContains(t, content, "- [ ] Keep acceptance.")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			boardDir := newCommandTestBoard(t)
			taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
			if err := os.WriteFile(taskPath, []byte(tc.initial), 0644); err != nil {
				t.Fatalf("write task: %v", err)
			}

			goalPath := ""
			if tc.goalInput != "" {
				goalPath = filepath.Join(t.TempDir(), "goal.md")
				if err := os.WriteFile(goalPath, []byte(tc.goalInput), 0644); err != nil {
					t.Fatalf("write goal input: %v", err)
				}
			}
			acceptPath := ""
			if tc.acceptInput != "" {
				acceptPath = filepath.Join(t.TempDir(), "acceptance.md")
				if err := os.WriteFile(acceptPath, []byte(tc.acceptInput), 0644); err != nil {
					t.Fatalf("write acceptance input: %v", err)
				}
			}

			run := func() {
				captureStdout(t, func() {
					cmdEdit(tc.args(goalPath, acceptPath))
				})
			}
			if tc.stdinInput != "" {
				withStdin(t, tc.stdinInput, run)
			} else {
				run()
			}

			data, err := os.ReadFile(taskPath)
			if err != nil {
				t.Fatalf("read task: %v", err)
			}
			tc.assert(t, string(data))

			if _, err := os.Stat(filepath.Join(boardDir, "BOARD.md")); err != nil {
				t.Fatalf("BOARD.md should be regenerated: %v", err)
			}
		})
	}
}

func TestEditTitleRename(t *testing.T) {
	initial := strings.Join([]string{
		"# TB-1: Original title",
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Body content unchanged.",
		"",
		"## Log",
		"",
		"- 2026-05-13: Created",
		"",
	}, "\n")

	t.Run("rewrites header and logs rename", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		out := captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--title", "Renamed title"})
		})

		data, err := os.ReadFile(taskPath)
		if err != nil {
			t.Fatalf("read task: %v", err)
		}
		content := string(data)

		if !strings.HasPrefix(content, "# TB-1: Renamed title\n") {
			t.Fatalf("expected header rewritten; got first line %q", strings.SplitN(content, "\n", 2)[0])
		}
		assertContains(t, content, "Body content unchanged.")
		assertContains(t, content, "**Module:** cli")
		assertContains(t, content, ": Edited title=Renamed title")
		assertContains(t, out, "Updated TB-1: title=Renamed title")
		if strings.Count(content, "## Log") != 1 {
			t.Fatalf("log section should appear exactly once:\n%s", content)
		}
		if _, err := os.Stat(filepath.Join(boardDir, "BOARD.md")); err != nil {
			t.Fatalf("BOARD.md should be regenerated: %v", err)
		}
	})

	t.Run("unchanged title is a no-op", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}
		preStat, err := os.Stat(taskPath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}

		out := captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--title", "Original title"})
		})

		data, err := os.ReadFile(taskPath)
		if err != nil {
			t.Fatalf("read task: %v", err)
		}
		if string(data) != initial {
			t.Fatalf("no-op rename should not modify file; got:\n%s", string(data))
		}
		assertContains(t, out, "Updated TB-1: no changes")
		postStat, err := os.Stat(taskPath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if !preStat.ModTime().Equal(postStat.ModTime()) {
			t.Fatalf("no-op rename rewrote the file (mtime changed: %v -> %v)", preStat.ModTime(), postStat.ModTime())
		}
	})

	t.Run("title combined with metadata edit produces one log entry", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--title", "New title", "-p", "P1"})
		})

		data, err := os.ReadFile(taskPath)
		if err != nil {
			t.Fatalf("read task: %v", err)
		}
		content := string(data)
		assertContains(t, content, "# TB-1: New title")
		assertContains(t, content, "**Priority:** P1")
		assertContains(t, content, ": Edited priority=P1, title=New title")
		if strings.Count(content, ": Edited ") != 1 {
			t.Fatalf("expected one combined edit log entry, got:\n%s", content)
		}
	})

}

func TestEditMetadataDoesNotScanBody(t *testing.T) {
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
		"Body examples must stay body examples:",
		"**Agent:** codex",
		"```md",
		"**Branch:** feat/body-example",
		"```",
		"",
		"## Log",
		"",
		"- 2026-05-13: Created",
		"",
	}, "\n")

	boardDir := newCommandTestBoard(t)
	taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	captureStdout(t, func() {
		cmdEdit([]string{"TB-1", "-a", "claude"})
	})
	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task after set: %v", err)
	}
	content := string(data)
	assertContains(t, content, "**Agent:** claude\n**Branch:** -")
	assertContains(t, content, "Body examples must stay body examples:\n**Agent:** codex")
	assertContains(t, content, "**Branch:** feat/body-example")
	if strings.Index(content, "**Agent:** claude") > strings.Index(content, "## Goal") {
		t.Fatalf("metadata agent was inserted into the body:\n%s", content)
	}

	captureStdout(t, func() {
		cmdEdit([]string{"TB-1", "-a", "none"})
	})
	data, err = os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task after clear: %v", err)
	}
	content = string(data)
	assertNotContains(t, metadataHeader(content), "**Agent:**")
	assertContains(t, content, "Body examples must stay body examples:\n**Agent:** codex")
}

// TestEditUserAttentionSection covers the TB-182 `--user-attention`
// managed-section path: insert, replace, strip-leading-heading, and
// placement relative to Acceptance Criteria / Related Tasks / Log.
func TestEditUserAttentionSection(t *testing.T) {
	t.Run("inserts ## User Attention before Related Tasks", func(t *testing.T) {
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
			"Keep this goal.",
			"",
			"## Acceptance Criteria",
			"",
			"- [ ] Keep this.",
			"",
			"## Related Tasks",
			"",
			"- TB-9 — sibling",
			"",
			"## Log",
			"",
			"- 2026-05-19: Created",
			"",
		}, "\n")

		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		inputPath := filepath.Join(t.TempDir(), "ua.md")
		body := "Reason: clarification needed.\n\nQuestion: should we keep legacy support?\n\nUnblock: user replies yes/no.\n"
		if err := os.WriteFile(inputPath, []byte(body), 0644); err != nil {
			t.Fatalf("write ua input: %v", err)
		}

		out := captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--user-attention", inputPath})
		})
		assertContains(t, out, "user-attention")

		data, err := os.ReadFile(taskPath)
		if err != nil {
			t.Fatalf("read task: %v", err)
		}
		content := string(data)
		assertContains(t, content, "## User Attention\n\nReason: clarification needed.")
		assertContains(t, content, "Unblock: user replies yes/no.")
		// Placement: after Acceptance Criteria, before Related Tasks.
		acceptIdx := strings.Index(content, "## Acceptance Criteria")
		uaIdx := strings.Index(content, "## User Attention")
		relIdx := strings.Index(content, "## Related Tasks")
		if !(acceptIdx < uaIdx && uaIdx < relIdx) {
			t.Fatalf("expected ## User Attention between Acceptance Criteria and Related Tasks; got order accept=%d ua=%d rel=%d\n%s",
				acceptIdx, uaIdx, relIdx, content)
		}
		assertContains(t, content, ": Edited user-attention")
	})

	t.Run("replaces existing ## User Attention", func(t *testing.T) {
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
			"Keep this goal.",
			"",
			"## User Attention",
			"",
			"Old ask.",
			"",
			"## Log",
			"",
			"- 2026-05-19: Created",
			"",
		}, "\n")

		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}

		inputPath := filepath.Join(t.TempDir(), "ua.md")
		body := "## User Attention\n\nFresh ask with leading heading that should be stripped.\n"
		if err := os.WriteFile(inputPath, []byte(body), 0644); err != nil {
			t.Fatalf("write ua input: %v", err)
		}

		captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--user-attention", inputPath})
		})

		data, err := os.ReadFile(taskPath)
		if err != nil {
			t.Fatalf("read task: %v", err)
		}
		content := string(data)
		if strings.Count(content, "## User Attention") != 1 {
			t.Fatalf("heading should not be duplicated:\n%s", content)
		}
		assertContains(t, content, "Fresh ask with leading heading that should be stripped.")
		assertNotContains(t, content, "Old ask.")
	})

}

// TestEditAgentStatusNeedsUser confirms the validator accepts the
// `needs-user` value (TB-182) that autonomous agents write when they
// stop because they need user input. The accompanying `## User
// Attention` section is exercised in TestEditUserAttentionSection.
func TestEditAgentStatusNeedsUser(t *testing.T) {
	initial := strings.Join([]string{
		"# TB-1: Existing Task",
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** cli",
		"**Agent:** claude",
		"**AgentStatus:** running",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Cover needs-user round-trip.",
		"",
		"## Log",
		"",
		"- 2026-05-19: Created",
		"",
	}, "\n")

	boardDir := newCommandTestBoard(t)
	taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	out := captureStdout(t, func() {
		cmdEdit([]string{"TB-1", "--agent-status", "needs-user"})
	})
	assertContains(t, out, "agentstatus=needs-user")

	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	content := string(data)
	assertContains(t, content, "**AgentStatus:** needs-user")
	assertContains(t, content, ": Edited agentstatus=needs-user")

	// Resolution path: clearing AgentStatus to "none" drops the field so
	// manual Run/Groom and the daemon can pick the task up again.
	captureStdout(t, func() {
		cmdEdit([]string{"TB-1", "--agent-status", "none"})
	})
	data, err = os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read after clear: %v", err)
	}
	content = string(data)
	assertNotContains(t, metadataHeader(content), "**AgentStatus:**")
}

// TestEditAgentStatusInterrupted confirms the validator accepts the
// `interrupted` value the recovery path needs to write. The "nothing
// manual writes interrupted" rule is convention-based (same precedent
// as `cancelled`), not enforced here.
func TestEditAgentStatusInterrupted(t *testing.T) {
	initial := strings.Join([]string{
		"# TB-1: Existing Task",
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** cli",
		"**Agent:** claude",
		"**AgentStatus:** running",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Cover the interrupted status round-trip.",
		"",
		"## Log",
		"",
		"- 2026-05-14: Created",
		"",
	}, "\n")

	boardDir := newCommandTestBoard(t)
	taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	if err := os.WriteFile(taskPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	out := captureStdout(t, func() {
		cmdEdit([]string{"TB-1", "--agent-status", "interrupted"})
	})
	assertContains(t, out, "agentstatus=interrupted")

	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	content := string(data)
	assertContains(t, content, "**AgentStatus:** interrupted")
	assertContains(t, content, ": Edited agentstatus=interrupted")
}

func metadataHeader(content string) string {
	if idx := strings.Index(content, "\n## "); idx != -1 {
		return content[:idx]
	}
	return content
}

func newCommandTestBoard(t *testing.T) string {
	t.Helper()

	prevCfg := cfg
	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, status := range allStatusDirs {
		if err := os.MkdirAll(filepath.Join(boardDir, status), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", status, err)
		}
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("2\n"), 0644); err != nil {
		t.Fatalf("write .next-id: %v", err)
	}

	cfg = tbConfig{
		RootDir:        root,
		BoardDir:       boardDir,
		Prefix:         "TB",
		WipLimit:       2,
		ScanExtensions: defaultScanExtensions(),
	}
	t.Cleanup(func() { cfg = prevCfg })
	return boardDir
}

func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	if _, err := io.WriteString(w, input); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	fn()
}

func assertContains(t *testing.T, text, needle string) {
	t.Helper()
	if !strings.Contains(text, needle) {
		t.Fatalf("missing %q in:\n%s", needle, text)
	}
}
