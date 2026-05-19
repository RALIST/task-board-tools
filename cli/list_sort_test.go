package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDoneSortsByCompletionLogBeforePriority(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeListSortTask(t, boardDir, "done", "TB-10", "Older P0", "P0", []string{
		"- 2026-05-01: Done",
	})
	writeListSortTask(t, boardDir, "done", "TB-20", "Newest P2", "P2", []string{
		"- 2026-05-10: Done - implementation complete",
	})
	writeListSortTask(t, boardDir, "done", "TB-30", "Middle P1", "P1", []string{
		"- 2026-05-09: Moved to done",
	})

	got := listSortJSON(t, "--json", "--status", "done")
	assertTaskIDs(t, got, []string{"TB-20", "TB-30", "TB-10"})
}

func TestListActiveKeepsOpenStatusesPrioritySortedAndDoneCompletionSorted(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeListSortTask(t, boardDir, "backlog", "TB-40", "Backlog P2", "P2", []string{"- 2026-05-01: Created"})
	writeListSortTask(t, boardDir, "backlog", "TB-05", "Backlog P0", "P0", []string{"- 2026-05-01: Created"})
	writeListSortTask(t, boardDir, "in-progress", "TB-60", "In Progress P1", "P1", []string{"- 2026-05-01: Started"})
	writeListSortTask(t, boardDir, "in-progress", "TB-07", "In Progress P0", "P0", []string{"- 2026-05-01: Started"})
	writeListSortTask(t, boardDir, "done", "TB-08", "Older Done P0", "P0", []string{"- 2026-05-01: Done"})
	writeListSortTask(t, boardDir, "done", "TB-09", "Newer Done P2", "P2", []string{"- 2026-05-12: Done"})
	writeListSortTask(t, boardDir, "archive", "TB-01", "Archived Done", "P0", []string{"- 2026-05-13: Done"})

	got := listSortJSON(t, "--json", "--status", "active")
	assertTaskIDs(t, tasksByStatus(got, "backlog"), []string{"TB-05", "TB-40"})
	assertTaskIDs(t, tasksByStatus(got, "in-progress"), []string{"TB-07", "TB-60"})
	assertTaskIDs(t, tasksByStatus(got, "done"), []string{"TB-09", "TB-08"})
	assertTaskIDs(t, tasksByStatus(got, "archive"), nil)
}

func TestListDoneFallbackForTasksWithoutCompletionLog(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeListSortTask(t, boardDir, "done", "TB-01", "Legacy P1", "P1", []string{
		"- 2026-05-01: Created",
	})
	writeListSortTask(t, boardDir, "done", "TB-02", "Malformed P0", "P0", []string{
		"- 2026-99-99: Done",
	})
	writeListSortTask(t, boardDir, "done", "TB-03", "Completed P2", "P2", []string{
		"- 2026-05-03: Done",
	})

	got := listSortJSON(t, "--json", "--status", "done")
	assertTaskIDs(t, got, []string{"TB-03", "TB-02", "TB-01"})
}

func listSortJSON(t *testing.T, args ...string) []taskJSON {
	t.Helper()

	out := captureStdout(t, func() {
		cmdList(args)
	})
	var got []taskJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal ls JSON: %v\n%s", err, out)
	}
	return got
}

func tasksByStatus(tasks []taskJSON, status string) []taskJSON {
	var out []taskJSON
	for _, task := range tasks {
		if task.Status == status {
			out = append(out, task)
		}
	}
	return out
}

func writeListSortTask(t *testing.T, boardDir, status, id, title, priority string, logEntries []string) {
	t.Helper()

	content := strings.Join([]string{
		"# " + id + ": " + title,
		"",
		"**Type:** improvement",
		"**Priority:** " + priority,
		"**Size:** S",
		"**Module:** cli",
		"**Branch:** -",
		"",
		"## Goal",
		"",
		"Sort fixture.",
		"",
		"## Acceptance Criteria",
		"",
		"- [ ] Exercise ordering.",
		"",
		"## Log",
		"",
		strings.Join(logEntries, "\n"),
		"",
	}, "\n")
	writeFileForTest(t, filepath.Join(boardDir, status, id+".md"), content)
}
