package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// buildBoardContent generates the board markdown content from the current directory state.
func buildBoardContent(boardDir string) (string, error) {
	var b strings.Builder

	b.WriteString("# Board\n\n")

	// Epics section uses the active board scope. Archived tasks are closed and
	// hidden from BOARD.md unless a command explicitly requests archive/all.
	allTasks, err := collectActiveTasks(boardDir)
	if err != nil {
		return "", err
	}
	var activeEpics, finishedEpics []Task
	for _, t := range allTasks {
		if !hasTag(t.Tags, "epic") {
			continue
		}
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

	epicProgress := func(epicID string) string {
		done, total := 0, 0
		for _, t := range allTasks {
			if strings.EqualFold(t.Parent, epicID) {
				total++
				if t.Status == "done" {
					done++
				}
			}
		}
		return fmt.Sprintf("%d/%d", done, total)
	}

	if len(activeEpics) > 0 {
		epicSort(activeEpics)
		b.WriteString("## Epics\n\n")
		b.WriteString("| ID | Title | Progress | Status | Module |\n")
		b.WriteString("|----|-------|----------|--------|--------|\n")
		for _, e := range activeEpics {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", e.ID, e.Title, epicProgress(e.ID), e.Status, e.Module)
		}
		b.WriteString("\n")
	}

	if len(finishedEpics) > 0 {
		epicSort(finishedEpics)
		b.WriteString("## Finished Epics\n\n")
		b.WriteString("| ID | Title | Progress | Module |\n")
		b.WriteString("|----|-------|----------|--------|\n")
		for _, e := range finishedEpics {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", e.ID, e.Title, epicProgress(e.ID), e.Module)
		}
		b.WriteString("\n")
	}

	// In Progress
	b.WriteString("## In Progress\n\n")
	b.WriteString("| ID | Title | Priority | Module | Branch |\n")
	b.WriteString("|----|-------|----------|--------|--------|\n")
	tasks, err := collectTasks(boardDir, "in-progress")
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		b.WriteString("| — | — | — | — | — |\n")
	} else {
		for _, t := range tasks {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", t.ID, t.Title, t.Priority, t.Module, t.Branch)
		}
	}
	b.WriteString("\n")

	// Code Review
	b.WriteString("## Code Review\n\n")
	b.WriteString("| ID | Title | Priority | Module | Branch |\n")
	b.WriteString("|----|-------|----------|--------|--------|\n")
	tasks, err = collectTasks(boardDir, "code-review")
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		b.WriteString("| — | — | — | — | — |\n")
	} else {
		for _, t := range tasks {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", t.ID, t.Title, t.Priority, t.Module, t.Branch)
		}
	}
	b.WriteString("\n")

	// Backlog
	b.WriteString("## Backlog\n\n")
	b.WriteString("| ID | Title | Type | Priority | Size | Module |\n")
	b.WriteString("|----|-------|------|----------|------|--------|\n")
	tasks, err = collectTasks(boardDir, "backlog")
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		b.WriteString("| — | — | — | — | — | — |\n")
	} else {
		for _, t := range tasks {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", t.ID, t.Title, t.Type, t.Priority, t.Size, t.Module)
		}
	}
	b.WriteString("\n")

	// Recently Done (last 50 by ID, descending)
	b.WriteString("## Recently Done\n\n")
	b.WriteString("| ID | Title | Type | Module |\n")
	b.WriteString("|----|-------|------|--------|\n")
	tasks, err = collectTasks(boardDir, "done")
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		b.WriteString("| — | — | — | — |\n")
	} else {
		// Reverse sort (highest ID first), take last 50.
		sort.Slice(tasks, func(i, j int) bool {
			return numericID(tasks[i].ID) > numericID(tasks[j].ID)
		})
		if len(tasks) > 50 {
			tasks = tasks[:50]
		}
		for _, t := range tasks {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", t.ID, t.Title, t.Type, t.Module)
		}
	}

	return b.String(), nil
}

// regenerateBoard generates BOARD.md from the current directory state.
// It does not acquire .board.lock itself because structured mutations already
// call it while holding that lock.
func regenerateBoard(boardDir string) error {
	content, err := buildBoardContent(boardDir)
	if err != nil {
		return err
	}

	// Atomic write via temp file.
	output := filepath.Join(boardDir, "BOARD.md")
	tmp := fmt.Sprintf("%s.tmp.%d", output, os.Getpid())
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, output); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("cannot rename %s to %s: %w", tmp, output, err)
	}
	return nil
}

func regenerateBoardLocked(boardDir string) error {
	lock, err := lockBoard(boardDir)
	if err != nil {
		return err
	}
	defer lock.unlock()

	return regenerateBoard(boardDir)
}

// collectTasks reads and parses all task files from a status directory,
// sorted by numeric ID ascending.
func collectTasks(boardDir, status string) ([]Task, error) {
	refs, err := discoverTaskRefs(boardDir, []string{status})
	if err != nil {
		return nil, err
	}
	var tasks []Task
	cwd, _ := os.Getwd()
	for _, ref := range refs {
		t, err := parseTaskRef(ref, cwd)
		if err != nil {
			warnSkippingTask(ref.Path, err)
			continue
		}
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return numericID(tasks[i].ID) < numericID(tasks[j].ID)
	})
	return tasks, nil
}

// collectActiveTasks reads and parses task files from active board
// directories only: backlog, in-progress, and done.
func collectActiveTasks(boardDir string) ([]Task, error) {
	var all []Task
	for _, status := range statusDirs {
		tasks, err := collectTasks(boardDir, status)
		if err != nil {
			return nil, err
		}
		all = append(all, tasks...)
	}
	return all, nil
}

func cmdRegenerate(_ []string) {
	boardDir := cfg.BoardDir

	if err := regenerateBoardLocked(boardDir); err != nil {
		fatal("%v", err)
	}

	fmt.Println("Regenerated BOARD.md")
}

func cmdBoard(args []string) {
	// Tiny flag surface: --json is the only flag for now. Avoid pulling in
	// flag.NewFlagSet machinery for one bool — scan the args inline.
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}
	if jsonOut {
		if err := emitBoardJSON(cfg.BoardDir); err != nil {
			fatal("%v", err)
		}
		return
	}
	content, err := buildBoardContent(cfg.BoardDir)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Print(content)
}
