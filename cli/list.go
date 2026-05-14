package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

func cmdList(args []string) {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	tagsFilter := fs.String("t", "", "filter by tag (exact match per tag)")
	sizeFilter := fs.String("s", "", "filter by size (exact match)")
	moduleFilter := fs.String("m", "", "filter by module (substring match)")
	typeFilter := fs.String("T", "", "filter by type (exact match)")
	priorityFilter := fs.String("p", "", "filter by priority (exact match)")
	parentFilter := fs.String("parent", "", "filter by parent epic ID")
	statusFilter := fs.String("status", "backlog", "status filter: backlog|in-progress|done|archive|active|all")
	limit := fs.Int("n", 0, "limit results to N tasks (default: no limit)")
	jsonOut := fs.Bool("json", false, "emit JSON array (empty selection → []) to stdout")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb ls [-t tags] [-s size] [-m module] [-T type] [-p priority] [--parent ID] [--status backlog|in-progress|done|archive|active|all] [-n N] [--json]\n\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	boardDir := cfg.BoardDir

	// Normalize parent filter.
	normalizedParent := ""
	if *parentFilter != "" {
		normalizedParent = normalizeTaskID(*parentFilter)
	}

	dirs, err := resolveStatusFilter(*statusFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cwd, _ := os.Getwd()

	refs, err := discoverTaskRefs(boardDir, dirs)
	if err != nil {
		fatal("%v", err)
	}
	var tasks []Task
	for _, ref := range refs {
		t, err := parseTaskRef(ref, cwd)
		if err != nil {
			warnSkippingTask(ref.Path, err)
			continue
		}
		if matchesFilters(t, *tagsFilter, *sizeFilter, *moduleFilter, *typeFilter, *priorityFilter, normalizedParent) {
			tasks = append(tasks, t)
		}
	}

	// Sort by priority (P0 first), then by numeric ID (lower first).
	sort.Slice(tasks, func(i, j int) bool {
		pi := priorityRank(tasks[i].Priority)
		pj := priorityRank(tasks[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return numericID(tasks[i].ID) < numericID(tasks[j].ID)
	})

	// Apply limit.
	if *limit > 0 && len(tasks) > *limit {
		tasks = tasks[:*limit]
	}

	if *jsonOut {
		// JSON contract: empty result is `[]\n`, not prose. tasks is already
		// nil-safe — emitTasksJSON allocates a length-0 slice.
		if err := emitTasksJSON(tasks); err != nil {
			fatal("%v", err)
		}
		return
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return
	}

	// Output with tabwriter for alignment.
	// Always emit all columns to keep tabwriter alignment consistent across rows.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, t := range tasks {
		module := orDash(t.Module)
		size := orDash(t.Size)
		tags := ""
		if t.Tags != "" {
			tags = t.Tags
		}

		title := t.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}

		if tags != "" {
			fmt.Fprintf(w, "%s\t%s\t%s\tT:%s\tM:%s\tS:%s\tTags:%s\n", t.FilePath, t.Priority, title, t.Type, module, size, tags)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\tT:%s\tM:%s\tS:%s\n", t.FilePath, t.Priority, title, t.Type, module, size)
		}
	}
	w.Flush()
}

// matchesFilters returns true if the task passes all active filters.
func matchesFilters(t Task, tags, size, module, taskType, priority, parent string) bool {
	if tags != "" && !hasTag(t.Tags, tags) {
		return false
	}
	if size != "" && !strings.EqualFold(t.Size, size) {
		return false
	}
	if module != "" && !strings.Contains(strings.ToLower(t.Module), strings.ToLower(module)) {
		return false
	}
	if taskType != "" && !strings.EqualFold(t.Type, taskType) {
		return false
	}
	if priority != "" && !strings.EqualFold(t.Priority, priority) {
		return false
	}
	if parent != "" && !strings.EqualFold(t.Parent, parent) {
		return false
	}
	return true
}

// orDash returns the string or "-" if empty.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// priorityRank maps priority strings to sortable integers.
// Unknown priorities sort last.
func priorityRank(p string) int {
	switch strings.ToUpper(p) {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	default:
		return 9
	}
}

// numericID extracts the numeric part of a task ID like "WS-1170".
func numericID(id string) int {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(parts[1])
	return n
}
