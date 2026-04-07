package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// buildBoardContent generates the board markdown content from the current directory state.
func buildBoardContent(boardDir string) string {
	var b strings.Builder

	b.WriteString("# Board\n\n")

	// Epics section — collect all tasks, find epics, compute progress.
	allTasks := collectAllTasks(boardDir)
	var epics []Task
	for _, t := range allTasks {
		if hasTag(t.Tags, "epic") {
			epics = append(epics, t)
		}
	}

	if len(epics) > 0 {
		sort.Slice(epics, func(i, j int) bool {
			ri := priorityRank(epics[i].Priority)
			rj := priorityRank(epics[j].Priority)
			if ri != rj {
				return ri < rj
			}
			return numericID(epics[i].ID) < numericID(epics[j].ID)
		})

		b.WriteString("## Epics\n\n")
		b.WriteString("| ID | Title | Progress | Status | Module |\n")
		b.WriteString("|----|-------|----------|--------|--------|\n")
		for _, e := range epics {
			done, total := 0, 0
			for _, t := range allTasks {
				if strings.EqualFold(t.Parent, e.ID) {
					total++
					if t.Status == "done" {
						done++
					}
				}
			}
			progress := fmt.Sprintf("%d/%d", done, total)
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", e.ID, e.Title, progress, e.Status, e.Module)
		}
		b.WriteString("\n")
	}

	// In Progress
	b.WriteString("## In Progress\n\n")
	b.WriteString("| ID | Title | Priority | Module | Branch |\n")
	b.WriteString("|----|-------|----------|--------|--------|\n")
	tasks := collectTasks(boardDir, "in-progress")
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
	tasks = collectTasks(boardDir, "backlog")
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
	tasks = collectTasks(boardDir, "done")
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

	return b.String()
}

// regenerateBoard generates BOARD.md from the current directory state.
func regenerateBoard(boardDir string) error {
	content := buildBoardContent(boardDir)

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

// collectTasks reads and parses all task files from a status directory,
// sorted by numeric ID ascending.
func collectTasks(boardDir, status string) []Task {
	dirPath := filepath.Join(boardDir, status)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	var tasks []Task
	for _, entry := range entries {
		if entry.IsDir() || !isTaskFile(entry.Name()) {
			continue
		}
		t, err := parseTaskFile(filepath.Join(dirPath, entry.Name()))
		if err != nil {
			continue
		}
		t.Status = status
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return numericID(tasks[i].ID) < numericID(tasks[j].ID)
	})
	return tasks
}

// collectAllTasks reads and parses all task files across all status directories.
func collectAllTasks(boardDir string) []Task {
	var all []Task
	for _, status := range statusDirs {
		all = append(all, collectTasks(boardDir, status)...)
	}
	return all
}

func cmdRegenerate(_ []string) {
	boardDir := cfg.BoardDir

	if err := regenerateBoard(boardDir); err != nil {
		fatal("error: %v", err)
	}

	fmt.Println("Regenerated BOARD.md")
}

func cmdBoard(_ []string) {
	fmt.Print(buildBoardContent(cfg.BoardDir))
}
