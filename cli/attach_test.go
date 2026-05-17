package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAttachPromotesLegacyFileTask(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	legacyPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	initial := legacyTaskContent("TB-1", "Legacy Task")
	if err := os.WriteFile(legacyPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}
	sourcePath := writeAttachmentSource(t, "design.txt", "design bytes")

	result, err := attachTask(boardDir, "TB-1", []string{sourcePath})
	if err != nil {
		t.Fatalf("attachTask: %v", err)
	}
	if !result.promoted {
		t.Fatal("attachTask did not report promotion")
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy file should be removed after promotion, stat err=%v", err)
	}
	taskPath := filepath.Join(boardDir, "backlog", "TB-1", folderTaskFileName)
	content := readFileString(t, taskPath)
	assertContains(t, content, "# TB-1: Legacy Task")
	assertContains(t, content, "**Module:** cli")
	assertContains(t, content, "- 2026-05-14: Created")
	assertContains(t, content, "## Attachments\n\n- design.txt\n\n## Log")
	assertContains(t, content, "Promoted to folder form")
	assertContains(t, content, "Attached design.txt")

	attachmentPath := filepath.Join(boardDir, "backlog", "TB-1", "design.txt")
	if got := readFileString(t, attachmentPath); got != "design bytes" {
		t.Fatalf("attachment content = %q", got)
	}

	boardContent, err := buildBoardContent(boardDir)
	if err != nil {
		t.Fatalf("buildBoardContent: %v", err)
	}
	assertContains(t, boardContent, "| TB-1 | Legacy Task | bug | P2 | M | cli |")
}

func TestAttachPromotionMigratesLegacyAgentArtifacts(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	legacyPath := filepath.Join(boardDir, "backlog", "TB-1.md")
	if err := os.WriteFile(legacyPath, []byte(legacyTaskContent("TB-1", "Legacy Agent")), 0644); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}

	rootStateDir := filepath.Join(boardDir, ".agent-state")
	if err := os.MkdirAll(rootStateDir, 0755); err != nil {
		t.Fatalf("mkdir root state: %v", err)
	}
	stateBytes := []byte(`{"event":"queued","run_id":"r_abc12345","task_id":"TB-1"}` + "\n" +
		`{"event":"finished","run_id":"r_abc12345","task_id":"TB-1","status":"success","exit_code":0}` + "\n")
	rootStatePath := filepath.Join(rootStateDir, "TB-1.jsonl")
	if err := os.WriteFile(rootStatePath, stateBytes, 0644); err != nil {
		t.Fatalf("write root state: %v", err)
	}

	rootLogsDir := filepath.Join(boardDir, ".agent-logs", "TB-1")
	if err := os.MkdirAll(rootLogsDir, 0755); err != nil {
		t.Fatalf("mkdir root logs: %v", err)
	}
	logBytes := []byte("stdout line one\nstdout line two\n")
	rootLogPath := filepath.Join(rootLogsDir, "r_abc12345.log")
	if err := os.WriteFile(rootLogPath, logBytes, 0644); err != nil {
		t.Fatalf("write root log: %v", err)
	}

	sourcePath := writeAttachmentSource(t, "design.txt", "design bytes")
	result, err := attachTask(boardDir, "TB-1", []string{sourcePath})
	if err != nil {
		t.Fatalf("attachTask: %v", err)
	}
	if !result.promoted {
		t.Fatal("attachTask did not report promotion")
	}

	taskDir := filepath.Join(boardDir, "backlog", "TB-1")
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	content := readFileString(t, taskPath)
	assertContains(t, content, "# TB-1: Legacy Agent")
	assertContains(t, content, "## Attachments\n\n- design.txt\n\n## Log")

	migratedState := filepath.Join(taskDir, ".agent-state.jsonl")
	if got := readFileString(t, migratedState); got != string(stateBytes) {
		t.Fatalf("migrated state bytes = %q, want %q", got, string(stateBytes))
	}
	migratedLog := filepath.Join(taskDir, ".agent-logs", "r_abc12345.log")
	if got := readFileString(t, migratedLog); got != string(logBytes) {
		t.Fatalf("migrated log bytes = %q, want %q", got, string(logBytes))
	}

	if _, err := os.Stat(rootStatePath); !os.IsNotExist(err) {
		t.Fatalf("legacy root state should be gone after promotion, stat err=%v", err)
	}
	if _, err := os.Stat(rootLogsDir); !os.IsNotExist(err) {
		t.Fatalf("legacy root logs dir should be gone after promotion, stat err=%v", err)
	}

	attachmentPath := filepath.Join(taskDir, "design.txt")
	if got := readFileString(t, attachmentPath); got != "design bytes" {
		t.Fatalf("attachment content = %q", got)
	}
}

