package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// ErrHeaderMutation is returned by EditTaskBody when the caller's new body
// would modify the header line (`# PREFIX-NNN: …`) or any of the metadata
// lines (the bold-field rows above the first blank line). Keeping the header
// + metadata block immutable is the contract that lets the frontend trust
// `tb show --json` for that data; structured changes go through `tb edit`.
var ErrHeaderMutation = errors.New("edit-body: header or metadata block changed; use EditTask for metadata")

// metadataScanCap bounds how far we look for the start of the body. It must
// be larger than any realistic metadata block; the CLI parser uses a similar
// cap (cli/task.go scans the first 20 lines for metadata).
const metadataScanCap = 30

// EditTaskBody overwrites a task `.md` file with newBody, after verifying the
// caller didn't touch the header or metadata block. Acquires `.board.lock`
// for the duration so no concurrent `tb …` mutation races the write. After
// the rename succeeds, runs `tb regenerate` to refresh BOARD.md.
//
// The contract (from docs/ARCHITECTURE.md → "GUI direct writes"):
//  1. Open .board.lock with LOCK_EX.
//  2. Read the existing file.
//  3. Reject if newBody's first metadataLineCap lines differ from disk.
//  4. Append `- YYYY-MM-DD: Edited body via GUI` to the ## Log section.
//  5. writeFileAtomic (temp + fsync + rename).
//  6. Release the lock.
//  7. exec `tb regenerate`.
func (b *BoardService) EditTaskBody(ctx context.Context, id, newBody string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}

	boardDir, err := b.resolveBoardDir(ctx)
	if err != nil {
		return err
	}

	lock, err := lockBoard(boardDir)
	if err != nil {
		return fmt.Errorf("edit-body: lock board: %w", err)
	}
	// released gates the defer so a manual unlock (required before calling
	// tb regenerate — see below) doesn't run twice.
	released := false
	defer func() {
		if !released {
			lock.unlock()
		}
	}()

	// Resolve the task path INSIDE the lock so a racing `tb mv` / `tb close`
	// can't move the file between lookup and read.
	taskPath, err := findTaskFile(boardDir, id)
	if err != nil {
		return err
	}

	current, err := os.ReadFile(taskPath)
	if err != nil {
		return fmt.Errorf("edit-body: read %s: %w", taskPath, err)
	}

	if !headerAndMetadataEqual(string(current), newBody) {
		return ErrHeaderMutation
	}

	withLog := appendBodyEditLog(newBody, time.Now())

	if err := writeFileAtomic(taskPath, []byte(withLog), 0o644); err != nil {
		return fmt.Errorf("edit-body: write %s: %w", taskPath, err)
	}

	// Release before invoking `tb regenerate`: the CLI takes the same flock
	// and would deadlock if we kept ours. The defer will skip via `released`.
	lock.unlock()
	released = true

	if err := c.Regenerate(ctx); err != nil {
		// The file write succeeded; only BOARD.md is stale.
		return fmt.Errorf("edit-body: written, but regenerate failed: %w", err)
	}
	return nil
}

// resolveBoardDir gets the active board dir from the settings service. We
// access it via the SettingsService that owns it; passing the dir directly
// avoids leaking a settings dependency into board service plumbing. For now
// SettingsService writes it on the BoardService via SetBoardDir.
func (b *BoardService) resolveBoardDir(ctx context.Context) (string, error) {
	_ = ctx
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.boardDir == "" {
		return "", ErrNoBoard
	}
	return b.boardDir, nil
}

// setBoardDir is called by SettingsService.OpenBoard when the active board
// changes. It records the directory used by direct-write paths (EditTaskBody).
//
// Intentionally unexported so Wails doesn't bind it to the frontend; a remote
// caller could otherwise retarget EditTaskBody at a different board than the
// active CLI client, then have the next exec'd `tb regenerate` clobber the
// wrong BOARD.md. The active board dir is owned by SettingsService.
func (b *BoardService) setBoardDir(dir string) {
	b.triageMu.Lock()
	defer b.triageMu.Unlock()

	b.mu.Lock()
	b.boardDir = dir
	b.mu.Unlock()

	b.triageCache = nil
	b.triageGen++
}

