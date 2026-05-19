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

// createShellQuotingHelp explains the safe quoting recipes for task text that
// contains Markdown command spans. It is printed after the flag list by
// `tb create --help` so the user sees it next to the values they're trying to
// quote.
const createShellQuotingHelp = `
Markdown command spans:
  Backticks inside DOUBLE quotes are evaluated by your shell BEFORE tb runs.
  For example, in bash/zsh:

    tb create "Try ` + "`tb init`" + `"          # WRONG — shell runs ` + "`tb init`" + `

  Use SINGLE quotes (or escape) so the backticks reach tb literally:

    tb create 'Try ` + "`tb init`" + `' -d 'Run ` + "`tb --help`" + `'

  For multi-line bodies or richer Markdown, prefer a heredoc via tb edit so
  shell quoting cannot eat the contents:

    tb create 'My task title' -m mymod
    tb edit TB-123 --goal - <<'EOF'
    See ` + "`tb init`" + ` and ` + "`tb --help`" + ` for details.
    EOF
`

// flagsWithValue lists flags that consume the next argument. reorderArgs uses
// this to know which args after a flag belong to that flag versus being a
// positional. Bool flags (e.g. --json, --epic) are NOT in this map.
var flagsWithValue = map[string]bool{
	"-p": true, "-T": true, "-s": true, "-m": true, "-t": true, "-d": true, "-a": true,
	"--status": true, "--parent": true, "--agent": true, "--agent-status": true,
	"--goal": true, "--acceptance": true, "--user-attention": true, "--title": true,
	"--review-ref": true,
	// TB-237: per-mode attribution flags accepted by `tb edit`.
	"--groomed-by": true, "--groom-status": true,
	"--implemented-by": true, "--implement-status": true,
	"--reviewed-by": true, "--review-status": true,
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
	legacyFile := fs.Bool("legacy-file", false, "write legacy <status>/<ID>.md instead of folder-form <status>/<ID>/TASK.md")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb create \"Title\" -m module [-d desc] [-p P2] [-T feature] [-s M] [-t tags] [--status backlog] [--legacy-file]\n\n")
		fs.PrintDefaults()
		fmt.Fprint(os.Stderr, createShellQuotingHelp)
	}

	// Go's flag package stops at the first non-flag argument, so
	// `tb create "Title" -m mod` would lose all flags after the title.
	// Reorder args: pull positional args out, put flags first.
	reordered := reorderArgs(args)
	if err := fs.Parse(reordered); err != nil {
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: title is required\n\nUsage: tb create \"My task title\" -m module [-d \"description\"] [--legacy-file]")
		fmt.Fprint(os.Stderr, createShellQuotingHelp)
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
	// Mask credential-like substrings in every user-supplied free-text
	// field before it lands in the task file or BOARD.md. The narrow AC
	// only required `-d`, but a token pasted into a title, module name,
	// or tag list would still leak through the H1 / Module / Tags lines
	// (TB-203 review finding).
	title = redactLine(title)
	redactedModule := redactLine(*module)
	redactedTags := redactLine(*tags)
	redactedDescription := redactText(*description)
	content := buildTaskContent(id, title, *taskType, *priority, *size, redactedModule, redactedTags, redactedDescription, parentID, today, !*legacyFile)

	taskID := fmt.Sprintf("%s-%d", cfg.Prefix, id)
	destDir := filepath.Join(boardDir, targetStatus)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		fatal("cannot create directory %s: %v", destDir, err)
	}

	var destPath string
	if *legacyFile {
		destPath = filepath.Join(destDir, taskID+".md")
	} else {
		taskDir := filepath.Join(destDir, taskID)
		if err := os.Mkdir(taskDir, 0755); err != nil {
			fatal("cannot create task directory %s: %v", taskDir, err)
		}
		destPath = filepath.Join(taskDir, folderTaskFileName)
	}
	if err := writeFileAtomic(destPath, []byte(content), 0644); err != nil {
		fatal("cannot write %s: %v", destPath, err)
	}

	// Update parent's Subtasks section.
	if parentID != "" {
		parentPath, _ := findTask(boardDir, parentID)
		if subErr := addChildToSubtasks(parentPath, taskID, *size, title); subErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not update parent subtasks: %v\n", subErr)
		}
	}

	if err := regenerateBoard(boardDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not regenerate BOARD.md: %v\n", err)
	}

	fmt.Printf("Created %s\n", relPath(cfg.RootDir, destPath))
}

// reorderArgs separates flags and positional arguments so that all flags come
// first. This allows `tb create "Title" -m module` to work even though Go's
// flag package stops parsing at the first non-flag argument.
//
// A literal "--" terminates flag scanning: everything after it is positional
// in original order, even if it starts with "-". The terminator is emitted
// directly after the collected flags so the underlying FlagSet consumes it
// and exposes the post-terminator args as positionals. Without this, an
// attachment name like "-dash.txt" passed via `tb attach --rm ID -- -dash.txt`
// would be reshuffled into the flag region and misread as the task ID.
func reorderArgs(args []string) []string {
	var flags, positional, afterTerminator []string
	terminatorSeen := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if terminatorSeen {
			afterTerminator = append(afterTerminator, arg)
			continue
		}
		if arg == "--" {
			terminatorSeen = true
			continue
		}
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
	if !terminatorSeen {
		return append(flags, positional...)
	}
	result := make([]string, 0, len(flags)+1+len(positional)+len(afterTerminator))
	result = append(result, flags...)
	result = append(result, "--")
	result = append(result, positional...)
	result = append(result, afterTerminator...)
	return result
}

func buildTaskContent(id int, title, taskType, priority, size, module, tags, description, parent, date string, includeAttachments bool) string {
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
	if includeAttachments {
		b.WriteString("\n## Attachments\n")
	}
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

	return writeFileAtomic(path, []byte(strings.Join(lines, "\n")), 0644)
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

	if section, ok := findTaskSection(content, "## Subtasks"); ok {
		before := strings.TrimRight(content[:section.end], "\n")
		after := content[section.end:]
		return writeFileAtomic(parentPath, []byte(before+"\n"+entry+"\n"+after), 0644)
	}

	// No Subtasks section — create one before ## Acceptance Criteria or ## Log.
	section := "\n## Subtasks\n\n" + entry + "\n"
	if target, ok := findFirstTaskSection(content, []string{"## Acceptance Criteria", "## Log"}); ok {
		before := strings.TrimRight(content[:target.start], "\n")
		after := content[target.start:]
		return writeFileAtomic(parentPath, []byte(before+"\n"+section+"\n"+after), 0644)
	}

	// Fallback: append at end.
	trimmed := strings.TrimRight(content, "\n")
	return writeFileAtomic(parentPath, []byte(trimmed+"\n"+section), 0644)
}
