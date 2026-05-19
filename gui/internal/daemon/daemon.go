// Package daemon embeds a background runner in the GUI process that
// auto-picks up tasks whose AgentStatus=queued and whose Agent field is
// set. It owns:
//
//   - a worker pool reading from an in-process buffered channel,
//   - in-memory active-set dedup keyed by task_id (cross-checked with
//     AgentService.HasActiveRun for manual UI runs),
//   - stale-running recovery on Activate (TB-60), with the cancelled
//     carve-out (TB-61),
//   - an emitter sink that re-enqueues on watcher events (TB-58),
//   - graceful shutdown via context cancellation + 5s grace (TB-62).
//
// Lifecycle:
//
//	New(opts)                    -- workers spawned, idle (no IO)
//	d.Activate(ctx, boardDir)   -- recovery → register sink → startup scan
//	d.Deactivate()                -- drain workers; clear active-set
//	d.Close()                     -- cancel ctx; 5s grace; return
//
// Activate is invoked from SettingsService.OpenBoard. The daemon stays
// alive across board switches; Deactivate + Activate gives a clean
// rebind without restarting the whole GUI.
package daemon

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// shutdownGrace is how long Close waits for in-flight workers to flush
// their JSONL finished events before returning. 5s matches F5.4.
const shutdownGrace = 5 * time.Second

// queueBufferDefault is the depth of the internal work channel. Bigger
// buffer = more queued events absorbed during a recovery burst without
// dropping. 256 is enough for any reasonable board.
const queueBufferDefault = 256

// AgentTask is the minimum surface from a Board task the daemon needs
// to decide whether to enqueue. Defined here (not imported from
// gui/app.Task) so tests can build fake board snapshots without pulling
// the full service struct.
type AgentTask struct {
	ID          string
	Agent       string
	AgentStatus string
}

// Board is the narrow read-side surface the daemon consumes from
// BoardService. Test fakes satisfy this interface; the production
// glue (an adapter in gui/main.go) wraps BoardService.LoadBoard +
// GetTask.
type Board interface {
	// ListActive returns every task in the active buckets (backlog +
	// in-progress + done). Used by both startup scan and recovery.
	ListActive(ctx context.Context) ([]AgentTask, error)
	// GetTask returns the latest metadata for a single task. Used by
	// watcher events that only know the ID.
	GetTask(ctx context.Context, id string) (AgentTask, error)
}

// Agent is the narrow surface the daemon needs from AgentService. The
// production wiring satisfies this via *AgentService.RunQueuedAgentSync
// + HasActiveRun. Tests use fakes that implement Run synchronously.
type Agent interface {
	// RunQueuedAgentSync runs the (already queued) task to terminal
	// status, blocking until done. Returns terminal status string.
	RunQueuedAgentSync(ctx context.Context, id string) (string, error)
	// HasActiveRun reports whether the AgentService is tracking an
	// in-flight run for this task (the manual UI path). The daemon
	// uses this so a watcher event doesn't duplicate a manual run.
	HasActiveRun(id string) bool
}

// Recovery is the narrow surface the daemon needs to reconcile stale
// AgentStatus=running tasks at activation time (TB-60). Split off from
// Agent so the test fake can leave it nil for non-recovery scenarios.
type Recovery interface {
	// RecoverStale scans the board's .agent-state for tasks with
	// AgentStatus=running whose JSONL lacks a `finished` event and
	// whose PID is dead. Writes synthetic finished{failed} (or
	// finished{cancelled} per the carve-out) and updates AgentStatus.
	RecoverStale(ctx context.Context, boardDir string) error
}

// Options bundles construction-time configuration. Zero MaxWorkers is
// coerced to MaxWorkersDefault in New so callers can pass a fresh
// struct without remembering to set it.
type Options struct {
	Board      Board
	Agent      Agent
	Recovery   Recovery
	Logger     *slog.Logger
	MaxWorkers int
	// QueueBuffer overrides the work-channel capacity. Zero = default.
	QueueBuffer int
}

