package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const attachmentsDirName = "attachments"
const legacyAttachmentPrefix = attachmentsDirName + "/"

type attachmentSource struct {
	path string
	name string
	perm os.FileMode
}

type attachResult struct {
	taskID   string
	taskDir  string
	files    []string
	promoted bool
}

type attachmentRemoval struct {
	arg  string
	ref  string
	path string
}

type attachmentRef struct {
	ref    string
	base   string
	legacy bool
}

func cmdAttach(args []string) {
	if err := runAttach(args, os.Stdout); err != nil {
		fatal("%v", err)
	}
}

func runAttach(args []string, stdout io.Writer) error {
	if containsAttachRemoveFlag(args) {
		fs := flag.NewFlagSet("attach", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		remove := fs.Bool("rm", false, "remove attachment(s) from a folder-form task")
		fs.Usage = func() {}

		if err := fs.Parse(reorderArgs(args)); err != nil {
			return fmt.Errorf("usage: tb attach --rm <ID> <attachment-name>...")
		}
		if !*remove || fs.NArg() < 2 {
			return fmt.Errorf("usage: tb attach --rm <ID> <attachment-name>...")
		}

		taskID := normalizeTaskID(fs.Arg(0))
		return removeTaskAttachments(cfg.BoardDir, taskID, fs.Args()[1:], stdout)
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: tb attach <ID> <path>...")
	}

	taskID := normalizeTaskID(args[0])
	paths := args[1:]
	// Strip an optional `--` terminator between the task ID and user paths.
	// The GUI inserts this so a path starting with `-` cannot be reinterpreted
	// as a flag.
	if len(paths) > 0 && paths[0] == "--" {
		paths = paths[1:]
	}
	if len(paths) == 0 {
		return fmt.Errorf("usage: tb attach <ID> <path>...")
	}
	result, err := attachTask(cfg.BoardDir, taskID, paths)
	if err != nil {
		return err
	}

	suffix := ""
	if result.promoted {
		suffix = " (promoted to folder form)"
	}
	if stdout != nil {
		fmt.Fprintf(stdout, "Attached %d file(s) to %s%s: %s\n", len(result.files), result.taskID, suffix, strings.Join(result.files, ", "))
	}
	return nil
}

func containsAttachRemoveFlag(args []string) bool {
	for _, arg := range args {
		// `--` terminates flag scanning so a user-controlled path/name that
		// happens to be literally "--rm" cannot retarget the command. The GUI
		// inserts `--` between the task ID and user paths/names to defend
		// against argv smuggling; the CLI must respect it.
		if arg == "--" {
			return false
		}
		// Only the bare flag forms toggle the remove path. `--rm=false`
		// previously matched here and still entered the remove branch even
		// though the FlagSet would have parsed it as off — drop the prefix
		// match and rely on the FlagSet for any future value-bearing forms.
		if arg == "-rm" || arg == "--rm" {
			return true
		}
	}
	return false
}

func attachTask(boardDir, taskID string, sourcePaths []string) (attachResult, error) {
	if len(sourcePaths) == 0 {
		return attachResult{}, fmt.Errorf("at least one attachment path is required")
	}
	taskID = normalizeTaskID(taskID)

	lock, err := lockBoard(boardDir)
	if err != nil {
		return attachResult{}, err
	}
	defer lock.unlock()

	ref, err := resolveTaskRef(boardDir, taskID, allStatusDirs)
	if err != nil {
		return attachResult{}, err
	}

	sources, err := prepareAttachmentSources(sourcePaths)
	if err != nil {
		return attachResult{}, err
	}

	var result attachResult
	if isFolderTaskPath(ref.Path) {
		result, err = attachToFolderTask(ref, sources)
	} else {
		result, err = promoteFileTaskWithAttachments(boardDir, ref, sources)
	}
	if err != nil {
		return attachResult{}, err
	}

	if err := cleanupOrphanFileFormSibling(boardDir, ref.Status, ref.ID); err != nil {
		return attachResult{}, err
	}

	if err := regenerateBoard(boardDir); err != nil {
		return attachResult{}, fmt.Errorf("cannot regenerate BOARD.md: %w", err)
	}
	return result, nil
}

func removeTaskAttachments(boardDir, taskID string, names []string, stdout io.Writer) error {
	lock, err := lockBoard(boardDir)
	if err != nil {
		return err
	}
	defer lock.unlock()

	ref, err := resolveTaskRef(boardDir, taskID, allStatusDirs)
	if err != nil {
		return err
	}
	if !isFolderTaskPath(ref.Path) {
		return fmt.Errorf("task %s is file-form; attachment removal requires folder form with %s/", taskID, attachmentsDirName)
	}

	taskDir := taskDirForRef(ref)
	if err := validateRealDirectory(taskDir, fmt.Sprintf("task directory for %s", taskID)); err != nil {
		return err
	}

	removals, err := validateAttachmentRemovals(taskDir, taskID, names)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}

	refs := attachmentRemovalRefs(removals)
	displayNames := attachmentRemovalArgs(removals)
	content := removeAttachmentEntries(string(data), refs)
	content = appendLogEntry(content, fmt.Sprintf("- %s: Removed attachments: %s\n", time.Now().Format("2006-01-02"), strings.Join(displayNames, ", ")))
	if err := writeFileAtomic(ref.Path, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write %s: %w", ref.Path, err)
	}

	for _, removal := range removals {
		if err := os.Remove(removal.path); err != nil {
			return fmt.Errorf("cannot remove attachment %q: %w", removal.arg, err)
		}
	}

	if err := cleanupOrphanFileFormSibling(boardDir, ref.Status, ref.ID); err != nil {
		return err
	}

	if err := regenerateBoard(boardDir); err != nil {
		return fmt.Errorf("cannot regenerate BOARD.md: %w", err)
	}

	if stdout != nil {
		label := "attachments"
		if len(displayNames) == 1 {
			label = "attachment"
		}
		fmt.Fprintf(stdout, "Removed %s from %s: %s\n", label, taskID, strings.Join(displayNames, ", "))
	}
	return nil
}

