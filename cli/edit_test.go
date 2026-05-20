package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
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

func TestEditHelpDocumentsContextAndConstraints(t *testing.T) {
	if os.Getenv("TB_TEST_EDIT_HELP") == "1" {
		cmdEdit([]string{"--help"})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestEditHelpDocumentsContextAndConstraints$", "-test.v")
	cmd.Env = append(os.Environ(), "TB_TEST_EDIT_HELP=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("help command should exit 0; got %v\nstderr:\n%s", err, stderr.String())
	}
	help := stderr.String()
	assertContains(t, help, "[--context file|-]")
	assertContains(t, help, "[--constraints file|-]")
	assertContains(t, help, "-context string")
	assertContains(t, help, "replace/insert ## Context from file path or - for stdin")
	assertContains(t, help, "-constraints string")
	assertContains(t, help, "replace/insert ## Constraints from file path or - for stdin")
}

func TestEditContextAndConstraintsSections(t *testing.T) {
	t.Run("replaces context from stdin and preserves folder task side files", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskDir := filepath.Join(boardDir, "in-progress", "TB-1")
		if err := os.MkdirAll(filepath.Join(taskDir, ".agent-logs"), 0755); err != nil {
			t.Fatalf("mkdir folder task: %v", err)
		}
		taskPath := filepath.Join(taskDir, folderTaskFileName)
		initial := strings.Join([]string{
			"# TB-1: Existing Task",
			"",
			"**Type:** improvement",
			"**Priority:** P2",
			"**Size:** S",
			"**Module:** cli",
			"**Agent:** codex",
			"**AgentStatus:** running",
			"**Branch:** feature/context",
			"",
			"## Goal",
			"",
			"Keep this goal.",
			"",
			"## Context",
			"",
			"Old context.",
			"",
			"## Constraints",
			"",
			"- Keep existing constraints.",
			"",
			"## Acceptance Criteria",
			"",
			"- [ ] Keep acceptance.",
			"",
			"## Related Tasks",
			"",
			"- **TB-9** — sibling",
			"",
			"## Attachments",
			"",
			"- evidence.txt",
			"",
			"## Log",
			"",
			"- 2026-05-19: Created",
			"- 2026-05-20: Previous history",
			"",
		}, "\n")
		writeFileForTest(t, taskPath, initial)
		writeFileForTest(t, filepath.Join(taskDir, "evidence.txt"), "attachment\n")
		writeFileForTest(t, filepath.Join(taskDir, ".agent-state.jsonl"), `{"event":"started"}`+"\n")
		writeFileForTest(t, filepath.Join(taskDir, ".agent-logs", "r_1.log"), "log\n")

		stdin := strings.Join([]string{
			"",
			"## Context",
			"",
			"Fresh context from stdin.",
			"",
			"```md",
			"## Constraints",
			"literal fenced heading, not the task constraints",
			"```",
			"",
			"Inline literal text `## Context` stays inside the section.",
			"",
		}, "\n")
		withStdin(t, stdin, func() {
			captureStdout(t, func() {
				cmdEdit([]string{"TB-1", "--context", "-"})
			})
		})

		content := readFileForTest(t, taskPath)
		assertContains(t, content, "**AgentStatus:** running")
		assertContains(t, content, "## Context\n\nFresh context from stdin.")
		assertContains(t, content, "```md\n## Constraints\nliteral fenced heading, not the task constraints\n```")
		assertContains(t, content, "Inline literal text `## Context` stays inside the section.")
		assertContains(t, content, "## Constraints\n\n- Keep existing constraints.")
		assertContains(t, content, "## Related Tasks\n\n- **TB-9**")
		assertContains(t, content, "## Attachments\n\n- evidence.txt")
		assertContains(t, content, "- 2026-05-20: Previous history")
		assertContains(t, content, ": Edited context")
		assertNotContains(t, content, "Old context.")
		assertSectionsInOrder(t, content, []string{"## Goal", "## Context", "## Constraints", "## Acceptance Criteria", "## Related Tasks", "## Attachments", "## Log"})
		assertPathExists(t, filepath.Join(taskDir, "evidence.txt"))
		assertPathExists(t, filepath.Join(taskDir, ".agent-state.jsonl"))
		assertPathExists(t, filepath.Join(taskDir, ".agent-logs", "r_1.log"))
		assertPathMissing(t, filepath.Join(boardDir, "backlog", "TB-1.md"))
	})

	t.Run("replaces constraints from file and strips leading heading", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		initial := strings.Join([]string{
			"# TB-1: Existing Task",
			"",
			"**Type:** improvement",
			"**Priority:** P2",
			"**Size:** S",
			"**Module:** cli",
			"**Branch:** -",
			"",
			"## Goal",
			"",
			"Keep this goal.",
			"",
			"## Context",
			"",
			"Keep this context.",
			"",
			"## Constraints",
			"",
			"Old constraints.",
			"```md",
			"## Context",
			"literal fenced heading, not context",
			"```",
			"",
			"## Acceptance Criteria",
			"",
			"- [ ] Keep acceptance.",
			"",
			"## Log",
			"",
			"- 2026-05-19: Created",
			"",
		}, "\n")
		writeFileForTest(t, taskPath, initial)
		inputPath := filepath.Join(t.TempDir(), "constraints.md")
		writeFileForTest(t, inputPath, strings.Join([]string{
			"## Constraints",
			"",
			"- Fresh constraint from file.",
			"- Literal text `## Constraints` remains content.",
			"",
		}, "\n"))

		captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--constraints", inputPath})
		})

		content := readFileForTest(t, taskPath)
		if strings.Count(content, "\n## Constraints\n") != 1 {
			t.Fatalf("constraints heading should not be duplicated:\n%s", content)
		}
		assertContains(t, content, "## Context\n\nKeep this context.")
		assertContains(t, content, "## Constraints\n\n- Fresh constraint from file.\n- Literal text `## Constraints` remains content.\n\n## Acceptance Criteria")
		assertContains(t, content, "- [ ] Keep acceptance.")
		assertContains(t, content, ": Edited constraints")
		assertNotContains(t, content, "Old constraints.")
		assertNotContains(t, content, "literal fenced heading, not context")
	})

	t.Run("inserts missing context and constraints in canonical order", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		initial := strings.Join([]string{
			"# TB-1: Existing Task",
			"",
			"**Type:** improvement",
			"**Priority:** P2",
			"**Size:** S",
			"**Module:** cli",
			"**Branch:** -",
			"",
			"## Goal",
			"",
			"Keep this goal.",
			"",
			"## Acceptance Criteria",
			"",
			"- [ ] Keep acceptance.",
			"",
			"## Log",
			"",
			"- 2026-05-19: Created",
			"",
		}, "\n")
		writeFileForTest(t, taskPath, initial)
		contextPath := filepath.Join(t.TempDir(), "context.md")
		constraintsPath := filepath.Join(t.TempDir(), "constraints.md")
		writeFileForTest(t, contextPath, "Inserted context.\n")
		writeFileForTest(t, constraintsPath, "Inserted constraints.\n")

		captureStdout(t, func() {
			cmdEdit([]string{"TB-1", "--constraints", constraintsPath, "--context", contextPath})
		})

		content := readFileForTest(t, taskPath)
		assertContains(t, content, "## Context\n\nInserted context.")
		assertContains(t, content, "## Constraints\n\nInserted constraints.")
		assertSectionsInOrder(t, content, []string{"## Goal", "## Context", "## Constraints", "## Acceptance Criteria", "## Log"})
		assertContains(t, content, ": Edited context, constraints")
	})

	t.Run("inserts missing context from stdin before existing constraints", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		initial := strings.Join([]string{
			"# TB-1: Existing Task",
			"",
			"**Type:** improvement",
			"**Priority:** P2",
			"**Size:** S",
			"**Module:** cli",
			"**Branch:** -",
			"",
			"## Goal",
			"",
			"Keep this goal.",
			"",
			"## Constraints",
			"",
			"Existing constraints.",
			"",
			"## Acceptance Criteria",
			"",
			"- [ ] Keep acceptance.",
			"",
			"## Log",
			"",
			"- 2026-05-19: Created",
			"",
		}, "\n")
		writeFileForTest(t, taskPath, initial)

		withStdin(t, "\nInserted context from stdin.\n\n", func() {
			captureStdout(t, func() {
				cmdEdit([]string{"TB-1", "--context", "-"})
			})
		})

		content := readFileForTest(t, taskPath)
		assertContains(t, content, "## Context\n\nInserted context from stdin.\n\n## Constraints")
		assertSectionsInOrder(t, content, []string{"## Goal", "## Context", "## Constraints", "## Acceptance Criteria", "## Log"})
	})

	t.Run("inserts missing constraints from stdin after existing context", func(t *testing.T) {
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		initial := strings.Join([]string{
			"# TB-1: Existing Task",
			"",
			"**Type:** improvement",
			"**Priority:** P2",
			"**Size:** S",
			"**Module:** cli",
			"**Branch:** -",
			"",
			"## Goal",
			"",
			"Keep this goal.",
			"",
			"## Context",
			"",
			"Existing context.",
			"",
			"## Acceptance Criteria",
			"",
			"- [ ] Keep acceptance.",
			"",
			"## Log",
			"",
			"- 2026-05-19: Created",
			"",
		}, "\n")
		writeFileForTest(t, taskPath, initial)

		withStdin(t, "\nInserted constraints from stdin.\n\n", func() {
			captureStdout(t, func() {
				cmdEdit([]string{"TB-1", "--constraints", "-"})
			})
		})

		content := readFileForTest(t, taskPath)
		assertContains(t, content, "## Constraints\n\nInserted constraints from stdin.\n\n## Acceptance Criteria")
		assertSectionsInOrder(t, content, []string{"## Goal", "## Context", "## Constraints", "## Acceptance Criteria", "## Log"})
	})
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

