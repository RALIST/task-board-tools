package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type folderTaskFixture struct {
	status      string
	id          string
	title       string
	taskType    string
	priority    string
	size        string
	module      string
	tags        string
	parent      string
	goal        string
	acceptance  string
	extra       string
	attachments []string
}

func TestStorageFormsProduceSameLogicalReadResults(t *testing.T) {
	cases := []struct {
		name  string
		forms map[string]string
	}{
		{
			name:  "all-file",
			forms: map[string]string{"TB-1": "file", "TB-2": "file", "TB-3": "file", "TB-4": "file", "TB-5": "file"},
		},
		{
			name:  "all-folder",
			forms: map[string]string{"TB-1": "folder", "TB-2": "folder", "TB-3": "folder", "TB-4": "folder", "TB-5": "folder"},
		},
		{
			name:  "mixed",
			forms: map[string]string{"TB-1": "folder", "TB-2": "file", "TB-3": "folder", "TB-4": "file", "TB-5": "folder"},
		},
	}

	var baseline logicalReadResult
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			boardDir := newFolderFixtureBoard(t, tc.forms)
			result := collectLogicalReadResult(t, boardDir)
			if tc.name == "all-file" {
				baseline = result
				return
			}
			if !reflect.DeepEqual(result, baseline) {
				t.Fatalf("logical read result for %s differs from all-file\n got: %#v\nwant: %#v", tc.name, result, baseline)
			}
		})
	}
}

func TestFolderTaskJSONFilePathsUseCanonicalMarkdownPath(t *testing.T) {
	newFolderFixtureBoard(t, map[string]string{
		"TB-1": "file",
		"TB-2": "folder",
		"TB-3": "file",
		"TB-4": "folder",
		"TB-5": "file",
	})

	out := captureStdout(t, func() {
		cmdList([]string{"--json", "--status", "all"})
	})

	var got []taskJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal ls JSON: %v\n%s", err, out)
	}
	byID := taskJSONByID(got)

	assertPathSuffix(t, byID["TB-1"].FilePath, filepath.Join("backlog", "TB-1.md"))
	assertPathSuffix(t, byID["TB-2"].FilePath, filepath.Join("done", "TB-2", folderTaskFileName))
	if byID["TB-2"].Status != "done" {
		t.Fatalf("TB-2 status = %q, want done", byID["TB-2"].Status)
	}

	showOut := captureStdout(t, func() {
		cmdShow([]string{"TB-2", "--json"})
	})
	var show struct {
		Metadata taskJSON `json:"metadata"`
		Body     string   `json:"body"`
	}
	if err := json.Unmarshal([]byte(showOut), &show); err != nil {
		t.Fatalf("unmarshal show JSON: %v\n%s", err, showOut)
	}
	if show.Metadata.Status != "done" {
		t.Fatalf("show status = %q, want done", show.Metadata.Status)
	}
	assertPathSuffix(t, show.Metadata.FilePath, filepath.Join("done", "TB-2", folderTaskFileName))
	assertContains(t, show.Body, "## Attachments")
	assertContains(t, show.Body, "- attachments/evidence.txt")
}

