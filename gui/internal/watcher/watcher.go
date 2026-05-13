// Package watcher wraps fsnotify to turn board-directory filesystem activity
// into two coarse-grained Wails events:
//
//   - "board:reloaded"    — column membership changed (Create / Remove / Rename)
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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// statusDirs are the four directories a board may contain. Missing dirs are
// silently skipped on attach — a fresh board may have only some of them.
var statusDirs = []string{"backlog", "in-progress", "done", "archive"}

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

	mu        sync.Mutex
	fsw       *fsnotify.Watcher
	cancelPmp context.CancelFunc
	debouncer *debouncer
	boardDir  string
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
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	added := 0
	for _, d := range statusDirs {
		full := filepath.Join(boardDir, d)
		if err := fsw.Add(full); err != nil {
			w.logger.Debug("watcher: skip status dir", "dir", full, "err", err)
			continue
		}
		added++
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
	w.mu.Unlock()

	// Cancelling the old pump's context also closes its fsnotify.Watcher
	// (see pump's defer), and stops the old debouncer cleanly.
	if oldCancel != nil {
		oldCancel()
	}
	if oldDeb != nil {
		oldDeb.stop()
	}
	w.logger.Info("watcher: attached", "boardDir", boardDir, "dirs", added)
	return nil
}

func (w *Watcher) detach() {
	w.mu.Lock()
	cancel := w.cancelPmp
	deb := w.debouncer
	w.cancelPmp = nil
	w.debouncer = nil
	w.fsw = nil
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
	switch {
	case ev.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0:
		w.scheduleBoardReload()
	case ev.Op&fsnotify.Write != 0:
		if id := taskIDFromPath(ev.Name); id != "" {
			w.emitter.Emit("task:updated:"+id, id)
		}
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
	if _, ok := ignoredBasenames[filepath.Base(path)]; ok {
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
// entry. Returns "" if the file is not a markdown task. Filenames in
// ignoredBasenames are also rejected so BOARD.md never produces a task event.
func taskIDFromPath(path string) string {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".md") {
		return ""
	}
	if _, ignored := ignoredBasenames[base]; ignored {
		return ""
	}
	return strings.TrimSuffix(base, ".md")
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