// TestEditAgentStatusRecoveryValues confirms the validator accepts the
// recovery-only values the daemon needs to write. The "nothing manual writes
// recovery statuses" rule is convention-based (same precedent as
// `cancelled`), not enforced here.
func TestEditAgentStatusRecoveryValues(t *testing.T) {
	for _, status := range []string{"interrupted", "lost"} {
		t.Run(status, func(t *testing.T) {
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
				"Cover the recovery status round-trip.",
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
				cmdEdit([]string{"TB-1", "--agent-status", status})
			})
			assertContains(t, out, "agentstatus="+status)

			data, err := os.ReadFile(taskPath)
			if err != nil {
				t.Fatalf("read task: %v", err)
			}
			content := string(data)
			assertContains(t, content, "**AgentStatus:** "+status)
			assertContains(t, content, ": Edited agentstatus="+status)
		})
	}
}

// TestEditPerModeAttribution exercises the TB-237 per-mode pairs end-to-end:
// set each pair, verify the metadata lines and log entry, then clear and
// verify the lines are dropped.
func TestEditPerModeAttribution(t *testing.T) {
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
		"Cover per-mode attribution round-trip.",
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
		cmdEdit([]string{
			"TB-1",
			"--groomed-by", "claude",
			"--groom-status", "success",
			"--implemented-by", "codex",
			"--implement-status", "running",
			"--reviewed-by", "claude",
			"--review-status", "failed",
		})
	})
	assertContains(t, out, "groomed-by=claude")
	assertContains(t, out, "groom-status=success")
	assertContains(t, out, "implemented-by=codex")
	assertContains(t, out, "implement-status=running")
	assertContains(t, out, "reviewed-by=claude")
	assertContains(t, out, "review-status=failed")

	data, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	content := string(data)
	assertContains(t, content, "**GroomedBy:** claude")
	assertContains(t, content, "**GroomStatus:** success")
	assertContains(t, content, "**ImplementedBy:** codex")
	assertContains(t, content, "**ImplementStatus:** running")
	assertContains(t, content, "**ReviewedBy:** claude")
	assertContains(t, content, "**ReviewStatus:** failed")

	// Round-trip the parser too: each per-mode pair must come back via
	// parseTaskFile so the JSON wire shape and the GUI see the values.
	parsed, err := parseTaskFile(taskPath)
	if err != nil {
		t.Fatalf("parseTaskFile: %v", err)
	}
	if parsed.GroomedBy != "claude" || parsed.GroomStatus != "success" {
		t.Fatalf("groom round-trip: got (%q,%q)", parsed.GroomedBy, parsed.GroomStatus)
	}
	if parsed.ImplementedBy != "codex" || parsed.ImplementStatus != "running" {
		t.Fatalf("implement round-trip: got (%q,%q)", parsed.ImplementedBy, parsed.ImplementStatus)
	}
	if parsed.ReviewedBy != "claude" || parsed.ReviewStatus != "failed" {
		t.Fatalf("review round-trip: got (%q,%q)", parsed.ReviewedBy, parsed.ReviewStatus)
	}

	// "none" sentinel clears the line, mirroring --agent / --agent-status.
	captureStdout(t, func() {
		cmdEdit([]string{
			"TB-1",
			"--groomed-by", "none",
			"--groom-status", "none",
		})
	})
	data, err = os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read after clear: %v", err)
	}
	content = string(data)
	assertNotContains(t, metadataHeader(content), "**GroomedBy:**")
	assertNotContains(t, metadataHeader(content), "**GroomStatus:**")
	// The other two pairs survive — clearing one pair must not collateral-
	// damage neighbouring fields.
	assertContains(t, metadataHeader(content), "**ImplementedBy:** codex")
	assertContains(t, metadataHeader(content), "**ReviewStatus:** failed")
}