func TestAttachPromotionTolerantsMissingLegacyArtifacts(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	legacyPath := filepath.Join(boardDir, "backlog", "TB-2.md")
	if err := os.WriteFile(legacyPath, []byte(legacyTaskContent("TB-2", "No Agent")), 0644); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}
	sourcePath := writeAttachmentSource(t, "spec.txt", "spec bytes")

	if _, err := attachTask(boardDir, "TB-2", []string{sourcePath}); err != nil {
		t.Fatalf("attachTask: %v", err)
	}
	taskDir := filepath.Join(boardDir, "backlog", "TB-2")
	if _, err := os.Stat(filepath.Join(taskDir, ".agent-state.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("unexpected migrated state file, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(taskDir, ".agent-logs")); !os.IsNotExist(err) {
		t.Fatalf("unexpected migrated logs dir, stat err=%v", err)
	}
}

func TestAttachAddRespectsDoubleDashTerminator(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	seedFolderTask(t, boardDir, "backlog", "TB-7", "Dash Paths")
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "-leading-dash.txt")
	if err := os.WriteFile(srcPath, []byte("payload"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var out bytes.Buffer
	if err := runAttach([]string{"TB-7", "--", srcPath}, &out); err != nil {
		t.Fatalf("runAttach: %v", err)
	}

	attached := filepath.Join(boardDir, "backlog", "TB-7", "-leading-dash.txt")
	if got := readFileString(t, attached); got != "payload" {
		t.Fatalf("attachment content = %q", got)
	}
	assertContains(t, out.String(), "Attached 1 file(s) to TB-7")
}

func TestAttachAddTreatsRemoveLikePathAfterTerminator(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	seedFolderTask(t, boardDir, "backlog", "TB-8", "Smuggled Flag")
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "--rm")
	if err := os.WriteFile(srcPath, []byte("payload"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := runAttach([]string{"TB-8", "--", srcPath}, nil); err != nil {
		t.Fatalf("runAttach: %v", err)
	}
	attached := filepath.Join(boardDir, "backlog", "TB-8", "--rm")
	if got := readFileString(t, attached); got != "payload" {
		t.Fatalf("attachment content = %q", got)
	}
}

func TestAttachAddsFileToFolderTask(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTask(t, boardDir, "in-progress", "TB-2", "Folder Task")
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
	if err := os.Mkdir(attachmentsDir, 0755); err != nil {
		t.Fatalf("mkdir attachments: %v", err)
	}
	if err := os.WriteFile(filepath.Join(attachmentsDir, "old.txt"), []byte("old"), 0644); err != nil {
		t.Fatalf("write existing attachment: %v", err)
	}
	sourcePath := writeAttachmentSource(t, "new.txt", "new")

	result, err := attachTask(boardDir, "2", []string{sourcePath})
	if err != nil {
		t.Fatalf("attachTask: %v", err)
	}
	if result.promoted {
		t.Fatal("folder-form task should not be promoted again")
	}

	content := readFileString(t, filepath.Join(taskDir, folderTaskFileName))
	assertContains(t, content, "## Attachments\n\n- attachments/old.txt\n- new.txt\n\n## Log")
	assertContains(t, content, "Attached new.txt")
	if got := readFileString(t, filepath.Join(taskDir, "new.txt")); got != "new" {
		t.Fatalf("new attachment content = %q", got)
	}
	if got := readFileString(t, filepath.Join(attachmentsDir, "old.txt")); got != "old" {
		t.Fatalf("existing attachment content = %q", got)
	}
}

func TestAttachAddsFileToTaskRootAndKeepsLegacyAttachments(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTaskWithAttachments(t, boardDir, "backlog", "TB-22", "Mixed Attach", []string{"legacy.txt"})
	sourcePath := writeAttachmentSource(t, "root.txt", "root bytes")

	if _, err := attachTask(boardDir, "TB-22", []string{sourcePath}); err != nil {
		t.Fatalf("attachTask: %v", err)
	}

	assertExists(t, filepath.Join(taskDir, "root.txt"))
	assertExists(t, filepath.Join(taskDir, attachmentsDirName, "legacy.txt"))
	content := readFileString(t, filepath.Join(taskDir, folderTaskFileName))
	assertContains(t, content, "## Attachments\n\n- attachments/legacy.txt\n- root.txt\n\n## Log")
}

func TestAttachImportsMultipleFiles(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTask(t, boardDir, "backlog", "TB-3", "Multi Attach")
	aPath := writeAttachmentSource(t, "b.txt", "bee")
	bPath := writeAttachmentSource(t, "a.txt", "aye")

	if _, err := attachTask(boardDir, "TB-3", []string{aPath, bPath}); err != nil {
		t.Fatalf("attachTask: %v", err)
	}

	if got := readFileString(t, filepath.Join(taskDir, "a.txt")); got != "aye" {
		t.Fatalf("a.txt = %q", got)
	}
	if got := readFileString(t, filepath.Join(taskDir, "b.txt")); got != "bee" {
		t.Fatalf("b.txt = %q", got)
	}
	content := readFileString(t, filepath.Join(taskDir, folderTaskFileName))
	assertContains(t, content, "## Attachments\n\n- a.txt\n- b.txt\n\n## Log")
	assertContains(t, content, "Attached b.txt, a.txt")
}

func TestAttachMissingSourceLeavesLegacyTaskUnchanged(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	legacyPath := filepath.Join(boardDir, "backlog", "TB-4.md")
	initial := legacyTaskContent("TB-4", "Missing Source")
	if err := os.WriteFile(legacyPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}
	validPath := writeAttachmentSource(t, "valid.txt", "valid")
	missingPath := filepath.Join(t.TempDir(), "missing.txt")

	if _, err := attachTask(boardDir, "TB-4", []string{validPath, missingPath}); err == nil {
		t.Fatal("attachTask succeeded, want missing-source error")
	}

	if got := readFileString(t, legacyPath); got != initial {
		t.Fatalf("legacy task changed after failed attach:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "backlog", "TB-4")); !os.IsNotExist(err) {
		t.Fatalf("promotion directory should not exist after failed attach, stat err=%v", err)
	}
}

func TestAttachNameCollisionLeavesFolderTaskUnchanged(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTask(t, boardDir, "backlog", "TB-5", "Collision")
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	initial := readFileString(t, taskPath)
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
	if err := os.Mkdir(attachmentsDir, 0755); err != nil {
		t.Fatalf("mkdir attachments: %v", err)
	}
	existingPath := filepath.Join(attachmentsDir, "same.txt")
	if err := os.WriteFile(existingPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("write existing attachment: %v", err)
	}
	sourcePath := writeAttachmentSource(t, "same.txt", "replacement")

	if _, err := attachTask(boardDir, "TB-5", []string{sourcePath}); err == nil {
		t.Fatal("attachTask succeeded, want collision error")
	}

	if got := readFileString(t, taskPath); got != initial {
		t.Fatalf("task content changed after collision:\n%s", got)
	}
	if got := readFileString(t, existingPath); got != "existing" {
		t.Fatalf("existing attachment was overwritten: %q", got)
	}
}

func TestAttachRejectsTaskRootCollision(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTask(t, boardDir, "backlog", "TB-6", "Root Collision")
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	initial := readFileString(t, taskPath)
	existingPath := filepath.Join(taskDir, "same.txt")
	if err := os.WriteFile(existingPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("write existing attachment: %v", err)
	}
	sourcePath := writeAttachmentSource(t, "same.txt", "replacement")

	if _, err := attachTask(boardDir, "TB-6", []string{sourcePath}); err == nil {
		t.Fatal("attachTask succeeded, want root collision error")
	}

	if got := readFileString(t, taskPath); got != initial {
		t.Fatalf("task content changed after collision:\n%s", got)
	}
	if got := readFileString(t, existingPath); got != "existing" {
		t.Fatalf("existing root attachment was overwritten: %q", got)
	}
}

func TestAttachRejectsReservedTaskRootNames(t *testing.T) {
	cases := []struct {
		name    string
		wantErr string
	}{
		{name: "TASK.md", wantErr: "reserved"},
		{name: ".agent-state.jsonl", wantErr: "reserved"},
		{name: ".hidden.tmp", wantErr: "reserved"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			boardDir := newCommandTestBoard(t)
			taskDir := seedFolderTask(t, boardDir, "backlog", "TB-9", "Reserved")
			taskPath := filepath.Join(taskDir, folderTaskFileName)
			initial := readFileString(t, taskPath)
			sourcePath := writeAttachmentSource(t, tc.name, "payload")

			_, err := attachTask(boardDir, "TB-9", []string{sourcePath})
			if err == nil {
				t.Fatal("attachTask succeeded, want reserved-name error")
			}
			assertContains(t, err.Error(), tc.wantErr)
			if after := readFileString(t, taskPath); after != initial {
				t.Fatalf("task content changed after reserved-name validation:\nbefore:\n%s\nafter:\n%s", initial, after)
			}
			if tc.name != folderTaskFileName {
				assertMissing(t, filepath.Join(taskDir, tc.name))
			}
			assertMissing(t, filepath.Join(taskDir, attachmentsDirName, tc.name))
		})
	}
}

func TestAttachRemoveSingleAttachment(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTaskWithAttachments(t, boardDir, "backlog", "TB-1", "Remove One", []string{"one.txt", "two.txt"})
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)

	var out bytes.Buffer
	if err := runAttach([]string{"--rm", "TB-1", "one.txt"}, &out); err != nil {
		t.Fatalf("runAttach: %v", err)
	}

	assertContains(t, out.String(), "Removed attachment from TB-1: one.txt")
	assertMissing(t, filepath.Join(attachmentsDir, "one.txt"))
	assertExists(t, filepath.Join(attachmentsDir, "two.txt"))

	content := readFileString(t, taskPath)
	assertNotContains(t, content, "- attachments/one.txt")
	assertContains(t, content, "- attachments/two.txt")
	assertContains(t, content, ": Removed attachments: one.txt")

	boardContent := readFileString(t, filepath.Join(boardDir, "BOARD.md"))
	assertContains(t, boardContent, "TB-1")
}

func TestAttachRemoveRootAndLegacyAttachments(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTaskWithAttachments(t, boardDir, "backlog", "TB-11", "Mixed Remove", []string{"legacy.txt"})
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	if err := os.WriteFile(filepath.Join(taskDir, "root.txt"), []byte("root\n"), 0644); err != nil {
		t.Fatalf("write root attachment: %v", err)
	}
	content := upsertAttachmentsSection(readFileString(t, taskPath), []string{"attachments/legacy.txt", "root.txt"})
	if err := os.WriteFile(taskPath, []byte(content), 0644); err != nil {
		t.Fatalf("write task attachments section: %v", err)
	}

	var out bytes.Buffer
	if err := runAttach([]string{"--rm", "TB-11", "root.txt", "attachments/legacy.txt"}, &out); err != nil {
		t.Fatalf("runAttach: %v", err)
	}

	assertContains(t, out.String(), "Removed attachments from TB-11: root.txt, attachments/legacy.txt")
	assertMissing(t, filepath.Join(taskDir, "root.txt"))
	assertMissing(t, filepath.Join(taskDir, attachmentsDirName, "legacy.txt"))
	content = readFileString(t, taskPath)
	assertNotContains(t, content, "- root.txt")
	assertNotContains(t, content, "- attachments/legacy.txt")
}

func TestAttachRemoveRejectsDuplicateLegacyAliasesWithoutMutation(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTaskWithAttachments(t, boardDir, "backlog", "TB-15", "Duplicate Aliases", []string{"old.txt"})
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
	before := readFileString(t, taskPath)

	err := runAttach([]string{"--rm", "TB-15", "old.txt", "attachments/old.txt"}, nil)
	if err == nil {
		t.Fatal("runAttach succeeded, want duplicate alias error")
	}
	assertContains(t, err.Error(), `duplicate attachment name "attachments/old.txt"`)
	assertExists(t, filepath.Join(attachmentsDir, "old.txt"))
	if after := readFileString(t, taskPath); after != before {
		t.Fatalf("task markdown changed after duplicate alias validation:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	assertMissing(t, filepath.Join(boardDir, "BOARD.md"))
}

func TestAttachRemoveDashLeadingFilenameAfterTerminator(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTaskWithAttachments(t, boardDir, "backlog", "TB-1", "Dash Remove", []string{"-dash.txt", "keep.txt"})
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)

	var out bytes.Buffer
	if err := runAttach([]string{"--rm", "TB-1", "--", "-dash.txt"}, &out); err != nil {
		t.Fatalf("runAttach: %v", err)
	}

	assertContains(t, out.String(), "Removed attachment from TB-1: -dash.txt")
	assertMissing(t, filepath.Join(attachmentsDir, "-dash.txt"))
	assertExists(t, filepath.Join(attachmentsDir, "keep.txt"))

	content := readFileString(t, taskPath)
	assertNotContains(t, content, "- attachments/-dash.txt")
	assertContains(t, content, "- attachments/keep.txt")
	assertContains(t, content, ": Removed attachments: -dash.txt")
}

func TestAttachRemoveMultipleAttachmentsInDone(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTaskWithAttachments(t, boardDir, "done", "TB-2", "Remove Many", []string{"a.txt", "b.txt", "keep.txt"})
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)

	var out bytes.Buffer
	if err := runAttach([]string{"--rm", "TB-2", "a.txt", "b.txt"}, &out); err != nil {
		t.Fatalf("runAttach: %v", err)
	}

	assertContains(t, out.String(), "Removed attachments from TB-2: a.txt, b.txt")
	assertMissing(t, filepath.Join(attachmentsDir, "a.txt"))
	assertMissing(t, filepath.Join(attachmentsDir, "b.txt"))
	assertExists(t, filepath.Join(attachmentsDir, "keep.txt"))

	content := readFileString(t, taskPath)
	assertNotContains(t, content, "- attachments/a.txt")
	assertNotContains(t, content, "- attachments/b.txt")
	assertContains(t, content, "- attachments/keep.txt")
	if got := strings.Count(content, ": Removed attachments: a.txt, b.txt"); got != 1 {
		t.Fatalf("removal log count = %d, want 1:\n%s", got, content)
	}
}

func TestAttachRemoveMissingAttachmentIsAllOrNothing(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTaskWithAttachments(t, boardDir, "backlog", "TB-1", "Missing Remove", []string{"one.txt"})
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
	before := readFileString(t, taskPath)

	err := runAttach([]string{"--rm", "TB-1", "one.txt", "missing.txt"}, nil)
	if err == nil {
		t.Fatal("runAttach succeeded, want missing attachment error")
	}
	assertContains(t, err.Error(), `attachment "missing.txt" not found`)
	assertExists(t, filepath.Join(attachmentsDir, "one.txt"))
	if after := readFileString(t, taskPath); after != before {
		t.Fatalf("task markdown changed after failed validation:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	assertMissing(t, filepath.Join(boardDir, "BOARD.md"))
}

func TestAttachRemoveRejectsNonFolderTargets(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T, boardDir string) string
		wantErr string
	}{
		{
			name: "legacy file form",
			setup: func(t *testing.T, boardDir string) string {
				path := filepath.Join(boardDir, "backlog", "TB-1.md")
				if err := os.WriteFile(path, []byte(legacyTaskContent("TB-1", "Legacy")), 0644); err != nil {
					t.Fatalf("write legacy task: %v", err)
				}
				return path
			},
			wantErr: `task TB-1 is file-form`,
		},
		{
			name: "folder form without matching attachment",
			setup: func(t *testing.T, boardDir string) string {
				taskDir := seedFolderTask(t, boardDir, "backlog", "TB-1", "No Attachments Dir")
				return filepath.Join(taskDir, folderTaskFileName)
			},
			wantErr: `attachment "one.txt" not found on TB-1`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			boardDir := newCommandTestBoard(t)
			taskPath := tc.setup(t, boardDir)
			before := readFileString(t, taskPath)

			err := runAttach([]string{"--rm", "TB-1", "one.txt"}, nil)
			if err == nil {
				t.Fatal("runAttach succeeded, want target-form error")
			}
			assertContains(t, err.Error(), tc.wantErr)
			if after := readFileString(t, taskPath); after != before {
				t.Fatalf("task markdown changed after failed validation:\nbefore:\n%s\nafter:\n%s", before, after)
			}
			assertMissing(t, filepath.Join(boardDir, "BOARD.md"))
		})
	}
}

