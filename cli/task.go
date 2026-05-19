package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Task represents a parsed task file. JSON tags describe the wire shape used
// by `--json` callers (the GUI in particular); see cli/json_output.go for the
// transformations applied (e.g. Tags is split into a list).
type Task struct {
	ID          string `json:"id"` // e.g. "WS-1170"
	Title       string `json:"title"`
	Type        string `json:"type"`     // feature, bug, tech-debt, improvement, spike
	Priority    string `json:"priority"` // P0, P1, P2, P3
	Size        string `json:"size"`     // S, M, L, XL
	Module      string `json:"module"`
	Tags        string `json:"tags"` // comma-separated; emitted as []string in JSON
	Branch      string `json:"branch"`
	Parent      string `json:"parent"`      // parent epic task ID (e.g., "WS-32")
	Status      string `json:"status"`      // directory name: backlog, in-progress, done, archive
	FilePath    string `json:"filePath"`    // relative path from project root
	Agent       string `json:"agent"`       // claude | codex | "" (optional)
	AgentStatus string `json:"agentStatus"` // queued | running | success | failed | cancelled | interrupted | needs-user | "" (optional)
}

// validAgents enumerates the agents `tb edit -a` accepts. Empty is also
// accepted (clears the field) but is not in this map.
var validAgents = map[string]bool{"claude": true, "codex": true}

// validAgentStatuses enumerates the AgentStatus enum. `cancelled` is
// user-initiated and `interrupted` is recovery-initiated; M5 stale-recovery
// must never overwrite either. The validator allows both values so the same
// `tb edit --agent-status` path used by recovery can write them; the
// "nothing manual writes interrupted" rule lives in code+docs, not here
// (matching the cancelled precedent).
//
// `needs-user` is the agent-attention handoff (TB-182): an autonomous agent
// stopped mid-run because user input is required. The agent writes both the
// status and a `## User Attention` section through managed `tb edit` calls.
// The carve-out in gui/app/agent_run.go's recordTerminal preserves this
// status when a runner exits, so the agent's "stop and ask" intent is not
// overwritten by the exit code.
var validAgentStatuses = map[string]bool{
	"queued":      true,
	"running":     true,
	"success":     true,
	"failed":      true,
	"cancelled":   true,
	"interrupted": true,
	"needs-user":  true,
}

// maxMetadataLines limits how many lines we scan for metadata (performance).
// Bumped from 15 to 20 in M1 to accommodate the Agent and AgentStatus fields
// without crowding out existing metadata on tasks that also have Parent.
const maxMetadataLines = 20

// parseTaskFile reads a task markdown file and extracts metadata from the first
// maxMetadataLines lines. Status and FilePath are set by the discovery wrapper,
// because folder-form task markdown lives one directory below its status.
func parseTaskFile(path string) (Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return Task{}, err
	}
	defer f.Close()

	var t Task

	scanner := bufio.NewScanner(f)
	lineNum := 0
	foundHeader := false
	for scanner.Scan() && lineNum < maxMetadataLines {
		lineNum++
		line := scanner.Text()

		// Extract title from "# PREFIX-NNN: Title".
		if id, title, malformed := parseTaskHeader(line); id != "" || malformed {
			if malformed {
				return Task{}, fmt.Errorf("malformed task header on line %d: expected \"# %s-N: title\" with a non-empty title", lineNum, cfg.Prefix)
			}
			t.ID = id
			t.Title = title
			foundHeader = true
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
		} else if val, ok := extractFieldAny(trimmed, "Agent"); ok {
			t.Agent = val
		} else if val, ok := extractFieldAny(trimmed, "AgentStatus"); ok {
			t.AgentStatus = val
		}
	}

	if err := scanner.Err(); err != nil {
		return Task{}, err
	}
	if !foundHeader {
		return Task{}, fmt.Errorf("missing task header in first %d lines: expected \"# %s-N: title\"", maxMetadataLines, cfg.Prefix)
	}

	return t, nil
}

func parseTaskHeader(line string) (id, title string, malformed bool) {
	prefix := "# " + cfg.Prefix + "-"
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}

	rest := line[len(prefix):]
	digitCount := 0
	for digitCount < len(rest) && rest[digitCount] >= '0' && rest[digitCount] <= '9' {
		digitCount++
	}
	if digitCount == 0 {
		return "", "", false
	}

	id = cfg.Prefix + "-" + rest[:digitCount]
	tail := rest[digitCount:]
	if !strings.HasPrefix(tail, ":") {
		return "", "", true
	}

	title = strings.TrimSpace(tail[1:])
	if title == "" {
		return "", "", true
	}
	return id, title, false
}

func warnSkippingTask(path string, err error) {
	fmt.Fprintf(os.Stderr, "warning: skipping malformed task file %s: %v\n", path, err)
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
