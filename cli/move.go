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
		fmt.Fprintln(os.Stderr, "Usage: tb mv <ID> <status>\n\nExample: tb mv 123 ip\nAliases: b=backlog, ip=in-progress, d=done")
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

	// Warn if WIP limit reached.
	if tasks, err := collectTasks(cfg.BoardDir, "in-progress"); err == nil && len(tasks) >= cfg.WipLimit {
		fmt.Fprintf(os.Stderr, "warning: WIP limit reached (%d/%d tasks in progress)\n", len(tasks), cfg.WipLimit)
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
			children := findActiveChildren(cfg.BoardDir, taskID)
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
	archiveTask(cfg.BoardDir, normalizeTaskID(args[0]))
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

var renameTaskPath = os.Rename

type taskMoveResult struct {
	TaskID       string
	SrcStatus    string
	TargetStatus string
	SrcPath      string
	DestPath     string
	Noop         bool
}

// moveTask is the shared command wrapper for moving a task between status
// directories. It acquires the board lock through moveTaskOnBoard.
func moveTask(taskID, targetStatus, logMessage string) {
	result, err := moveTaskOnBoard(cfg.BoardDir, taskID, targetStatus, logMessage)
	if err != nil {
		fatal("%v", err)
	}
	if result.Noop {
		fmt.Fprintf(os.Stderr, "%s is already in %s — nothing to do\n", taskID, targetStatus)
		os.Exit(0)
	}
	fmt.Printf("Moved %s from %s to %s\n", taskID, result.SrcStatus, result.TargetStatus)
}

func archiveTaskOnBoard(boardDir, taskID string) (taskMoveResult, error) {
	return moveTaskOnBoardWithLog(boardDir, taskID, "archive", func(srcStatus string) string {
		return "Closed (archived from " + srcStatus + ")"
	})
}

func moveTaskOnBoard(boardDir, taskID, targetStatus, logMessage string) (taskMoveResult, error) {
	return moveTaskOnBoardWithLog(boardDir, taskID, targetStatus, func(string) string {
		return logMessage
	})
}

func moveTaskOnBoardWithLog(boardDir, taskID, targetStatus string, logMessage func(srcStatus string) string) (taskMoveResult, error) {
	lock, err := lockBoard(boardDir)
	if err != nil {
		return taskMoveResult{}, err
	}
	defer lock.unlock()

	srcRef, noop, err := resolveMoveSource(boardDir, taskID, targetStatus)
	if err != nil {
		return taskMoveResult{}, err
	}

	result := taskMoveResult{
		TaskID:       taskID,
		SrcStatus:    srcRef.Status,
		TargetStatus: targetStatus,
		SrcPath:      ownerPathFromTaskPath(srcRef.Path),
		Noop:         noop,
	}
	if noop {
		return result, nil
	}

	destDir := filepath.Join(boardDir, targetStatus)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return result, fmt.Errorf("cannot create directory %s: %w", destDir, err)
	}
	if err := ensureMoveDestinationFree(boardDir, targetStatus, taskID); err != nil {
		return result, err
	}

	data, err := os.ReadFile(srcRef.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return result, fmt.Errorf("%s was moved or deleted by another process. Run `tb ls --status all` to locate it", taskID)
		}
		return result, fmt.Errorf("cannot read %s: %w", srcRef.Path, err)
	}

	today := time.Now().Format("2006-01-02")
	content := appendLogEntry(string(data), fmt.Sprintf("- %s: %s\n", today, logMessage(srcRef.Status)))
	if err := writeFileAtomic(srcRef.Path, []byte(content), 0644); err != nil {
		return result, fmt.Errorf("cannot write %s: %w", srcRef.Path, err)
	}

	if isFolderTaskPath(srcRef.Path) {
		if err := removeDualFormSibling(boardDir, srcRef.Status, taskID); err != nil {
			return result, err
		}
		result.DestPath = taskFolderPath(boardDir, targetStatus, taskID)
		if err := renameTaskPath(result.SrcPath, result.DestPath); err != nil {
			return result, fmt.Errorf("cannot rename task directory %s -> %s: %w", result.SrcPath, result.DestPath, err)
		}
	} else {
		result.DestPath = taskFilePath(boardDir, targetStatus, taskID)
		if err := renameTaskPath(srcRef.Path, result.DestPath); err != nil {
			return result, fmt.Errorf("cannot rename task file %s -> %s: %w", srcRef.Path, result.DestPath, err)
		}
	}

	if err := regenerateBoard(boardDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not regenerate BOARD.md: %v\n", err)
	}
	return result, nil
}