func validateRealDirectory(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink; refusing to remove attachments", label)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", label)
	}
	return nil
}

func validateAttachmentRemovals(taskDir, taskID string, names []string) ([]attachmentRemoval, error) {
	realTaskDir, err := filepath.EvalSymlinks(taskDir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve task directory for %s: %w", taskID, err)
	}

	seen := make(map[string]bool, len(names))
	resolvedSeen := make(map[string]bool, len(names))
	removals := make([]attachmentRemoval, 0, len(names))
	for _, raw := range names {
		ref, err := parseAttachmentRef(raw)
		if err != nil {
			return nil, err
		}
		if seen[ref.ref] {
			return nil, fmt.Errorf("duplicate attachment name %q", raw)
		}
		seen[ref.ref] = true

		candidate, realBase, actualRef, err := resolveAttachmentRemovalPath(taskDir, realTaskDir, ref)
		if err != nil {
			return nil, fmt.Errorf("attachment %q not found on %s", raw, taskID)
		}
		if resolvedSeen[actualRef] {
			return nil, fmt.Errorf("duplicate attachment name %q", raw)
		}
		resolvedSeen[actualRef] = true

		info, err := os.Lstat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("attachment %q not found on %s", raw, taskID)
			}
			return nil, fmt.Errorf("cannot stat attachment %q: %w", raw, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("attachment %q is a directory; refusing to remove it", raw)
		}

		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return nil, fmt.Errorf("attachment %q cannot be resolved safely: %w", raw, err)
		}
		if !pathWithin(realBase, resolved) {
			location := "task directory"
			if strings.HasPrefix(actualRef, legacyAttachmentPrefix) {
				location = attachmentsDirName + "/"
			}
			return nil, fmt.Errorf("attachment %q resolves outside %s; refusing to remove it", raw, location)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("attachment %q is a symlink; refusing to remove it", raw)
		}

		removals = append(removals, attachmentRemoval{arg: raw, ref: actualRef, path: candidate})
	}
	return removals, nil
}

