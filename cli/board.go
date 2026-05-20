package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// statusDirs lists the active board directories in canonical kanban order.
// `ready` is the commitment column: tasks that have been groomed in backlog
// and pulled forward for upcoming work, but are not yet being actively
// worked. Archive is intentionally not active: it is a closed/hidden status
// for explicit inspection, not a synonym for done. `code-review` sits
// between in-progress and done and represents implementation work awaiting
// reviewer signoff.
var statusDirs = []string{"backlog", "ready", "in-progress", "code-review", "done"}

// allStatusDirs adds archive to the active set; this is the expansion of
// `--status all`.
var allStatusDirs = []string{"backlog", "ready", "in-progress", "code-review", "done", "archive"}

// tbConfig holds the resolved per-project configuration.
type tbConfig struct {
	RootDir        string          // absolute path to directory containing .tb.yaml
	BoardDir       string          // absolute path to board directory
	Prefix         string          // task ID prefix (e.g., "PR", "WS")
	WipLimit       int             // legacy: max in-progress tasks (mirrors WipLimits["in-progress"])
	WipLimits      map[string]int  // per-status WIP limits; missing/zero means no limit for that column
	WipEnforcement string          // "warn" (default) or "strict": strict blocks moves that would exceed the limit
	ScanExtensions map[string]bool // file extensions to scan for TODOs
}

// wipLimitConfigKey maps a status directory name to the flat YAML key used in
// .tb.yaml. Underscores are used in place of hyphens because the minimal YAML
// parser is flat key/value and hyphen-bearing keys would still work but look
// less idiomatic. Add to this map when introducing a new column that should
// support WIP limits.
var wipLimitConfigKey = map[string]string{
	"ready":       "wip_limit_ready",
	"in-progress": "wip_limit_in_progress",
	"code-review": "wip_limit_code_review",
}

// wipLimitFor returns the configured WIP limit for status, or (0, false) if
// the status is not WIP-limited in the current config.
func (c tbConfig) wipLimitFor(status string) (int, bool) {
	if c.WipLimits == nil {
		return 0, false
	}
	n, ok := c.WipLimits[status]
	if !ok || n <= 0 {
		return 0, false
	}
	return n, true
}

// cfg is the global configuration, set once by loadProjectConfig().
var cfg tbConfig

// configFileName is the per-project configuration file name.
const configFileName = ".tb.yaml"

// parseSimpleYAML parses a minimal key-value YAML format.
// It handles lines of the form "key: value", skipping empty lines and comments.
// Surrounding quotes (single or double) are stripped from values.
func parseSimpleYAML(data []byte) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip surrounding quotes.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		result[key] = val
	}
	return result
}