func TestFolderTaskDuplicateFormSelfHeals(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	task := baseFolderFixtures()[0]
	writeFolderFixtureTask(t, boardDir, task, "file")
	writeFolderFixtureTask(t, boardDir, task, "folder")

	filePath := taskFilePath(boardDir, "backlog", task.id)
	folderPath := taskFolderMarkdownPath(boardDir, "backlog", task.id)

	var refs []taskRef
	stderr := captureStderr(t, func() {
		var err error
		refs, err = discoverTaskRefs(boardDir, []string{"backlog"})
		if err != nil {
			t.Fatalf("discoverTaskRefs: %v", err)
		}
	})
	if len(refs) != 1 || refs[0].ID != task.id {
		t.Fatalf("discoverTaskRefs returned %v, want single %s ref", refs, task.id)
	}
	if refs[0].Path != folderPath {
		t.Fatalf("resolver returned %q, want folder form %q", refs[0].Path, folderPath)
	}
	assertContains(t, stderr, "warning")
	assertContains(t, stderr, task.id)
	assertContains(t, stderr, "preferring folder form")

	// Orphan still present — self-heal happens at the next mutation, not at read.
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("file-form orphan should still exist before mutation: %v", err)
	}

	if _, err := moveTaskOnBoard(boardDir, task.id, "in-progress", "Started — moved to in-progress"); err != nil {
		t.Fatalf("moveTaskOnBoard: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("file-form orphan should be removed after mutation: err=%v", err)
	}
	movedFolder := taskFolderMarkdownPath(boardDir, "in-progress", task.id)
	if _, err := os.Stat(movedFolder); err != nil {
		t.Fatalf("folder form should be at new status after move: %v", err)
	}
}

func TestMalformedFolderTaskWarnsAndIsSkipped(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeFolderFixtureTask(t, boardDir, baseFolderFixtures()[0], "folder")

	malformedDir := filepath.Join(boardDir, "backlog", "TB-9")
	if err := os.MkdirAll(malformedDir, 0755); err != nil {
		t.Fatalf("mkdir malformed folder: %v", err)
	}
	malformedPath := filepath.Join(malformedDir, folderTaskFileName)
	writeFileForTest(t, malformedPath, "# TB-9\n\n**Priority:** P0\n")

	var stderr string
	out := captureStdout(t, func() {
		stderr = captureStderr(t, func() {
			cmdList([]string{"--json", "--status", "backlog"})
		})
	})

	var got []taskJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal ls JSON: %v\n%s", err, out)
	}
	assertTaskIDs(t, got, []string{"TB-1"})
	assertContains(t, stderr, "warning: skipping malformed task file")
	assertContains(t, stderr, malformedPath)
	assertNotContains(t, out, "TB-9")
}

func TestArchiveScopeDiscoversFolderTasks(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeFolderFixtureTask(t, boardDir, folderTaskFixture{
		status:     "archive",
		id:         "TB-7",
		title:      "Archived Folder Task",
		taskType:   "bug",
		priority:   "P1",
		size:       "S",
		module:     "cli",
		goal:       "Keep archived folder tasks readable.",
		acceptance: "- [ ] Archive scope sees the task.",
	}, "folder")

	out := captureStdout(t, func() {
		cmdList([]string{"--json", "--status", "archive"})
	})
	var got []taskJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal archive ls JSON: %v\n%s", err, out)
	}
	assertTaskIDs(t, got, []string{"TB-7"})
	if got[0].Status != "archive" {
		t.Fatalf("archive folder status = %q, want archive", got[0].Status)
	}
	assertPathSuffix(t, got[0].FilePath, filepath.Join("archive", "TB-7", folderTaskFileName))
}

func TestParseTaskRefUsesStatusDirForFolderTask(t *testing.T) {
	boardDir := newCommandTestBoard(t)
	writeFolderFixtureTask(t, boardDir, folderTaskFixture{
		status:     "in-progress",
		id:         "TB-8",
		title:      "Folder Status",
		taskType:   "feature",
		priority:   "P0",
		size:       "M",
		module:     "cli",
		goal:       "Parse from TASK.md.",
		acceptance: "- [ ] Status is in-progress.",
	}, "folder")

	ref, err := resolveTaskRef(boardDir, "TB-8", []string{"in-progress"})
	if err != nil {
		t.Fatalf("resolveTaskRef: %v", err)
	}
	task, err := parseTaskRef(ref, cfg.RootDir)
	if err != nil {
		t.Fatalf("parseTaskRef: %v", err)
	}
	if task.Status != "in-progress" {
		t.Fatalf("task status = %q, want in-progress", task.Status)
	}
	if task.ID != "TB-8" || task.Title != "Folder Status" {
		t.Fatalf("task metadata = %+v, want TB-8 / Folder Status", task)
	}
}

