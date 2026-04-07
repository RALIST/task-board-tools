package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func cmdMove(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tb mv <ID> <status>\n\nExample: tb mv 123 ip\nAliases: b=backlog, ip=in-progress, r=review, d=done")
		os.Exit(1)
	}
	taskID := normalizeTaskID(args[0])
	targetStatus, err := resolveStatus(args[1])
	if err != nil {
		fatal("%v", err)
	}
	moveTask(taskID, targetStatus, "Moved to "+targetStatus)
}

func cmdStart(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tb start <ID>\n\nExample: tb start 123")
		os.Exit(1)
	}
	moveTask(normalizeTaskID(args[0]), "in-progress", "Started — moved to in-progress")
}

func cmdDone(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tb done <ID>\n\nExample: tb done 123")
		os.Exit(1)
	}
	taskID := normalizeTaskID(args[0])

	// Warn if completing an epic with open children.
	taskPath, findErr := findTask(cfg.BoardDir, taskID)
	if findErr == nil {
		t, parseErr := parseTaskFile(taskPath)
		if parseErr == nil && hasTag(t.Tags, "epic") {
			children := findChildren(cfg.BoardDir, taskID)
			var openChildren []string
			for _, c := range children {
				if c.Status != "done" {
					openChildren = append(openChildren, c.ID)
				}
			}
			if len(openChildren) > 0 {
				fmt.Fprintf(os.Stderr, "warning: %s is an epic with %d/%d children still open (%s)\n",
					taskID, len(openChildren), len(children), strings.Join(openChildren, ", "))
			}
		}
	}

	moveTask(taskID, "done", "Done")
}

func cmdClose(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tb close <ID>\n\nExample: tb close 123")
		os.Exit(1)
	}
	taskID := normalizeTaskID(args[0])

	boardDir := cfg.BoardDir

	lock, err := lockBoard(boardDir)
	if err != nil {
		fatal("%v", err)
	}
	defer lock.unlock()

	srcPath, err := findTask(boardDir, taskID)
	if err != nil {
		fatal("%v", err)
	}

	srcStatus := filepath.Base(filepath.Dir(srcPath))
	if err := os.Remove(srcPath); err != nil {
		fatal("cannot remove %s: %v", srcPath, err)
	}

	_ = regenerateBoard(boardDir)
	fmt.Printf("Closed %s (deleted from %s)\n", taskID, srcStatus)
}

// normalizeTaskID accepts "PREFIX-1170" or "1170" and returns "PREFIX-1170".
func normalizeTaskID(raw string) string {
	id := raw
	prefix := strings.ToUpper(cfg.Prefix)
	if !strings.HasPrefix(strings.ToUpper(id), prefix+"-") {
		id = cfg.Prefix + "-" + id
	}
	return strings.ToUpper(id)
}

// moveTask is the shared logic for moving a task between status directories.
// It acquires the board lock to prevent concurrent move races.
func moveTask(taskID, targetStatus, logMessage string) {
	boardDir := cfg.BoardDir

	lock, err := lockBoard(boardDir)
	if err != nil {
		fatal("%v", err)
	}
	defer lock.unlock()

	srcPath, err := findTask(boardDir, taskID)
	if err != nil {
		fatal("%v", err)
	}

	srcStatus := filepath.Base(filepath.Dir(srcPath))
	if srcStatus == targetStatus {
		fmt.Fprintf(os.Stderr, "%s is already in %s — nothing to do\n", taskID, targetStatus)
		os.Exit(0)
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			fatal("%s was moved or deleted by another process. Run `tb ls --status all` to locate it", taskID)
		}
		fatal("cannot read %s: %v", srcPath, err)
	}

	content := string(data)
	today := time.Now().Format("2006-01-02")
	logEntry := fmt.Sprintf("- %s: %s\n", today, logMessage)
	content = appendLogEntry(content, logEntry)

	destDir := filepath.Join(boardDir, targetStatus)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		fatal("cannot create directory %s: %v", destDir, err)
	}

	destPath := filepath.Join(destDir, taskID+".md")
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		fatal("cannot write %s: %v", destPath, err)
	}

	if err := os.Remove(srcPath); err != nil && !os.IsNotExist(err) {
		// Source already gone (concurrent delete) — destination is written, so the move succeeded.
		fmt.Fprintf(os.Stderr, "warning: source file already removed (concurrent operation), destination written successfully\n")
	}

	_ = regenerateBoard(boardDir)
	fmt.Printf("Moved %s from %s to %s\n", taskID, srcStatus, targetStatus)
}

// appendLogEntry inserts a log entry at the end of the ## Log section.
func appendLogEntry(content, entry string) string {
	// Find the ## Log section.
	logIdx := strings.Index(content, "## Log")
	if logIdx == -1 {
		// No log section — append one.
		return content + "\n## Log\n\n" + entry
	}

	// Find the end of the log section: either the next "## " heading or EOF.
	afterLog := content[logIdx+len("## Log"):]
	nextSection := strings.Index(afterLog, "\n## ")

	if nextSection == -1 {
		// Log is the last section — append at the very end.
		trimmed := strings.TrimRight(content, "\n")
		return trimmed + "\n" + entry
	}

	// Insert before the next section.
	insertPos := logIdx + len("## Log") + nextSection
	before := strings.TrimRight(content[:insertPos], "\n")
	after := content[insertPos:]
	return before + "\n" + entry + "\n" + after
}
