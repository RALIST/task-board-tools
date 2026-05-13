// Package main: atomicfs is the only sanctioned write path for task .md files.
//
// Any callsite that mutates a task `.md` file MUST go through writeFileAtomic.
// The invariant exists so the GUI can parse task files without holding the
// board lock: temp+rename guarantees that a reader on the same filesystem
// either sees the previous content in full or the new content in full —
// never a torn or zero-length file.
//
// Enforcement: `grep -nE 'os\.WriteFile\([^)]*\.md' cli/` must return no
// hits outside this file. The BOARD.md writer in regenerate.go also uses an
// inline temp+rename for the same reason and is intentionally not migrated
// here (BOARD.md is not a task file and has its own atomicity reasoning).
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to path atomically by creating a temp file in
// the same directory and renaming it onto the target. The rename is POSIX-
// atomic, so any concurrent reader observes either the pre-existing file
// (if any) or the new content in full.
//
// On any error, the temp file is removed before returning so we do not leak
// `.tmp.*` siblings next to the destination.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// 8 bytes of randomness is plenty to avoid collisions even under heavy
	// concurrency; the PID prefix keeps the tmp name traceable.
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return fmt.Errorf("writeFileAtomic: cannot read random bytes: %w", err)
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".%s.tmp.%d.%s", base, os.Getpid(), hex.EncodeToString(rnd[:])))

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("writeFileAtomic: cannot create temp file %s: %w", tmp, err)
	}
	// Track whether we still need to clean the temp file on the error path.
	cleanup := func() { _ = os.Remove(tmp) }

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("writeFileAtomic: write to %s: %w", tmp, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("writeFileAtomic: fsync %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("writeFileAtomic: close %s: %w", tmp, err)
	}
	// Some filesystems honour umask on O_CREATE despite the perm arg. Re-chmod
	// to be safe; ignore errors on platforms where this is a no-op.
	if err := os.Chmod(tmp, perm); err != nil {
		cleanup()
		return fmt.Errorf("writeFileAtomic: chmod %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return fmt.Errorf("writeFileAtomic: rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
