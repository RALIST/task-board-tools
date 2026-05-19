package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

type listItem struct {
	task             Task
	doneCompletedAt  time.Time
	doneCompletionOK bool
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	tagsFilter := fs.String("t", "", "filter by tag (exact match per tag)")
	sizeFilter := fs.String("s", "", "filter by size (exact match)")
	moduleFilter := fs.String("m", "", "filter by module (substring match)")
	typeFilter := fs.String("T", "", "filter by type (exact match)")
	priorityFilter := fs.String("p", "", "filter by priority (exact match)")
	parentFilter := fs.String("parent", "", "filter by parent epic ID")
	statusFilter := fs.String("status", "backlog", "status filter: backlog|ready|in-progress|code-review|done|archive|active|all")
	limit := fs.Int("n", 0, "limit results to N tasks (default: no limit)")
	jsonOut := fs.Bool("json", false, "emit JSON array (empty selection → []) to stdout")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb ls [-t tags] [-s size] [-m module] [-T type] [-p priority] [--parent ID] [--status backlog|ready|in-progress|code-review|done|archive|active|all] [-n N] [--json]\n\n")
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
	var items []listItem
	for _, ref := range refs {
		t, err := parseTaskRef(ref, cwd)
		if err != nil {
			warnSkippingTask(ref.Path, err)
			continue
		}
		if matchesFilters(t, *tagsFilter, *sizeFilter, *moduleFilter, *typeFilter, *priorityFilter, normalizedParent) {
			items = append(items, newListItem(t, ref.Path))
		}
	}

	sortListItems(items)

	// Apply limit.
	if *limit > 0 && len(items) > *limit {
		items = items[:*limit]
	}

	if *jsonOut {
		// JSON contract: empty result is `[]\n`, not prose. tasks is already
		// nil-safe — emitTasksJSON allocates a length-0 slice.
		if err := emitTasksJSON(tasksFromListItems(items)); err != nil {
			fatal("%v", err)
		}
		return
	}

	if len(items) == 0 {
		fmt.Println("No tasks found.")
		return
	}

	// Output with tabwriter for alignment.
	// Always emit all columns to keep tabwriter alignment consistent across rows.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, item := range items {
		t := item.task
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

func newListItem(t Task, taskPath string) listItem {
	item := listItem{task: t}
	if t.Status == "done" {
		item.doneCompletedAt, item.doneCompletionOK = latestDoneCompletionTime(taskPath)
	}
	return item
}

func tasksFromListItems(items []listItem) []Task {
	tasks := make([]Task, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, item.task)
	}
	return tasks
}

func sortListItems(items []listItem) {
	// Start from the historical list order: priority (P0 first), then numeric
	// ID. This keeps Backlog, Ready, In Progress, and Code Review ordering
	// unchanged for active-board callers.
	sort.Slice(items, func(i, j int) bool {
		return listPriorityLess(items[i].task, items[j].task)
	})
	sortDoneSlotsByCompletion(items)
}

func sortDoneSlotsByCompletion(items []listItem) {
	var doneIndexes []int
	var doneItems []listItem
	for i, item := range items {
		if item.task.Status == "done" {
			doneIndexes = append(doneIndexes, i)
			doneItems = append(doneItems, item)
		}
	}
	if len(doneItems) < 2 {
		return
	}

	// Done tasks with a parseable completion log sort first by completion date
	// descending. Legacy or malformed done tasks with no parseable entry sort
	// after parseable completions, then fall back to the same priority/ID order
	// used by the rest of `tb ls`.
	sort.Slice(doneItems, func(i, j int) bool {
		left, right := doneItems[i], doneItems[j]
		if left.doneCompletionOK != right.doneCompletionOK {
			return left.doneCompletionOK
		}
		if left.doneCompletionOK && !left.doneCompletedAt.Equal(right.doneCompletedAt) {
			return left.doneCompletedAt.After(right.doneCompletedAt)
		}
		return listPriorityLess(left.task, right.task)
	})

	for i, item := range doneItems {
		items[doneIndexes[i]] = item
	}
}

func latestDoneCompletionTime(taskPath string) (time.Time, bool) {
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return time.Time{}, false
	}
	content := string(data)
	logSection, ok := findTaskSection(content, "## Log")
	if !ok {
		return time.Time{}, false
	}

	var latest time.Time
	found := false
	for _, line := range strings.Split(content[logSection.bodyStart:logSection.end], "\n") {
		completedAt, ok := parseDoneCompletionLogLine(line)
		if !ok {
			continue
		}
		if !found || completedAt.After(latest) {
			latest = completedAt
			found = true
		}
	}
	return latest, found
}

func parseDoneCompletionLogLine(line string) (time.Time, bool) {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "- ")
	dateText, message, ok := strings.Cut(line, ":")
	if !ok {
		return time.Time{}, false
	}
	completedAt, err := time.Parse("2006-01-02", strings.TrimSpace(dateText))
	if err != nil {
		return time.Time{}, false
	}

	message = strings.ToLower(strings.TrimSpace(message))
	if hasCompletionMessagePrefix(message, "done") || hasCompletionMessagePrefix(message, "moved to done") {
		return completedAt, true
	}
	return time.Time{}, false
}

func hasCompletionMessagePrefix(message, prefix string) bool {
	if message == prefix {
		return true
	}
	if !strings.HasPrefix(message, prefix) {
		return false
	}
	next := message[len(prefix)]
	return !isASCIILetterOrDigit(next)
}

func isASCIILetterOrDigit(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
}

func listPriorityLess(left, right Task) bool {
	pi := priorityRank(left.Priority)
	pj := priorityRank(right.Priority)
	if pi != pj {
		return pi < pj
	}
	return numericID(left.ID) < numericID(right.ID)
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
