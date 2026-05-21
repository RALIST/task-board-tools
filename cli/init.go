package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

func cmdInit(args []string) {
	err := runInit(args, initRunOptions{
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		stdin:       os.Stdin,
		interactive: isTerminal(os.Stdin),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

type initRunOptions struct {
	stdout      io.Writer
	stderr      io.Writer
	stdin       io.Reader
	interactive bool
}

func (o initRunOptions) withDefaults() initRunOptions {
	if o.stdout == nil {
		o.stdout = os.Stdout
	}
	if o.stderr == nil {
		o.stderr = os.Stderr
	}
	if o.stdin == nil {
		o.stdin = os.Stdin
	}
	if _, ok := o.stdin.(*bufio.Reader); !ok {
		o.stdin = bufio.NewReader(o.stdin)
	}
	return o
}

func runInit(args []string, opts initRunOptions) error {
	opts = opts.withDefaults()
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(opts.stderr)
	boardPath := fs.String("board-path", "board", "relative path from root to board directory")
	prefix := fs.String("prefix", "PR", "task ID prefix (e.g., WS, PR)")
	installSkills := fs.String("install-skills", "auto", "project skill install: auto, all, claude, codex, or none")
	fs.Bool("refresh-docs", false, "compatibility flag: existing boards refresh generated docs by default, with .bak backups")

	fs.Usage = func() {
		fmt.Fprintf(opts.stderr, "Usage: tb init [path] [--board-path=board] [--prefix=PR] [--install-skills=auto|all|claude|codex|none] [--refresh-docs]\n\n")
		fmt.Fprintf(opts.stderr, "Initializes a task board, or reconciles an existing board by refreshing generated project files.\n\n")
		fmt.Fprintf(opts.stderr, "On existing boards, generated docs and the annotated .tb.yaml config surface are refreshed by default. Previous versions are saved as .bak files before overwrite.\n\n")
		fs.PrintDefaults()
	}

	// Reorder so positional arg can come before flags.
	reordered := reorderInitArgs(args)
	if err := fs.Parse(reordered); err != nil {
		return err
	}
	if err := validateProjectSkillMode(*installSkills); err != nil {
		return err
	}

	// Determine root directory: positional arg or CWD.
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	if fs.NArg() > 0 && fs.Arg(0) != "" {
		root, err = filepath.Abs(fs.Arg(0))
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
	}

	configFile := filepath.Join(root, configFileName)

	var configData []byte
	configValues := map[string]string{}
	configExists := false
	boardPathSet := false
	prefixSet := false
	installSkillsSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "board-path" {
			boardPathSet = true
		}
		if f.Name == "prefix" {
			prefixSet = true
		}
		if f.Name == "install-skills" {
			installSkillsSet = true
		}
	})

	// On re-init: read existing config as defaults, override with explicitly provided flags.
	if data, readErr := os.ReadFile(configFile); readErr == nil {
		configExists = true
		configData = data
		existing := parseSimpleYAML(data)
		for key, value := range existing {
			configValues[key] = value
		}
		// Use existing values as defaults for flags that were not explicitly set.
		if !boardPathSet && existing["board"] != "" {
			*boardPath = existing["board"]
		}
		if !prefixSet && existing["prefix"] != "" {
			*prefix = existing["prefix"]
		}
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("cannot read %s: %w", configFile, readErr)
	}

	boardDir := filepath.Join(root, *boardPath)

	// Check if already initialized.
	alreadyExists := false
	if _, statErr := os.Stat(filepath.Join(boardDir, ".next-id")); statErr == nil {
		alreadyExists = true
	}

	// Ensure status directories exist. MkdirAll is idempotent so it is safe to
	// run for both fresh inits and refreshes — important so adding a new
	// canonical column (e.g. `ready`) lands automatically on existing boards
	// the next time the user runs `tb init`.
	for _, dir := range allStatusDirs {
		p := filepath.Join(boardDir, dir)
		if err := os.MkdirAll(p, 0755); err != nil {
			return fmt.Errorf("cannot create %s: %w", p, err)
		}
	}

	if !alreadyExists {
		// Create .next-id starting at 1.
		if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("1\n"), 0644); err != nil {
			return fmt.Errorf("cannot create .next-id: %w", err)
		}

		// Generate initial BOARD.md.
		if err := regenerateBoard(boardDir); err != nil {
			return fmt.Errorf("cannot create BOARD.md: %w", err)
		}

		// Generate CONVENTIONS.md and SKILL.md templates.
		if _, err := refreshGeneratedDocs(boardDir, *prefix, *boardPath); err != nil {
			return fmt.Errorf("cannot create generated board docs: %w", err)
		}
	}

	var refreshResults []refreshedDoc
	if alreadyExists {
		var refreshErr error
		refreshResults, refreshErr = refreshGeneratedDocs(boardDir, *prefix, *boardPath)
		if refreshErr != nil {
			return fmt.Errorf("cannot refresh generated board docs: %w", refreshErr)
		}
	}

	// Write .tb.yaml.
	configChanged, configBackup := false, ""
	if configValues["board"] != *boardPath {
		configValues["board"] = *boardPath
		configChanged = true
	}
	if configValues["prefix"] != *prefix {
		configValues["prefix"] = *prefix
		configChanged = true
	}
	renderedConfig := renderConfigTemplate(configValues)
	if configExists && !configChanged && string(configData) != string(renderedConfig) {
		configChanged = true
	}
	if !configExists || configChanged {
		var writeErr error
		if configExists {
			configBackup, _, writeErr = writeFileWithBackup(configFile, renderedConfig)
		} else {
			writeErr = writeFileAtomic(configFile, renderedConfig, 0644)
			configChanged = true
		}
		if writeErr != nil {
			return fmt.Errorf("cannot write %s: %w", configFile, writeErr)
		}
	}

	if alreadyExists {
		fmt.Fprintf(opts.stderr, "Board already exists at %s\n", boardDir)
		if configChanged {
			fmt.Fprintf(opts.stdout, "Config updated: %s\n", configFile)
			if configBackup != "" {
				fmt.Fprintf(opts.stdout, "  backup: %s\n", filepath.Base(configBackup))
			}
		} else {
			fmt.Fprintf(opts.stdout, "Config already current: %s\n", configFile)
		}
		printRefreshResults(opts.stdout, boardDir, refreshResults)
	} else {
		fmt.Fprintf(opts.stdout, "Initialized board at %s\n", boardDir)
		fmt.Fprintf(opts.stdout, "Config saved to %s\n", configFile)
		fmt.Fprintln(opts.stdout, "\nCreated:")
		for _, dir := range statusDirs {
			fmt.Fprintf(opts.stdout, "  %s/\n", dir)
		}
		fmt.Fprintln(opts.stdout, "  .next-id")
		fmt.Fprintln(opts.stdout, "  BOARD.md")
		fmt.Fprintln(opts.stdout, "  CONVENTIONS.md")
		fmt.Fprintln(opts.stdout, "  SKILL.md")
	}

	targets, skippedSkills, err := resolveProjectSkillTargets(*installSkills, opts)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if skippedSkills {
			fmt.Fprintln(opts.stdout, "Skipped project task-board skills")
		}
		return nil
	}
	confirmSkillOverwrite := opts.interactive && shouldConfirmProjectSkillOverwrite(*installSkills, installSkillsSet)
	if err := installProjectSkills(root, *prefix, *boardPath, targets, opts, confirmSkillOverwrite); err != nil {
		return err
	}
	return nil
}

