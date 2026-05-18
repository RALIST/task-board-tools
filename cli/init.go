package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	boardPath := fs.String("board-path", "board", "relative path from root to board directory")
	prefix := fs.String("prefix", "PR", "task ID prefix (e.g., WS, PR)")
	fs.Bool("refresh-docs", false, "compatibility flag: existing boards refresh generated docs by default, with .bak backups")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb init [path] [--board-path=board] [--prefix=PR] [--refresh-docs]\n\n")
		fmt.Fprintf(os.Stderr, "Initializes a task board, or reconciles an existing board by refreshing generated project files.\n\n")
		fmt.Fprintf(os.Stderr, "On existing boards, generated docs are refreshed by default and previous versions are saved as .bak files before overwrite.\n\n")
		fs.PrintDefaults()
	}

	// Reorder so positional arg can come before flags.
	reordered := reorderInitArgs(args)
	if err := fs.Parse(reordered); err != nil {
		os.Exit(1)
	}

	// Determine root directory: positional arg or CWD.
	root, err := os.Getwd()
	if err != nil {
		fatal("cannot determine working directory: %v", err)
	}
	if fs.NArg() > 0 && fs.Arg(0) != "" {
		root, err = filepath.Abs(fs.Arg(0))
		if err != nil {
			fatal("invalid path: %v", err)
		}
	}

	configFile := filepath.Join(root, configFileName)

	configValues := map[string]string{}
	configExists := false
	boardPathSet := false
	prefixSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "board-path" {
			boardPathSet = true
		}
		if f.Name == "prefix" {
			prefixSet = true
		}
	})

	// On re-init: read existing config as defaults, override with explicitly provided flags.
	if data, readErr := os.ReadFile(configFile); readErr == nil {
		configExists = true
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
		fatal("cannot read %s: %v", configFile, readErr)
	}

	boardDir := filepath.Join(root, *boardPath)

	// Check if already initialized.
	alreadyExists := false
	if _, statErr := os.Stat(filepath.Join(boardDir, ".next-id")); statErr == nil {
		alreadyExists = true
	}

	if !alreadyExists {
		// Create status directories + archive.
		for _, dir := range append(statusDirs, "archive") {
			p := filepath.Join(boardDir, dir)
			if err := os.MkdirAll(p, 0755); err != nil {
				fatal("cannot create %s: %v", p, err)
			}
		}

		// Create .next-id starting at 1.
		if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("1\n"), 0644); err != nil {
			fatal("cannot create .next-id: %v", err)
		}

		// Generate initial BOARD.md.
		if err := regenerateBoard(boardDir); err != nil {
			fatal("cannot create BOARD.md: %v", err)
		}

		// Generate CONVENTIONS.md and SKILL.md templates.
		if _, err := refreshGeneratedDocs(boardDir, *prefix, *boardPath); err != nil {
			fatal("cannot create generated board docs: %v", err)
		}
	}

	var refreshResults []refreshedDoc
	if alreadyExists {
		var refreshErr error
		refreshResults, refreshErr = refreshGeneratedDocs(boardDir, *prefix, *boardPath)
		if refreshErr != nil {
			fatal("cannot refresh generated board docs: %v", refreshErr)
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
	if !configExists || configChanged {
		var writeErr error
		if configExists {
			configBackup, _, writeErr = writeFileWithBackup(configFile, writeSimpleYAML(configValues))
		} else {
			writeErr = writeFileAtomic(configFile, writeSimpleYAML(configValues), 0644)
			configChanged = true
		}
		if writeErr != nil {
			fatal("cannot write %s: %v", configFile, writeErr)
		}
	}

	if alreadyExists {
		fmt.Fprintf(os.Stderr, "Board already exists at %s\n", boardDir)
		if configChanged {
			fmt.Printf("Config updated: %s\n", configFile)
			if configBackup != "" {
				fmt.Printf("  backup: %s\n", filepath.Base(configBackup))
			}
		} else {
			fmt.Printf("Config already current: %s\n", configFile)
		}
		printRefreshResults(boardDir, refreshResults)
	} else {
		fmt.Printf("Initialized board at %s\n", boardDir)
		fmt.Printf("Config saved to %s\n", configFile)
		fmt.Println("\nCreated:")
		for _, dir := range statusDirs {
			fmt.Printf("  %s/\n", dir)
		}
		fmt.Println("  .next-id")
		fmt.Println("  BOARD.md")
		fmt.Println("  CONVENTIONS.md")
		fmt.Println("  SKILL.md")
	}
}

// reorderInitArgs separates --flag and --flag=value from positional args,
// putting flags first so flag.FlagSet.Parse works correctly.
func reorderInitArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--board-path" || arg == "--prefix" {
			// Flag that takes a value as next arg.
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		} else if strings.HasPrefix(arg, "--board-path=") || strings.HasPrefix(arg, "--prefix=") {
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