func TestAttachRemoveRejectsUnsafeNamesWithoutMutation(t *testing.T) {
	cases := []struct {
		name    string
		arg     string
		wantErr string
	}{
		{name: "empty", arg: "", wantErr: "attachment name cannot be empty"},
		{name: "absolute", arg: "/tmp/outside.txt", wantErr: "must not be an absolute path"},
		{name: "dotdot", arg: "..", wantErr: "is not allowed"},
		{name: "unsupported slash separator", arg: "nested/file.txt", wantErr: "must be a file name or attachments/<name>"},
		{name: "backslash separator", arg: `nested\file.txt`, wantErr: "must not contain path separators"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			boardDir := newCommandTestBoard(t)
			taskDir := seedFolderTaskWithAttachments(t, boardDir, "backlog", "TB-1", "Unsafe Name", []string{"one.txt"})
			taskPath := filepath.Join(taskDir, folderTaskFileName)
			attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
			before := readFileString(t, taskPath)

			err := runAttach([]string{"--rm", "TB-1", tc.arg}, nil)
			if err == nil {
				t.Fatal("runAttach succeeded, want unsafe-name error")
			}
			assertContains(t, err.Error(), tc.wantErr)
			assertExists(t, filepath.Join(attachmentsDir, "one.txt"))
			if after := readFileString(t, taskPath); after != before {
				t.Fatalf("task markdown changed after failed validation:\nbefore:\n%s\nafter:\n%s", before, after)
			}
			assertMissing(t, filepath.Join(boardDir, "BOARD.md"))
		})
	}
}