// TestEditPerModeAttributionInvalidValues confirms the validator rejects
// bad agent names and status values on the new TB-237 flags. The validator
// calls os.Exit(1), so we exec a child test binary scoped to one of the
// invalid-value branches and inspect its exit code + stderr.
func TestEditPerModeAttributionInvalidValues(t *testing.T) {
	if mode := os.Getenv("TB_TEST_PER_MODE_INVALID"); mode != "" {
		// Child re-exec branch: build a board, write a task, then let
		// cmdEdit reject the bogus value via os.Exit(1).
		boardDir := newCommandTestBoard(t)
		taskPath := filepath.Join(boardDir, "backlog", "TB-1.md")
		body := "# TB-1: child\n\n**Type:** bug\n**Priority:** P2\n**Size:** M\n**Module:** cli\n**Branch:** -\n\n## Goal\n\nfoo\n\n## Log\n\n- 2026-05-19: Created\n"
		if err := os.WriteFile(taskPath, []byte(body), 0644); err != nil {
			t.Fatalf("write task: %v", err)
		}
		switch mode {
		case "agent":
			cmdEdit([]string{"TB-1", "--implemented-by", "bogus"})
		case "status":
			cmdEdit([]string{"TB-1", "--review-status", "bogus"})
		}
		return
	}

	cases := []struct {
		mode       string
		wantStderr string
	}{
		{mode: "agent", wantStderr: "invalid implemented-by"},
		{mode: "status", wantStderr: "invalid review-status"},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestEditPerModeAttributionInvalidValues$", "-test.v")
			cmd.Env = append(os.Environ(), "TB_TEST_PER_MODE_INVALID="+tc.mode)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			exitErr := &exec.ExitError{}
			if !errors.As(err, &exitErr) {
				t.Fatalf("child should exit non-zero; got err=%v\nstderr:\n%s", err, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("expected stderr to contain %q; got:\n%s", tc.wantStderr, stderr.String())
			}
		})
	}
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
		RootDir:  root,
		BoardDir: boardDir,
		Prefix:   "TB",
		WipLimit: 2,
		// WipLimits intentionally left nil so tests opt-in to WIP-limited
		// columns explicitly. enforceWipLimit returns silently when no
		// limit is configured for a status, so default behavior matches
		// the pre-canonical-kanban code path. Tests that exercise WIP
		// enforcement should seed cfg.WipLimits and cfg.WipEnforcement.
		WipEnforcement: "warn",
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
