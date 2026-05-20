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

// listFilters holds the multi-value filter selections parsed from `tb ls`
// flags. Each slice is OR-within: a task matches the field when any value
// matches. Across fields, listFilters.match requires every populated field
// to match (AND-across). Empty slices skip that filter entirely.
type listFilters struct {
	tags       []string
	sizes      []string
	modules    []string
	types      []string
	priorities []string
	parents    []string
	agents     []string
	search     string
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	tagsFilter := fs.String("t", "", "filter by tag (comma-separated, matches any; exact case-insensitive tag-name match)")
	sizeFilter := fs.String("s", "", "filter by size (comma-separated, matches any; case-insensitive equality)")
	moduleFilter := fs.String("m", "", "filter by module (comma-separated, matches any; case-insensitive substring match)")
	typeFilter := fs.String("T", "", "filter by type (comma-separated, matches any; case-insensitive equality)")
	priorityFilter := fs.String("p", "", "filter by priority (comma-separated, matches any; case-insensitive equality)")
	parentFilter := fs.String("parent", "", "filter by parent epic ID (comma-separated, matches any; normalized + case-insensitive equality)")
	agentFilter := fs.String("agent", "", "filter by agent: claude, codex, none (comma-separated, matches any; 'none' matches unassigned)")
	searchFilter := fs.String("search", "", "free-text search; case-insensitive substring match against id and title")
	statusFilter := fs.String("status", "backlog", "status filter: backlog|ready|in-progress|code-review|done|archive|active|all")
	limit := fs.Int("n", 0, "limit results to N tasks (default: no limit)")
	jsonOut := fs.Bool("json", false, "emit JSON array (empty selection → []) to stdout")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb ls [-t tag[,tag...]] [-s size[,size...]] [-m module[,module...]] [-T type[,type...]] [-p priority[,priority...]] [--parent ID[,ID...]] [--agent claude|codex|none[,...]] [--search term] [--status backlog|ready|in-progress|code-review|done|archive|active|all] [-n N] [--json]\n\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	filters, err := buildListFilters(*tagsFilter, *sizeFilter, *moduleFilter, *typeFilter, *priorityFilter, *parentFilter, *agentFilter, *searchFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	boardDir := cfg.BoardDir

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
		if filters.match(t) {
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

// splitCSVFilter splits a comma-separated filter value, trims whitespace
// around each segment, and drops empty segments. Returns nil for the empty
// or all-whitespace input so callers can treat "no filter on this field" as
// the same as the flag being unset.
func splitCSVFilter(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildListFilters parses the raw flag strings into a listFilters value and
// validates the --agent flag against the supported agent set
// (`tb edit -a` valid agents plus the `none` sentinel for unassigned tasks).
// Unknown --agent values produce an error; all other filters keep the
// historical no-match behavior for unknown values so single-value callers
// remain byte-identical.
func buildListFilters(tags, sizes, modules, types, priorities, parents, agents, search string) (listFilters, error) {
	f := listFilters{
		tags:       splitCSVFilter(tags),
		sizes:      splitCSVFilter(sizes),
		modules:    splitCSVFilter(modules),
		types:      splitCSVFilter(types),
		priorities: splitCSVFilter(priorities),
		search:     strings.TrimSpace(search),
	}

	for _, p := range splitCSVFilter(parents) {
		f.parents = append(f.parents, normalizeTaskID(p))
	}

	agentValues := splitCSVFilter(agents)
	if len(agentValues) > 0 {
		f.agents = make([]string, 0, len(agentValues))
		for _, raw := range agentValues {
			canonical := strings.ToLower(raw)
			if canonical != "none" && !validAgents[canonical] {
				return listFilters{}, fmt.Errorf("invalid agent %q — use: claude, codex, none", raw)
			}
			f.agents = append(f.agents, canonical)
		}
	}

	return f, nil
}

// match returns true if the task passes all populated filters. A nil/empty
// slice for a field skips that filter.
func (f listFilters) match(t Task) bool {
	if len(f.tags) > 0 && !anyTagMatches(t.Tags, f.tags) {
		return false
	}
	if len(f.sizes) > 0 && !anyEqualFold(f.sizes, t.Size) {
		return false
	}
	if len(f.modules) > 0 && !anyModuleSubstring(f.modules, t.Module) {
		return false
	}
	if len(f.types) > 0 && !anyEqualFold(f.types, t.Type) {
		return false
	}
	if len(f.priorities) > 0 && !anyEqualFold(f.priorities, t.Priority) {
		return false
	}
	if len(f.parents) > 0 && !anyEqualFold(f.parents, t.Parent) {
		return false
	}
	if len(f.agents) > 0 && !anyAgentMatches(f.agents, t.Agent) {
		return false
	}
	if f.search != "" && !matchesSearch(f.search, t.ID, t.Title) {
		return false
	}
	return true
}

// anyEqualFold returns true if any value in values is case-insensitively
// equal to candidate. It mirrors the historical `strings.EqualFold(field, v)`
// check used by single-value filters.
func anyEqualFold(values []string, candidate string) bool {
	for _, v := range values {
		if strings.EqualFold(candidate, v) {
			return true
		}
	}
	return false
}

// anyTagMatches returns true if any of the supplied filter tags equals any
// of the task's tags (case-insensitive, whitespace-trimmed). Preserves the
// per-value tag-name match implemented by hasTag.
func anyTagMatches(taskTags string, filterTags []string) bool {
	for _, ft := range filterTags {
		if hasTag(taskTags, ft) {
			return true
		}
	}
	return false
}

// anyModuleSubstring keeps the historical case-insensitive substring match
// for the module field, applied OR-within across multiple filter values.
func anyModuleSubstring(filterModules []string, taskModule string) bool {
	lowered := strings.ToLower(taskModule)
	for _, m := range filterModules {
		if strings.Contains(lowered, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// anyAgentMatches treats the "none" sentinel as "blank Agent". Other values
// have already been canonicalized to lowercase by buildListFilters and are
// compared case-insensitively to the task's Agent field.
func anyAgentMatches(filterAgents []string, taskAgent string) bool {
	taskAgentLower := strings.ToLower(strings.TrimSpace(taskAgent))
	for _, a := range filterAgents {
		if a == "none" {
			if taskAgentLower == "" {
				return true
			}
			continue
		}
		if a == taskAgentLower {
			return true
		}
	}
	return false
}

// matchesSearch implements the GUI FilterBar search-box semantics:
// case-insensitive substring match against either the task ID or title.
func matchesSearch(term, id, title string) bool {
	t := strings.ToLower(term)
	return strings.Contains(strings.ToLower(id), t) || strings.Contains(strings.ToLower(title), t)
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
