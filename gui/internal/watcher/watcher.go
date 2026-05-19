// Package watcher wraps fsnotify to turn board-directory filesystem activity
// into two coarse-grained Wails events:
//
//   - "board:reloaded"    — column membership or folder-task contents changed
//     (Create / Remove / Rename)
//   - "task:updated:<id>" — a single task file was rewritten (Write)
//
// A 200ms debounce window coalesces the fan-out that every logical CLI
// mutation produces (a "move" is remove + create + regenerate). The watcher
// never reads file contents — it only reports paths — and it relies on the
// CLI's atomic-write invariant so consumers reading on these events never see
// a half-written file.
//
// Architecture invariant: do not subscribe to BOARD.md, .next-id, .board.lock,
// .agent-state/, or .agent-logs/. Watching BOARD.md would create a feedback
// loop (regenerate runs after every mutation).
package watcher

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// statusDirs are the six directories a board may contain (canonical kanban:
// backlog → ready → in-progress → code-review → done → archive). Missing
// dirs are silently skipped on attach — a fresh board may have only some
// of them.
var statusDirs = []string{"backlog", "ready", "in-progress", "code-review", "done", "archive"}

var statusDirSet = map[string]struct{}{
	"backlog":     {},
	"ready":       {},
	"in-progress": {},
	"code-review": {},
	"done":        {},
	"archive":     {},
}

const (
	folderTaskFileName = "TASK.md"
	attachmentsDirName = "attachments"
)

// ignoredBasenames are files whose events we never report.
var ignoredBasenames = map[string]struct{}{
	"BOARD.md":    {},
	".next-id":    {},
	".board.lock": {},
	".gitkeep":    {},
	".DS_Store":   {},
}

// ignoredDirSegments are path components anywhere under boardDir whose
// contents are never reported. Membership is by segment, not prefix.
var ignoredDirSegments = []string{".agent-state", ".agent-logs"}

// debounceWindow coalesces bursts from a single logical mutation.
const debounceWindow = 200 * time.Millisecond

// Emitter is the contract the watcher needs from the host application.
// In production it's satisfied by *application.App.Event from Wails3; tests
// pass an in-memory implementation.
type Emitter interface {
	Emit(name string, data ...any)
}

// fsEvent is the internal pump channel payload.
type fsEvent struct {
	ev  fsnotify.Event
	err error
}

// Watcher subscribes to a board directory and forwards filtered events to an
// Emitter. Safe for concurrent use — Switch and Start coexist via a shared
// pump channel; the underlying fsnotify watcher can be hot-swapped without
// disturbing Start's loop.
type Watcher struct {
	emitter Emitter
	logger  *slog.Logger

	// out is the stable internal channel that Start reads from. Each attach
	// spawns a pump goroutine that copies its fsnotify events here until its
	// own context is cancelled.
	out chan fsEvent

	// attachMu serialises full attach() invocations so two concurrent
	// Switch/Start calls cannot interleave their fsw.Add loops on overlapping
	// watchDir maps. Distinct from mu, which protects per-event field reads
	// and addWatchDir/removeWatchDir mutations.
	attachMu sync.Mutex

	mu        sync.Mutex
	fsw       *fsnotify.Watcher
	cancelPmp context.CancelFunc
	debouncer *debouncer
	boardDir  string
	watchDirs map[string]struct{}
}

// New creates a Watcher. The Watcher is inert until Start is called.
func New(emitter Emitter, logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Watcher{
		emitter: emitter,
		logger:  logger.With("component", "watcher"),
		// Buffered so a debounce-window burst doesn't block the pump.
		out: make(chan fsEvent, 64),
	}
}

// Run drains the internal pump channel until ctx is cancelled. It is safe to
// call Run before any Switch — events stay buffered in the channel until a
// Switch points the watcher at a real board.
//
// Typical lifecycle: dispatch Run in a goroutine at app startup, then call
// Switch every time the user opens or changes a board.
func (w *Watcher) Run(ctx context.Context) error {
	defer w.detach()

	for {
		select {
		case <-ctx.Done():
			return nil
		case fe := <-w.out:
			if fe.err != nil {
				w.logger.Warn("fsnotify error", "err", fe.err)
				continue
			}
			w.handle(fe.ev)
		}
	}
}

// Start is a convenience for tests and for callers that want a single call to
// attach + run. Equivalent to Switch(boardDir) followed by Run(ctx).
func (w *Watcher) Start(ctx context.Context, boardDir string) error {
	if err := w.attach(boardDir); err != nil {
		return err
	}
	return w.Run(ctx)
}

// Switch detaches from the previous board (if any) and attaches to a new one.
// Safe to call while Run is running.
func (w *Watcher) Switch(boardDir string) error {
	return w.attach(boardDir)
}