func TestAttachRemoveRejectsReservedNamesWithoutMutation(t *testing.T) {
	cases := []string{"TASK.md", ".agent-state.jsonl", ".agent-logs", ".attach.123.tmp"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			boardDir := newCommandTestBoard(t)
			taskDir := seedFolderTask(t, boardDir, "backlog", "TB-12", "Reserved Remove")
			taskPath := filepath.Join(taskDir, folderTaskFileName)
			if err := os.WriteFile(filepath.Join(taskDir, ".agent-state.jsonl"), []byte("state\n"), 0644); err != nil {
				t.Fatalf("write state: %v", err)
			}
			if err := os.Mkdir(filepath.Join(taskDir, ".agent-logs"), 0755); err != nil {
				t.Fatalf("mkdir logs: %v", err)
			}
			before := readFileString(t, taskPath)

			err := runAttach([]string{"--rm", "TB-12", name}, nil)
			if err == nil {
				t.Fatal("runAttach succeeded, want reserved-name error")
			}
			assertContains(t, err.Error(), "reserved")
			if after := readFileString(t, taskPath); after != before {
				t.Fatalf("task markdown changed after failed validation:\nbefore:\n%s\nafter:\n%s", before, after)
			}
			assertExists(t, filepath.Join(taskDir, folderTaskFileName))
			assertExists(t, filepath.Join(taskDir, ".agent-state.jsonl"))
			assertExists(t, filepath.Join(taskDir, ".agent-logs"))
		})
	}
}