// loadProjectConfig walks up from the current working directory looking for .tb.yaml.
// If found, it parses the config and resolves the board path relative to the config file.
// Falls back to TB_BOARD_DIR env var if no .tb.yaml is found.
func loadProjectConfig() (tbConfig, error) {
	dir, err := os.Getwd()
	if err != nil {
		return tbConfig{}, fmt.Errorf("cannot determine working directory: %w", err)
	}

	// Walk up looking for .tb.yaml.
	for {
		candidate := filepath.Join(dir, configFileName)
		if _, statErr := os.Stat(candidate); statErr == nil {
			data, readErr := os.ReadFile(candidate)
			if readErr != nil {
				return tbConfig{}, fmt.Errorf("cannot read %s: %w", candidate, readErr)
			}
			values := parseSimpleYAML(data)

			boardPath := values["board"]
			if boardPath == "" {
				boardPath = "board"
			}

			prefix := strings.ToUpper(values["prefix"])
			if prefix == "" {
				prefix = "PR"
			}

			boardDir := boardPath
			if !filepath.IsAbs(boardDir) {
				boardDir = filepath.Join(dir, boardDir)
			}

			// Validate: board dir must contain .next-id.
			info, statErr := os.Stat(filepath.Join(boardDir, ".next-id"))
			if statErr != nil || info.IsDir() {
				return tbConfig{}, fmt.Errorf("board directory %s does not contain .next-id — run `tb init`", boardDir)
			}

			wipLimits, legacyLimit := parseWipLimits(values)

			enforcement := strings.ToLower(strings.TrimSpace(values["wip_enforcement"]))
			if enforcement != "strict" {
				enforcement = "warn"
			}

			scanExts := defaultScanExtensions()
			if se, ok := values["scan_extensions"]; ok && se != "" {
				scanExts = parseScanExtensions(se)
			}

			return tbConfig{
				RootDir:        dir,
				BoardDir:       boardDir,
				Prefix:         prefix,
				WipLimit:       legacyLimit,
				WipLimits:      wipLimits,
				WipEnforcement: enforcement,
				ScanExtensions: scanExts,
			}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fallback: TB_BOARD_DIR env var.
	if env := os.Getenv("TB_BOARD_DIR"); env != "" {
		info, statErr := os.Stat(filepath.Join(env, ".next-id"))
		if statErr != nil || info.IsDir() {
			return tbConfig{}, fmt.Errorf("TB_BOARD_DIR=%s does not contain .next-id", env)
		}
		// Use CWD as project root (best effort — no .tb.yaml to derive from).
		cwd, _ := os.Getwd()
		if cwd == "" {
			cwd = filepath.Dir(env)
		}
		prefix := strings.ToUpper(os.Getenv("TB_PREFIX"))
		if prefix == "" {
			prefix = "PR"
		}
		return tbConfig{
			RootDir:        cwd,
			BoardDir:       env,
			Prefix:         prefix,
			WipLimit:       2,
			WipLimits:      map[string]int{"in-progress": 2},
			WipEnforcement: "warn",
			ScanExtensions: defaultScanExtensions(),
		}, nil
	}

	return tbConfig{}, fmt.Errorf("board not found — run `tb init` to create .tb.yaml")
}

// isTaskFile returns true if the filename matches the configured prefix pattern (e.g., "WS-123.md").
func isTaskFile(name string) bool {
	return strings.HasPrefix(name, cfg.Prefix+"-") && strings.HasSuffix(name, ".md")
}

const folderTaskFileName = "TASK.md"

type taskRef struct {
	ID     string
	Status string
	Path   string
}

func taskIDFromFileName(name string) (string, bool) {
	if !isTaskFile(name) {
		return "", false
	}
	return strings.TrimSuffix(name, ".md"), true
}

func isTaskDirName(name string) bool {
	return !strings.HasPrefix(name, ".") &&
		strings.HasPrefix(name, cfg.Prefix+"-") &&
		!strings.HasSuffix(name, ".md")
}

func isFolderTaskPath(path string) bool {
	return filepath.Base(path) == folderTaskFileName
}

func ownerPathFromTaskPath(path string) string {
	if isFolderTaskPath(path) {
		return filepath.Dir(path)
	}
	return path
}

func taskFilePath(boardDir, status, taskID string) string {
	return filepath.Join(boardDir, status, taskID+".md")
}

func taskFolderPath(boardDir, status, taskID string) string {
	return filepath.Join(boardDir, status, taskID)
}

func taskFolderMarkdownPath(boardDir, status, taskID string) string {
	return filepath.Join(taskFolderPath(boardDir, status, taskID), folderTaskFileName)
}

func resolveTaskRefInStatus(boardDir, status, taskID string) (taskRef, bool, error) {
	folderPath := taskFolderMarkdownPath(boardDir, status, taskID)
	folderExists := false
	if info, err := os.Stat(folderPath); err == nil {
		if info.IsDir() {
			return taskRef{}, false, fmt.Errorf("folder-form task %s is a directory, expected markdown file", folderPath)
		}
		folderExists = true
	} else if !os.IsNotExist(err) {
		return taskRef{}, false, fmt.Errorf("cannot stat folder-form task %s: %w", folderPath, err)
	}

	filePath := taskFilePath(boardDir, status, taskID)
	fileExists := false
	if info, err := os.Stat(filePath); err == nil {
		if info.IsDir() {
			return taskRef{}, false, fmt.Errorf("file-form task %s is a directory, expected markdown file", filePath)
		}
		fileExists = true
	} else if !os.IsNotExist(err) {
		return taskRef{}, false, fmt.Errorf("cannot stat file-form task %s: %w", filePath, err)
	}

	if folderExists {
		if fileExists {
			warnDualForm(taskID, status, filePath, folderPath)
		}
		return taskRef{ID: taskID, Status: status, Path: folderPath}, true, nil
	}
	if fileExists {
		return taskRef{ID: taskID, Status: status, Path: filePath}, true, nil
	}
	return taskRef{}, false, nil
}

func resolveTaskRefsForID(boardDir, taskID string, statuses []string) ([]taskRef, error) {
	var refs []taskRef
	for _, status := range statuses {
		ref, ok, err := resolveTaskRefInStatus(boardDir, status, taskID)
		if err != nil {
			return nil, err
		}
		if ok {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

func discoverTaskRefs(boardDir string, statuses []string) ([]taskRef, error) {
	var refs []taskRef
	seen := make(map[string]taskRef)
	for _, status := range statuses {
		statusRefs, err := discoverTaskRefsInStatus(boardDir, status)
		if err != nil {
			return nil, err
		}
		for _, ref := range statusRefs {
			if prev, ok := seen[ref.ID]; ok {
				return nil, fmt.Errorf("task %s resolves to multiple canonical markdown paths in requested status scope: %s and %s", ref.ID, prev.Path, ref.Path)
			}
			seen[ref.ID] = ref
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

func discoverTaskRefsInStatus(boardDir, status string) ([]taskRef, error) {
	dirPath := filepath.Join(boardDir, status)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read status directory %s: %w", dirPath, err)
	}

	type seenEntry struct {
		folderPath string
		filePath   string
	}
	seen := make(map[string]*seenEntry)
	getEntry := func(id string) *seenEntry {
		if e, ok := seen[id]; ok {
			return e
		}
		e := &seenEntry{}
		seen[id] = e
		return e
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			if !isTaskDirName(name) {
				continue
			}
			taskPath := filepath.Join(dirPath, name, folderTaskFileName)
			info, err := os.Stat(taskPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("cannot stat folder-form task %s: %w", taskPath, err)
			}
			if info.IsDir() {
				return nil, fmt.Errorf("folder-form task %s is a directory, expected markdown file", taskPath)
			}
			getEntry(name).folderPath = taskPath
			continue
		}

		id, ok := taskIDFromFileName(name)
		if !ok {
			continue
		}
		getEntry(id).filePath = filepath.Join(dirPath, name)
	}

	refs := make([]taskRef, 0, len(seen))
	for id, e := range seen {
		switch {
		case e.folderPath != "" && e.filePath != "":
			warnDualForm(id, status, e.filePath, e.folderPath)
			refs = append(refs, taskRef{ID: id, Status: status, Path: e.folderPath})
		case e.folderPath != "":
			refs = append(refs, taskRef{ID: id, Status: status, Path: e.folderPath})
		case e.filePath != "":
			refs = append(refs, taskRef{ID: id, Status: status, Path: e.filePath})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		return numericID(refs[i].ID) < numericID(refs[j].ID)
	})
	return refs, nil
}

// dualFormWarned dedupes warnDualForm output per process. The same dual-form
// (taskID, status) tuple is discovered by both resolveTaskRefInStatus and
// discoverTaskRefsInStatus, and each `tb` invocation typically triggers both
// (a mutation calls resolveTaskRef, then regenerateBoard walks every status
// directory). Without dedupe a single dual-form ID can emit 6+ identical
// warnings during one command. Keyed on the absolute file path so different
// boards or temp dirs (in tests) do not suppress each other.
var dualFormWarned sync.Map

// warnDualForm logs a one-line stderr warning when a task is present in both
// file form (<status>/<ID>.md) and folder form (<status>/<ID>/TASK.md). This is
// a crash-recovery transient that can only arise from a process dying mid
// promotion (see docs/ARCHITECTURE.md "File → folder promotion"). The resolver
// prefers folder form; the next structured mutation removes the orphan file via
// cleanupOrphanFileFormSibling. Duplicate emissions within a single process are
// suppressed via dualFormWarned.
func warnDualForm(taskID, status, filePath, folderPath string) {
	if _, loaded := dualFormWarned.LoadOrStore(filePath, struct{}{}); loaded {
		return
	}
	fmt.Fprintf(os.Stderr, "warning: task %s in %s has both file-form (%s) and folder-form (%s); preferring folder form. The orphan file will be removed by the next structured mutation.\n", taskID, status, filePath, folderPath)
}

// cleanupOrphanFileFormSibling removes <status>/<ID>.md if a folder form
// markdown exists at <status>/<ID>/TASK.md. Idempotent and safe to call when
// the sibling does not exist. Callers MUST hold .board.lock.
func cleanupOrphanFileFormSibling(boardDir, status, taskID string) error {
	folderPath := taskFolderMarkdownPath(boardDir, status, taskID)
	if _, err := os.Stat(folderPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot stat folder-form task %s: %w", folderPath, err)
	}
	sibling := taskFilePath(boardDir, status, taskID)
	if err := os.Remove(sibling); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove dual-form file sibling %s: %w", sibling, err)
	}
	return nil
}

func resolveTaskRef(boardDir, taskID string, statuses []string) (taskRef, error) {
	refs, err := discoverTaskRefs(boardDir, statuses)
	if err != nil {
		return taskRef{}, err
	}
	for _, ref := range refs {
		if strings.EqualFold(ref.ID, taskID) {
			return ref, nil
		}
	}
	return taskRef{}, fmt.Errorf("task %s not found in requested status scope (%s). Verify the ID with `tb ls --status all`", taskID, strings.Join(statuses, ", "))
}

func parseTaskRef(ref taskRef, cwd string) (Task, error) {
	t, err := parseTaskFile(ref.Path)
	if err != nil {
		return Task{}, err
	}
	t.Status = ref.Status
	t.FilePath = relPath(cwd, ref.Path)
	return t, nil
}

func taskDirForRef(ref taskRef) string {
	if isFolderTaskPath(ref.Path) {
		return filepath.Dir(ref.Path)
	}
	return ""
}

// boardLock holds an exclusive file lock on .board.lock to serialize mutations.
// Use lockBoard/unlockBoard around any read-modify-write sequence.
type boardLock struct {
	file *os.File
}

// lockBoard acquires an exclusive lock on the board directory.
// Multiple concurrent tb processes (e.g., agents) will block here until the lock is free.
func lockBoard(boardDir string) (*boardLock, error) {
	lockPath := filepath.Join(boardDir, ".board.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("cannot open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return nil, fmt.Errorf("cannot acquire board lock: %w; also failed closing lock file: %w", err, closeErr)
		}
		return nil, fmt.Errorf("cannot acquire board lock: %w", err)
	}
	return &boardLock{file: f}, nil
}

// unlock releases the board lock. Unlock/close failures are only warnable at
// defer time, but they are still surfaced instead of silently ignored.
func (l *boardLock) unlock() {
	if l == nil || l.file == nil {
		return
	}
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to release board lock: %v\n", err)
	}
	if err := l.file.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to close board lock file: %v\n", err)
	}
	l.file = nil
}

// allocateID reads .next-id, returns the current value, and atomically writes
// back the incremented value. Caller MUST hold the board lock.
//
// If the candidate ID collides with an existing task file in any status
// directory (backlog, in-progress, done, archive), the ID is bumped forward
// until a free slot is found. A stale .next-id can happen when two concurrent
// tb processes race, or when the counter gets out of sync with directory
// state. Without this scan, a `tb create` call would silently overwrite
// existing task content.
func allocateID(boardDir string) (int, error) {
	path := filepath.Join(boardDir, ".next-id")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("cannot read .next-id: %w — is the board initialized? Run `tb init`", err)
	}

	id, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid .next-id content %q: %w — the file may be corrupted, expected a number", string(data), err)
	}

	// Scan all directories where a task file might already exist. statusDirs
	// intentionally excludes "archive"; extend locally so we catch closed tasks.
	searchDirs := append(append([]string{}, statusDirs...), "archive")
	original := id
	for idExists(boardDir, searchDirs, id) {
		id++
	}
	if id != original {
		fmt.Fprintf(os.Stderr, "WARN: .next-id was stale, bumped from %d to %d\n", original, id)
	}

	next := fmt.Sprintf("%d\n", id+1)
	if err := writeFileAtomic(path, []byte(next), 0644); err != nil {
		return 0, fmt.Errorf("cannot write .next-id: %w", err)
	}

	return id, nil
}