type projectSkillTarget struct {
	name    string
	relPath string
}

func resolveProjectSkillTargets(mode string, opts initRunOptions) ([]projectSkillTarget, bool, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "auto"
	}
	if mode == "auto" {
		if !opts.interactive {
			return nil, false, nil
		}
		fmt.Fprint(opts.stdout, "Install project-local task-board skills for Claude Code and Codex? [all/claude/codex/none] (default: all): ")
		line, err := readPromptLine(opts.stdin)
		if err != nil {
			return nil, false, err
		}
		mode = strings.ToLower(strings.TrimSpace(line))
		if mode == "" {
			mode = "all"
		}
	}

	switch mode {
	case "all", "both":
		return allProjectSkillTargets(), false, nil
	case "claude":
		return []projectSkillTarget{claudeProjectSkillTarget()}, false, nil
	case "codex":
		return []projectSkillTarget{codexProjectSkillTarget()}, false, nil
	case "none", "skip", "no":
		return nil, true, nil
	default:
		return nil, false, invalidProjectSkillModeError(mode)
	}
}

func validateProjectSkillMode(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return nil
	}
	switch mode {
	case "auto", "all", "both", "claude", "codex", "none", "skip", "no":
		return nil
	default:
		return invalidProjectSkillModeError(mode)
	}
}

