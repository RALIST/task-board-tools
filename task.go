package main

import (
	"bufio"
	"os"
	"strings"
)

// Task represents a parsed task file.
type Task struct {
	ID       string // e.g. "WS-1170"
	Title    string
	Type     string // feature, bug, tech-debt, improvement, spike
	Priority string // P0, P1, P2, P3
	Size     string // S, M, L, XL
	Module   string
	Tags     string // comma-separated
	Branch   string
	Parent   string // parent epic task ID (e.g., "WS-32")
	Status   string // directory name: backlog, in-progress, review, done
	FilePath string // relative path from project root
}

// maxMetadataLines limits how many lines we scan for metadata (performance).
const maxMetadataLines = 15

// parseTaskFile reads a task markdown file and extracts metadata from the first
// 15 lines. It sets Status based on the parent directory name and FilePath as
// the relative path from the board root.
func parseTaskFile(path string) (Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return Task{}, err
	}
	defer f.Close()

	var t Task

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() && lineNum < maxMetadataLines {
		lineNum++
		line := scanner.Text()

		// Extract title from "# PREFIX-NNN: Title"
		if strings.HasPrefix(line, "# "+cfg.Prefix+"-") {
			if idx := strings.Index(line, ": "); idx != -1 {
				// ID is between "# " and ":"
				t.ID = strings.TrimPrefix(line[:idx], "# ")
				t.Title = line[idx+2:]
			}
			continue
		}

		// Extract metadata fields — handle both "**Field:** value" and "- **Field:** value"
		trimmed := line
		trimmed = strings.TrimPrefix(trimmed, "- ")

		if val, ok := extractFieldAny(trimmed, "Type"); ok {
			t.Type = val
		} else if val, ok := extractFieldAny(trimmed, "Priority"); ok {
			t.Priority = val
		} else if val, ok := extractFieldAny(trimmed, "Size"); ok {
			t.Size = val
		} else if val, ok := extractFieldAny(trimmed, "Module"); ok {
			t.Module = val
		} else if val, ok := extractFieldAny(trimmed, "Tags"); ok {
			t.Tags = val
		} else if val, ok := extractFieldAny(trimmed, "Branch"); ok {
			t.Branch = val
		} else if val, ok := extractFieldAny(trimmed, "Parent"); ok {
			t.Parent = val
		}
	}

	return t, scanner.Err()
}

// hasTag checks whether tag appears in a comma-separated tags string (case-insensitive).
func hasTag(tags, tag string) bool {
	for _, t := range strings.Split(tags, ",") {
		if strings.EqualFold(strings.TrimSpace(t), tag) {
			return true
		}
	}
	return false
}

// addTag appends tag to a comma-separated tags string if not already present.
func addTag(tags, tag string) string {
	if hasTag(tags, tag) {
		return tags
	}
	if tags == "" {
		return tag
	}
	return tags + "," + tag
}

// extractFieldAny matches both "**Field:** value" and "**Field**: value" formats.
func extractFieldAny(line, name string) (string, bool) {
	// Try "**Field:** value" (colon inside bold)
	prefix1 := "**" + name + ":**"
	if strings.HasPrefix(line, prefix1) {
		val := strings.TrimSpace(strings.TrimPrefix(line, prefix1))
		return val, true
	}
	// Try "**Field**: value" (colon outside bold)
	prefix2 := "**" + name + "**:"
	if strings.HasPrefix(line, prefix2) {
		val := strings.TrimSpace(strings.TrimPrefix(line, prefix2))
		return val, true
	}
	return "", false
}
