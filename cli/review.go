package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// cmdReview owns the code-review workflow surface:
//
//	tb review --submit  <ID>             move in-progress (or review-failed ready/backlog) task to code-review
//	tb review --target  <ID> file|-      replace ## Review Target
//	tb review --notes   <ID> file|-      replace ## Reviewer Notes
//	tb review --findings <ID> file|-     replace ## Review Findings
//	tb review --fail    <ID> file|-      write findings + move task to ready with review-failed tag
//
// Exactly one mode flag is required. Section flags read replacement content
// from a file path or "-" for stdin, mirroring `tb edit --goal`. Submit and
// section/fail writes acquire the board lock and regenerate BOARD.md.
func cmdReview(args []string) {
	fs := flag.NewFlagSet("review", flag.ExitOnError)
	submit := fs.Bool("submit", false, "submit an in-progress (or review-failed ready/backlog) task to code-review")
	targetPath := fs.String("target", "", "write ## Review Target from file path or - for stdin")
	notesPath := fs.String("notes", "", "write ## Reviewer Notes from file path or - for stdin")
	findingsPath := fs.String("findings", "", "write ## Review Findings from file path or - for stdin")
	failPath := fs.String("fail", "", "fail review: write findings from file|- and move task back to ready with review-failed tag")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  tb review --submit <ID>\n")
		fmt.Fprintf(os.Stderr, "  tb review --target <ID> file|-\n")
		fmt.Fprintf(os.Stderr, "  tb review --notes <ID> file|-\n")
		fmt.Fprintf(os.Stderr, "  tb review --findings <ID> file|-\n")
		fmt.Fprintf(os.Stderr, "  tb review --fail <ID> file|-\n\n")
		fs.PrintDefaults()
	}

	reordered := reorderArgs(args)
	if err := fs.Parse(reordered); err != nil {
		os.Exit(1)
	}

	// Exactly one mode must be selected. Section/fail modes take the source
	// path as the flag value, so a non-empty *Path implies the mode.
	modeCount := 0
	if *submit {
		modeCount++
	}
	if *targetPath != "" {
		modeCount++
	}
	if *notesPath != "" {
		modeCount++
	}
	if *findingsPath != "" {
		modeCount++
	}
	if *failPath != "" {
		modeCount++
	}
	if modeCount == 0 {
		fmt.Fprintln(os.Stderr, "error: one of --submit, --target, --notes, --findings, or --fail is required")
		fs.Usage()
		os.Exit(1)
	}
	if modeCount > 1 {
		fmt.Fprintln(os.Stderr, "error: only one of --submit, --target, --notes, --findings, --fail may be set")
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: task ID is required")
		fs.Usage()
		os.Exit(1)
	}
	taskID := normalizeTaskID(fs.Arg(0))

	switch {
	case *submit:
		if msg, err := reviewSubmit(taskID); err != nil {
			fatal("%v", err)
		} else {
			fmt.Println(msg)
		}
	case *targetPath != "":
		runReviewSection(taskID, *targetPath, "## Review Target", "review-target", "review target")
	case *notesPath != "":
		runReviewSection(taskID, *notesPath, "## Reviewer Notes", "reviewer-notes", "reviewer notes")
	case *findingsPath != "":
		runReviewSection(taskID, *findingsPath, "## Review Findings", "review-findings", "review findings")
	case *failPath != "":
		if msg, err := reviewFail(taskID, *failPath); err != nil {
			fatal("%v", err)
		} else {
			fmt.Println(msg)
		}
	}
}

func runReviewSection(taskID, sourcePath, heading, metaLabel, humanLabel string) {
	if msg, err := reviewWriteSection(taskID, sourcePath, heading, metaLabel, humanLabel); err != nil {
		fatal("%v", err)
	} else {
		fmt.Println(msg)
	}
}

