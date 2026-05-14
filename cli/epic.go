package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

func cmdEpic(args []string) {
	fs := flag.NewFlagSet("epic", flag.ExitOnError)
	statusFilter := fs.String("status", "active", "status filter: active|archive|all")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: tb epic <ID> [--status active|archive|all]\n\nShows an epic and children in the requested status scope.")
		fs.PrintDefaults()
	}

	reordered := reorderArgs(args)
	if err := fs.Parse(reordered); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}
	epicID := normalizeTaskID(fs.Arg(0))
	boardDir := cfg.BoardDir
	statuses, err := resolveStatusFilter(*statusFilter)
	if err != nil {
		fatal("%v", err)
	}

	epicPath, err := findTaskInStatuses(boardDir, epicID, statuses)
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

	children := findChildrenInStatuses(boardDir, epicID, statuses)

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

	// Sort active work first, then archived children when an explicit archive
	// scope requested them; within each group by numeric ID.
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
		case "archive":
			icon = "-"
		}
		fmt.Fprintf(w, "  %s %s\t%s\tS:%s\t[%s]\n", icon, c.ID, c.Title, c.Size, c.Status)
	}
	w.Flush()
}

func findActiveChildren(boardDir, epicID string) []Task {
	return findChildrenInStatuses(boardDir, epicID, statusDirs)
}

func findChildrenInStatuses(boardDir, epicID string, statuses []string) []Task {
	var children []Task
	for _, status := range statuses {
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

// statusRank returns a sort rank for status ordering.
func statusRank(status string) int {
	switch status {
	case "in-progress":
		return 0
	case "backlog":
		return 1
	case "done":
		return 2
	case "archive":
		return 3
	default:
		return 4
	}
}