// idExists reports whether a task file for the given numeric id exists in any
// of the provided status directories. Used by allocateID to skip collisions.
func idExists(boardDir string, dirs []string, id int) bool {
	filename := fmt.Sprintf("%s-%d.md", cfg.Prefix, id)
	dirname := fmt.Sprintf("%s-%d", cfg.Prefix, id)
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(boardDir, d, filename)); err == nil {
			return true
		}
		if _, err := os.Stat(filepath.Join(boardDir, d, dirname)); err == nil {
			return true
		}
	}
	return false
}

// findTask searches all status directories (including archive) for a task
// file matching taskID. Returns the full path or an error if not found.
//
// Archive is included so that `tb mv <id> backlog` can resurrect an archived
// task — without this, archived tasks would be permanently unreachable
// through the normal command surface.
func findTask(boardDir, taskID string) (string, error) {
	path, err := findTaskInStatuses(boardDir, taskID, allStatusDirs)
	if err == nil {
		return path, nil
	}
	return "", fmt.Errorf("task %s not found in any directory (%s). Verify the ID with `tb ls --status all`", taskID, strings.Join(allStatusDirs, ", "))
}

func findTaskInStatuses(boardDir, taskID string, statuses []string) (string, error) {
	ref, err := resolveTaskRef(boardDir, taskID, statuses)
	if err != nil {
		return "", err
	}
	return ref.Path, nil
}

