package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"
)

// defaultScanExtensions returns the default file extensions to scan for TODO comments.
func defaultScanExtensions() map[string]bool {
	return map[string]bool{
		".go": true, ".ts": true, ".svelte": true, ".js": true, ".tsx": true, ".jsx": true,
	}
}

// markerPattern matches untagged TODO/FIXME/HACK/WORKAROUND comments.
// It captures: (1) the marker keyword, (2) optional existing parenthetical, (3) the description.
// Examples:
//
//	// TODO: refactor this           → marker="TODO", paren="", desc="refactor this"
//	// FIXME(someone): broken         → marker="FIXME", paren="someone", desc="broken"
//	// HACK — workaround for X       → marker="HACK", paren="", desc="— workaround for X"
var markerPattern = regexp.MustCompile(`\b(TODO|FIXME|HACK|WORKAROUND)(\([^)]*\))?[:\s—-]+(.+)`)

// taskRefPattern matches an existing PREFIX-NNN reference. Lazily initialized in cmdScan().
var taskRefPattern *regexp.Regexp

type taskRef struct {
	hit todoHit
	id  int
}

type todoHit struct {
	File    string // absolute path
	RelFile string // relative to project root
	Line    int
	Marker  string // TODO, FIXME, etc.
	Desc    string // the comment text after the marker
	Module  string // inferred module
	Type    string // inferred type: bug or tech-debt
}

func cmdScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	apply := fs.Bool("apply", false, "create tasks and update source files (default: dry-run)")
	scanPath := fs.String("path", "", "limit scan to this directory (relative to project root)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tb scan [--apply] [--path dir]\n\n")
		fmt.Fprintf(os.Stderr, "Scans for untagged TODO/FIXME/HACK/WORKAROUND comments.\n")
		fmt.Fprintf(os.Stderr, "Default is dry-run — use --apply to create tasks and update comments.\n\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Initialize task reference pattern from configured prefix.
	taskRefPattern = regexp.MustCompile(regexp.QuoteMeta(cfg.Prefix) + `-\d+`)

	boardDir := cfg.BoardDir
	projectRoot := cfg.RootDir

	searchRoot := projectRoot
	if *scanPath != "" {
		searchRoot = filepath.Join(projectRoot, *scanPath)
		if _, err := os.Stat(searchRoot); err != nil {
			fatal("path %q does not exist relative to project root", *scanPath)
		}
	}

	hits := scanForTodos(searchRoot, projectRoot)

	if len(hits) == 0 {
		fmt.Println("No untagged TODO/FIXME/HACK/WORKAROUND comments found.")
		return
	}

	if !*apply {
		// Dry-run: just show what would be created.
		fmt.Printf("Found %d untagged comment(s) — run with --apply to create tasks and update source:\n\n", len(hits))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, h := range hits {
			desc := h.Desc
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			fmt.Fprintf(w, "  %s:%d\t%s\t%s\tT:%s\tM:%s\n", h.RelFile, h.Line, h.Marker, desc, h.Type, h.Module)
		}
		w.Flush()
		return
	}

	// Apply: create tasks and update source files.
	lock, err := lockBoard(boardDir)
	if err != nil {
		fatal("%v", err)
	}
	defer lock.unlock()

	var refs []taskRef

	for _, h := range hits {
		id, err := allocateID(boardDir)
		if err != nil {
			fatal("%v", err)
		}

		taskID := fmt.Sprintf("%s-%d", cfg.Prefix, id)
		today := fmt.Sprintf("%s", currentDate())
		content := buildScanTaskContent(id, h, today)

		filename := fmt.Sprintf("%s.md", taskID)
		destPath := filepath.Join(boardDir, "backlog", filename)
		if err := writeFileAtomic(destPath, []byte(content), 0644); err != nil {
			fatal("cannot write %s: %v", destPath, err)
		}

		refs = append(refs, taskRef{hit: h, id: id})
		fmt.Printf("  %s:%d  %s: %s → %s\n", h.RelFile, h.Line, h.Marker, truncate(h.Desc, 50), taskID)
	}

	// Update source files — group by file for efficiency.
	fileHits := make(map[string][]taskRef)
	for _, r := range refs {
		fileHits[r.hit.File] = append(fileHits[r.hit.File], r)
	}
	for file, tasks := range fileHits {
		if err := updateSourceFile(file, tasks); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not update %s: %v\n", file, err)
		}
	}

	_ = regenerateBoard(boardDir)
	fmt.Printf("\nCreated %d task(s) and updated source comments.\n", len(refs))
}

