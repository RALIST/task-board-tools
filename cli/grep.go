package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
)

func cmdGrep(args []string) {
	fs := flag.NewFlagSet("grep", flag.ExitOnError)
	statusFilter := fs.String("status", "all", "status filter: backlog|in-progress|code-review|done|archive|active|all (default: all)")
	caseSensitive := fs.Bool("s", false, "case-sensitive search (default: case-insensitive)")
	filesOnly := fs.Bool("l", false, "show only matching task IDs, no matched lines")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb grep <pattern> [--status backlog|in-progress|code-review|done|archive|active|all] [-s] [-l]\n\n")
		fmt.Fprintf(os.Stderr, "Search across all task files using a regular expression.\n\n")
		fs.PrintDefaults()
	}

	// Reorder args so flags can appear after the pattern.
	reordered := reorderArgs(args)

	if err := fs.Parse(reordered); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: pattern is required")
		fs.Usage()
		os.Exit(1)
	}

	pattern := fs.Arg(0)
	// Normalize BRE alternation (\|) to ERE/RE2 alternation (|).
	// Agents and shell scripts often use grep-style \| which is literal in Go regexp.
	pattern = strings.ReplaceAll(pattern, `\|`, "|")
	if !*caseSensitive {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid regex %q: %v\n", fs.Arg(0), err)
		os.Exit(1)
	}

	boardDir := cfg.BoardDir

	dirs, err := resolveStatusFilter(*statusFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cwd, _ := os.Getwd()

	type matchLine struct {
		lineNum int
		text    string
	}
	type match struct {
		task    Task
		lines   []matchLine // empty when filesOnly
		matches int
	}

	var results []match

	refs, err := discoverTaskRefs(boardDir, dirs)
	if err != nil {
		fatal("%v", err)
	}
	for _, ref := range refs {
		t, err := parseTaskRef(ref, cwd)
		if err != nil {
			warnSkippingTask(ref.Path, err)
			continue
		}

		// Read full file content and search.
		f, err := os.Open(ref.Path)
		if err != nil {
			continue
		}

		var matchedLines []matchLine
		matchCount := 0
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				matchCount++
				if !*filesOnly {
					matchedLines = append(matchedLines, matchLine{lineNum, line})
				}
			}
		}
		f.Close()

		if matchCount > 0 {
			results = append(results, match{task: t, lines: matchedLines, matches: matchCount})
		}
	}

	// Sort by priority then numeric ID.
	sort.Slice(results, func(i, j int) bool {
		pi := priorityRank(results[i].task.Priority)
		pj := priorityRank(results[j].task.Priority)
		if pi != pj {
			return pi < pj
		}
		return numericID(results[i].task.ID) < numericID(results[j].task.ID)
	})

	if len(results) == 0 {
		fmt.Printf("No tasks matching %q\n", fs.Arg(0))
		return
	}

	fmt.Printf("Found %d task(s) matching %q:\n\n", len(results), fs.Arg(0))

	if *filesOnly {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, r := range results {
			title := r.task.Title
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t(%d matches)\n", r.task.ID, r.task.Priority, title, r.matches)
		}
		w.Flush()
		return
	}

	// Detailed output with matched lines.
	for i, r := range results {
		t := r.task
		title := t.Title
		module := orDash(t.Module)
		fmt.Printf("── %s: %s [%s] M:%s ──\n", t.ID, title, t.Priority, module)
		fmt.Printf("   %s\n", t.FilePath)

		for _, ml := range r.lines {
			fmt.Printf("   %3d: %s\n", ml.lineNum, ml.text)
		}

		if i < len(results)-1 {
			fmt.Println()
		}
	}
}