// attach acquires a new fsnotify.Watcher, registers every existing status
// dir, spawns a pump goroutine, and atomically swaps the previous watcher
// out. Errors leave the prior subscription intact.
func (w *Watcher) attach(boardDir string) error {
	if boardDir == "" {
		return errors.New("watcher: empty boardDir")
	}
	// Serialise full attach setup. Two concurrent Switch invocations would
	// otherwise each build their own watchDirs maps in parallel; an event
	// delivered on the old fsw between the swap and oldCancel() could call
	// addWatchDir on the new watchDirs while the second attach is still
	// populating it.
	w.attachMu.Lock()
	defer w.attachMu.Unlock()

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	watchDirs := make(map[string]struct{})
	added := 0
	for _, d := range statusDirs {
		full := filepath.Join(boardDir, d)
		if err := addFSWatch(fsw, watchDirs, full); err != nil {
			w.logger.Debug("watcher: skip status dir", "dir", full, "err", err)
			continue
		}
		added++
		w.addExistingFolderTaskWatches(fsw, watchDirs, full)
	}
	if added == 0 {
		_ = fsw.Close()
		return errors.New("watcher: no status dirs found under " + boardDir)
	}

	pumpCtx, cancel := context.WithCancel(context.Background())
	go pump(pumpCtx, fsw, w.out)

	w.mu.Lock()
	oldCancel := w.cancelPmp
	oldDeb := w.debouncer
	w.fsw = fsw
	w.boardDir = boardDir
	w.cancelPmp = cancel
	w.debouncer = newDebouncer(debounceWindow, w.flushBoard)
	w.watchDirs = watchDirs
	w.mu.Unlock()

	// Cancelling the old pump's context also closes its fsnotify.Watcher
	// (see pump's defer), and stops the old debouncer cleanly.
	if oldCancel != nil {
		oldCancel()
	}
	if oldDeb != nil {
		oldDeb.stop()
	}
	w.logger.Info("watcher: attached", "boardDir", boardDir, "statusDirs", added, "watchDirs", len(watchDirs))
	return nil
}

func (w *Watcher) addExistingFolderTaskWatches(fsw *fsnotify.Watcher, watchDirs map[string]struct{}, statusDir string) {
	entries, err := os.ReadDir(statusDir)
	if err != nil {
		w.logger.Debug("watcher: cannot scan status dir", "dir", statusDir, "err", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !isTaskIDName(entry.Name()) {
			continue
		}
		taskDir := filepath.Join(statusDir, entry.Name())
		if err := addFSWatch(fsw, watchDirs, taskDir); err != nil {
			w.logger.Debug("watcher: skip task dir", "dir", taskDir, "err", err)
			continue
		}
		attachmentsDir := filepath.Join(taskDir, attachmentsDirName)
		if isRealDir(attachmentsDir) {
			if err := addFSWatch(fsw, watchDirs, attachmentsDir); err != nil {
				w.logger.Debug("watcher: skip attachments dir", "dir", attachmentsDir, "err", err)
			}
		}
	}
}

func (w *Watcher) detach() {
	w.mu.Lock()
	cancel := w.cancelPmp
	deb := w.debouncer
	w.cancelPmp = nil
	w.debouncer = nil
	w.fsw = nil
	w.watchDirs = nil
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if deb != nil {
		deb.stop()
	}
}

func (w *Watcher) handle(ev fsnotify.Event) {
	if isIgnored(ev.Name) {
		return
	}
	w.reconcileDirWatches(ev)
	switch {
	case ev.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0:
		w.scheduleBoardReload()
	case ev.Op&fsnotify.Write != 0:
		if id := taskIDFromPath(ev.Name); id != "" {
			w.emitter.Emit("task:updated:"+id, id)
		}
	}
}

func (w *Watcher) reconcileDirWatches(ev fsnotify.Event) {
	if ev.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
		w.watchCreatedDir(ev.Name)
	}
	if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 && !pathExists(ev.Name) {
		w.removeWatchDirAndChildren(ev.Name)
	}
}

func (w *Watcher) watchCreatedDir(path string) {
	if !isRealDir(path) {
		return
	}
	if isTaskDirPath(path) {
		added := w.addWatchDir(path)
		if added {
			// File→folder promotion publishes via os.Rename(staging, taskDir)
			// — TASK.md is already inside the renamed dir before fsnotify
			// fires the Create event, so the subsequent atomic write of the
			// `Promoted to folder form` log entry (or any same-rename-window
			// edit) can be missed by the just-registered watch. Sample the
			// state now and synthesise a task:updated emission so the drawer
			// auto-refresh path is correct even if no further Write fires.
			if base := filepath.Base(path); isTaskIDName(base) {
				taskMD := filepath.Join(path, folderTaskFileName)
				if info, err := os.Lstat(taskMD); err == nil && info.Mode().IsRegular() {
					w.emitter.Emit("task:updated:"+base, base)
				}
			}
		}
		attachmentsDir := filepath.Join(path, attachmentsDirName)
		if isRealDir(attachmentsDir) {
			w.addWatchDir(attachmentsDir)
		}
		return
	}
	if isAttachmentsDirPath(path) {
		w.addWatchDir(path)
	}
}