type logicalReadResult struct {
	ListIDs      []string
	BoardContent string
	BoardJSON    logicalBoardJSON
	Triage       []logicalTriageJSON
	GrepIDs      []string
	EpicOutput   string
}

type logicalBoardJSON struct {
	Epics        []string
	InProgress   []string
	Backlog      []string
	RecentlyDone []string
}

type logicalTriageJSON struct {
	ID      string
	Reasons []string
}

func collectLogicalReadResult(t *testing.T, boardDir string) logicalReadResult {
	t.Helper()

	listOut := captureStdout(t, func() {
		cmdList([]string{"--json", "--status", "all"})
	})
	var list []taskJSON
	if err := json.Unmarshal([]byte(listOut), &list); err != nil {
		t.Fatalf("unmarshal ls JSON: %v\n%s", err, listOut)
	}

	boardContent, err := buildBoardContent(boardDir)
	if err != nil {
		t.Fatalf("buildBoardContent: %v", err)
	}
	boardSnapshot, err := buildBoardSnapshot(boardDir)
	if err != nil {
		t.Fatalf("buildBoardSnapshot: %v", err)
	}

	triageOut := captureStdout(t, func() {
		cmdTriage([]string{"--json"})
	})
	var triage []triageReasonJSON
	if err := json.Unmarshal([]byte(triageOut), &triage); err != nil {
		t.Fatalf("unmarshal triage JSON: %v\n%s", err, triageOut)
	}

	grepOut := captureStdout(t, func() {
		cmdGrep([]string{"shared-needle", "--status", "all", "-l"})
	})

	epicOut := captureStdout(t, func() {
		cmdEpic([]string{"TB-1", "--status", "all"})
	})

	return logicalReadResult{
		ListIDs:      taskJSONIDs(list),
		BoardContent: boardContent,
		BoardJSON: logicalBoardJSON{
			Epics:        taskJSONIDs(boardSnapshot.Epics),
			InProgress:   taskJSONIDs(boardSnapshot.InProgress),
			Backlog:      taskJSONIDs(boardSnapshot.Backlog),
			RecentlyDone: taskJSONIDs(boardSnapshot.RecentlyDone),
		},
		Triage:     logicalTriage(triage),
		GrepIDs:    grepTaskIDs(grepOut),
		EpicOutput: epicOut,
	}
}

func newFolderFixtureBoard(t *testing.T, forms map[string]string) string {
	t.Helper()

	boardDir := newCommandTestBoard(t)
	for _, task := range baseFolderFixtures() {
		form := forms[task.id]
		if form == "" {
			t.Fatalf("missing storage form for %s", task.id)
		}
		writeFolderFixtureTask(t, boardDir, task, form)
	}
	return boardDir
}

func baseFolderFixtures() []folderTaskFixture {
	return []folderTaskFixture{
		{
			status:     "backlog",
			id:         "TB-1",
			title:      "Storage Epic",
			taskType:   "feature",
			priority:   "P0",
			size:       "L",
			module:     "cli",
			tags:       "epic,folder-parity",
			goal:       "Group storage parity work.",
			acceptance: "- [ ] Children are visible.",
		},
		{
			status:      "done",
			id:          "TB-2",
			title:       "Done Child",
			taskType:    "feature",
			priority:    "P1",
			size:        "S",
			module:      "cli",
			tags:        "folder-parity",
			parent:      "TB-1",
			goal:        "Finish the done child with shared-needle.",
			acceptance:  "- [x] Done child is counted.",
			extra:       "\n## Notes\n\nshared-needle in done child.\n",
			attachments: []string{"evidence.txt"},
		},
		{
			status:     "archive",
			id:         "TB-3",
			title:      "Archived Child",
			taskType:   "bug",
			priority:   "P2",
			size:       "M",
			module:     "cli",
			tags:       "folder-parity",
			parent:     "TB-1",
			goal:       "Keep archived child searchable with shared-needle.",
			acceptance: "- [ ] Archived child is explicit only.",
		},
		{
			status:     "backlog",
			id:         "TB-4",
			title:      "Needs Grooming",
			taskType:   "bug",
			priority:   "P2",
			size:       "M",
			tags:       "folder-parity",
			goal:       "(to be filled)",
			acceptance: "- [ ] (to be filled)",
		},
		{
			status:     "in-progress",
			id:         "TB-5",
			title:      "Active Work",
			taskType:   "improvement",
			priority:   "P1",
			size:       "S",
			module:     "cli",
			tags:       "folder-parity",
			goal:       "Keep in-progress sorting stable.",
			acceptance: "- [ ] In-progress row appears.",
		},
	}
}

