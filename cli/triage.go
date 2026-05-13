package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
)

// triageReason describes why a task needs grooming.
type triageReason struct {
	task    Task
	reasons []string
}

func cmdTriage(_ []string) {
	boardDir := cfg.BoardDir

	// Scan backlog for tasks needing grooming.
	var results []triageReason
	for _, status := range []string{"backlog"} {
		dirPath := filepath.Join(boardDir, status)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		cwd, _ := os.Getwd()
		for _, entry := range entries {
			if entry.IsDir() || !isTaskFile(entry.Name()) {
				continue
			}
			fullPath := filepath.Join(dirPath, entry.Name())
			t, err := parseTaskFile(fullPath)
			if err != nil {
				continue
			}
			t.Status = status
			t.FilePath = relPath(cwd, fullPath)

			reasons := checkNeedsGrooming(fullPath, t)
			if len(reasons) > 0 {
				results = append(results, triageReason{task: t, reasons: reasons})
			}
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

	// 2. Goal is placeholder or missing.
	if hasPlaceholderSection(content, "## Goal") {
		reasons = append(reasons, "no goal")
	}

	// 3. Acceptance criteria is placeholder or missing.
	if hasPlaceholderSection(content, "## Acceptance Criteria") {
		reasons = append(reasons, "no acceptance criteria")
	}

	// 4. Auto-created by tb scan.
	if strings.Contains(content, "Created by `tb scan`") {
		reasons = append(reasons, "auto-created by scan")
	}

	return reasons
}

// hasPlaceholderSection checks if a section exists and contains only placeholder text.
func hasPlaceholderSection(content, heading string) bool {
	idx := strings.Index(content, heading)
	if idx == -1 {
		return true // section missing entirely
	}

	// Extract content between this heading and the next "## " heading.
	after := content[idx+len(heading):]
	nextHeading := strings.Index(after, "\n## ")
	var sectionBody string
	if nextHeading == -1 {
		sectionBody = after
	} else {
		sectionBody = after[:nextHeading]
	}

	sectionBody = strings.TrimSpace(sectionBody)

	// Check for common placeholders.
	return sectionBody == "" ||
		sectionBody == "(to be filled)" ||
		sectionBody == "- [ ] (to be filled)"
}
