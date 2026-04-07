package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// statusDirs lists all valid status directories in the board.
var statusDirs = []string{"backlog", "in-progress", "done"}

// tbConfig holds the resolved per-project configuration.
type tbConfig struct {
	RootDir  string // absolute path to directory containing .tb.yaml
	BoardDir string // absolute path to board directory
	Prefix   string // task ID prefix (e.g., "PR", "WS")
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

			return tbConfig{
				RootDir:  dir,
				BoardDir: boardDir,
				Prefix:   prefix,
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
			RootDir:  cwd,
			BoardDir: env,
			Prefix:   prefix,
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

	next := fmt.Sprintf("%d\n", id+1)
	if err := os.WriteFile(path, []byte(next), 0644); err != nil {
		return 0, fmt.Errorf("cannot write .next-id: %w", err)
	}

	return id, nil
}

// findTask searches all status directories for a task file matching taskID.
// Returns the full path or an error if not found.
func findTask(boardDir, taskID string) (string, error) {
	filename := taskID + ".md"
	for _, status := range statusDirs {
		path := filepath.Join(boardDir, status, filename)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("task %s not found in any directory (backlog, in-progress, review, done). Verify the ID with `tb ls --status all -a`", taskID)
}

// relPath returns the relative path from base to target, falling back to the absolute path.
func relPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

// resolveStatus maps status aliases to canonical directory names.
func resolveStatus(input string) (string, error) {
	switch strings.ToLower(input) {
	case "b", "backlog":
		return "backlog", nil
	case "ip", "in-progress", "wip":
		return "in-progress", nil
	case "d", "done":
		return "done", nil
	default:
		return "", fmt.Errorf("unknown status %q — valid values: b (backlog), ip (in-progress), d (done)", input)
	}
}
