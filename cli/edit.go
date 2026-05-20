package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type editChange struct {
	field string
	value string
	label string
}

type bodyEdit struct {
	heading string
	body    string
	label   string
}

func cmdEdit(args []string) {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	priority := fs.String("p", "", "priority (P0, P1, P2, P3)")
	taskType := fs.String("T", "", "type (feature, bug, tech-debt, improvement, spike)")
	size := fs.String("s", "", "size (S, M, L, XL)")
	module := fs.String("m", "", "module")
	tags := fs.String("t", "", "tags (comma-separated, replaces existing)")
	agent := fs.String("a", "", "agent (claude, codex)")
	agentStatus := fs.String("agent-status", "", "agent status (queued, running, success, failed, cancelled, interrupted, lost, needs-user, none)")
	reviewRef := fs.String("review-ref", "", "review reference (branch, PR URL, commit, worktree, or short ref); pass `none` to clear (TB-235)")
	// TB-237: per-mode attribution. Mirrors the (agent, status) pair shape
	// of `-a` / `--agent-status` but persisted on a separate **GroomedBy:**
	// / **GroomStatus:** / **ImplementedBy:** / … line each. Pass `none`
	// to clear the line.
	groomedBy := fs.String("groomed-by", "", "agent that ran groom mode (claude, codex, none)")
	groomStatus := fs.String("groom-status", "", "terminal status of last groom run (queued, running, success, failed, cancelled, interrupted, lost, needs-user, none)")
	implementedBy := fs.String("implemented-by", "", "agent that ran implement mode (claude, codex, none)")
	implementStatus := fs.String("implement-status", "", "terminal status of last implement run (queued, running, success, failed, cancelled, interrupted, lost, needs-user, none)")
	reviewedBy := fs.String("reviewed-by", "", "agent that ran review mode (claude, codex, none)")
	reviewStatus := fs.String("review-status", "", "terminal status of last review run (queued, running, success, failed, cancelled, interrupted, lost, needs-user, none)")
	title := fs.String("title", "", "task title (replaces the H1 header)")
	goalPath := fs.String("goal", "", "replace/insert ## Goal from file path or - for stdin")
	contextPath := fs.String("context", "", "replace/insert ## Context from file path or - for stdin")
	constraintsPath := fs.String("constraints", "", "replace/insert ## Constraints from file path or - for stdin")
	acceptancePath := fs.String("acceptance", "", "replace/insert ## Acceptance Criteria from file path or - for stdin")
	userAttentionPath := fs.String("user-attention", "", "replace/insert ## User Attention from file path or - for stdin")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb edit <ID> [-p P0] [-T feature] [-s M] [-m module] [-t tags] [-a claude] [--agent-status queued|running|success|failed|cancelled|interrupted|lost|needs-user|none] [--review-ref value|none] [--groomed-by claude|none] [--groom-status status|none] [--implemented-by claude|none] [--implement-status status|none] [--reviewed-by claude|none] [--review-status status|none] [--title \"New title\"] [--goal file|-] [--context file|-] [--constraints file|-] [--acceptance file|-] [--user-attention file|-]\n\n")
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
			fmt.Fprintf(os.Stderr, "error: invalid agent-status %q — use: queued, running, success, failed, cancelled, interrupted, lost, needs-user, none\n", *agentStatus)
			os.Exit(1)
		}
	}
	// TB-237: per-mode attribution flags share the same enums as `-a` /
	// `--agent-status` plus the `none` sentinel that clears the metadata
	// line. Mode labels appear in the validator error so callers see which
	// flag they got wrong.
	perModeAgents := []*struct {
		val  *string
		flag string
	}{
		{groomedBy, "groomed-by"},
		{implementedBy, "implemented-by"},
		{reviewedBy, "reviewed-by"},
	}
	for _, m := range perModeAgents {
		if *m.val == "" {
			continue
		}
		*m.val = strings.ToLower(*m.val)
		if *m.val != "none" && !validAgents[*m.val] {
			fmt.Fprintf(os.Stderr, "error: invalid %s %q — use: claude, codex, none\n", m.flag, *m.val)
			os.Exit(1)
		}
	}
	perModeStatuses := []*struct {
		val  *string
		flag string
	}{
		{groomStatus, "groom-status"},
		{implementStatus, "implement-status"},
		{reviewStatus, "review-status"},
	}
	for _, m := range perModeStatuses {
		if *m.val == "" {
			continue
		}
		*m.val = strings.ToLower(*m.val)
		if *m.val != "none" && !validAgentStatuses[*m.val] {
			fmt.Fprintf(os.Stderr, "error: invalid %s %q — use: queued, running, success, failed, cancelled, interrupted, lost, needs-user, none\n", m.flag, *m.val)
			os.Exit(1)
		}
	}
	// --review-ref takes a free-form value (branch, PR URL, commit, worktree
	// path…) or the literal `none` sentinel that clears the field. Reject
	// whitespace-only values for the same reason --title does: forcing
	// callers to either supply a real ref or omit the flag avoids writing a
	// blank metadata line that fails the move-to-code-review gate anyway.
	reviewRefProvided := false
	newReviewRef := ""
	clearReviewRef := false
	if *reviewRef != "" {
		trimmed := strings.TrimSpace(*reviewRef)
		if trimmed == "" {
			fmt.Fprintln(os.Stderr, "error: --review-ref must not be empty or whitespace (pass `none` to clear)")
			os.Exit(1)
		}
		if strings.EqualFold(trimmed, "none") {
			clearReviewRef = true
		} else {
			newReviewRef = redactLine(trimmed)
		}
		reviewRefProvided = true
	}
	// Whitespace-only --title is ambiguous; rejecting it forces callers
	// to either supply a real title or omit the flag.
	titleProvided := false
	newTitle := ""
	if *title != "" {
		newTitle = strings.TrimSpace(*title)
		if newTitle == "" {
			fmt.Fprintln(os.Stderr, "error: --title must not be empty or whitespace")
			os.Exit(1)
		}
		// Mask credential-like substrings in the title before it lands in
		// the H1 line / log entry (TB-203 review finding: prior fix only
		// covered -d / --goal / --acceptance bodies).
		newTitle = redactLine(newTitle)
		titleProvided = true
	}

	// Free-text metadata that flows into both **Field:** lines and log
	// entry labels gets redacted up front. Priority/Type/Size/Agent/
	// AgentStatus are enum-validated above, so they can't carry secrets.
	if *module != "" {
		*module = redactLine(*module)
	}
	if *tags != "" {
		*tags = redactLine(*tags)
	}

	// Collect metadata changes in flag order so stdout and log entries are stable.
	var changes []editChange
	if *priority != "" {
		changes = append(changes, editChange{field: "Priority", value: *priority, label: "priority=" + *priority})
	}
	if *taskType != "" {
		changes = append(changes, editChange{field: "Type", value: *taskType, label: "type=" + *taskType})
	}
	if *size != "" {
		changes = append(changes, editChange{field: "Size", value: *size, label: "size=" + *size})
	}
	if *module != "" {
		changes = append(changes, editChange{field: "Module", value: *module, label: "module=" + *module})
	}
	if *tags != "" {
		changes = append(changes, editChange{field: "Tags", value: *tags, label: "tags=" + *tags})
	}
	if *agent != "" {
		changes = append(changes, editChange{field: "Agent", value: *agent, label: "agent=" + *agent})
	}
	if *agentStatus != "" {
		changes = append(changes, editChange{field: "AgentStatus", value: *agentStatus, label: "agentstatus=" + *agentStatus})
	}
	// TB-237: per-mode attribution. The label uses kebab-case so the log
	// entry reads `groomed-by=claude, groom-status=success`.
	for _, m := range []struct {
		field string
		flag  string
		val   string
	}{
		{"GroomedBy", "groomed-by", *groomedBy},
		{"GroomStatus", "groom-status", *groomStatus},
		{"ImplementedBy", "implemented-by", *implementedBy},
		{"ImplementStatus", "implement-status", *implementStatus},
		{"ReviewedBy", "reviewed-by", *reviewedBy},
		{"ReviewStatus", "review-status", *reviewStatus},
	} {
		if m.val == "" {
			continue
		}
		changes = append(changes, editChange{field: m.field, value: m.val, label: m.flag + "=" + m.val})
	}
	if reviewRefProvided {
		if clearReviewRef {
			changes = append(changes, editChange{field: "ReviewRef", value: "none", label: "reviewref=none"})
		} else {
			changes = append(changes, editChange{field: "ReviewRef", value: newReviewRef, label: "reviewref=" + newReviewRef})
		}
	}

	stdinSources := 0
	if *goalPath == "-" {
		stdinSources++
	}
	if *contextPath == "-" {
		stdinSources++
	}
	if *constraintsPath == "-" {
		stdinSources++
	}
	if *acceptancePath == "-" {
		stdinSources++
	}
	if *userAttentionPath == "-" {
		stdinSources++
	}
	if stdinSources > 1 {
		fmt.Fprintln(os.Stderr, "error: only one of --goal, --context, --constraints, --acceptance, --user-attention may read from stdin (-); use files for the others")
		os.Exit(1)
	}

	var bodyEdits []bodyEdit
	if *goalPath != "" {
		body, err := readBodyEditInput(*goalPath, "goal")
		if err != nil {
			fatal("%v", err)
		}
		// Mask credential-like substrings in user-supplied body so a pasted
		// agent transcript can't write a real token into the task file or
		// the generated BOARD.md.
		body = redactText(body)
		bodyEdits = append(bodyEdits, bodyEdit{heading: "## Goal", body: body, label: "goal"})
	}
	if *contextPath != "" {
		body, err := readBodyEditInput(*contextPath, "context")
		if err != nil {
			fatal("%v", err)
		}
		body = redactText(body)
		bodyEdits = append(bodyEdits, bodyEdit{heading: "## Context", body: body, label: "context"})
	}
	if *constraintsPath != "" {
		body, err := readBodyEditInput(*constraintsPath, "constraints")
		if err != nil {
			fatal("%v", err)
		}
		body = redactText(body)
		bodyEdits = append(bodyEdits, bodyEdit{heading: "## Constraints", body: body, label: "constraints"})
	}
	if *acceptancePath != "" {
		body, err := readBodyEditInput(*acceptancePath, "acceptance")
		if err != nil {
			fatal("%v", err)
		}
		body = redactText(body)
		bodyEdits = append(bodyEdits, bodyEdit{heading: "## Acceptance Criteria", body: body, label: "acceptance"})
	}
	if *userAttentionPath != "" {
		body, err := readBodyEditInput(*userAttentionPath, "user-attention")
		if err != nil {
			fatal("%v", err)
		}
		body = redactText(body)
		bodyEdits = append(bodyEdits, bodyEdit{heading: "## User Attention", body: body, label: "user-attention"})
	}

	if len(changes) == 0 && len(bodyEdits) == 0 && !titleProvided && !reviewRefProvided {
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

	ref, err := resolveTaskRef(boardDir, taskID, allStatusDirs)
	if err != nil {
		fatal("%v", err)
	}
	taskPath := ref.Path

	data, err := os.ReadFile(taskPath)
	if err != nil {
		fatal("cannot read %s: %v", taskPath, err)
	}

	lines := strings.Split(string(data), "\n")

	// Apply a title rename if requested. If the new title matches the
	// existing one we treat the call as a no-op so callers can submit the
	// flag unconditionally without forcing a redundant write + log entry.
	titleApplied := false
	if titleProvided {
		updated, prevTitle, err := replaceTaskTitle(lines, newTitle)
		if err != nil {
			fatal("%v", err)
		}
		if prevTitle != newTitle {
			lines = updated
			titleApplied = true
		}
	}

	// Apply each metadata change.
	// Fields that accept the sentinel "none" to mean "clear the field" —
	// for these a value of "none" deletes the metadata line instead of
	// writing it. Every other field is set verbatim. The set tracks both
	// the legacy single Agent/AgentStatus pair and the TB-237 per-mode
	// pairs so the same `--<flag> none` UX clears each of them uniformly.
	clearable := map[string]bool{
		"Agent":           true,
		"AgentStatus":     true,
		"ReviewRef":       true,
		"GroomedBy":       true,
		"GroomStatus":     true,
		"ImplementedBy":   true,
		"ImplementStatus": true,
		"ReviewedBy":      true,
		"ReviewStatus":    true,
	}
	var applied []string
	for _, change := range changes {
		if change.value == "none" && clearable[change.field] {
			lines = clearField(lines, change.field)
		} else {
			lines = setField(lines, change.field, change.value)
		}
		applied = append(applied, change.label)
	}

	if titleApplied {
		applied = append(applied, "title="+newTitle)
	}

	content := strings.Join(lines, "\n")
	for _, edit := range bodyEdits {
		content = upsertTaskSection(content, edit.heading, edit.body)
		applied = append(applied, edit.label)
	}

	if len(applied) == 0 {
		// Title was supplied but matched the existing one — silent no-op.
		fmt.Printf("Updated %s: no changes (title unchanged)\n", taskID)
		return
	}

	// Append one combined log entry for metadata and body changes.
	today := time.Now().Format("2006-01-02")
	content = appendLogEntry(content, fmt.Sprintf("- %s: Edited %s\n", today, strings.Join(applied, ", ")))

	if err := writeFileAtomic(taskPath, []byte(content), 0644); err != nil {
		fatal("cannot write %s: %v", taskPath, err)
	}

	if err := cleanupOrphanFileFormSibling(boardDir, ref.Status, ref.ID); err != nil {
		fatal("%v", err)
	}

	if err := regenerateBoard(boardDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not regenerate BOARD.md: %v\n", err)
	}

	fmt.Printf("Updated %s: %s\n", taskID, strings.Join(applied, ", "))
}

