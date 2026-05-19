package main

import (
	"flag"
	"fmt"
	"os"
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

	epicRef, err := resolveTaskRef(boardDir, epicID, statuses)
	if err != nil {
		fatal("%v", err)
	}

	cwd, _ := os.Getwd()
	epicTask, err := parseTaskRef(epicRef, cwd)
	if err != nil {
		fatal("cannot read %s: %v", epicID, err)
	}

	if !hasTag(epicTask.Tags, "epic") {
		fatal("%s is not tagged as an epic", epicID)
	}

	children, err := findChildrenInStatuses(boardDir, epicID, statuses)
	if err != nil {
		fatal("%v", err)
	}

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
	children, err := findChildrenInStatuses(boardDir, epicID, statusDirs)
	if err != nil {
		warnSkippingTask(epicID, err)
		return nil
	}
	return children
}

func findChildrenInStatuses(boardDir, epicID string, statuses []string) ([]Task, error) {
	var children []Task
	refs, err := discoverTaskRefs(boardDir, statuses)
	if err != nil {
		return nil, err
	}
	cwd, _ := os.Getwd()
	for _, ref := range refs {
		t, err := parseTaskRef(ref, cwd)
		if err != nil {
			warnSkippingTask(ref.Path, err)
			continue
		}
		if strings.EqualFold(t.Parent, epicID) {
			children = append(children, t)
		}
	}
	return children, nil
}

// statusRank returns a sort rank for status ordering. "Active work first"
// in epic listings means: in-progress (the agent is actively touching it),
// then code-review (waiting for human/agent review — still in flight from
// the contributor's perspective), then ready (committed and pullable), then
// backlog (queued for grooming), then done, then archive. An unknown status
// sorts last so a misspelled or future status is visible at the bottom
// rather than silently grouped.
func statusRank(status string) int {
	switch status {
	case "in-progress":
		return 0
	case "code-review":
		return 1
	case "ready":
		return 2
	case "backlog":
		return 3
	case "done":
		return 4
	case "archive":
		return 5
	default:
		return 6
	}
}