// relPath returns the relative path from base to target, falling back to the absolute path.
func relPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err == nil && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		return rel
	}

	if realBase, baseErr := filepath.EvalSymlinks(base); baseErr == nil {
		if realTarget, targetErr := filepath.EvalSymlinks(target); targetErr == nil {
			if realRel, realErr := filepath.Rel(realBase, realTarget); realErr == nil {
				return realRel
			}
		}
	}

	if err == nil {
		return rel
	}
	return target
}

func taskStatusFromPath(boardDir, taskPath string) string {
	rel, err := filepath.Rel(boardDir, taskPath)
	if err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) > 1 {
			return parts[0]
		}
	}
	return filepath.Base(filepath.Dir(taskPath))
}

// resolveStatus maps a single status alias to the canonical concrete directory
// name. Used by commands that need a single target directory (`tb mv`,
// `tb create --status`). Multi-dir filters (active/all) are not valid here —
// use resolveStatusFilter for filtering use cases.
func resolveStatus(input string) (string, error) {
	switch strings.ToLower(input) {
	case "b", "backlog":
		return "backlog", nil
	case "r", "ready":
		return "ready", nil
	case "ip", "in-progress", "wip":
		return "in-progress", nil
	case "cr", "code-review", "review":
		return "code-review", nil
	case "d", "done":
		return "done", nil
	case "archive":
		return "archive", nil
	default:
		return "", fmt.Errorf("unknown status %q — valid values: backlog (b), ready (r), in-progress (ip), code-review (cr/review), done (d), archive", input)
	}
}