func TestAttachRemoveRejectsTaskRootDirectoryWithoutMutation(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTask(t, boardDir, "backlog", "TB-13", "Directory Remove")
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	dirPath := filepath.Join(taskDir, "notes-dir")
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("mkdir directory candidate: %v", err)
	}
	before := readFileString(t, taskPath)

	err := runAttach([]string{"--rm", "TB-13", "notes-dir"}, nil)
	if err == nil {
		t.Fatal("runAttach succeeded, want directory refusal")
	}
	assertContains(t, err.Error(), `attachment "notes-dir" is a directory; refusing to remove it`)
	assertExists(t, dirPath)
	if after := readFileString(t, taskPath); after != before {
		t.Fatalf("task markdown changed after directory refusal:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestAttachRemoveRejectsOutsideResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on Windows")
	}

	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTaskWithAttachments(t, boardDir, "backlog", "TB-1", "Escape", []string{"escape.txt"})
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
	before := readFileString(t, taskPath)

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0644); err != nil {
		t.Fatalf("write outside target: %v", err)
	}
	linkPath := filepath.Join(attachmentsDir, "escape.txt")
	if err := os.Remove(linkPath); err != nil {
		t.Fatalf("replace seeded attachment: %v", err)
	}
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("symlink outside attachment: %v", err)
	}

	err := runAttach([]string{"--rm", "TB-1", "escape.txt"}, nil)
	if err == nil {
		t.Fatal("runAttach succeeded, want outside-resolution error")
	}
	assertContains(t, err.Error(), `attachment "escape.txt" resolves outside attachments/`)
	assertExists(t, linkPath)
	assertExists(t, outside)
	if after := readFileString(t, taskPath); after != before {
		t.Fatalf("task markdown changed after failed validation:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	assertMissing(t, filepath.Join(boardDir, "BOARD.md"))
}

func TestAttachRemoveRejectsRootSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on Windows")
	}

	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTask(t, boardDir, "backlog", "TB-14", "Root Escape")
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0644); err != nil {
		t.Fatalf("write outside target: %v", err)
	}
	linkPath := filepath.Join(taskDir, "escape.txt")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("symlink outside attachment: %v", err)
	}
	content := upsertAttachmentsSection(readFileString(t, taskPath), []string{"escape.txt"})
	if err := os.WriteFile(taskPath, []byte(content), 0644); err != nil {
		t.Fatalf("write task attachments section: %v", err)
	}
	before := readFileString(t, taskPath)

	err := runAttach([]string{"--rm", "TB-14", "escape.txt"}, nil)
	if err == nil {
		t.Fatal("runAttach succeeded, want outside-resolution error")
	}
	assertContains(t, err.Error(), `attachment "escape.txt" resolves outside task directory`)
	assertExists(t, linkPath)
	assertExists(t, outside)
	if after := readFileString(t, taskPath); after != before {
		t.Fatalf("task markdown changed after failed validation:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	assertMissing(t, filepath.Join(boardDir, "BOARD.md"))
}

func TestAttachRemoveWaitsForBoardLock(t *testing.T) {
	if os.Getenv("TB_TEST_ATTACH_HOLD_LOCK") == "1" {
		holdAttachLockForTest(t)
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("flock-based locking is not portable on Windows")
	}

	boardDir := newCommandTestBoard(t)
	taskDir := seedFolderTaskWithAttachments(t, boardDir, "backlog", "TB-1", "Locked", []string{"locked.txt"})
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)

	controlDir := t.TempDir()
	readyPath := filepath.Join(controlDir, "ready")
	releasePath := filepath.Join(controlDir, "release")
	cmd := exec.Command(os.Args[0], "-test.run=^TestAttachRemoveWaitsForBoardLock$")
	cmd.Env = append(os.Environ(),
		"TB_TEST_ATTACH_HOLD_LOCK=1",
		"TB_TEST_BOARD_DIR="+boardDir,
		"TB_TEST_LOCK_READY="+readyPath,
		"TB_TEST_RELEASE_LOCK="+releasePath,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start lock holder: %v", err)
	}
	defer func() {
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	waitForFile(t, readyPath)

	done := make(chan error, 1)
	go func() {
		done <- runAttach([]string{"--rm", "TB-1", "locked.txt"}, nil)
	}()

	select {
	case err := <-done:
		t.Fatalf("runAttach returned before lock release: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	if err := os.WriteFile(releasePath, []byte("release\n"), 0644); err != nil {
		t.Fatalf("release lock holder: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("lock holder failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runAttach after lock release: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runAttach did not finish after lock release")
	}
	assertMissing(t, filepath.Join(attachmentsDir, "locked.txt"))
}

func TestUsageIncludesAttachRemove(t *testing.T) {
	out := captureStdout(t, usage)
	assertContains(t, out, "tb attach --rm <ID> <attachment-name>...")
	assertContains(t, out, "Remove task attachments")
	assertContains(t, out, "New attachments are stored in the task directory")
	assertContains(t, out, "Reserved attachment names")
}

func holdAttachLockForTest(t *testing.T) {
	t.Helper()

	boardDir := os.Getenv("TB_TEST_BOARD_DIR")
	readyPath := os.Getenv("TB_TEST_LOCK_READY")
	releasePath := os.Getenv("TB_TEST_RELEASE_LOCK")
	if boardDir == "" || readyPath == "" || releasePath == "" {
		t.Fatal("missing lock-holder test environment")
	}

	lock, err := lockBoard(boardDir)
	if err != nil {
		t.Fatalf("lockBoard: %v", err)
	}
	defer lock.unlock()

	if err := os.WriteFile(readyPath, []byte("ready\n"), 0644); err != nil {
		t.Fatalf("write ready file: %v", err)
	}
	waitForFile(t, releasePath)
}

func seedFolderTask(t *testing.T, boardDir, status, id, title string) string {
	t.Helper()

	taskDir := filepath.Join(boardDir, status, id)
	if err := os.Mkdir(taskDir, 0755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	taskPath := filepath.Join(taskDir, folderTaskFileName)
	if err := os.WriteFile(taskPath, []byte(legacyTaskContent(id, title)), 0644); err != nil {
		t.Fatalf("write folder task: %v", err)
	}
	return taskDir
}

func seedFolderTaskWithAttachments(t *testing.T, boardDir, status, id, title string, attachmentNames []string) string {
	t.Helper()

	taskDir := seedFolderTask(t, boardDir, status, id, title)
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
	if err := os.Mkdir(attachmentsDir, 0755); err != nil {
		t.Fatalf("mkdir attachments: %v", err)
	}
	for _, name := range attachmentNames {
		if err := os.WriteFile(filepath.Join(attachmentsDir, name), []byte(name+"\n"), 0644); err != nil {
			t.Fatalf("write attachment %s: %v", name, err)
		}
	}

	taskPath := filepath.Join(taskDir, folderTaskFileName)
	refs := make([]string, 0, len(attachmentNames))
	for _, name := range attachmentNames {
		refs = append(refs, legacyAttachmentPrefix+name)
	}
	content := upsertAttachmentsSection(readFileString(t, taskPath), refs)
	if err := os.WriteFile(taskPath, []byte(content), 0644); err != nil {
		t.Fatalf("write task attachments section: %v", err)
	}
	return taskDir
}

func legacyTaskContent(id, title string) string {
	return strings.Join([]string{
		"# " + id + ": " + title,
		"",
		"**Type:** bug",
		"**Priority:** P2",
		"**Size:** M",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Keep the task body.",
		"",
		"## Acceptance Criteria",
		"",
		"- [ ] Existing criterion",
		"",
		"## Log",
		"",
		"- 2026-05-14: Created",
		"",
	}, "\n")
}

func writeAttachmentSource(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write source %s: %v", name, err)
	}
	return path
}

func readFileString(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
