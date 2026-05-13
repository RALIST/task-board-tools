package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func cmdShow(args []string) {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit task as JSON ({metadata, body}) instead of raw markdown")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb show <ID> [--json]\n\nExample: tb show 123\n         tb show 123 --json\n\n")
		fs.PrintDefaults()
	}

	// reorderArgs lets --json appear before or after the positional ID.
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

	boardDir := cfg.BoardDir

	path, err := findTask(boardDir, taskID)
	if err != nil {
		fatal("%v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fatal("cannot read %s: %v", path, err)
	}

	if *jsonOut {
		if err := emitShowJSON(path, data); err != nil {
			fatal("%v", err)
		}
		return
	}

	fmt.Print(string(data))
}

func cmdOpen(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tb open <ID>\n\nExample: tb open 123")
		os.Exit(1)
	}
	taskID := normalizeTaskID(args[0])

	boardDir := cfg.BoardDir

	path, err := findTask(boardDir, taskID)
	if err != nil {
		fatal("%v", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", path)
	default:
		fatal("unsupported OS: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		fatal("cannot open %s: %v", path, err)
	}

	fmt.Printf("Opened %s\n", taskID)
}
