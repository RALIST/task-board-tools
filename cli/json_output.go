// JSON serializers for --json output on `tb ls`, `tb show`, and `tb board`.
//
// Contract (M1):
//   - stdout = data; stderr = errors/warnings.
//   - empty selections return `[]` or `{}`, never prose like "No tasks found.".
//   - all Task fields are camelCase: id, title, type, priority, size, module,
//     tags, branch, parent, status, filePath, agent, agentStatus.
//   - body strings in `tb show --json` are the full raw markdown.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// taskJSON is the wire shape of a Task. Tags are split on "," and trimmed so
// consumers don't have to re-parse the comma-separated string; the markdown
// file still stores them as a single line.
type taskJSON struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	Size        string   `json:"size"`
	Module      string   `json:"module"`
	Tags        []string `json:"tags"`
	Branch      string   `json:"branch"`
	Parent      string   `json:"parent"`
	Status      string   `json:"status"`
	FilePath    string   `json:"filePath"`
	Agent       string   `json:"agent"`
	AgentStatus string   `json:"agentStatus"`
}

func marshalTask(t Task) taskJSON {
	tags := []string{}
	if t.Tags != "" {
		for _, tag := range strings.Split(t.Tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	// "—" is the placeholder sentinel the CLI writes for the Branch line in
	// fresh task files (see buildTaskContent). Treat it as empty in the JSON
	// wire shape; preserve any other literal Branch value verbatim.
	branch := t.Branch
	if strings.TrimSpace(branch) == "—" {
		branch = ""
	}
	return taskJSON{
		ID:          t.ID,
		Title:       t.Title,
		Type:        t.Type,
		Priority:    t.Priority,
		Size:        t.Size,
		Module:      t.Module,
		Tags:        tags,
		Branch:      branch,
		Parent:      t.Parent,
		Status:      t.Status,
		FilePath:    t.FilePath,
		Agent:       t.Agent,
		AgentStatus: t.AgentStatus,
	}
}

func marshalTasks(tasks []Task) []taskJSON {
	out := make([]taskJSON, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, marshalTask(t))
	}
	return out
}

// emitTasksJSON writes a JSON array of tasks to stdout. Empty input still
// produces `[]\n`, never prose.
func emitTasksJSON(tasks []Task) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(marshalTasks(tasks))
}

// emitShowJSON writes {metadata, body} for `tb show <ID> --json`.
//
// `metadata` is the parsed Task (camelCase JSON keys); `body` is the raw
// markdown content of the file unchanged.
func emitShowJSON(path string, data []byte) error {
	t, err := parseTaskFile(path)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	// Status is the parent directory name. FilePath is relative to CWD.
	t.Status = filepath.Base(filepath.Dir(path))
	if cwd, err := os.Getwd(); err == nil {
		t.FilePath = relPath(cwd, path)
	} else {
		t.FilePath = path
	}

	payload := struct {
		Metadata taskJSON `json:"metadata"`
		Body     string   `json:"body"`
	}{
		Metadata: marshalTask(t),
		Body:     string(data),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// boardSnapshotJSON is the shape of `tb board --json`. Every slice is
// initialised so JSON output is always `[]` rather than `null` on empty.
type boardSnapshotJSON struct {
	Epics         []taskJSON `json:"epics"`
	ActiveEpics   []taskJSON `json:"activeEpics"`
	FinishedEpics []taskJSON `json:"finishedEpics"`
	InProgress    []taskJSON `json:"inProgress"`
	Backlog       []taskJSON `json:"backlog"`
	RecentlyDone  []taskJSON `json:"recentlyDone"`
}

// buildBoardSnapshot mirrors buildBoardContent but emits structured data.
// Recently Done is capped at 50 items (highest ID first) to match the
// markdown view.
func buildBoardSnapshot(boardDir string) boardSnapshotJSON {
	all := collectAllTasks(boardDir)

	var activeEpics, finishedEpics, allEpics []Task
	for _, t := range all {
		if !hasTag(t.Tags, "epic") {
			continue
		}
		allEpics = append(allEpics, t)
		if t.Status == "done" {
			finishedEpics = append(finishedEpics, t)
		} else {
			activeEpics = append(activeEpics, t)
		}
	}

	epicSort := func(epics []Task) {
		sort.Slice(epics, func(i, j int) bool {
			ri := priorityRank(epics[i].Priority)
			rj := priorityRank(epics[j].Priority)
			if ri != rj {
				return ri < rj
			}
			return numericID(epics[i].ID) < numericID(epics[j].ID)
		})
	}
	epicSort(allEpics)
	epicSort(activeEpics)
	epicSort(finishedEpics)

	inProgress := collectTasks(boardDir, "in-progress")
	backlog := collectTasks(boardDir, "backlog")

	done := collectTasks(boardDir, "done")
	sort.Slice(done, func(i, j int) bool {
		return numericID(done[i].ID) > numericID(done[j].ID)
	})
	if len(done) > 50 {
		done = done[:50]
	}

	return boardSnapshotJSON{
		Epics:         marshalTasks(allEpics),
		ActiveEpics:   marshalTasks(activeEpics),
		FinishedEpics: marshalTasks(finishedEpics),
		InProgress:    marshalTasks(inProgress),
		Backlog:       marshalTasks(backlog),
		RecentlyDone:  marshalTasks(done),
	}
}

func emitBoardJSON(boardDir string) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(buildBoardSnapshot(boardDir))
}
