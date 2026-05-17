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

const (
	folderTaskFileName = "TASK.md"
	attachmentsDirName = "attachments"
	legacyAttachPrefix = attachmentsDirName + "/"
)

// Attachment is a single user-managed file under <status>/<ID>/, or a legacy
// compatibility file under <status>/<ID>/attachments/. Size is in bytes. The
// frontend renders these as drawer rows.
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

// ListAttachments returns task-root user attachments plus legacy
// attachments/<name> files, sorted by name. Returns an empty slice (not nil)
// for folder tasks with no attachments, or for legacy file-form tasks. Returns
// ErrNotFound when the task id has no folder-form directory.
//
// Reads from disk directly: the CLI owns *writes* to the task dir, but
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
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return nil, fmt.Errorf("read task dir: %w", err)
	}
	out := make([]Attachment, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == attachmentsDirName {
			out = append(out, listLegacyAttachments(taskDir)...)
			continue
		}
		if isReservedAttachmentName(name) || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		out = append(out, Attachment{Name: name, Size: info.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func listLegacyAttachments(taskDir string) []Attachment {
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
	info, err := os.Lstat(attachmentsDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	entries, err := os.ReadDir(attachmentsDir)
	if err != nil {
		return nil
	}
	out := make([]Attachment, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if isReservedAttachmentName(name) || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		out = append(out, Attachment{Name: legacyAttachPrefix + name, Size: info.Size()})
	}
	return out
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
// must be either a task-root filename or a legacy attachments/<filename> ref,
// and the resolved path must stay inside that attachment root.
func (b *BoardService) OpenAttachment(ctx context.Context, id, name string) error {
	ref, err := parseAttachmentRef(name)
	if err != nil {
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
	candidate, realBase, err := attachmentPath(taskDir, ref)
	if err != nil {
		return err
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
	if info.IsDir() {
		return fmt.Errorf("attachment %q is a directory; refusing to open", name)
	}
	if !pathWithin(realBase, mustResolveOrSelf(candidate)) {
		return fmt.Errorf("attachment %q resolves outside its attachment root", name)
	}
	opener := b.openFile
	if opener == nil {
		opener = defaultOpener{}
	}
	return opener.Open(ctx, candidate)
}

type attachmentRef struct {
	base   string
	legacy bool
}

func attachmentPath(taskDir string, ref attachmentRef) (string, string, error) {
	if ref.legacy {
		attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
		info, err := os.Lstat(attachmentsDir)
		if err != nil {
			if os.IsNotExist(err) {
				return "", "", fmt.Errorf("legacy attachments directory not found")
			}
			return "", "", fmt.Errorf("stat legacy attachments dir: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", "", fmt.Errorf("legacy attachments directory is not a real directory")
		}
		realDir, err := filepath.EvalSymlinks(attachmentsDir)
		if err != nil {
			return "", "", fmt.Errorf("resolve legacy attachments dir: %w", err)
		}
		return filepath.Join(attachmentsDir, ref.base), realDir, nil
	}
	realTaskDir, err := filepath.EvalSymlinks(taskDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve task dir: %w", err)
	}
	return filepath.Join(taskDir, ref.base), realTaskDir, nil
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

func parseAttachmentRef(name string) (attachmentRef, error) {
	if strings.TrimSpace(name) == "" {
		return attachmentRef{}, &MutationValidationError{Msg: "attachment name is required"}
	}
	if strings.ContainsRune(name, 0) {
		return attachmentRef{}, &MutationValidationError{Msg: "attachment name contains a NUL byte"}
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || filepath.VolumeName(name) != "" {
		return attachmentRef{}, &MutationValidationError{Msg: "attachment name must not be an absolute path"}
	}
	if strings.Contains(name, `\`) {
		return attachmentRef{}, &MutationValidationError{Msg: "attachment name must not contain path separators"}
	}
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		if len(parts) != 2 || parts[0] != attachmentsDirName {
			return attachmentRef{}, &MutationValidationError{Msg: "attachment name must be a file name or " + legacyAttachPrefix + "<name>"}
		}
		if err := validateAttachmentLeafName(parts[1]); err != nil {
			return attachmentRef{}, err
		}
		return attachmentRef{base: parts[1], legacy: true}, nil
	}
	if err := validateAttachmentLeafName(name); err != nil {
		return attachmentRef{}, err
	}
	return attachmentRef{base: name}, nil
}

func validateAttachmentLeafName(name string) error {
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
	if isReservedAttachmentName(name) {
		return &MutationValidationError{Msg: "attachment name is reserved for task internals"}
	}
	return nil
}

func isReservedAttachmentName(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch strings.ToLower(name) {
	case strings.ToLower(folderTaskFileName), strings.ToLower(attachmentsDirName):
		return true
	default:
		return false
	}
}
