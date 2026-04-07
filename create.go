package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	validPriorities = map[string]bool{"P0": true, "P1": true, "P2": true, "P3": true}
	validTypes      = map[string]bool{"feature": true, "bug": true, "tech-debt": true, "improvement": true, "spike": true}
	validSizes      = map[string]bool{"S": true, "M": true, "L": true, "XL": true}
)

// flagsWithValue lists single-letter flags that consume the next argument.
var flagsWithValue = map[string]bool{
	"-p": true, "-T": true, "-s": true, "-m": true, "-t": true, "-d": true,
	"--status": true, "--parent": true,
}

func cmdCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	priority := fs.String("p", "P2", "priority (P0, P1, P2, P3)")
	taskType := fs.String("T", "bug", "type (feature, bug, tech-debt, improvement, spike)")
	size := fs.String("s", "M", "size (S, M, L, XL)")
	module := fs.String("m", "", "module (required)")
	tags := fs.String("t", "", "tags (comma-separated)")
	description := fs.String("d", "", "goal/description")
	status := fs.String("status", "backlog", "initial status directory")
	parent := fs.String("parent", "", "parent epic task ID")
	epic := fs.Bool("epic", false, "create as epic (type=feature, tag=epic)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb create \"Title\" -m module [-d desc] [-p P2] [-T feature] [-s M] [-t tags] [--status backlog]\n\n")
		fs.PrintDefaults()
	}

	// Go's flag package stops at the first non-flag argument, so
	// `tb create "Title" -m mod` would lose all flags after the title.
	// Reorder args: pull positional args out, put flags first.
	reordered := reorderArgs(args)
	if err := fs.Parse(reordered); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: title is required\n\nUsage: tb create \"My task title\" -m module [-d \"description\"]")
		os.Exit(1)
	}
	title := fs.Arg(0)

	*priority = strings.ToUpper(*priority)
	if !validPriorities[*priority] {
		fmt.Fprintf(os.Stderr, "error: invalid priority %q — use P0, P1, P2, or P3\n", *priority)
		os.Exit(1)
	}

	*taskType = strings.ToLower(*taskType)
	if !validTypes[*taskType] {
		fmt.Fprintf(os.Stderr, "error: invalid type %q — use: feature, bug, tech-debt, improvement, spike\n", *taskType)
		os.Exit(1)
	}

	*size = strings.ToUpper(*size)
	if !validSizes[*size] {
		fmt.Fprintf(os.Stderr, "error: invalid size %q — use: S, M, L, XL\n", *size)
		os.Exit(1)
	}

	// Handle --epic flag: override type and add epic tag.
	if *epic {
		*taskType = "feature"
		*tags = addTag(*tags, "epic")
	}

	targetStatus, err := resolveStatus(*status)
	if err != nil {
		fatal("%v", err)
	}

	boardDir := cfg.BoardDir

	// Lock the board for the entire create operation (ID allocation + file write).
	lock, err := lockBoard(boardDir)
	if err != nil {
		fatal("%v", err)
	}
	defer lock.unlock()

	id, err := allocateID(boardDir)
	if err != nil {
		fatal("%v", err)
	}

	// Handle --parent flag: validate parent exists and auto-tag it as epic.
	var parentID string
	if *parent != "" {
		parentID = normalizeTaskID(*parent)
		parentPath, findErr := findTask(boardDir, parentID)
		if findErr != nil {
			fatal("parent task not found: %v", findErr)
		}
		parentTask, parseErr := parseTaskFile(parentPath)
		if parseErr != nil {
			fatal("cannot read parent task: %v", parseErr)
		}
		if !hasTag(parentTask.Tags, "epic") {
			if tagErr := addTagToTaskFile(parentPath, "epic"); tagErr != nil {
				fatal("cannot add epic tag to parent: %v", tagErr)
			}
		}
	}

	today := time.Now().Format("2006-01-02")
	content := buildTaskContent(id, title, *taskType, *priority, *size, *module, *tags, *description, parentID, today)

	filename := fmt.Sprintf("%s-%d.md", cfg.Prefix, id)
	destDir := filepath.Join(boardDir, targetStatus)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		fatal("cannot create directory %s: %v", destDir, err)
	}

	destPath := filepath.Join(destDir, filename)
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		fatal("cannot write %s: %v", destPath, err)
	}

	// Update parent's Subtasks section.
	if parentID != "" {
		parentPath, _ := findTask(boardDir, parentID)
		childID := fmt.Sprintf("%s-%d", cfg.Prefix, id)
		if subErr := addChildToSubtasks(parentPath, childID, *size, title); subErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not update parent subtasks: %v\n", subErr)
		}
	}

	cwd, _ := os.Getwd()
	fmt.Printf("Created %s\n", relPath(cwd, destPath))
}