func invalidProjectSkillModeError(mode string) error {
	return fmt.Errorf("invalid --install-skills value %q (want auto, all, claude, codex, or none)", mode)
}

func allProjectSkillTargets() []projectSkillTarget {
	return []projectSkillTarget{claudeProjectSkillTarget(), codexProjectSkillTarget()}
}

func claudeProjectSkillTarget() projectSkillTarget {
	return projectSkillTarget{name: "Claude Code", relPath: ".claude/skills/task-board/SKILL.md"}
}

func codexProjectSkillTarget() projectSkillTarget {
	return projectSkillTarget{name: "Codex", relPath: ".agents/skills/task-board/SKILL.md"}
}

func shouldConfirmProjectSkillOverwrite(mode string, flagSet bool) bool {
	if !flagSet {
		return true
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	return mode == "" || mode == "auto"
}

func installProjectSkills(root, prefix, boardPath string, targets []projectSkillTarget, opts initRunOptions, confirmCustomized bool) error {
	content := []byte(skillTemplate(strings.ToUpper(prefix), boardPath))
	results := make([]refreshedDoc, 0, len(targets))
	skipped := make([]string, 0)
	for _, target := range targets {
		path := filepath.Join(root, filepath.FromSlash(target.relPath))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("cannot create %s skill directory: %w", target.name, err)
		}
		existing, err := os.ReadFile(path)
		if err == nil && string(existing) != string(content) && confirmCustomized {
			fmt.Fprintf(opts.stdout, "Replace customized %s? [y/N]: ", target.relPath)
			line, readErr := readPromptLine(opts.stdin)
			if readErr != nil {
				return readErr
			}
			answer := strings.ToLower(strings.TrimSpace(line))
			if answer != "y" && answer != "yes" {
				fmt.Fprintf(opts.stdout, "Skipped %s\n", target.relPath)
				skipped = append(skipped, target.relPath)
				continue
			}
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cannot read %s: %w", path, err)
		}
		backupPath, changed, writeErr := writeFileWithBackup(path, content)
		if writeErr != nil {
			return writeErr
		}
		results = append(results, refreshedDoc{path: path, backupPath: backupPath, changed: changed})
	}
	printProjectSkillResults(opts.stdout, root, results, skipped)
	return nil
}

func printProjectSkillResults(out io.Writer, root string, results []refreshedDoc, skipped []string) {
	changed := false
	for _, result := range results {
		if result.changed {
			changed = true
			break
		}
	}
	if !changed && len(skipped) == 0 {
		fmt.Fprintln(out, "Project task-board skills already current")
		return
	}
	if changed {
		fmt.Fprintln(out, "Installed project task-board skills:")
		for _, result := range results {
			if !result.changed {
				continue
			}
			rel, err := filepath.Rel(root, result.path)
			if err != nil {
				rel = result.path
			}
			fmt.Fprintf(out, "  %s\n", filepath.ToSlash(rel))
			if result.backupPath != "" {
				fmt.Fprintf(out, "    backup: %s\n", filepath.Base(result.backupPath))
			}
		}
	}
	for _, rel := range skipped {
		fmt.Fprintf(out, "Skipped %s\n", rel)
	}
}

func readPromptLine(in io.Reader) (string, error) {
	reader, ok := in.(interface {
		ReadString(byte) (string, error)
	})
	if !ok {
		reader = bufio.NewReader(in)
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// reorderInitArgs separates --flag and --flag=value from positional args,
// putting flags first so flag.FlagSet.Parse works correctly.
func reorderInitArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--board-path" || arg == "--prefix" || arg == "--install-skills" {
			// Flag that takes a value as next arg.
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else if strings.HasPrefix(arg, "--board-path=") || strings.HasPrefix(arg, "--prefix=") || strings.HasPrefix(arg, "--install-skills=") {
			flags = append(flags, arg)
		} else if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}