// reviewSubmit moves a task into the code-review directory. It accepts
// in-progress tasks (the common path) and ready/backlog tasks tagged
// review-failed (resubmit after rework — `ready` is the canonical
// post-fail home, `backlog` accepted for backwards compatibility). Other
// source statuses are rejected.
//
// If the task lacks a ## Review Target section, a warning is emitted to
// stderr and the move proceeds (MVP behavior — TB-194 explicitly calls for a
// warning, not a hard fail).
func reviewSubmit(taskID string) (string, error) {
	boardDir := cfg.BoardDir

	ref, err := resolveTaskRef(boardDir, taskID, allStatusDirs)
	if err != nil {
		return "", err
	}

	t, err := parseTaskFile(ref.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}

	switch ref.Status {
	case "in-progress":
		// happy path
	case "ready", "backlog":
		if !hasTag(t.Tags, "review-failed") {
			return "", fmt.Errorf("tb review --submit only accepts in-progress tasks (or ready/backlog tasks tagged review-failed); %s is in %s without review-failed", taskID, ref.Status)
		}
		// Resubmit after rework: clear the marker on move.
	case "code-review":
		return fmt.Sprintf("%s is already in code-review — nothing to do", taskID), nil
	default:
		return "", fmt.Errorf("tb review --submit only accepts in-progress (or ready/backlog with review-failed); %s is in %s", taskID, ref.Status)
	}

	// TB-235: pre-flight ReviewRef check so a missing-ref submit leaves
	// the review-failed tag, log entries, and tags untouched on the source
	// task. This is NOT the authoritative gate — moveTaskOnBoardWithLog
	// re-validates inside the board lock so a TOCTOU edit between this
	// check and the move still rejects safely. The pre-flight is purely
	// for tag preservation: without it, reviewClearFailedMarker would run
	// before the move's gate fires and the rejection would still consume
	// the review-failed marker.
	if err := ensureReviewRefForCodeReview(ref.Path, taskID); err != nil {
		return "", err
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}
	if _, ok := findMarkdownSection(string(data), "## Review Target"); !ok {
		fmt.Fprintf(os.Stderr, "warning: %s has no ## Review Target section — set one with `tb review --target %s -` to tell reviewers where to inspect the change\n", taskID, taskID)
	}

	clearedMarker := false
	if (ref.Status == "ready" || ref.Status == "backlog") && hasTag(t.Tags, "review-failed") {
		if err := reviewClearFailedMarker(boardDir, ref); err != nil {
			return "", err
		}
		clearedMarker = true
	}

	result, err := moveTaskOnBoard(boardDir, taskID, "code-review", "Submitted to code-review")
	if err != nil {
		return "", err
	}
	if result.Noop {
		return fmt.Sprintf("%s is already in code-review — nothing to do", taskID), nil
	}
	if clearedMarker {
		return fmt.Sprintf("Submitted %s from %s to code-review (cleared review-failed)", taskID, result.SrcStatus), nil
	}
	return fmt.Sprintf("Submitted %s from %s to code-review", taskID, result.SrcStatus), nil
}

// reviewClearFailedMarker removes the `review-failed` tag from the task
// metadata. Called by reviewSubmit on backlog → code-review resubmit. The
// caller already holds knowledge that the tag is present; we re-read under
// the lock to avoid a TOCTOU race.
func reviewClearFailedMarker(boardDir string, ref taskRef) error {
	lock, err := lockBoard(boardDir)
	if err != nil {
		return err
	}
	defer lock.unlock()

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}

	lines := strings.Split(string(data), "\n")
	t, err := parseTaskFile(ref.Path)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %w", ref.Path, err)
	}

	newTags := removeTag(t.Tags, "review-failed")
	if newTags == t.Tags {
		return nil
	}
	if newTags == "" {
		lines = clearField(lines, "Tags")
	} else {
		lines = setField(lines, "Tags", newTags)
	}

	today := time.Now().Format("2006-01-02")
	content := strings.Join(lines, "\n")
	content = appendLogEntry(content, fmt.Sprintf("- %s: Cleared review-failed marker on resubmit\n", today))

	if err := writeFileAtomic(ref.Path, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", ref.Path, err)
	}
	return nil
}

// reviewWriteSection replaces (or inserts) a managed body section. label is
// used in error messages and the log entry. metaLabel is the short token
// appended to the log line so multiple section edits remain easy to grep.
func reviewWriteSection(taskID, sourcePath, heading, metaLabel, humanLabel string) (string, error) {
	body, err := readReviewBodyInput(sourcePath, humanLabel)
	if err != nil {
		return "", err
	}
	body = redactText(body)

	boardDir := cfg.BoardDir

	lock, err := lockBoard(boardDir)
	if err != nil {
		return "", err
	}
	defer lock.unlock()

	ref, err := resolveTaskRef(boardDir, taskID, allStatusDirs)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}
	content := upsertTaskSection(string(data), heading, body)

	today := time.Now().Format("2006-01-02")
	content = appendLogEntry(content, fmt.Sprintf("- %s: Edited %s\n", today, metaLabel))

	if err := writeFileAtomic(ref.Path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("cannot write %s: %w", ref.Path, err)
	}

	if err := cleanupOrphanFileFormSibling(boardDir, ref.Status, ref.ID); err != nil {
		return "", err
	}

	if err := regenerateBoard(boardDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not regenerate BOARD.md: %v\n", err)
	}

	return fmt.Sprintf("Updated %s: %s", taskID, metaLabel), nil
}