// reorderArgs separates flags and positional arguments so that all flags come
// first. This allows `tb create "Title" -m module` to work even though Go's
// flag package stops parsing at the first non-flag argument.
func reorderArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsWithValue[arg] {
			// Flag that takes a value: consume this and next arg.
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else if strings.HasPrefix(arg, "-") {
			// Flag (possibly with = value, e.g. --status=backlog).
			flags = append(flags, arg)
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}

func buildTaskContent(id int, title, taskType, priority, size, module, tags, description, parent, date string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s-%d: %s\n\n", cfg.Prefix, id, title)
	fmt.Fprintf(&b, "**Type:** %s\n", taskType)
	fmt.Fprintf(&b, "**Priority:** %s\n", priority)
	fmt.Fprintf(&b, "**Size:** %s\n", size)
	if module != "" {
		fmt.Fprintf(&b, "**Module:** %s\n", module)
	}
	if tags != "" {
		fmt.Fprintf(&b, "**Tags:** %s\n", tags)
	}
	b.WriteString("**Branch:** —\n")
	if parent != "" {
		fmt.Fprintf(&b, "**Parent:** %s\n", parent)
	}
	if description != "" {
		fmt.Fprintf(&b, "\n## Goal\n\n%s\n", description)
	} else {
		b.WriteString("\n## Goal\n\n(to be filled)\n")
	}
	b.WriteString("\n## Acceptance Criteria\n\n- [ ] (to be filled)\n")
	fmt.Fprintf(&b, "\n## Log\n\n- %s: Created\n", date)
	return b.String()
}

// addTagToTaskFile reads a task file, finds the Tags line, and appends the tag.
// If no Tags line exists, one is inserted before the Branch line.
func addTagToTaskFile(path, tag string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimPrefix(line, "- ")
		if val, ok := extractFieldAny(trimmed, "Tags"); ok {
			lines[i] = "**Tags:** " + addTag(val, tag)
			found = true
			break
		}
	}

	if !found {
		// Insert Tags line before Branch line.
		for i, line := range lines {
			trimmed := strings.TrimPrefix(line, "- ")
			if _, ok := extractFieldAny(trimmed, "Branch"); ok {
				newLines := make([]string, 0, len(lines)+1)
				newLines = append(newLines, lines[:i]...)
				newLines = append(newLines, "**Tags:** "+tag)
				newLines = append(newLines, lines[i:]...)
				lines = newLines
				break
			}
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// addChildToSubtasks appends a child entry to the parent's ## Subtasks section.
// If no Subtasks section exists, one is created before ## Acceptance Criteria or ## Log.
func addChildToSubtasks(parentPath, childID, childSize, childTitle string) error {
	data, err := os.ReadFile(parentPath)
	if err != nil {
		return err
	}

	entry := fmt.Sprintf("- **%s** (%s) — %s", childID, childSize, childTitle)
	content := string(data)

	// Check if ## Subtasks section already exists.
	subtasksIdx := strings.Index(content, "## Subtasks")
	if subtasksIdx != -1 {
		// Find end of Subtasks section (next ## heading or EOF).
		afterSubtasks := content[subtasksIdx+len("## Subtasks"):]
		nextSection := strings.Index(afterSubtasks, "\n## ")
		if nextSection == -1 {
			// Subtasks is last section — append at end.
			trimmed := strings.TrimRight(content, "\n")
			return os.WriteFile(parentPath, []byte(trimmed+"\n"+entry+"\n"), 0644)
		}
		insertPos := subtasksIdx + len("## Subtasks") + nextSection
		before := strings.TrimRight(content[:insertPos], "\n")
		after := content[insertPos:]
		return os.WriteFile(parentPath, []byte(before+"\n"+entry+"\n"+after), 0644)
	}

	// No Subtasks section — create one before ## Acceptance Criteria or ## Log.
	section := "\n## Subtasks\n\n" + entry + "\n"
	for _, marker := range []string{"## Acceptance Criteria", "## Log"} {
		idx := strings.Index(content, marker)
		if idx != -1 {
			before := strings.TrimRight(content[:idx], "\n")
			after := content[idx:]
			return os.WriteFile(parentPath, []byte(before+"\n"+section+"\n"+after), 0644)
		}
	}

	// Fallback: append at end.
	trimmed := strings.TrimRight(content, "\n")
	return os.WriteFile(parentPath, []byte(trimmed+"\n"+section), 0644)
}
