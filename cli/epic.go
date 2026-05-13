package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

func cmdEpic(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tb epic <ID>\n\nShows an epic and all its children with progress.")
		os.Exit(1)
	}
	epicID := normalizeTaskID(args[0])
	boardDir := cfg.BoardDir

	epicPath, err := findTask(boardDir, epicID)
	if err != nil {
		fatal("%v", err)
	}

	epicTask, err := parseTaskFile(epicPath)
	if err != nil {
		fatal("cannot read %s: %v", epicID, err)
	}
	epicTask.Status = filepath.Base(filepath.Dir(epicPath))

	if !hasTag(epicTask.Tags, "epic") {
		fatal("%s is not tagged as an epic", epicID)
	}

	// Find all children across all status directories.
	children := findChildren(boardDir, epicID)

	// Count progress.
	doneCount := 0
	for _, c := range children {
		if c.Status == "done" {
			doneCount++
		}
	}

	fmt.Printf("Epic %s: %s\n", epicTask.ID, epicTask.Title)
	fmt.Printf("Status: %s | Progress: %d/%d\n\n", epicTask.Status, doneCount, len(children))

	if len(children) == 0 {
		fmt.Println("No children found.")
		return
	}

	// Sort: in-progress first, then backlog, then done; within each group by numeric ID.
	sort.Slice(children, func(i, j int) bool {
		ri := statusRank(children[i].Status)
		rj := statusRank(children[j].Status)
		if ri != rj {
			return ri < rj
		}
		return numericID(children[i].ID) < numericID(children[j].ID)
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, c := range children {
		icon := " "
		switch c.Status {
		case "in-progress":
			icon = ">"
		case "done":
			icon = "x"
		}
		fmt.Fprintf(w, "  %s %s\t%s\tS:%s\t[%s]\n", icon, c.ID, c.Title, c.Size, c.Status)
	}
	w.Flush()
}

// findChildren scans all status directories for tasks with Parent matching epicID.
func findChildren(boardDir, epicID string) []Task {
	var children []Task
	for _, status := range statusDirs {
		dirPath := filepath.Join(boardDir, status)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !isTaskFile(entry.Name()) {
				continue
			}
			t, err := parseTaskFile(filepath.Join(dirPath, entry.Name()))
			if err != nil {
				continue
			}
			if strings.EqualFold(t.Parent, epicID) {
				t.Status = status
				children = append(children, t)
			}
		}
	}
	return children
}

// statusRank returns a sort rank for status ordering (in-progress=0, backlog=1, done=2).
func statusRank(status string) int {
	switch status {
	case "in-progress":
		return 0
	case "backlog":
		return 1
	case "done":
		return 2
	default:
		return 3
	}
}