func readBodyEditInput(source, label string) (string, error) {
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
	body = stripLeadingBodyHeading(body, label)
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("%s content is empty after trimming leading/trailing blank lines", label)
	}
	return body, nil
}

func trimBlankLines(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")

	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

func stripLeadingBodyHeading(body, label string) string {
	heading := ""
	switch label {
	case "goal":
		heading = "## Goal"
	case "context":
		heading = "## Context"
	case "constraints":
		heading = "## Constraints"
	case "acceptance":
		heading = "## Acceptance Criteria"
	case "user-attention":
		heading = "## User Attention"
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

func upsertTaskSection(content, heading, body string) string {
	if r, ok := findMarkdownSection(content, heading); ok {
		return content[:r.start] + markdownSectionBlock(heading, body) + content[r.end:]
	}

	switch heading {
	case "## Goal":
		if idx, ok := findFirstMarkdownHeading(content, []string{"## Context", "## Constraints", "## Acceptance Criteria", "## User Attention", "## Related Tasks", "## Attachments", "## Log"}); ok {
			return insertMarkdownSectionBefore(content, idx, markdownSectionBlock(heading, body))
		}
	case "## Context":
		if idx, ok := findFirstMarkdownHeading(content, []string{"## Constraints", "## Acceptance Criteria", "## User Attention", "## Review Target", "## Reviewer Notes", "## Review Findings", "## Related Tasks", "## Attachments", "## Log"}); ok {
			return insertMarkdownSectionBefore(content, idx, markdownSectionBlock(heading, body))
		}
	case "## Constraints":
		if idx, ok := findFirstMarkdownHeading(content, []string{"## Acceptance Criteria", "## User Attention", "## Review Target", "## Reviewer Notes", "## Review Findings", "## Related Tasks", "## Attachments", "## Log"}); ok {
			return insertMarkdownSectionBefore(content, idx, markdownSectionBlock(heading, body))
		}
	case "## Acceptance Criteria":
		if idx, ok := findFirstMarkdownHeading(content, []string{"## User Attention", "## Related Tasks", "## Attachments", "## Log"}); ok {
			return insertMarkdownSectionBefore(content, idx, markdownSectionBlock(heading, body))
		}
	case "## User Attention":
		// Place above Related Tasks / Attachments / Log so the ask is
		// visible immediately after Acceptance Criteria.
		if idx, ok := findFirstMarkdownHeading(content, []string{"## Review Target", "## Reviewer Notes", "## Review Findings", "## Related Tasks", "## Attachments", "## Log"}); ok {
			return insertMarkdownSectionBefore(content, idx, markdownSectionBlock(heading, body))
		}
	case "## Review Target", "## Reviewer Notes", "## Review Findings":
		// Review metadata sits between User Attention and Related Tasks
		// so reviewers see target/notes/findings before scrolling to the
		// log. Order within the triplet is preserved by anchoring each
		// section to the next existing one further down.
		anchors := map[string][]string{
			"## Review Target":   {"## Reviewer Notes", "## Review Findings", "## Related Tasks", "## Attachments", "## Log"},
			"## Reviewer Notes":  {"## Review Findings", "## Related Tasks", "## Attachments", "## Log"},
			"## Review Findings": {"## Related Tasks", "## Attachments", "## Log"},
		}
		if idx, ok := findFirstMarkdownHeading(content, anchors[heading]); ok {
			return insertMarkdownSectionBefore(content, idx, markdownSectionBlock(heading, body))
		}
	case "## Attachments":
		if idx, ok := findFirstMarkdownHeading(content, []string{"## Log"}); ok {
			return insertMarkdownSectionBefore(content, idx, markdownSectionBlock(heading, body))
		}
	}

	return appendMarkdownSection(content, markdownSectionBlock(heading, body))
}

type markdownSectionRange struct {
	start int
	end   int
}

var taskMarkdownHeadings = map[string]bool{
	"## Goal":                true,
	"## Context":             true,
	"## Constraints":         true,
	"## Subtasks":            true,
	"## Acceptance Criteria": true,
	"## User Attention":      true,
	"## Review Target":       true,
	"## Reviewer Notes":      true,
	"## Review Findings":     true,
	"## Related Tasks":       true,
	"## Attachments":         true,
	"## Log":                 true,
}

func findMarkdownSection(content, heading string) (markdownSectionRange, bool) {
	offset := 0
	inFence := false
	for offset <= len(content) {
		lineEnd, nextOffset := markdownLineBounds(content, offset)
		line := strings.TrimSpace(strings.TrimSuffix(content[offset:lineEnd], "\r"))
		if isMarkdownFence(line) {
			inFence = !inFence
		} else if !inFence && line == heading {
			nextHeading := findNextTaskMarkdownHeading(content, nextOffset)
			if nextHeading == -1 {
				nextHeading = len(content)
			}
			return markdownSectionRange{start: offset, end: nextHeading}, true
		}
		if nextOffset > len(content) {
			break
		}
		offset = nextOffset
	}
	return markdownSectionRange{}, false
}

func findFirstMarkdownHeading(content string, headings []string) (int, bool) {
	wanted := map[string]bool{}
	for _, heading := range headings {
		wanted[heading] = true
	}

	offset := 0
	inFence := false
	for offset <= len(content) {
		lineEnd, nextOffset := markdownLineBounds(content, offset)
		line := strings.TrimSpace(strings.TrimSuffix(content[offset:lineEnd], "\r"))
		if isMarkdownFence(line) {
			inFence = !inFence
		} else if !inFence && wanted[line] {
			return offset, true
		}
		if nextOffset > len(content) {
			break
		}
		offset = nextOffset
	}
	return 0, false
}

func findNextTaskMarkdownHeading(content string, offset int) int {
	inFence := false
	for offset <= len(content) {
		lineEnd, nextOffset := markdownLineBounds(content, offset)
		line := strings.TrimSpace(strings.TrimSuffix(content[offset:lineEnd], "\r"))
		if isMarkdownFence(line) {
			inFence = !inFence
		} else if !inFence && taskMarkdownHeadings[line] {
			return offset
		}
		if nextOffset > len(content) {
			break
		}
		offset = nextOffset
	}
	return -1
}

func isMarkdownFence(line string) bool {
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}

func markdownLineBounds(content string, offset int) (lineEnd, nextOffset int) {
	if offset >= len(content) {
		return len(content), len(content) + 1
	}
	if idx := strings.IndexByte(content[offset:], '\n'); idx != -1 {
		lineEnd = offset + idx
		return lineEnd, lineEnd + 1
	}
	return len(content), len(content) + 1
}

func markdownSectionBlock(heading, body string) string {
	return heading + "\n\n" + body + "\n\n"
}

func insertMarkdownSectionBefore(content string, idx int, block string) string {
	before := strings.TrimRight(content[:idx], "\n")
	after := content[idx:]
	if before == "" {
		return block + after
	}
	return before + "\n\n" + block + after
}

func appendMarkdownSection(content string, block string) string {
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return strings.TrimRight(block, "\n") + "\n"
	}
	return trimmed + "\n\n" + strings.TrimRight(block, "\n") + "\n"
}

func metadataRange(lines []string) (int, int) {
	limit := len(lines)
	if limit > maxMetadataLines {
		limit = maxMetadataLines
	}

	start := 0
	if start < limit && strings.HasPrefix(strings.TrimSpace(lines[start]), "# ") {
		start++
	}
	for start < limit && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	end := start
	for end < limit {
		line := strings.TrimSpace(lines[end])
		// Metadata ends at any body heading; section replacement uses a
		// whitelisted heading set because user content may contain ## examples.
		if line == "" || strings.HasPrefix(line, "## ") {
			break
		}
		end++
	}
	return start, end
}

func insertLine(lines []string, idx int, line string) []string {
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:idx]...)
	result = append(result, line)
	result = append(result, lines[idx:]...)
	return result
}

