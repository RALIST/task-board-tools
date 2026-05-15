package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Attachment is a single file under <status>/<ID>/attachments/. Size is in
// bytes. The frontend renders these as drawer rows.
type Attachment struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// attachmentOpener launches a file in the OS default handler. Pulled behind an
// interface so tests can substitute a fake that records invocations without
// actually invoking `open`/`xdg-open`/`explorer`.
type attachmentOpener interface {
	Open(ctx context.Context, path string) error
}

// defaultOpener is the production implementation. macOS uses `open`, Linux
// uses `xdg-open`, Windows uses `rundll32 url.dll,FileProtocolHandler`.
type defaultOpener struct{}

func (defaultOpener) Open(ctx context.Context, path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", path)
	case "windows":
		// rundll32 invokes the default file handler via ShellExecute, bypassing
		// cmd.exe entirely. The previous `cmd /c start "" <path>` form was
		// vulnerable to cmd metacharacter injection (&, |, ^, >, <, parens,
		// trailing space/dot) carried by a tampered attachment filename — Go's
		// exec.Command treats cmd.exe specially and passes its argv raw rather
		// than escaping it. rundll32 is a normal program, so Go's
		// syscall.EscapeArg quotes the path safely and ShellExecute opens it
		// with the default handler.
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", path)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("open %s: %v: %s", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// resolveTaskDir locates the folder-form task directory for id under
// boardDir/<status>/<id>/. Returns ErrNotFound if no such directory exists
// (e.g. legacy file-form task or unknown id).
func resolveTaskDir(boardDir, id string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(id))
	if upper == "" {
		return "", &MutationValidationError{Msg: "task id is required"}
	}
	for _, dir := range []string{"backlog", "in-progress", "done", "archive"} {
		candidate := filepath.Join(boardDir, dir, upper)
		info, err := os.Lstat(candidate)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		return candidate, nil
	}
	return "", ErrNotFound
}

// MutationValidationError is returned by methods that reject malformed input
// before exec. Carries a stable Error() shape the frontend can match on.
type MutationValidationError struct{ Msg string }

func (e *MutationValidationError) Error() string { return e.Msg }

// ListAttachments returns the files in <status>/<ID>/attachments/, sorted by
// name. Returns an empty slice (not nil) for tasks that exist but have no
// attachments directory, or for legacy file-form tasks. Returns ErrNotFound
// when the task id has no folder-form directory.
//
// Reads from disk directly: the CLI owns *writes* to the attachments dir, but
// reads are lock-free per the architecture invariant (same model the rest of
// the GUI uses).
func (b *BoardService) ListAttachments(ctx context.Context, id string) ([]Attachment, error) {
	boardDir, err := b.resolveBoardDir(ctx)
	if err != nil {
		return nil, err
	}
	taskDir, err := resolveTaskDir(boardDir, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Legacy file-form tasks have no attachments. Return an empty
			// slice so the frontend's "no attachments" state matches the
			// folder-form-with-zero-files state.
			if _, ferr := findTaskFile(boardDir, id); ferr == nil {
				return []Attachment{}, nil
			}
		}
		return nil, err
	}
	attachmentsDir := filepath.Join(taskDir, "attachments")
	entries, err := os.ReadDir(attachmentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Attachment{}, nil
		}
		return nil, fmt.Errorf("read attachments dir: %w", err)
	}
	out := make([]Attachment, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, Attachment{Name: entry.Name(), Size: info.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// AddAttachments runs `tb attach <id> <paths...>`. The CLI owns source
// validation, collision handling, and legacy-task auto-promotion; this
// wrapper just forwards. After success, the watcher's debounced
// board:reloaded event refreshes the drawer — callers MUST NOT manually
// re-fetch the task, that would violate TB-104's "no duplicate refresh"
// criterion.
func (b *BoardService) AddAttachments(ctx context.Context, id string, paths []string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.Attach(ctx, id, paths)
}

// RemoveAttachments runs `tb attach --rm <id> <names...>`.
func (b *BoardService) RemoveAttachments(ctx context.Context, id string, names []string) error {
	c := b.snapshot()
	if c == nil {
		return ErrNoBoard
	}
	return c.RemoveAttachments(ctx, id, names)
}

// OpenAttachment launches the attachment in the OS default handler. The name
// must be a bare filename (no separators) — same validation the CLI applies
// to `tb attach --rm` — and the resolved path must stay inside the task's
// attachments dir, defending against `..`/symlink escapes if the on-disk
// state was tampered with out-of-band.
func (b *BoardService) OpenAttachment(ctx context.Context, id, name string) error {
	if err := validateAttachmentName(name); err != nil {
		return err
	}
	boardDir, err := b.resolveBoardDir(ctx)
	if err != nil {
		return err
	}
	taskDir, err := resolveTaskDir(boardDir, id)
	if err != nil {
		return err
	}
	attachmentsDir := filepath.Join(taskDir, "attachments")
	realAttachmentsDir, err := filepath.EvalSymlinks(attachmentsDir)
	if err != nil {
		return fmt.Errorf("resolve attachments dir: %w", err)
	}
	candidate := filepath.Join(attachmentsDir, name)
	if !pathWithin(realAttachmentsDir, mustResolveOrSelf(candidate)) {
		return fmt.Errorf("attachment %q resolves outside attachments/", name)
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("attachment %q not found on %s", name, id)
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("attachment %q is a symlink; refusing to open", name)
	}
	opener := b.openFile
	if opener == nil {
		opener = defaultOpener{}
	}
	return opener.Open(ctx, candidate)
}

func mustResolveOrSelf(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func pathWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func validateAttachmentName(name string) error {
	if strings.TrimSpace(name) == "" {
		return &MutationValidationError{Msg: "attachment name is required"}
	}
	if strings.ContainsRune(name, 0) {
		return &MutationValidationError{Msg: "attachment name contains a NUL byte"}
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || filepath.VolumeName(name) != "" {
		return &MutationValidationError{Msg: "attachment name must not be an absolute path"}
	}
	if name == "." || name == ".." {
		return &MutationValidationError{Msg: "attachment name is not allowed"}
	}
	if strings.ContainsAny(name, `/\`) {
		return &MutationValidationError{Msg: "attachment name must not contain path separators"}
	}
	return nil
}