// Daemon coordinates the worker pool, active-set dedup, and lifecycle
// callbacks. Construction is cheap and side-effect-free; activation
// (Activate) does the file IO.
type Daemon struct {
	board       Board
	agent       Agent
	recovery    Recovery
	logger      *slog.Logger
	maxWorkers  int
	queue       chan string
	rootCtx     context.Context
	rootCancel  context.CancelFunc
	workersWG   sync.WaitGroup
	closeOnce   sync.Once

	// activeMu guards active. A task ID is in active while it is
	// either sitting in the queue OR being executed by a worker.
	activeMu sync.Mutex
	active   map[string]struct{}

	// boardMu guards boardDir + activated. Switched on Activate /
	// cleared on Deactivate.
	boardMu   sync.Mutex
	boardDir  string
	activated bool
}

// ErrNotActivated is returned by Enqueue (and other state-dependent
// methods) when called before Activate. The watcher sink uses it to
// drop events that arrive before a board is open.
var ErrNotActivated = errors.New("daemon: not activated")

// New constructs a Daemon with worker goroutines started and idle. No
// file IO happens until Activate is called.
func New(opts Options) *Daemon {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("component", "daemon")

	mw := opts.MaxWorkers
	if mw < 1 {
		mw = 1
	}
	qb := opts.QueueBuffer
	if qb < 1 {
		qb = queueBufferDefault
	}

	ctx, cancel := context.WithCancel(context.Background())
	d := &Daemon{
		board:      opts.Board,
		agent:      opts.Agent,
		recovery:   opts.Recovery,
		logger:     logger,
		maxWorkers: mw,
		queue:      make(chan string, qb),
		rootCtx:    ctx,
		rootCancel: cancel,
		active:     make(map[string]struct{}),
	}

	for i := 0; i < mw; i++ {
		d.workersWG.Add(1)
		go d.worker(i)
	}
	logger.Info("daemon: workers spawned", "max_workers", mw, "queue_buffer", qb)
	return d
}

// MaxWorkers reports the configured semaphore capacity. Useful for
// telemetry and tests.
func (d *Daemon) MaxWorkers() int { return d.maxWorkers }

// BoardDir returns the active board directory, or "" if not yet
// activated. Test-only and metrics use.
func (d *Daemon) BoardDir() string {
	d.boardMu.Lock()
	defer d.boardMu.Unlock()
	return d.boardDir
}

// Activate is the post-OpenBoard hook the daemon registers with
// SettingsService. It runs (in order):
//
//  1. stale-running recovery (TB-60), which never overrides
//     AgentStatus=cancelled per TB-61 carve-out
//  2. (caller is expected to have registered the watcher sink
//     BEFORE invoking Activate — TB-58 sequencing)
//  3. startup queue scan (TB-57) — enumerate AgentStatus=queued
//     tasks and enqueue
//
// Re-activation: when the user opens a different board, Deactivate is
// called first to drain the previous board's in-flight runs before
// Activate runs again.
func (d *Daemon) Activate(ctx context.Context, boardDir string) error {
	if boardDir == "" {
		return errors.New("daemon: empty boardDir")
	}
	d.boardMu.Lock()
	if d.activated {
		d.boardMu.Unlock()
		return errors.New("daemon: already activated; call Deactivate first")
	}
	d.boardDir = boardDir
	d.activated = true
	d.boardMu.Unlock()

	// Stale-running recovery FIRST so the subsequent scan reads a
	// post-reconciled view of the board. TB-57's ordering AC.
	if d.recovery != nil {
		if err := d.recovery.RecoverStale(ctx, boardDir); err != nil {
			d.logger.Warn("daemon: stale recovery failed; continuing", "err", err)
		}
	}

	// Startup queue scan (TB-57).
	if err := d.scanQueued(ctx); err != nil {
		// Scan failures are non-fatal — the watcher event sink (TB-58)
		// will pick up tasks on the next mutation.
		d.logger.Warn("daemon: startup scan failed; continuing", "err", err)
	}
	return nil
}

// Deactivate drains the active set and resets boardDir. Called when
// the user switches boards (or before Close). It does NOT cancel the
// root context — Close does that. Workers stay parked until Close or
// a new Activate.
func (d *Daemon) Deactivate() error {
	d.boardMu.Lock()
	if !d.activated {
		d.boardMu.Unlock()
		return nil
	}
	d.activated = false
	d.boardDir = ""
	d.boardMu.Unlock()

	// Drop in-memory enqueue state — any in-flight worker run will
	// reach its own terminal status and release naturally.
	d.activeMu.Lock()
	d.active = make(map[string]struct{})
	d.activeMu.Unlock()
	return nil
}