func resolveAttachmentRemovalPath(taskDir, realTaskDir string, ref attachmentRef) (string, string, string, error) {
	if ref.legacy {
		legacyDir := filepath.Join(taskDir, attachmentsDirName)
		realLegacyDir, err := realDirectory(legacyDir)
		if err != nil {
			return "", "", "", err
		}
		return filepath.Join(legacyDir, ref.base), realLegacyDir, ref.ref, nil
	}

	candidate := filepath.Join(taskDir, ref.base)
	if _, err := os.Lstat(candidate); err == nil {
		return candidate, realTaskDir, ref.ref, nil
	} else if !os.IsNotExist(err) {
		return "", "", "", err
	}

	// Compatibility: pre-TB-224 users removed legacy attachments by basename
	// (`tb attach --rm TB-1 old.txt`). Keep that working when no task-root file
	// with that basename exists. Use attachments/<name> to target legacy
	// explicitly when both forms are present.
	legacyDir := filepath.Join(taskDir, attachmentsDirName)
	legacyCandidate := filepath.Join(legacyDir, ref.base)
	if _, err := os.Lstat(legacyCandidate); err != nil {
		return "", "", "", err
	}
	realLegacyDir, err := realDirectory(legacyDir)
	if err != nil {
		return "", "", "", err
	}
	return legacyCandidate, realLegacyDir, legacyAttachmentPrefix + ref.base, nil
}