// scanForTodos walks the directory tree and finds untagged TODO/FIXME/HACK/WORKAROUND comments.
func scanForTodos(searchRoot, projectRoot string) []todoHit {
	var hits []todoHit

	filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories, node_modules, vendor, .claude/board.
		name := info.Name()
		if info.IsDir() {
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "build" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(name)
		if !cfg.ScanExtensions[ext] {
			return nil
		}

		fileHits := scanFile(path, projectRoot)
		hits = append(hits, fileHits...)
		return nil
	})

	return hits
}

// scanFile reads a single file and returns untagged TODO hits.
func scanFile(path, projectRoot string) []todoHit {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	relFile, _ := filepath.Rel(projectRoot, path)
	module := inferModule(relFile)

	var hits []todoHit
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip lines that already have a WS-NNN reference.
		if taskRefPattern != nil && taskRefPattern.MatchString(line) {
			continue
		}

		loc := markerPattern.FindStringSubmatchIndex(line)
		if loc == nil {
			continue
		}

		// Only match markers inside comments (// or /* or # or <!-- before the marker).
		before := line[:loc[0]]
		if !strings.Contains(before, "//") && !strings.Contains(before, "/*") &&
			!strings.Contains(before, "#") && !strings.Contains(before, "<!--") {
			continue
		}

		matches := markerPattern.FindStringSubmatch(line)
		marker := matches[1]
		desc := strings.TrimSpace(matches[3])

		taskType := "tech-debt"
		if marker == "FIXME" {
			taskType = "bug"
		}

		hits = append(hits, todoHit{
			File:    path,
			RelFile: relFile,
			Line:    lineNum,
			Marker:  marker,
			Desc:    desc,
			Module:  module,
			Type:    taskType,
		})
	}

	return hits
}

// inferModule guesses the module from the relative file path.
func inferModule(relPath string) string {
	parts := strings.Split(filepath.ToSlash(relPath), "/")

	// internal/{module}/... → module
	for i, p := range parts {
		if p == "internal" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	// frontend/src/lib/modules/{module}/... → module
	for i, p := range parts {
		if p == "modules" && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	// api/... → api
	if len(parts) > 0 && parts[0] == "api" {
		return "api"
	}

	return ""
}

// buildScanTaskContent creates the task markdown for a scanned TODO.
func buildScanTaskContent(id int, h todoHit, date string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s-%d: %s\n\n", cfg.Prefix, id, h.Desc)
	fmt.Fprintf(&b, "**Type:** %s\n", h.Type)
	b.WriteString("**Priority:** P2\n")
	b.WriteString("**Size:** S\n")
	if h.Module != "" {
		fmt.Fprintf(&b, "**Module:** %s\n", h.Module)
	}
	b.WriteString("**Branch:** —\n")
	fmt.Fprintf(&b, "\n## Goal\n\nResolve %s at `%s:%d`.\n", h.Marker, h.RelFile, h.Line)
	b.WriteString("\n## Acceptance Criteria\n\n- [ ] (to be filled)\n")
	fmt.Fprintf(&b, "\n## Log\n\n- %s: Created by `tb scan` from %s comment\n", date, h.Marker)
	return b.String()
}

// updateSourceFile replaces TODO markers in a file with WS-NNN tagged versions.
func updateSourceFile(path string, tasks []taskRef) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	for _, t := range tasks {
		lineIdx := t.hit.Line - 1 // 0-indexed
		if lineIdx < 0 || lineIdx >= len(lines) {
			continue
		}

		line := lines[lineIdx]
		taskID := fmt.Sprintf("%s-%d", cfg.Prefix, t.id)

		// Replace "TODO:" with "TODO(PREFIX-123):", "FIXME:" with "FIXME(PREFIX-123):", etc.
		// Also handle "TODO —", "TODO -", "TODO :" variants.
		old := t.hit.Marker
		replacement := fmt.Sprintf("%s(%s)", old, taskID)

		// Handle existing parenthetical: TODO(someone) → TODO(WS-123)
		parenPattern := regexp.MustCompile(regexp.QuoteMeta(old) + `(\([^)]*\))?`)
		lines[lineIdx] = parenPattern.ReplaceAllString(line, replacement)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func currentDate() string {
	return time.Now().Format("2006-01-02")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