// Close cancels the daemon's root context (propagating to any worker's
// RunQueuedAgentSync ctx) and waits up to shutdownGrace for the workers
// to flush their finished JSONL records. Any worker still running
// after the grace expires is logged at WARN; whatever state it left
// behind will be reconciled by the next-start recovery (TB-60).
//
// Close is idempotent. Workers exit when rootCtx is done; we never
// close the queue channel — doing so would race a concurrent Enqueue's
// select{send} branch and produce a "send on closed channel" panic.
// Tradeoff: the queue may have unread task IDs at exit; those are
// dropped, and the next-start recovery (or the watcher event sink on
// next launch) picks them up.
func (d *Daemon) Close() error {
	var err error
	d.closeOnce.Do(func() {
		_ = d.Deactivate()
		d.rootCancel()

		done := make(chan struct{})
		go func() {
			d.workersWG.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(shutdownGrace):
			d.logger.Warn("daemon: shutdown grace expired; some workers still active")
			// Workers are blocked on RunQueuedAgentSync; they will
			// eventually exit but we don't block the caller.
			err = errShutdownGraceExpired
		}
	})
	return err
}

// errShutdownGraceExpired is reported by Close when the workers did
// not all return within the 5s window. Exposed only via Close's return.
var errShutdownGraceExpired = errors.New("daemon: shutdown grace expired")

// Enqueue is the public enqueue path used by the startup scan (TB-57)
// and the watcher event sink (TB-58). Idempotent: a task already in
// the active set (queued or running) is dropped without error.
//
// Returns ErrNotActivated if called before Activate. Returns true when
// the call resulted in a new enqueue.
func (d *Daemon) Enqueue(taskID string) (bool, error) {
	if !d.isActivated() {
		return false, ErrNotActivated
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false, errors.New("daemon: empty task ID")
	}
	if !d.tryEnqueue(taskID) {
		return false, nil
	}
	select {
	case d.queue <- taskID:
		return true, nil
	case <-d.rootCtx.Done():
		d.release(taskID)
		return false, d.rootCtx.Err()
	default:
		// Queue full — release dedup slot and let the next watcher
		// event re-trigger. The dedup is conservative; a dropped
		// enqueue is recoverable.
		d.release(taskID)
		d.logger.Warn("daemon: queue full; dropping enqueue", "task", taskID)
		return false, nil
	}
}

// tryEnqueue inserts taskID into the active set iff not already present,
// AND AgentService.HasActiveRun reports no in-flight manual run.
// Returns true when the daemon now owns the slot.
func (d *Daemon) tryEnqueue(taskID string) bool {
	if d.agent != nil && d.agent.HasActiveRun(taskID) {
		return false
	}
	d.activeMu.Lock()
	defer d.activeMu.Unlock()
	if _, ok := d.active[taskID]; ok {
		return false
	}
	d.active[taskID] = struct{}{}
	return true
}

// release drops taskID from the active set. Called by workers in a
// defer so terminal-status runs always free the slot.
func (d *Daemon) release(taskID string) {
	d.activeMu.Lock()
	delete(d.active, taskID)
	d.activeMu.Unlock()
}

// IsActive reports whether the given task is currently queued or
// running (daemon's view). Useful for tests; not surfaced to Wails.
func (d *Daemon) IsActive(taskID string) bool {
	d.activeMu.Lock()
	_, ok := d.active[taskID]
	d.activeMu.Unlock()
	return ok
}

func (d *Daemon) isActivated() bool {
	d.boardMu.Lock()
	defer d.boardMu.Unlock()
	return d.activated
}

// worker runs a worker goroutine: pull a task ID off the queue, call
// the executor, release the slot, repeat. Exits when the root ctx is
// done (Close calls rootCancel). The queue channel is intentionally
// never closed — see Close for the rationale.
func (d *Daemon) worker(idx int) {
	defer d.workersWG.Done()
	logger := d.logger.With("worker", idx)
	for {
		select {
		case <-d.rootCtx.Done():
			logger.Debug("daemon: worker exited (ctx cancelled)")
			return
		case id := <-d.queue:
			d.runOne(logger, id)
		}
	}
}

