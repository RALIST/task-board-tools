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
		fmt.Fprintln(os.Stderr, "Usage: tb mv <ID> <status>\n\nExample: tb mv 123 ip\nAliases: b=backlog, r=ready, ip=in-progress, cr=code-review (review), d=done")
		os.Exit(1)
	}
	taskID := normalizeTaskID(args[0])
	targetStatus, err := resolveStatus(args[1])
	if err != nil {
		fatal("%v", err)
	}
	// Pre-flight WIP check for fast-fail UX; the authoritative recheck
	// inside the lock is wired through the guard below so concurrent
	// invocations cannot race past `len(dest) < limit`.
	if err := enforceWipLimit(targetStatus, cfg.BoardDir); err != nil {
		fatal("%v", err)
	}
	moveTaskWithGuard(taskID, targetStatus, "Moved to "+targetStatus, wipLimitGuard())
}

func cmdStart(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tb start <ID>\n\nExample: tb start 123")
		os.Exit(1)
	}
	taskID := normalizeTaskID(args[0])

	// Push-vs-pull warning: skipping ready violates canonical kanban. Keep
	// the source status warning soft (stderr) so existing workflows still
	// work — the user can always opt out by promoting via `tb ready` first.
	if ref, err := resolveTaskRef(cfg.BoardDir, taskID, allStatusDirs); err == nil {
		if ref.Status == "backlog" {
			fmt.Fprintf(os.Stderr, "warning: %s is in backlog — `tb start` is push-style; canonical kanban pulls from ready. Promote it first with `tb ready %s`, or use `tb pull` to pull the next groomed task.\n", taskID, taskID)
		}
	}

	if err := enforceWipLimit("in-progress", cfg.BoardDir); err != nil {
		fatal("%v", err)
	}

	moveTaskWithGuard(taskID, "in-progress", "Started — moved to in-progress", wipLimitGuard())
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
	moveTaskWithGuard(taskID, targetStatus, logMessage, nil)
}