// addWatchDir subscribes to path and returns true iff a new subscription was
// added (false if the watch already existed or the watcher state was missing).
func (w *Watcher) addWatchDir(path string) bool {
	clean := filepath.Clean(path)

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fsw == nil || w.watchDirs == nil || !pathWithinDir(w.boardDir, clean) {
		return false
	}
	if _, ok := w.watchDirs[clean]; ok {
		return false
	}
	if err := w.fsw.Add(clean); err != nil {
		w.logger.Debug("watcher: add dir failed", "dir", clean, "err", err)
		return false
	}
	w.watchDirs[clean] = struct{}{}
	return true
}

func (w *Watcher) removeWatchDirAndChildren(path string) {
	clean := filepath.Clean(path)
	sep := string(filepath.Separator)

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fsw == nil || w.watchDirs == nil {
		return
	}
	for watched := range w.watchDirs {
		if watched != clean && !strings.HasPrefix(watched, clean+sep) {
			continue
		}
		_ = w.fsw.Remove(watched)
		delete(w.watchDirs, watched)
	}
}

func (w *Watcher) scheduleBoardReload() {
	w.mu.Lock()
	d := w.debouncer
	w.mu.Unlock()
	if d != nil {
		d.fire()
	}
}

func (w *Watcher) flushBoard() {
	w.emitter.Emit("board:reloaded")
}

// pump forwards the given fsnotify watcher's events onto out until ctx is
// cancelled. On exit it closes the underlying fsnotify.Watcher.
func pump(ctx context.Context, fsw *fsnotify.Watcher, out chan<- fsEvent) {
	defer func() { _ = fsw.Close() }()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-fsw.Events:
			if !ok {
				return
			}
			select {
			case out <- fsEvent{ev: ev}:
			case <-ctx.Done():
				return
			}
		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			select {
			case out <- fsEvent{err: err}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// isIgnored returns true if path is inside one of the ignored dir segments or
// matches an ignored basename.
func isIgnored(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return true
	}
	if strings.HasSuffix(base, ".tmp") || strings.Contains(base, ".tmp.") {
		return true
	}
	if _, ok := ignoredBasenames[base]; ok {
		return true
	}
	sep := string(filepath.Separator)
	for _, seg := range ignoredDirSegments {
		if strings.Contains(path, sep+seg+sep) || strings.HasSuffix(path, sep+seg) {
			return true
		}
	}
	return false
}

// taskIDFromPath strips the directory and `.md` suffix from a status-dir
// entry, or from a folder-form TASK.md path. Returns "" if the file is not a
// task markdown file. Filenames in ignoredBasenames are also rejected so
// BOARD.md never produces a task event.
func taskIDFromPath(path string) string {
	base := filepath.Base(path)
	if isIgnored(path) {
		return ""
	}
	if base == folderTaskFileName {
		id := filepath.Base(filepath.Dir(path))
		if isTaskIDName(id) && isStatusDirName(filepath.Base(filepath.Dir(filepath.Dir(path)))) {
			return id
		}
		return ""
	}
	if !strings.HasSuffix(base, ".md") || !isStatusDirName(filepath.Base(filepath.Dir(path))) {
		return ""
	}
	id := strings.TrimSuffix(base, ".md")
	if !isTaskIDName(id) {
		return ""
	}
	return id
}

func addFSWatch(fsw *fsnotify.Watcher, watchDirs map[string]struct{}, dir string) error {
	clean := filepath.Clean(dir)
	if _, ok := watchDirs[clean]; ok {
		return nil
	}
	if err := fsw.Add(clean); err != nil {
		return err
	}
	watchDirs[clean] = struct{}{}
	return nil
}

func isRealDir(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil || !os.IsNotExist(err)
}

func isTaskDirPath(path string) bool {
	return isTaskIDName(filepath.Base(path)) && isStatusDirName(filepath.Base(filepath.Dir(path)))
}

func isAttachmentsDirPath(path string) bool {
	return filepath.Base(path) == attachmentsDirName && isTaskDirPath(filepath.Dir(path))
}

func isStatusDirName(name string) bool {
	_, ok := statusDirSet[name]
	return ok
}

func isTaskIDName(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".md") {
		return false
	}
	dash := strings.LastIndex(name, "-")
	if dash <= 0 || dash == len(name)-1 {
		return false
	}
	for _, r := range name[:dash] {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	for _, r := range name[dash+1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func pathWithinDir(parent, child string) bool {
	if parent == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// --- debouncer ---

// debouncer fires fn once after every burst of fire() calls separated by less
// than window. Safe for concurrent fire().
type debouncer struct {
	window time.Duration
	fn     func()

	mu     sync.Mutex
	timer  *time.Timer
	closed bool
}

func newDebouncer(window time.Duration, fn func()) *debouncer {
	return &debouncer{window: window, fn: fn}
}

func (d *debouncer) fire() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.window, d.fn)
}

func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
