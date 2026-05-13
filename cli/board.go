package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// statusDirs lists the "active" status directories in the board (everything
// except archive). Used as the default scope for collect helpers and as the
// expansion of `--status active`.
var statusDirs = []string{"backlog", "in-progress", "done"}

// allStatusDirs adds archive to the active set; this is the expansion of
// `--status all`.
var allStatusDirs = []string{"backlog", "in-progress", "done", "archive"}

// tbConfig holds the resolved per-project configuration.
type tbConfig struct {
	RootDir        string          // absolute path to directory containing .tb.yaml
	BoardDir       string          // absolute path to board directory
	Prefix         string          // task ID prefix (e.g., "PR", "WS")
	WipLimit       int             // max tasks in in-progress before warning (default: 2)
	ScanExtensions map[string]bool // file extensions to scan for TODOs
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

// writeSimpleYAML serializes a map to minimal YAML (sorted keys for determinism).
func writeSimpleYAML(values map[string]string) []byte {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\n", k, values[k])
	}
	return []byte(b.String())
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

			wipLimit := 2
			if wl, ok := values["wip_limit"]; ok {
				if n, err := strconv.Atoi(wl); err == nil && n > 0 {
					wipLimit = n
				}
			}

			scanExts := defaultScanExtensions()
			if se, ok := values["scan_extensions"]; ok && se != "" {
				scanExts = parseScanExtensions(se)
			}

			return tbConfig{
				RootDir:        dir,
				BoardDir:       boardDir,
				Prefix:         prefix,
				WipLimit:       wipLimit,
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
			ScanExtensions: defaultScanExtensions(),
		}, nil
	}

	return tbConfig{}, fmt.Errorf("board not found — run `tb init` to create .tb.yaml")
}

// isTaskFile returns true if the filename matches the configured prefix pattern (e.g., "WS-123.md").
func isTaskFile(name string) bool {
	return strings.HasPrefix(name, cfg.Prefix+"-") && strings.HasSuffix(name, ".md")
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
		f.Close()
		return nil, fmt.Errorf("cannot acquire board lock: %w", err)
	}
	return &boardLock{file: f}, nil
}

// unlock releases the board lock.
func (l *boardLock) unlock() {
	if l.file != nil {
		syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
		l.file.Close()
	}
}

// allocateID reads .next-id, returns the current value, and writes back
// the incremented value. Caller MUST hold the board lock.
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
	if err := os.WriteFile(path, []byte(next), 0644); err != nil {
		return 0, fmt.Errorf("cannot write .next-id: %w", err)
	}

	return id, nil
}

// idExists reports whether a task file for the given numeric id exists in any
// of the provided status directories. Used by allocateID to skip collisions.
func idExists(boardDir string, dirs []string, id int) bool {
	filename := fmt.Sprintf("%s-%d.md", cfg.Prefix, id)
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(boardDir, d, filename)); err == nil {
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
	filename := taskID + ".md"
	for _, status := range allStatusDirs {
		path := filepath.Join(boardDir, status, filename)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("task %s not found in any directory (backlog, in-progress, done, archive). Verify the ID with `tb ls --status all`", taskID)
}

// relPath returns the relative path from base to target, falling back to the absolute path.
func relPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

// resolveStatus maps a single status alias to the canonical concrete directory
// name. Used by commands that need a single target directory (`tb mv`,
// `tb create --status`). Multi-dir filters (active/all) are not valid here —
// use resolveStatusFilter for filtering use cases.
func resolveStatus(input string) (string, error) {
	switch strings.ToLower(input) {
	case "b", "backlog":
		return "backlog", nil
	case "ip", "in-progress", "wip":
		return "in-progress", nil
	case "d", "done":
		return "done", nil
	case "archive":
		return "archive", nil
	default:
		return "", fmt.Errorf("unknown status %q — valid values: backlog (b), in-progress (ip), done (d), archive", input)
	}
}

// resolveStatusFilter expands a `--status` filter input into the set of
// directories to scan. Aliases:
//
//	b / backlog          -> [backlog]
//	ip / wip / in-progress -> [in-progress]
//	d / done             -> [done]
//	archive              -> [archive]
//	active               -> [backlog, in-progress, done]
//	all                  -> [backlog, in-progress, done, archive]
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
			return nil, fmt.Errorf("unknown status %q — valid values: backlog (b), in-progress (ip), done (d), archive, active, all", input)
		}
		return []string{single}, nil
	}
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

// archiveTask moves a task file to the archive directory with a log entry.
func archiveTask(boardDir, taskID string) {
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

	data, err := os.ReadFile(srcPath)
	if err != nil {
		fatal("cannot read %s: %v", srcPath, err)
	}

	today := time.Now().Format("2006-01-02")
	content := appendLogEntry(string(data), fmt.Sprintf("- %s: Closed (archived from %s)\n", today, srcStatus))

	archiveDir := filepath.Join(boardDir, "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		fatal("cannot create archive directory: %v", err)
	}

	destPath := filepath.Join(archiveDir, taskID+".md")
	if err := writeFileAtomic(destPath, []byte(content), 0644); err != nil {
		fatal("cannot write %s: %v", destPath, err)
	}

	if err := os.Remove(srcPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "warning: source file already removed\n")
	}

	_ = regenerateBoard(boardDir)
	fmt.Printf("Closed %s (archived from %s)\n", taskID, srcStatus)
}