// moveTaskWithGuard is moveTask with an in-lock invariant. Used by
// commands that want a destination-WIP, expected-source, or any other
// rule re-checked under .board.lock so concurrent CLI/agent invocations
// cannot race a pre-flight gate.
func moveTaskWithGuard(taskID, targetStatus, logMessage string, guard moveGuardFunc) {
	logFn := func(string) string { return logMessage }
	result, err := moveTaskOnBoardWithGuard(cfg.BoardDir, taskID, targetStatus, guard, logFn)
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

// moveGuardFunc is a command-specific invariant the move pipeline must
// re-validate AFTER taking the board lock but BEFORE any filesystem
// mutation. Use it to lift expected-source-status checks (e.g. `tb ready`
// only accepts backlog) or destination WIP enforcement into the lock so
// concurrent CLI/agent invocations cannot race the gate.
//
// guard returns nil to allow the move; any other error aborts and is
// returned to the caller without touching disk.
type moveGuardFunc func(boardDir, taskID, srcStatus, targetStatus string) error

func moveTaskOnBoardWithLog(boardDir, taskID, targetStatus string, logMessage func(srcStatus string) string) (taskMoveResult, error) {
	return moveTaskOnBoardWithGuard(boardDir, taskID, targetStatus, nil, logMessage)
}

// moveTaskOnBoardWithGuard is moveTaskOnBoardWithLog with an in-lock guard.
// All command-specific gates (expected source, WIP, etc.) belong here so
// they can't TOCTOU around the board lock — callers that pre-check OUTSIDE
// the lock get the friendly fast-fail error message, but the authoritative
// check still runs serialized inside the lock.
func moveTaskOnBoardWithGuard(boardDir, taskID, targetStatus string, guard moveGuardFunc, logMessage func(srcStatus string) string) (taskMoveResult, error) {
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

	// TB-235: every entry into code-review must carry a non-placeholder
	// ReviewRef so reviewers know which branch/PR/commit/worktree to
	// inspect. Validation runs after the lock + source resolution and
	// before any filesystem mutation, so a rejection leaves the source
	// status, log history, tags, and review sections untouched. The
	// noop-already-in-code-review branch above skips this gate.
	if targetStatus == "code-review" {
		if err := ensureReviewRefForCodeReview(srcRef.Path, taskID); err != nil {
			return result, err
		}
	}

	if guard != nil {
		if err := guard(boardDir, taskID, srcRef.Status, targetStatus); err != nil {
			return result, err
		}
	}

	destDir := filepath.Join(boardDir, targetStatus)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return result, fmt.Errorf("cannot create directory %s: %w", destDir, err)
	}
	if err := ensureMoveDestinationFree(boardDir, targetStatus, taskID); err != nil {
		return result, err
	}

	// Read source first; log entry is appended after the rename succeeds so a
	// failed rename leaves the source content untouched.
	data, err := os.ReadFile(srcRef.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return result, fmt.Errorf("%s was moved or deleted by another process. Run `tb ls --status all` to locate it", taskID)
		}
		return result, fmt.Errorf("cannot read %s: %w", srcRef.Path, err)
	}

	var destTaskMarkdown string
	if isFolderTaskPath(srcRef.Path) {
		if err := cleanupOrphanFileFormSibling(boardDir, srcRef.Status, taskID); err != nil {
			return result, err
		}
		result.DestPath = taskFolderPath(boardDir, targetStatus, taskID)
		if err := renameTaskPath(result.SrcPath, result.DestPath); err != nil {
			return result, fmt.Errorf("cannot rename task directory %s -> %s: %w", result.SrcPath, result.DestPath, err)
		}
		destTaskMarkdown = taskFolderMarkdownPath(boardDir, targetStatus, taskID)
	} else {
		result.DestPath = taskFilePath(boardDir, targetStatus, taskID)
		if err := renameTaskPath(srcRef.Path, result.DestPath); err != nil {
			return result, fmt.Errorf("cannot rename task file %s -> %s: %w", srcRef.Path, result.DestPath, err)
		}
		destTaskMarkdown = result.DestPath
	}

	today := time.Now().Format("2006-01-02")
	content := appendLogEntry(string(data), fmt.Sprintf("- %s: %s\n", today, logMessage(srcRef.Status)))
	if err := writeFileAtomic(destTaskMarkdown, []byte(content), 0644); err != nil {
		return result, fmt.Errorf("cannot write %s: %w", destTaskMarkdown, err)
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
		return taskRef{}, false, fmt.Errorf("task %s not found in any directory (%s). Verify the ID with `tb ls --status all`", taskID, strings.Join(allStatusDirs, ", "))
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

// wipLimitGuard returns a moveGuardFunc that re-checks the destination's
// WIP limit while holding the board lock. Use it on every command path
// that wants WIP enforcement: pre-flight calls outside the lock stay for
// fast-fail UX, but this guard is the authoritative check.
func wipLimitGuard() moveGuardFunc {
	return func(boardDir, _ string, _, targetStatus string) error {
		return enforceWipLimit(targetStatus, boardDir)
	}
}

// expectedSourceGuard rejects the move when the resolved source status
// isn't in expected. Use it on push/pull commands that require a specific
// origin column so a concurrent move between the pre-flight check and the
// in-lock resolution cannot smuggle the task through the wrong path.
func expectedSourceGuard(expected ...string) moveGuardFunc {
	allowed := make(map[string]struct{}, len(expected))
	for _, s := range expected {
		allowed[s] = struct{}{}
	}
	return func(_ string, taskID, srcStatus, _ string) error {
		if _, ok := allowed[srcStatus]; ok {
			return nil
		}
		return fmt.Errorf("%s is no longer in %s (now in %s); aborting move to avoid an unintended transition", taskID, strings.Join(expected, "/"), srcStatus)
	}
}

// composeGuards runs each non-nil guard in order, returning the first
// error. Lets callers stack independent invariants (source + WIP) without
// hand-rolling a closure each time.
func composeGuards(guards ...moveGuardFunc) moveGuardFunc {
	return func(boardDir, taskID, srcStatus, targetStatus string) error {
		for _, g := range guards {
			if g == nil {
				continue
			}
			if err := g(boardDir, taskID, srcStatus, targetStatus); err != nil {
				return err
			}
		}
		return nil
	}
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

// appendLogEntry inserts a log entry at the end of the ## Log section.
// User-supplied fragments (titles, tag lists, edited labels) can flow into
// entries via callers like cmdEdit, so the entry is run through redactLine
// here to keep credential-like substrings out of the on-disk markdown and
// the regenerated BOARD.md. Purely system-generated entries match no
// patterns and pass through unchanged.
func appendLogEntry(content, entry string) string {
	entry = redactLine(entry)
	logSection, ok := findTaskSection(content, "## Log")
	if !ok {
		// No log section — append one.
		return content + "\n## Log\n\n" + entry
	}

	before := strings.TrimRight(content[:logSection.end], "\n")
	after := content[logSection.end:]
	return before + "\n" + entry + "\n" + after
}