func resolveMoveSource(boardDir, taskID, targetStatus string) (taskRef, bool, error) {
	refs, err := resolveTaskRefsForID(boardDir, taskID, allStatusDirs)
	if err != nil {
		return taskRef{}, false, err
	}
	if len(refs) == 0 {
		return taskRef{}, false, fmt.Errorf("task %s not found in any directory (backlog, in-progress, done, archive). Verify the ID with `tb ls --status all`", taskID)
	}

	var sourceRefs []taskRef
	targetExists := false
	for _, ref := range refs {
		if ref.Status == targetStatus {
			targetExists = true
			continue
		}
		sourceRefs = append(sourceRefs, ref)
	}

	if targetExists && len(sourceRefs) > 0 {
		return taskRef{}, false, destinationCollisionError(boardDir, targetStatus, taskID)
	}
	if len(sourceRefs) == 0 {
		return refs[0], true, nil
	}
	if len(sourceRefs) > 1 {
		paths := make([]string, 0, len(sourceRefs))
		for _, ref := range sourceRefs {
			paths = append(paths, ref.Path)
		}
		return taskRef{}, false, fmt.Errorf("task %s exists in multiple source statuses (%s); refusing ambiguous move", taskID, strings.Join(paths, ", "))
	}
	return sourceRefs[0], false, nil
}

func ensureMoveDestinationFree(boardDir, targetStatus, taskID string) error {
	if taskDestinationExists(boardDir, targetStatus, taskID) {
		return destinationCollisionError(boardDir, targetStatus, taskID)
	}
	return nil
}

func taskDestinationExists(boardDir, status, taskID string) bool {
	for _, path := range taskDestinationPaths(boardDir, status, taskID) {
		if _, err := os.Lstat(path); err == nil {
			return true
		}
	}
	return false
}

func destinationCollisionError(boardDir, status, taskID string) error {
	var existing []string
	for _, path := range taskDestinationPaths(boardDir, status, taskID) {
		if _, err := os.Lstat(path); err == nil {
			existing = append(existing, path)
		}
	}
	if len(existing) == 0 {
		existing = append(existing, taskFilePath(boardDir, status, taskID), taskFolderPath(boardDir, status, taskID))
	}
	return fmt.Errorf("destination %s already contains task %s (%s); refusing to overwrite or merge", status, taskID, strings.Join(existing, ", "))
}

func taskDestinationPaths(boardDir, status, taskID string) []string {
	return []string{
		taskFilePath(boardDir, status, taskID),
		taskFolderPath(boardDir, status, taskID),
	}
}

func removeDualFormSibling(boardDir, status, taskID string) error {
	sibling := taskFilePath(boardDir, status, taskID)
	if err := os.Remove(sibling); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove dual-form source file %s before moving folder task: %w", sibling, err)
	}
	return nil
}

// appendLogEntry inserts a log entry at the end of the ## Log section.
func appendLogEntry(content, entry string) string {
	logSection, ok := findTaskSection(content, "## Log")
	if !ok {
		// No log section — append one.
		return content + "\n## Log\n\n" + entry
	}

	before := strings.TrimRight(content[:logSection.end], "\n")
	after := content[logSection.end:]
	return before + "\n" + entry + "\n" + after
}