// findTaskFile searches the same status directories the CLI does, honoring
// both folder-form tasks (<status>/<ID>/TASK.md) and legacy file-form tasks
// (<status>/<ID>.md). Folder form wins when both exist, matching the CLI's
// resolveTaskRefInStatus precedence. Accepts either a fully-qualified ID
// ("TB-1") or a bare number ("1"); the latter is resolved by scanning each
// status directory for a matching folder or file entry. This mirrors
// cli/move.go:normalizeTaskID's tolerance.
func findTaskFile(boardDir, id string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(id))
	dirs := []string{"backlog", "in-progress", "done", "archive"}

	if strings.Contains(upper, "-") {
		for _, dir := range dirs {
			folder := filepath.Join(boardDir, dir, upper, "TASK.md")
			if info, err := os.Stat(folder); err == nil && !info.IsDir() {
				return folder, nil
			}
			file := filepath.Join(boardDir, dir, upper+".md")
			if info, err := os.Stat(file); err == nil && !info.IsDir() {
				return file, nil
			}
		}
		return "", ErrNotFound
	}

	// Bare number — search every status dir for a folder ending in `-<id>`
	// or a file ending in `-<id>.md`. Folder form wins on tie within a dir.
	folderSuffix := "-" + upper
	fileSuffix := "-" + upper + ".md"
	for _, dir := range dirs {
		entries, err := os.ReadDir(filepath.Join(boardDir, dir))
		if err != nil {
			continue
		}
		var folderHit, fileHit string
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if e.IsDir() && strings.HasSuffix(name, folderSuffix) {
				candidate := filepath.Join(boardDir, dir, name, "TASK.md")
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					folderHit = candidate
				}
				continue
			}
			if !e.IsDir() && strings.HasSuffix(name, fileSuffix) {
				fileHit = filepath.Join(boardDir, dir, name)
			}
		}
		if folderHit != "" {
			return folderHit, nil
		}
		if fileHit != "" {
			return fileHit, nil
		}
	}
	return "", ErrNotFound
}

// headerAndMetadataEqual returns true when the protected prefix (header line
// + metadata block) of `a` and `b` are byte-identical. The protected prefix
// ends at the start of the first body section heading (`## Goal`, etc.); if
// no body heading appears within metadataScanCap lines, the whole input is
// treated as protected.
func headerAndMetadataEqual(a, b string) bool {
	return protectedPrefix(a) == protectedPrefix(b)
}

// protectedPrefix returns the substring of s that EditTaskBody refuses to
// change: the title line, the bold-field metadata block, and the trailing
// blank line(s) up to (but not including) the first `## ` body heading.
func protectedPrefix(s string) string {
	scanned := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '\n' {
			continue
		}
		// Peek at the next line.
		next := i + 1
		if next >= len(s) {
			return s
		}
		// Skip any leading whitespace on that next line to find the marker.
		lineStart := next
		for lineStart < len(s) && (s[lineStart] == ' ' || s[lineStart] == '\t') {
			lineStart++
		}
		// First body heading is the boundary. The title is `# ` (one hash);
		// section headings are `## ` (two hashes). Stop at the first `## `.
		if lineStart+2 < len(s) && s[lineStart] == '#' && s[lineStart+1] == '#' && (s[lineStart+2] == ' ' || s[lineStart+2] == '\t') {
			return s[:next]
		}
		scanned++
		if scanned >= metadataScanCap {
			return s[:next]
		}
	}
	return s
}

// appendBodyEditLog inserts a log entry at the end of the ## Log section. If
// there is no Log section it creates one. Mirrors cli/move.go:appendLogEntry
// so the GUI's log entries look identical to the CLI's.
func appendBodyEditLog(content string, when time.Time) string {
	entry := fmt.Sprintf("- %s: Edited body via GUI", when.Format("2006-01-02"))

	logIdx := strings.Index(content, "## Log")
	if logIdx == -1 {
		trimmed := strings.TrimRight(content, "\n")
		return trimmed + "\n\n## Log\n\n" + entry + "\n"
	}

	after := content[logIdx+len("## Log"):]
	nextSection := strings.Index(after, "\n## ")
	if nextSection == -1 {
		// Log is last — append at the very end.
		trimmed := strings.TrimRight(content, "\n")
		return trimmed + "\n" + entry + "\n"
	}
	insertPos := logIdx + len("## Log") + nextSection
	before := strings.TrimRight(content[:insertPos], "\n")
	tail := content[insertPos:]
	return before + "\n" + entry + "\n" + tail
}

// --- file locking (matches cli/board.go) ---

type boardLockHandle struct {
	file *os.File
}

func (l *boardLockHandle) unlock() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
}

func lockBoard(boardDir string) (*boardLockHandle, error) {
	lockPath := filepath.Join(boardDir, ".board.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("flock LOCK_EX: %w", err)
	}
	return &boardLockHandle{file: f}, nil
}

// --- atomic writes (mirrors cli/atomicfs.go writeFileAtomic) ---

// writeFileAtomic must match the CLI's semantics: temp file in same dir,
// fsync, rename. Same invariant: lock-free readers (parser + watcher) either
// see the previous contents or the new contents, never a torn write.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return fmt.Errorf("writeFileAtomic: read random: %w", err)
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.tmp.%d.%s", base, os.Getpid(), hex.EncodeToString(rnd[:])))

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("writeFileAtomic: create temp: %w", err)
	}
	cleanup := func() { _ = os.Remove(tmp) }

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("writeFileAtomic: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("writeFileAtomic: fsync: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("writeFileAtomic: close: %w", err)
	}
	if err := os.Chmod(tmp, perm); err != nil {
		cleanup()
		return fmt.Errorf("writeFileAtomic: chmod: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return fmt.Errorf("writeFileAtomic: rename: %w", err)
	}
	return nil
}
