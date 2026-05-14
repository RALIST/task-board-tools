package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

// triageReason describes why a task needs grooming.
type triageReason struct {
	task    Task
	reasons []string
}

type triageReasonJSON struct {
	taskJSON
	Reasons []string `json:"reasons"`
}

func cmdTriage(args []string) {
	fs := flag.NewFlagSet("triage", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON array with task metadata and grooming reasons")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb triage [--json]\n\n")
		fs.PrintDefaults()
	}

	reordered := reorderArgs(args)
	if err := fs.Parse(reordered); err != nil {
		os.Exit(1)
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		os.Exit(1)
	}

	boardDir := cfg.BoardDir

	cwd, _ := os.Getwd()

	// Scan backlog for tasks needing grooming.
	var results []triageReason
	refs, err := discoverTaskRefs(boardDir, []string{"backlog"})
	if err != nil {
		fatal("%v", err)
	}
	for _, ref := range refs {
		t, err := parseTaskRef(ref, cwd)
		if err != nil {
			warnSkippingTask(ref.Path, err)
			continue
		}

		reasons := checkNeedsGrooming(ref.Path, t)
		if len(reasons) > 0 {
			results = append(results, triageReason{task: t, reasons: reasons})
		}
	}

	// Sort by priority then ID.
	sort.Slice(results, func(i, j int) bool {
		pi := priorityRank(results[i].task.Priority)
		pj := priorityRank(results[j].task.Priority)
		if pi != pj {
			return pi < pj
		}
		return numericID(results[i].task.ID) < numericID(results[j].task.ID)
	})

	if *jsonOut {
		if err := emitTriageJSON(results); err != nil {
			fatal("%v", err)
		}
		return
	}

	if len(results) == 0 {
		fmt.Println("No tasks need grooming.")
		return
	}

	fmt.Printf("Found %d task(s) needing grooming:\n\n", len(results))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range results {
		t := r.task
		title := t.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		module := orDash(t.Module)
		size := orDash(t.Size)
		priority := orDash(t.Priority)
		fmt.Fprintf(w, "%s\t%s\t%s\tT:%s\tM:%s\tS:%s\t[%s]\n", t.FilePath, priority, title, t.Type, module, size, strings.Join(r.reasons, ", "))
	}
	w.Flush()
	fmt.Printf("\nUse `tb show <ID>` to inspect, `/groom <ID>` to run a grooming session.\n")
}

func emitTriageJSON(results []triageReason) error {
	payload := make([]triageReasonJSON, 0, len(results))
	for _, result := range results {
		payload = append(payload, triageReasonJSON{
			taskJSON: marshalTask(result.task),
			Reasons:  result.reasons,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// checkNeedsGrooming reads the full task file and returns reasons it needs grooming.
// Returns nil if the task is well-groomed.
func checkNeedsGrooming(path string, t Task) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)

	var reasons []string

	// 1. No priority.
	if t.Priority == "" {
		reasons = append(reasons, "no priority")
	}

	// 2. No module.
	if t.Module == "" {
		reasons = append(reasons, "no module")
	}

	// 3. Goal is placeholder or missing.
	if hasPlaceholderSection(content, "## Goal") {
		reasons = append(reasons, "no goal")
	}

	// 4. Acceptance criteria is placeholder or missing.
	if hasPlaceholderSection(content, "## Acceptance Criteria") {
		reasons = append(reasons, "no acceptance criteria")
	}

	// 5. Auto-created by tb scan.
	if strings.Contains(content, "Created by `tb scan`") {
		reasons = append(reasons, "auto-created by scan")
	}

	return reasons
}

// hasPlaceholderSection checks if a section exists and contains only placeholder text.
func hasPlaceholderSection(content, heading string) bool {
	section, ok := findTaskSection(content, heading)
	if !ok {
		return true // section missing entirely
	}

	sectionBody := strings.TrimSpace(content[section.bodyStart:section.end])

	// Check for common placeholders.
	return sectionBody == "" ||
		sectionBody == "(to be filled)" ||
		sectionBody == "- [ ] (to be filled)"
}