func (d *Daemon) runOne(logger *slog.Logger, taskID string) {
	defer d.release(taskID)
	if d.agent == nil {
		logger.Warn("daemon: no agent service; dropping", "task", taskID)
		return
	}
	status, err := d.agent.RunQueuedAgentSync(d.rootCtx, taskID)
	if err != nil {
		logger.Warn("daemon: run failed", "task", taskID, "err", err)
		return
	}
	logger.Info("daemon: run finished", "task", taskID, "status", status)
}

// scanQueued enumerates the active board and enqueues every task whose
// AgentStatus is "queued" and Agent is non-empty. Called from Activate
// (after recovery, after the watcher sink is registered by the caller).
func (d *Daemon) scanQueued(ctx context.Context) error {
	if d.board == nil {
		return errors.New("daemon: no board service")
	}
	tasks, err := d.board.ListActive(ctx)
	if err != nil {
		return err
	}
	enqueued := 0
	for _, t := range tasks {
		if !isReadyForDaemon(t) {
			if t.AgentStatus == "queued" && t.Agent == "" {
				d.logger.Warn("daemon: queued task without agent; skipping",
					"task", t.ID)
			}
			continue
		}
		ok, err := d.Enqueue(t.ID)
		if err != nil {
			d.logger.Warn("daemon: enqueue failed", "task", t.ID, "err", err)
			continue
		}
		if ok {
			enqueued++
		}
	}
	d.logger.Info("daemon: startup scan complete", "enqueued", enqueued)
	return nil
}

// isReadyForDaemon returns true when a task has AgentStatus=queued AND
// a non-empty Agent field. Both must hold; an explicit guard in the
// watcher and scan paths keeps the dedup cheap.
//
// TB-182 (`needs-user`) is naturally filtered here: the daemon only
// enqueues `queued` tasks, so a `needs-user` AgentStatus is skipped by
// construction. Auto-groom / auto-implement entry points (TB-174 /
// TB-179) consume this same predicate so they get the skip behavior for
// free.
func isReadyForDaemon(t AgentTask) bool {
	return t.AgentStatus == "queued" && strings.TrimSpace(t.Agent) != ""
}

// IsAutomationEligible mirrors isReadyForDaemon's intent for future
// automation entry points (auto-groom TB-174, auto-implement TB-179)
// that need a single predicate to skip tasks that are not in a runnable
// state. `needs-user` returns false so unresolved user-attention tasks
// never enter an automated retry loop.
func IsAutomationEligible(t AgentTask) bool {
	if t.AgentStatus == "needs-user" {
		return false
	}
	return isReadyForDaemon(t)
}

// EnqueueIfReady is the watcher-sink helper: GetTask the latest
// metadata and Enqueue iff the task is ready (queued + agent). Returns
// (true, nil) when the call resulted in an enqueue.
func (d *Daemon) EnqueueIfReady(ctx context.Context, id string) (bool, error) {
	if !d.isActivated() {
		return false, ErrNotActivated
	}
	if d.board == nil {
		return false, errors.New("daemon: no board service")
	}
	t, err := d.board.GetTask(ctx, id)
	if err != nil {
		return false, err
	}
	if !isReadyForDaemon(t) {
		return false, nil
	}
	return d.Enqueue(t.ID)
}

// RescanActive scans the active board (used by the watcher sink when
// it receives a board:reloaded event — the atomic-rename path the CLI
// uses for every metadata edit). Returns the count of newly enqueued
// tasks.
func (d *Daemon) RescanActive(ctx context.Context) (int, error) {
	if !d.isActivated() {
		return 0, ErrNotActivated
	}
	if d.board == nil {
		return 0, errors.New("daemon: no board service")
	}
	tasks, err := d.board.ListActive(ctx)
	if err != nil {
		return 0, err
	}
	enqueued := 0
	for _, t := range tasks {
		if !isReadyForDaemon(t) {
			continue
		}
		ok, err := d.Enqueue(t.ID)
		if err != nil {
			continue
		}
		if ok {
			enqueued++
		}
	}
	return enqueued, nil
}