// reviewFail is the failure flow: write/replace ## Review Findings, move the
// task back to ready (canonical kanban — the task was already groomed, no
// re-triage needed), add the review-failed tag, regenerate BOARD.md.
// Rejects tasks not currently in code-review.
func reviewFail(taskID, sourcePath string) (string, error) {
	body, err := readReviewBodyInput(sourcePath, "review findings")
	if err != nil {
		return "", err
	}
	body = redactText(body)

	boardDir := cfg.BoardDir

	if err := reviewWriteFailMetadata(boardDir, taskID, body); err != nil {
		return "", err
	}

	result, err := moveTaskOnBoard(boardDir, taskID, "ready", "Failed code review — moved to ready with review-failed marker")
	if err != nil {
		return "", err
	}
	if result.Noop {
		return fmt.Sprintf("%s is already in ready — review-failed marker and findings were written, but no move occurred", taskID), nil
	}
	return fmt.Sprintf("Failed review for %s: moved %s -> ready with review-failed marker", taskID, result.SrcStatus), nil
}

// reviewWriteFailMetadata writes the findings section and adds the
// review-failed tag under the board lock. Separate from the move so the move
// helper handles the rename + log entry on the destination file.
func reviewWriteFailMetadata(boardDir, taskID, findingsBody string) error {
	lock, err := lockBoard(boardDir)
	if err != nil {
		return err
	}
	defer lock.unlock()

	ref, err := resolveTaskRef(boardDir, taskID, allStatusDirs)
	if err != nil {
		return err
	}
	if ref.Status != "code-review" {
		return fmt.Errorf("tb review --fail only accepts tasks in code-review; %s is in %s", taskID, ref.Status)
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}

	t, err := parseTaskFile(ref.Path)
	if err != nil {
		return fmt.Errorf("cannot parse %s: %w", ref.Path, err)
	}

	lines := strings.Split(string(data), "\n")
	newTags := addTag(t.Tags, "review-failed")
	if newTags != t.Tags {
		lines = setField(lines, "Tags", newTags)
	}

	// TB-268: a failed review returns the task to ready for rework. The
	// generic AgentStatus cursor reflected the prior implement run (often
	// `success`); leaving it set blocks auto-implement's retry-pickup
	// predicate, which requires a blank cursor. Per-mode attribution
	// (ImplementedBy / ImplementStatus / ReviewedBy / ReviewStatus) is
	// preserved so review history stays intact.
	lines = clearField(lines, "AgentStatus")

	content := upsertTaskSection(strings.Join(lines, "\n"), "## Review Findings", findingsBody)

	if err := writeFileAtomic(ref.Path, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", ref.Path, err)
	}
	return nil
}

// readReviewBodyInput reads a section replacement body from a file path or
// stdin ("-"), trimming surrounding blank lines and any leading duplicate
// heading. Empty input after trimming is an error: callers must produce
// content or skip the flag entirely.
func readReviewBodyInput(source, label string) (string, error) {
	if source == "" {
		return "", fmt.Errorf("missing source for %s — pass a file path or - for stdin", label)
	}

	var (
		data []byte
		err  error
	)
	if source == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(source)
	}
	if err != nil {
		return "", fmt.Errorf("cannot read %s content from %s: %w", label, source, err)
	}

	body := trimBlankLines(string(data))
	body = stripLeadingReviewHeading(body, label)
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("%s content is empty after trimming leading/trailing blank lines", label)
	}
	return body, nil
}

func stripLeadingReviewHeading(body, label string) string {
	heading := ""
	switch label {
	case "review target":
		heading = "## Review Target"
	case "reviewer notes":
		heading = "## Reviewer Notes"
	case "review findings":
		heading = "## Review Findings"
	}
	if heading == "" {
		return body
	}

	lines := strings.Split(body, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != heading {
		return body
	}
	return trimBlankLines(strings.Join(lines[1:], "\n"))
}

// ensureReviewRefForCodeReview parses the task at taskPath and rejects the
// caller when the **ReviewRef:** metadata line is missing, blank, or a
// placeholder (`—` / `-`). Shared by `tb mv <ID> code-review` and
// `tb review --submit <ID>` so both code-review entry paths enforce the
// TB-235 gate identically. The error names the precise CLI command needed
// to fix it so toasts and stderr messages can route the user there
// directly.
func ensureReviewRefForCodeReview(taskPath, taskID string) error {
	t, err := parseTaskFile(taskPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", taskPath, err)
	}
	if normalizeReviewRef(t.ReviewRef) == "" {
		return fmt.Errorf(
			"%s has no ReviewRef metadata — set one with `tb edit %s --review-ref <branch|PR URL|commit|worktree>` before moving to code-review",
			taskID, taskID,
		)
	}
	return nil
}

// removeTag drops tag from a comma-separated tags string (case-insensitive),
// preserving the order of remaining tags. Mirrors addTag's input format.
func removeTag(tags, tag string) string {
	if tags == "" {
		return ""
	}
	var kept []string
	for _, t := range strings.Split(tags, ",") {
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			continue
		}
		if strings.EqualFold(trimmed, tag) {
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, ",")
}