func writeFolderFixtureTask(t *testing.T, boardDir string, task folderTaskFixture, form string) string {
	t.Helper()

	content := folderFixtureContent(task, form)
	switch form {
	case "file":
		path := filepath.Join(boardDir, task.status, task.id+".md")
		writeFileForTest(t, path, content)
		return path
	case "folder":
		taskDir := filepath.Join(boardDir, task.status, task.id)
		if err := os.MkdirAll(taskDir, 0755); err != nil {
			t.Fatalf("mkdir task dir: %v", err)
		}
		if len(task.attachments) > 0 {
			attachmentsDir := filepath.Join(taskDir, "attachments")
			if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
				t.Fatalf("mkdir attachments dir: %v", err)
			}
			for _, name := range task.attachments {
				writeFileForTest(t, filepath.Join(attachmentsDir, name), "attachment fixture\n")
			}
		}
		path := filepath.Join(taskDir, folderTaskFileName)
		writeFileForTest(t, path, content)
		return path
	default:
		t.Fatalf("unknown storage form %q", form)
		return ""
	}
}

func folderFixtureContent(task folderTaskFixture, form string) string {
	var b strings.Builder
	b.WriteString("# " + task.id + ": " + task.title + "\n\n")
	b.WriteString("**Type:** " + task.taskType + "\n")
	b.WriteString("**Priority:** " + task.priority + "\n")
	b.WriteString("**Size:** " + task.size + "\n")
	if task.module != "" {
		b.WriteString("**Module:** " + task.module + "\n")
	}
	if task.tags != "" {
		b.WriteString("**Tags:** " + task.tags + "\n")
	}
	b.WriteString("**Branch:** -\n")
	if task.parent != "" {
		b.WriteString("**Parent:** " + task.parent + "\n")
	}
	b.WriteString("\n## Goal\n\n" + task.goal + "\n\n")
	b.WriteString("## Acceptance Criteria\n\n" + task.acceptance + "\n")
	if form == "folder" && len(task.attachments) > 0 {
		b.WriteString("\n## Attachments\n\n")
		for _, name := range task.attachments {
			b.WriteString("- attachments/" + name + "\n")
		}
	}
	if task.extra != "" {
		b.WriteString(task.extra)
	}
	b.WriteString("\n## Log\n\n- 2026-05-14: Created\n")
	return b.String()
}

func taskJSONByID(tasks []taskJSON) map[string]taskJSON {
	byID := make(map[string]taskJSON, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	return byID
}

func taskJSONIDs(tasks []taskJSON) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func logicalTriage(items []triageReasonJSON) []logicalTriageJSON {
	out := make([]logicalTriageJSON, 0, len(items))
	for _, item := range items {
		out = append(out, logicalTriageJSON{ID: item.ID, Reasons: item.Reasons})
	}
	return out
}

func grepTaskIDs(output string) []string {
	var ids []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.HasPrefix(fields[0], "TB-") {
			ids = append(ids, fields[0])
		}
	}
	sort.Strings(ids)
	return ids
}

func assertPathSuffix(t *testing.T, got, want string) {
	t.Helper()
	got = filepath.ToSlash(got)
	want = filepath.ToSlash(want)
	if !strings.HasSuffix(got, want) {
		t.Fatalf("path = %q, want suffix %q", got, want)
	}
}