// resolveStatusFilter expands a `--status` filter input into the set of
// directories to scan. Aliases:
//
//	b / backlog              -> [backlog]
//	r / ready                -> [ready]
//	ip / wip / in-progress   -> [in-progress]
//	cr / review / code-review -> [code-review]
//	d / done                 -> [done]
//	archive                  -> [archive]
//	active                   -> [backlog, ready, in-progress, code-review, done]
//	all                      -> [backlog, ready, in-progress, code-review, done, archive]
//
// Returned slice is safe to range over without further mutation.
func resolveStatusFilter(input string) ([]string, error) {
	switch strings.ToLower(input) {
	case "active":
		return append([]string{}, statusDirs...), nil
	case "all":
		return append([]string{}, allStatusDirs...), nil
	default:
		single, err := resolveStatus(input)
		if err != nil {
			return nil, fmt.Errorf("unknown status %q — valid values: backlog (b), ready (r), in-progress (ip), code-review (cr/review), done (d), archive, active, all", input)
		}
		return []string{single}, nil
	}
}

// parseWipLimits reads the WIP-limit keys from a parsed .tb.yaml map and
// returns the per-status limit map plus a legacy mirror of the in-progress
// limit. Precedence: a per-status `wip_limit_*` value wins over the legacy
// scalar `wip_limit` for that same column; `wip_limit` (legacy) seeds the
// in-progress slot when no explicit `wip_limit_in_progress` was set so old
// configs keep their behavior.
//
// An explicit `0` is honoured as "disabled" (wipLimitFor returns ok=false),
// matching the comment in the generated config template. Default
// in-progress limit is 2 when the user supplied NO `wip_limit` /
// `wip_limit_in_progress` key at all; other columns default to "no limit"
// until the user opts in.
func parseWipLimits(values map[string]string) (map[string]int, int) {
	limits := make(map[string]int)
	explicit := make(map[string]bool)

	if wl, ok := values["wip_limit"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(wl)); err == nil && n >= 0 {
			limits["in-progress"] = n
			explicit["in-progress"] = true
		}
	}

	for status, key := range wipLimitConfigKey {
		raw, ok := values[key]
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || n < 0 {
			continue
		}
		limits[status] = n
		explicit[status] = true
	}

	if !explicit["in-progress"] {
		limits["in-progress"] = 2
	}

	return limits, limits["in-progress"]
}

// parseScanExtensions parses a comma-separated list of file extensions.
func parseScanExtensions(s string) map[string]bool {
	exts := make(map[string]bool)
	for _, ext := range strings.Split(s, ",") {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		exts[ext] = true
	}
	return exts
}

// archiveTask moves a task to the archive directory with a log entry.
func archiveTask(boardDir, taskID string) {
	result, err := archiveTaskOnBoard(boardDir, taskID)
	if err != nil {
		fatal("%v", err)
	}
	if result.Noop {
		fmt.Fprintf(os.Stderr, "%s is already in archive — nothing to do\n", taskID)
		os.Exit(0)
	}
	fmt.Printf("Closed %s (archived from %s)\n", taskID, result.SrcStatus)
}