func realDirectory(path string) (string, error) {
	if err := validateRealDirectory(path, path); err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func parseAttachmentRef(name string) (attachmentRef, error) {
	if strings.ContainsRune(name, 0) {
		return attachmentRef{}, fmt.Errorf("attachment name %q contains a NUL byte", name)
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || filepath.VolumeName(name) != "" {
		return attachmentRef{}, fmt.Errorf("attachment name %q must not be an absolute path", name)
	}
	if strings.Contains(name, `\`) {
		return attachmentRef{}, fmt.Errorf("attachment name %q must not contain path separators", name)
	}
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		if len(parts) != 2 || parts[0] != attachmentsDirName {
			return attachmentRef{}, fmt.Errorf("attachment name %q must be a file name or %s<name>", name, legacyAttachmentPrefix)
		}
		if err := validateAttachmentLeafName(parts[1]); err != nil {
			return attachmentRef{}, err
		}
		return attachmentRef{ref: legacyAttachmentPrefix + parts[1], base: parts[1], legacy: true}, nil
	}
	if err := validateAttachmentLeafName(name); err != nil {
		return attachmentRef{}, err
	}
	return attachmentRef{ref: name, base: name}, nil
}

func validateAttachmentLeafName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("attachment name cannot be empty")
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("attachment name %q contains a NUL byte", name)
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || filepath.VolumeName(name) != "" {
		return fmt.Errorf("attachment name %q must not be an absolute path", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("attachment name %q is not allowed", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("attachment name %q must not contain path separators", name)
	}
	if isReservedAttachmentName(name) {
		return fmt.Errorf("attachment name %q is reserved for task internals", name)
	}
	return nil
}

func pathWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// prepareAttachmentSources validates each source path before staging.
// os.Stat (not os.Lstat) is intentional: tb attach is a user-driven copy
// command, so following a symlink to the underlying file is the expected
// behavior — the user already had read access via the link they handed us.
func prepareAttachmentSources(paths []string) ([]attachmentSource, error) {
	sources := make([]attachmentSource, 0, len(paths))
	seen := make(map[string]string, len(paths))
	for _, raw := range paths {
		clean := filepath.Clean(raw)
		info, err := os.Stat(clean)
		if err != nil {
			return nil, fmt.Errorf("cannot read attachment source %s: %w", raw, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("attachment source %s is a directory; attach files only", raw)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("attachment source %s is not a regular file", raw)
		}
		name := filepath.Base(clean)
		if name == "." || name == string(filepath.Separator) || name == "" {
			return nil, fmt.Errorf("attachment source %s does not have a usable file name", raw)
		}
		// Reject names the --rm path also rejects (NUL, separators, abs paths,
		// `.`/`..`, and task-internal names) so add/remove cannot diverge on
		// what a valid attachment name is.
		if err := validateAttachmentLeafName(name); err != nil {
			return nil, fmt.Errorf("attachment source %s: %w", raw, err)
		}
		if prev, ok := seen[name]; ok {
			return nil, fmt.Errorf("attachment name collision: %s and %s both import as %q", prev, raw, name)
		}
		seen[name] = raw
		perm := info.Mode().Perm()
		if perm == 0 {
			perm = 0644
		}
		sources = append(sources, attachmentSource{path: clean, name: name, perm: perm})
	}
	return sources, nil
}

func promoteFileTaskWithAttachments(boardDir string, ref taskRef, sources []attachmentSource) (attachResult, error) {
	statusDir := filepath.Dir(ref.Path)
	taskDir := filepath.Join(statusDir, ref.ID)
	if _, err := os.Lstat(taskDir); err == nil {
		return attachResult{}, fmt.Errorf("cannot promote %s: target task directory already exists at %s", ref.ID, taskDir)
	} else if !os.IsNotExist(err) {
		return attachResult{}, fmt.Errorf("cannot inspect task directory %s: %w", taskDir, err)
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return attachResult{}, fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}

	stagingDir, err := makeHiddenWorkDir(statusDir, "."+ref.ID+".promote")
	if err != nil {
		return attachResult{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	if err := copySourcesIntoDir(sources, stagingDir); err != nil {
		return attachResult{}, err
	}

	if err := stageLegacyAgentArtifacts(boardDir, ref.ID, stagingDir); err != nil {
		return attachResult{}, err
	}

	names := attachmentSourceNames(sources)
	today := time.Now().Format("2006-01-02")
	content := upsertAttachmentsSection(string(data), names)
	content = appendLogEntry(content, fmt.Sprintf("- %s: Promoted to folder form\n", today))
	content = appendLogEntry(content, fmt.Sprintf("- %s: Attached %s\n", today, strings.Join(names, ", ")))

	if err := writeFileAtomic(filepath.Join(stagingDir, folderTaskFileName), []byte(content), 0644); err != nil {
		return attachResult{}, fmt.Errorf("cannot write promoted TASK.md: %w", err)
	}

	if err := os.Rename(stagingDir, taskDir); err != nil {
		return attachResult{}, fmt.Errorf("cannot publish promoted task directory %s: %w", taskDir, err)
	}
	published = true

	if err := os.Remove(ref.Path); err != nil && !os.IsNotExist(err) {
		return attachResult{}, fmt.Errorf("promoted %s but could not remove legacy file %s: %w", ref.ID, ref.Path, err)
	}

	// Legacy artifact cleanup is best-effort once the folder is published:
	// the task is already migrated and a stray root-level state/log file is
	// cosmetic (the daemon now reads task-local paths and the next promotion
	// can't re-trigger). A hard error here would abort what is otherwise a
	// successful publish and leave the migrated artifacts duplicated.
	if err := removeLegacyAgentArtifacts(boardDir, ref.ID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: promoted %s but could not remove legacy agent artifacts: %v\n", ref.ID, err)
	}

	return attachResult{taskID: ref.ID, taskDir: taskDir, files: names, promoted: true}, nil
}

// Legacy file-form tasks keep their JSONL run history at
// <boardDir>/.agent-state/<ID>.jsonl and their per-run log files at
// <boardDir>/.agent-logs/<ID>/. Folder-form tasks own .agent-state.jsonl and
// .agent-logs/ inside the task directory (see docs/ARCHITECTURE.md
// "Folder-form tasks"). Promotion must move these artifacts so a task keeps
// its run history across the layout change.
const (
	legacyAgentStateDirName  = ".agent-state"
	legacyAgentLogsDirName   = ".agent-logs"
	folderAgentStateFileName = ".agent-state.jsonl"
	folderAgentLogsDirName   = ".agent-logs"
)

// stageLegacyAgentArtifacts copies any pre-existing legacy agent state file
// and log directory for taskID into stagingDir using folder-form filenames.
// Absent artifacts are not errors — promotion may run on tasks that never had
// an agent assigned. Source bytes are preserved verbatim via copyFileAtomic.
func stageLegacyAgentArtifacts(boardDir, taskID, stagingDir string) error {
	if err := stageLegacyAgentState(boardDir, taskID, stagingDir); err != nil {
		return err
	}
	return stageLegacyAgentLogs(boardDir, taskID, stagingDir)
}

func stageLegacyAgentState(boardDir, taskID, stagingDir string) error {
	src := filepath.Join(boardDir, legacyAgentStateDirName, taskID+".jsonl")
	info, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat legacy agent state %s: %w", src, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("legacy agent state %s is not a regular file", src)
	}
	dst := filepath.Join(stagingDir, folderAgentStateFileName)
	if err := copyFileAtomic(src, dst, info.Mode().Perm()); err != nil {
		return fmt.Errorf("stage legacy agent state for %s: %w", taskID, err)
	}
	return nil
}

func stageLegacyAgentLogs(boardDir, taskID, stagingDir string) error {
	src := filepath.Join(boardDir, legacyAgentLogsDirName, taskID)
	info, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat legacy agent logs %s: %w", src, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("legacy agent logs path %s is not a directory", src)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read legacy agent logs %s: %w", src, err)
	}
	dstDir := filepath.Join(stagingDir, folderAgentLogsDirName)
	if err := os.Mkdir(dstDir, 0755); err != nil {
		return fmt.Errorf("create staging logs dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			// Agent logs are flat run-id files; skip any unexpected subdir.
			continue
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat legacy log %s: %w", entry.Name(), err)
		}
		if !entryInfo.Mode().IsRegular() {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())
		if err := copyFileAtomic(srcPath, dstPath, entryInfo.Mode().Perm()); err != nil {
			return fmt.Errorf("stage legacy log %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// removeLegacyAgentArtifacts is called only after the promoted folder task
// has been published. It deletes the root-level state file and log directory
// for taskID. Absent paths are not errors — they may never have existed.
func removeLegacyAgentArtifacts(boardDir, taskID string) error {
	state := filepath.Join(boardDir, legacyAgentStateDirName, taskID+".jsonl")
	if err := os.Remove(state); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy agent state %s: %w", state, err)
	}
	logs := filepath.Join(boardDir, legacyAgentLogsDirName, taskID)
	if err := os.RemoveAll(logs); err != nil {
		return fmt.Errorf("remove legacy agent logs %s: %w", logs, err)
	}
	return nil
}

func attachToFolderTask(ref taskRef, sources []attachmentSource) (attachResult, error) {
	taskDir := taskDirForRef(ref)
	if taskDir == "" {
		return attachResult{}, fmt.Errorf("task %s is not in folder form", ref.ID)
	}
	existing, err := readAttachmentRefs(taskDir)
	if err != nil {
		return attachResult{}, err
	}
	existingSet := make(map[string]bool, len(existing))
	for _, ref := range existing {
		existingSet[attachmentRefBase(ref)] = true
	}
	for _, source := range sources {
		if existingSet[source.name] {
			return attachResult{}, fmt.Errorf("attachment %q already exists on %s; refusing to overwrite", source.name, ref.ID)
		}
		if _, err := os.Lstat(filepath.Join(taskDir, source.name)); err == nil {
			return attachResult{}, fmt.Errorf("attachment %q already exists on %s; refusing to overwrite", source.name, ref.ID)
		} else if err != nil && !os.IsNotExist(err) {
			return attachResult{}, fmt.Errorf("cannot inspect attachment destination %q: %w", source.name, err)
		}
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return attachResult{}, fmt.Errorf("cannot read %s: %w", ref.Path, err)
	}

	stagingDir, err := makeHiddenWorkDir(taskDir, ".attach")
	if err != nil {
		return attachResult{}, err
	}
	defer os.RemoveAll(stagingDir)

	if err := copySourcesIntoDir(sources, stagingDir); err != nil {
		return attachResult{}, err
	}

	// A crash between publishing the last attachment (below) and writing TASK.md
	// leaves attachment files on disk that are not yet listed in `## Attachments`.
	// The task directory is the source of truth; the section is derived from it,
	// so the next `tb attach` rebuilds the section via readAttachmentRefs
	// + mergeAttachmentNames. The window is cosmetic, not data-loss.
	var published []string
	for _, source := range sources {
		src := filepath.Join(stagingDir, source.name)
		dst := filepath.Join(taskDir, source.name)
		if _, err := os.Lstat(dst); err == nil {
			bestEffortRemoveFiles(published)
			return attachResult{}, fmt.Errorf("attachment %q already exists on %s; refusing to overwrite", source.name, ref.ID)
		} else if err != nil && !os.IsNotExist(err) {
			bestEffortRemoveFiles(published)
			return attachResult{}, fmt.Errorf("cannot inspect attachment destination %q: %w", source.name, err)
		}
		if err := os.Rename(src, dst); err != nil {
			bestEffortRemoveFiles(published)
			return attachResult{}, fmt.Errorf("cannot publish attachment %s: %w", source.name, err)
		}
		published = append(published, dst)
	}

	names := mergeAttachmentNames(existing, attachmentSourceNames(sources))
	today := time.Now().Format("2006-01-02")
	content := upsertAttachmentsSection(string(data), names)
	content = appendLogEntry(content, fmt.Sprintf("- %s: Attached %s\n", today, strings.Join(attachmentSourceNames(sources), ", ")))

	if err := writeFileAtomic(ref.Path, []byte(content), 0644); err != nil {
		bestEffortRemoveFiles(published)
		return attachResult{}, fmt.Errorf("cannot update %s: %w", ref.Path, err)
	}

	return attachResult{taskID: ref.ID, taskDir: taskDir, files: attachmentSourceNames(sources), promoted: false}, nil
}

func copySourcesIntoDir(sources []attachmentSource, dir string) error {
	for _, source := range sources {
		dst := filepath.Join(dir, source.name)
		if err := copyFileAtomic(source.path, dst, source.perm); err != nil {
			return fmt.Errorf("cannot copy %s to %s: %w", source.path, dst, err)
		}
	}
	return nil
}

func copyFileAtomic(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := tempSiblingPath(dst)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	cleanup := func() { _ = os.Remove(tmp) }

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		cleanup()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		cleanup()
		return err
	}
	if err := out.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		cleanup()
		return err
	}
	return nil
}

func tempSiblingPath(path string) (string, error) {
	token, err := randomHex(8)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	return filepath.Join(dir, fmt.Sprintf(".%s.tmp.%d.%s", base, os.Getpid(), token)), nil
}

func makeHiddenWorkDir(parent, prefix string) (string, error) {
	for i := 0; i < 10; i++ {
		token, err := randomHex(8)
		if err != nil {
			return "", err
		}
		path := filepath.Join(parent, fmt.Sprintf("%s.%d.%s", prefix, os.Getpid(), token))
		if err := os.Mkdir(path, 0755); err == nil {
			return path, nil
		} else if !os.IsExist(err) {
			return "", fmt.Errorf("cannot create staging directory %s: %w", path, err)
		}
	}
	return "", fmt.Errorf("cannot create unique staging directory in %s", parent)
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func readAttachmentRefs(taskDir string) ([]string, error) {
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read task directory %s: %w", taskDir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == attachmentsDirName {
			names = append(names, readLegacyAttachmentRefs(taskDir)...)
			continue
		}
		if isReservedAttachmentName(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("cannot inspect attachment candidate %s: %w", filepath.Join(taskDir, name), err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func readLegacyAttachmentRefs(taskDir string) []string {
	attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
	info, err := os.Lstat(attachmentsDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	entries, err := os.ReadDir(attachmentsDir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if isReservedAttachmentName(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		names = append(names, legacyAttachmentPrefix+name)
	}
	return names
}

func attachmentSourceNames(sources []attachmentSource) []string {
	names := make([]string, 0, len(sources))
	for _, source := range sources {
		names = append(names, source.name)
	}
	return names
}

func mergeAttachmentNames(existing, added []string) []string {
	seen := make(map[string]bool, len(existing)+len(added))
	names := make([]string, 0, len(existing)+len(added))
	for _, name := range append(append([]string{}, existing...), added...) {
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func upsertAttachmentsSection(content string, names []string) string {
	names = mergeAttachmentNames(nil, names)
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "- %s\n", filepath.ToSlash(name))
	}
	return upsertTaskSection(content, "## Attachments", strings.TrimRight(b.String(), "\n"))
}

func removeAttachmentEntries(content string, names []string) string {
	section, ok := findTaskSection(content, "## Attachments")
	if !ok {
		return content
	}

	remove := make(map[string]bool, len(names))
	for _, name := range names {
		remove[name] = true
	}

	body := content[section.bodyStart:section.end]
	lines := strings.SplitAfter(body, "\n")
	kept := make([]string, 0, len(lines))
	hasBody := false
	for _, line := range lines {
		if name, ok := attachmentEntryRef(line); ok && remove[name] {
			continue
		}
		kept = append(kept, line)
		if strings.TrimSpace(line) != "" {
			hasBody = true
		}
	}

	if !hasBody {
		return removeTaskSection(content, section)
	}
	return content[:section.bodyStart] + strings.Join(kept, "") + content[section.end:]
}

func attachmentEntryRef(line string) (string, bool) {
	trimmed := strings.TrimSpace(strings.TrimRight(line, "\r\n"))
	if !strings.HasPrefix(trimmed, "- ") {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	ref, err := parseAttachmentRef(value)
	if err != nil {
		return "", false
	}
	return ref.ref, true
}

func attachmentRefBase(ref string) string {
	return strings.TrimPrefix(ref, legacyAttachmentPrefix)
}

func attachmentRemovalRefs(removals []attachmentRemoval) []string {
	refs := make([]string, 0, len(removals))
	for _, removal := range removals {
		refs = append(refs, removal.ref)
	}
	return refs
}

func attachmentRemovalArgs(removals []attachmentRemoval) []string {
	args := make([]string, 0, len(removals))
	for _, removal := range removals {
		args = append(args, removal.arg)
	}
	return args
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

func removeTaskSection(content string, section taskSectionRange) string {
	before := strings.TrimRight(content[:section.start], "\n")
	after := strings.TrimLeft(content[section.end:], "\n")
	switch {
	case before == "":
		if after == "" {
			return ""
		}
		return after
	case after == "":
		return before + "\n"
	default:
		return before + "\n\n" + after
	}
}

// bestEffortRemoveFiles deletes each path and warns to stderr on any failure.
// Used to roll back partial attachment publishes when a subsequent step fails;
// a remaining file is cosmetic (the next `tb attach` rebuilds the `##
// Attachments` section from the task directory) but the user still deserves to
// know cleanup was incomplete.
func bestEffortRemoveFiles(paths []string) {
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: failed to remove %s during rollback: %v\n", path, err)
		}
	}
}
