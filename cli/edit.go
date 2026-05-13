package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func cmdEdit(args []string) {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	priority := fs.String("p", "", "priority (P0, P1, P2, P3)")
	taskType := fs.String("T", "", "type (feature, bug, tech-debt, improvement, spike)")
	size := fs.String("s", "", "size (S, M, L, XL)")
	module := fs.String("m", "", "module")
	tags := fs.String("t", "", "tags (comma-separated, replaces existing)")
	agent := fs.String("a", "", "agent (claude, codex)")
	agentStatus := fs.String("agent-status", "", "agent status (queued, running, success, failed, cancelled)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb edit <ID> [-p P0] [-T feature] [-s M] [-m module] [-t tags] [-a claude] [--agent-status queued]\n\n")
		fs.PrintDefaults()
	}

	reordered := reorderArgs(args)
	if err := fs.Parse(reordered); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: task ID is required")
		fs.Usage()
		os.Exit(1)
	}

	taskID := normalizeTaskID(fs.Arg(0))

	// Validate provided values.
	if *priority != "" {
		*priority = strings.ToUpper(*priority)
		if !validPriorities[*priority] {
			fmt.Fprintf(os.Stderr, "error: invalid priority %q — use P0, P1, P2, or P3\n", *priority)
			os.Exit(1)
		}
	}
	if *taskType != "" {
		*taskType = strings.ToLower(*taskType)
		if !validTypes[*taskType] {
			fmt.Fprintf(os.Stderr, "error: invalid type %q — use: feature, bug, tech-debt, improvement, spike\n", *taskType)
			os.Exit(1)
		}
	}
	if *size != "" {
		*size = strings.ToUpper(*size)
		if !validSizes[*size] {
			fmt.Fprintf(os.Stderr, "error: invalid size %q — use: S, M, L, XL\n", *size)
			os.Exit(1)
		}
	}
	if *agent != "" {
		*agent = strings.ToLower(*agent)
		// "none" is the clear sentinel — see applyClearable below.
		if *agent != "none" && !validAgents[*agent] {
			fmt.Fprintf(os.Stderr, "error: invalid agent %q — use: claude, codex, none\n", *agent)
			os.Exit(1)
		}
	}
	if *agentStatus != "" {
		*agentStatus = strings.ToLower(*agentStatus)
		if *agentStatus != "none" && !validAgentStatuses[*agentStatus] {
			fmt.Fprintf(os.Stderr, "error: invalid agent-status %q — use: queued, running, success, failed, cancelled, none\n", *agentStatus)
			os.Exit(1)
		}
	}

	// Collect changes.
	changes := map[string]string{}
	if *priority != "" {
		changes["Priority"] = *priority
	}
	if *taskType != "" {
		changes["Type"] = *taskType
	}
	if *size != "" {
		changes["Size"] = *size
	}
	if *module != "" {
		changes["Module"] = *module
	}
	if *tags != "" {
		changes["Tags"] = *tags
	}
	if *agent != "" {
		changes["Agent"] = *agent
	}
	if *agentStatus != "" {
		changes["AgentStatus"] = *agentStatus
	}

	if len(changes) == 0 {
		fmt.Fprintln(os.Stderr, "error: no changes specified")
		fs.Usage()
		os.Exit(1)
	}

	boardDir := cfg.BoardDir

	lock, err := lockBoard(boardDir)
	if err != nil {
		fatal("%v", err)
	}
	defer lock.unlock()

	taskPath, err := findTask(boardDir, taskID)
	if err != nil {
		fatal("%v", err)
	}

	data, err := os.ReadFile(taskPath)
	if err != nil {
		fatal("cannot read %s: %v", taskPath, err)
	}

	lines := strings.Split(string(data), "\n")

	// Apply each change.
	// `Agent` and `AgentStatus` accept the sentinel "none" to mean "clear
	// the field"; for those a value of "none" deletes the metadata line
	// instead of writing it. Every other field is set verbatim.
	var applied []string
	for field, value := range changes {
		if value == "none" && (field == "Agent" || field == "AgentStatus") {
			lines = clearField(lines, field)
		} else {
			lines = setField(lines, field, value)
		}
		applied = append(applied, fmt.Sprintf("%s=%s", strings.ToLower(field), value))
	}

	// Append log entry.
	today := time.Now().Format("2006-01-02")
	content := strings.Join(lines, "\n")
	content = appendLogEntry(content, fmt.Sprintf("- %s: Edited %s\n", today, strings.Join(applied, ", ")))

	if err := writeFileAtomic(taskPath, []byte(content), 0644); err != nil {
		fatal("cannot write %s: %v", taskPath, err)
	}

	if err := regenerateBoard(boardDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not regenerate BOARD.md: %v\n", err)
	}

	fmt.Printf("Updated %s: %s\n", taskID, strings.Join(applied, ", "))
}

// clearField removes the metadata line for `field` from lines (if present).
// Used by `tb edit -a none` and `tb edit --agent-status none` to drop a
// field rather than overwrite it with a sentinel value.
func clearField(lines []string, field string) []string {
	for i, line := range lines {
		trimmed := strings.TrimPrefix(line, "- ")
		if _, ok := extractFieldAny(trimmed, field); ok {
			return append(lines[:i], lines[i+1:]...)
		}
	}
	return lines
}

// setField replaces **Field:** value in lines, or inserts it before **Branch:** if missing.
func setField(lines []string, field, value string) []string {
	for i, line := range lines {
		trimmed := strings.TrimPrefix(line, "- ")
		if _, ok := extractFieldAny(trimmed, field); ok {
			lines[i] = "**" + field + ":** " + value
			return lines
		}
	}

	// Field not found — insert before Branch line.
	newLine := "**" + field + ":** " + value
	for i, line := range lines {
		trimmed := strings.TrimPrefix(line, "- ")
		if _, ok := extractFieldAny(trimmed, "Branch"); ok {
			result := make([]string, 0, len(lines)+1)
			result = append(result, lines[:i]...)
			result = append(result, newLine)
			result = append(result, lines[i:]...)
			return result
		}
	}

	// No Branch line — insert after last metadata line (first blank line after header).
	for i, line := range lines {
		if i > 0 && line == "" {
			result := make([]string, 0, len(lines)+1)
			result = append(result, lines[:i]...)
			result = append(result, newLine)
			result = append(result, lines[i:]...)
			return result
		}
	}

	return append(lines, newLine)
}