// replaceTaskTitle rewrites the H1 header (`# PREFIX-N: <title>`) with newTitle,
// preserving the ID. Returns the updated lines and the previous title (after
// trimming surrounding whitespace) so callers can detect no-op renames. The
// returned slice shares storage with the input.
func replaceTaskTitle(lines []string, newTitle string) ([]string, string, error) {
	headerIdx := -1
	for i := 0; i < len(lines) && i < maxMetadataLines; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "# "+cfg.Prefix+"-") {
			headerIdx = i
			break
		}
	}
	if headerIdx == -1 {
		return lines, "", fmt.Errorf("cannot rename: task header line not found in first %d lines", maxMetadataLines)
	}

	id, prev, malformed := parseTaskHeader(lines[headerIdx])
	if malformed || id == "" {
		return lines, "", fmt.Errorf("cannot rename: malformed task header on line %d", headerIdx+1)
	}

	lines[headerIdx] = "# " + id + ": " + newTitle
	return lines, prev, nil
}

// clearField removes the metadata line for `field` from lines (if present).
// Used by `tb edit -a none` and `tb edit --agent-status none` to drop a
// field rather than overwrite it with a sentinel value.
func clearField(lines []string, field string) []string {
	start, end := metadataRange(lines)
	for i := start; i < end; i++ {
		line := lines[i]
		trimmed := strings.TrimPrefix(line, "- ")
		if _, ok := extractFieldAny(trimmed, field); ok {
			return append(lines[:i], lines[i+1:]...)
		}
	}
	return lines
}

// setField replaces **Field:** value in lines, or inserts it before **Branch:** if missing.
func setField(lines []string, field, value string) []string {
	start, end := metadataRange(lines)
	for i := start; i < end; i++ {
		line := lines[i]
		trimmed := strings.TrimPrefix(line, "- ")
		if _, ok := extractFieldAny(trimmed, field); ok {
			lines[i] = "**" + field + ":** " + value
			return lines
		}
	}

	// Field not found — insert before Branch line.
	newLine := "**" + field + ":** " + value
	for i := start; i < end; i++ {
		line := lines[i]
		trimmed := strings.TrimPrefix(line, "- ")
		if _, ok := extractFieldAny(trimmed, "Branch"); ok {
			return insertLine(lines, i, newLine)
		}
	}

	// No Branch line — insert after last metadata line (first blank line after header).
	return insertLine(lines, end, newLine)
}
