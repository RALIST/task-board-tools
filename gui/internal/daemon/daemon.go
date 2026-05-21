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
	"runtime"
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

// periodicRecoveryIntervalDefault is the steady-state stale recovery cadence.
// Startup recovery still runs immediately on Activate; this ticker covers
// runs that become stale while the GUI process stays alive.
const periodicRecoveryIntervalDefault = 60 * time.Second

const (
	cancelReasonBoardSwitch = "board switch"
	cancelReasonShutdown    = "shutdown"
)

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
	// ListActive returns every task in the active buckets (backlog,
	// ready, in-progress, code-review, and done). Used by startup
	// scan, recovery, and reconciliation.
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

type activeRunLister interface {
	ActiveTaskIDs() []string
}

// Recovery is the narrow surface the daemon needs to reconcile stale
// AgentStatus=running tasks at activation time (TB-60). Split off from
// Agent so the test fake can leave it nil for non-recovery scenarios.
type Recovery interface {
	// RecoverStale scans the board's .agent-state for tasks with
	// AgentStatus=running whose JSONL lacks a `finished` event and
	// whose PID is dead. Writes synthetic finished{lost/interrupted}
	// (or finished{cancelled} per the carve-out) and updates AgentStatus.
	RecoverStale(ctx context.Context, boardDir string) error
}

// Reconciler is optional deterministic housekeeping for staged autonomous
// transitions. It runs around the existing queue hooks; it is not a scheduler
// and does not enqueue work itself.
type Reconciler interface {
	ReconcileActive(ctx context.Context) error
	ReconcileTask(ctx context.Context, id string) error
}

type queuedTask struct {
	id         string
	generation uint64
}

// UntrackedRecovery is implemented by recovery services that can skip
// task/run pairs already owned by this daemon instance's AgentService.active
// map. The periodic tick uses this narrower path so it never races a live
// stream managed by the current process.
type UntrackedRecovery interface {
	RecoverStaleUntracked(ctx context.Context, boardDir string) error
}

// Options bundles construction-time configuration. Zero MaxWorkers is
// coerced to MaxWorkersDefault in New so callers can pass a fresh
// struct without remembering to set it.
type Options struct {
	Board      Board
	Agent      Agent
	Recovery   Recovery
	Reconciler Reconciler
	Logger     *slog.Logger
	MaxWorkers int
	// QueueBuffer overrides the work-channel capacity. Zero = default.
	QueueBuffer int
	// PeriodicRecoveryInterval controls the steady-state recovery cadence.
	// Zero = 60s. DisablePeriodicRecovery turns the ticker off while leaving
	// activation-time recovery intact.
	PeriodicRecoveryInterval time.Duration
	DisablePeriodicRecovery  bool
}

// Daemon coordinates the worker pool, active-set dedup, and lifecycle
// callbacks. Construction is cheap and side-effect-free; activation
// (Activate) does the file IO.
type Daemon struct {
	board      Board
	agent      Agent
	recovery   Recovery
	reconciler Reconciler
	logger     *slog.Logger
	queue      chan queuedTask
	rootCtx    context.Context
	rootCancel context.CancelFunc
	workersWG  sync.WaitGroup
	closeOnce  sync.Once

	workerLimitMu      sync.Mutex
	maxWorkers         int
	runningWorkers     int
	workerLimitChanged chan struct{}

	periodicRecoveryInterval time.Duration
	disablePeriodicRecovery  bool
	periodicMu               sync.Mutex
	periodicCancel           context.CancelFunc
	periodicDone             <-chan struct{}

	// activeMu guards active. A task ID is in active while it is
	// either sitting in the queue OR being executed by a worker.
	activeMu sync.Mutex
	active   map[string]struct{}
	// automationActive is the subset of active reserved by automation
	// coordinators that start runs through AgentService directly. These
	// reservations consume the same worker slot gate as daemon workers.
	automationActive map[string]struct{}

	// boardMu guards boardDir + activated. Switched on Activate /
	// cleared on Deactivate.
	boardMu          sync.Mutex
	boardDir         string
	activated        bool
	generation       uint64
	activationCtx    context.Context
	activationCancel context.CancelCauseFunc
	startupScanTimer *time.Timer
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
	recoveryInterval := opts.PeriodicRecoveryInterval
	if recoveryInterval <= 0 {
		recoveryInterval = periodicRecoveryIntervalDefault
	}

	ctx, cancel := context.WithCancel(context.Background())
	d := &Daemon{
		board:                    opts.Board,
		agent:                    opts.Agent,
		recovery:                 opts.Recovery,
		reconciler:               opts.Reconciler,
		logger:                   logger,
		maxWorkers:               mw,
		workerLimitChanged:       make(chan struct{}),
		queue:                    make(chan queuedTask, qb),
		rootCtx:                  ctx,
		rootCancel:               cancel,
		active:                   make(map[string]struct{}),
		automationActive:         make(map[string]struct{}),
		periodicRecoveryInterval: recoveryInterval,
		disablePeriodicRecovery:  opts.DisablePeriodicRecovery,
	}

	d.workersWG.Add(1)
	go d.worker(0)
	logger.Info("daemon: dispatcher spawned", "max_workers", mw, "queue_buffer", qb)
	return d
}

// MaxWorkers reports the configured semaphore capacity. Useful for
// telemetry and tests.
func (d *Daemon) MaxWorkers() int {
	d.workerLimitMu.Lock()
	defer d.workerLimitMu.Unlock()
	return d.maxWorkers
}

// SetMaxWorkers changes the live concurrency budget for queued daemon work.
// Already-running jobs are not cancelled when the budget shrinks; new jobs wait
// until the number of active worker slots falls below the updated value.
func (d *Daemon) SetMaxWorkers(n int) {
	if n < 1 {
		n = 1
	}
	maxLimit := runtime.NumCPU()
	if maxLimit < 1 {
		maxLimit = 1
	}
	if n > maxLimit {
		n = maxLimit
	}
	d.workerLimitMu.Lock()
	if d.maxWorkers != n {
		d.maxWorkers = n
	}
	d.signalWorkerLimitChangedLocked()
	d.workerLimitMu.Unlock()
	d.logger.Info("daemon: max workers updated", "max_workers", n)
}

// NotifyAgentActiveChanged wakes dispatcher waiters whose capacity check
// depends on AgentService.ActiveTaskIDs rather than a daemon-owned slot.
func (d *Daemon) NotifyAgentActiveChanged() {
	d.workerLimitMu.Lock()
	d.signalWorkerLimitChangedLocked()
	d.workerLimitMu.Unlock()
}

// ActiveTaskIDs returns a snapshot of task IDs currently queued or running in
// this daemon instance. Auto-implement uses it to avoid scheduling more ready
// work than the worker pool can actually run.
func (d *Daemon) ActiveTaskIDs() []string {
	d.activeMu.Lock()
	defer d.activeMu.Unlock()

	ids := make([]string, 0, len(d.active))
	for id := range d.active {
		ids = append(ids, id)
	}
	return ids
}

// SetPeriodicRecoveryEnabled toggles the steady-state stale-recovery ticker at
// runtime. Startup recovery still runs during Activate regardless of this
// setting.
func (d *Daemon) SetPeriodicRecoveryEnabled(enabled bool) {
	d.periodicMu.Lock()
	d.disablePeriodicRecovery = !enabled
	d.periodicMu.Unlock()

	if !enabled {
		d.stopPeriodicRecovery()
		return
	}

	d.boardMu.Lock()
	boardDir := d.boardDir
	activated := d.activated
	d.boardMu.Unlock()
	if activated {
		d.startPeriodicRecovery(boardDir)
	}
}

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
	return d.ActivateWithStartupGrace(ctx, boardDir, 0)
}

func (d *Daemon) ActivateWithStartupGrace(ctx context.Context, boardDir string, grace time.Duration) error {
	if boardDir == "" {
		return errors.New("daemon: empty boardDir")
	}
	activationCtx, activationCancel := context.WithCancelCause(d.rootCtx)
	d.boardMu.Lock()
	if d.activated {
		d.boardMu.Unlock()
		activationCancel(nil)
		return errors.New("daemon: already activated; call Deactivate first")
	}
	d.generation++
	d.boardDir = boardDir
	d.activated = true
	d.activationCtx = activationCtx
	d.activationCancel = activationCancel
	generation := d.generation
	d.boardMu.Unlock()

	// Stale-running recovery FIRST so the subsequent scan reads a
	// post-reconciled view of the board. TB-57's ordering AC.
	if d.recovery != nil {
		if err := d.recovery.RecoverStale(ctx, boardDir); err != nil {
			d.logger.Warn("daemon: stale recovery failed; continuing", "err", err)
		}
	}

	if d.reconciler != nil {
		if err := d.reconciler.ReconcileActive(ctx); err != nil {
			d.logger.Warn("daemon: reconciliation failed; continuing", "err", err)
		}
	}

	if grace <= 0 {
		if err := d.scanQueued(ctx); err != nil {
			// Scan failures are non-fatal — the watcher event sink (TB-58)
			// will pick up tasks on the next mutation.
			d.logger.Warn("daemon: startup scan failed; continuing", "err", err)
		}
	} else {
		d.scheduleStartupScan(boardDir, generation, grace)
	}
	d.startPeriodicRecovery(boardDir)
	return nil
}

func (d *Daemon) scheduleStartupScan(boardDir string, generation uint64, grace time.Duration) {
	d.boardMu.Lock()
	if d.startupScanTimer != nil {
		d.startupScanTimer.Stop()
	}
	d.startupScanTimer = time.AfterFunc(grace, func() {
		d.boardMu.Lock()
		if !d.activated || d.boardDir != boardDir || d.generation != generation {
			d.boardMu.Unlock()
			return
		}
		ctx := d.activationCtx
		d.startupScanTimer = nil
		d.boardMu.Unlock()

		if ctx == nil {
			return
		}
		if err := d.scanQueued(ctx); err != nil {
			d.logger.Warn("daemon: startup scan failed; continuing", "err", err)
		}
	})
	d.boardMu.Unlock()
	d.logger.Info("daemon: startup scan delayed", "grace", grace)
}

func (d *Daemon) startupGraceActive() bool {
	d.boardMu.Lock()
	defer d.boardMu.Unlock()
	return d.activated && d.startupScanTimer != nil
}

func (d *Daemon) clearStartupScanTimerLocked() {
	if d.startupScanTimer != nil {
		d.startupScanTimer.Stop()
		d.startupScanTimer = nil
	}
}

// Deactivate cancels work owned by the currently-open board and resets
// boardDir. Called when the user switches boards. Workers remain alive for a
// later Activate, but in-flight work receives a "board switch" cancellation
// cause and queued old-board items are drained before the switch completes.
func (d *Daemon) Deactivate() error {
	return d.deactivate(cancelReasonBoardSwitch)
}

func (d *Daemon) deactivate(reason string) error {
	d.stopPeriodicRecovery()

	d.boardMu.Lock()
	if !d.activated {
		d.boardMu.Unlock()
		return nil
	}
	cancel := d.activationCancel
	d.clearStartupScanTimerLocked()
	d.activated = false
	d.boardDir = ""
	d.activationCtx = nil
	d.activationCancel = nil
	d.boardMu.Unlock()

	if cancel != nil {
		cancel(errors.New(reason))
	}
	if !d.waitActiveEmpty(shutdownGrace) {
		d.logger.Warn("daemon: deactivate grace expired; some work still active")
		return errShutdownGraceExpired
	}

	d.activeMu.Lock()
	d.active = make(map[string]struct{})
	d.automationActive = make(map[string]struct{})
	d.activeMu.Unlock()
	return nil
}

func (d *Daemon) waitActiveEmpty(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		d.activeMu.Lock()
		active := len(d.active)
		d.activeMu.Unlock()
		if active == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (d *Daemon) startPeriodicRecovery(boardDir string) {
	if d.recovery == nil {
		return
	}
	recovery, ok := d.recovery.(UntrackedRecovery)
	if !ok {
		d.logger.Warn("daemon: periodic recovery disabled; recovery service lacks untracked recovery")
		return
	}

	d.periodicMu.Lock()
	if d.disablePeriodicRecovery || d.periodicCancel != nil {
		d.periodicMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(d.rootCtx)
	done := make(chan struct{})
	d.periodicCancel = cancel
	d.periodicDone = done
	interval := d.periodicRecoveryInterval
	d.periodicMu.Unlock()

	go func() {
		defer close(done)
		d.periodicRecoveryLoop(ctx, boardDir, interval, recovery)
	}()
}

func (d *Daemon) stopPeriodicRecovery() {
	d.periodicMu.Lock()
	cancel := d.periodicCancel
	done := d.periodicDone
	d.periodicCancel = nil
	d.periodicDone = nil
	d.periodicMu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		<-done
	}
}

func (d *Daemon) periodicRecoveryLoop(ctx context.Context, boardDir string, interval time.Duration, recovery UntrackedRecovery) {
	if interval <= 0 {
		interval = periodicRecoveryIntervalDefault
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := recovery.RecoverStaleUntracked(ctx, boardDir); err != nil {
				d.logger.Warn("daemon: periodic stale recovery failed; continuing", "err", err)
			}
		}
	}
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
		err = errors.Join(err, d.deactivate(cancelReasonShutdown))
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
			err = errors.Join(err, errShutdownGraceExpired)
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
	generation, activationCtx, ok := d.activationState()
	if !ok {
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
	case d.queue <- queuedTask{id: taskID, generation: generation}:
		return true, nil
	case <-activationCtx.Done():
		d.release(taskID)
		return false, ErrNotActivated
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

// TryReserveAutomationRun reserves a shared worker slot for coordinator-owned
// runs that bypass the daemon queue and start through AgentService directly.
// The reservation is intentionally non-blocking: callers should skip this scan
// when capacity is full and try again on the next watcher/settings event.
func (d *Daemon) TryReserveAutomationRun(taskID string) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false
	}
	d.activeMu.Lock()
	if _, ok := d.active[taskID]; ok {
		d.activeMu.Unlock()
		return false
	}
	d.activeMu.Unlock()

	if d.agent != nil && d.agent.HasActiveRun(taskID) {
		return false
	}
	extraActive := d.unreservedAgentActiveCount()
	if d.activeTaskCount()+extraActive >= d.MaxWorkers() {
		return false
	}
	if !d.tryAcquireWorkerSlotNow(extraActive) {
		return false
	}

	maxWorkers := d.MaxWorkers()
	d.activeMu.Lock()
	if _, ok := d.active[taskID]; ok {
		d.activeMu.Unlock()
		d.releaseWorkerSlot()
		return false
	}
	if len(d.active) >= maxWorkers {
		d.activeMu.Unlock()
		d.releaseWorkerSlot()
		return false
	}
	d.active[taskID] = struct{}{}
	d.automationActive[taskID] = struct{}{}
	d.activeMu.Unlock()
	return true
}

func (d *Daemon) activeTaskCount() int {
	d.activeMu.Lock()
	defer d.activeMu.Unlock()
	return len(d.active)
}

func (d *Daemon) unreservedAgentActiveCount() int {
	lister, ok := d.agent.(activeRunLister)
	if !ok {
		return 0
	}
	ids := lister.ActiveTaskIDs()
	if len(ids) == 0 {
		return 0
	}
	d.activeMu.Lock()
	defer d.activeMu.Unlock()
	count := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, reserved := d.active[id]; !reserved {
			count++
		}
	}
	return count
}

// ReleaseAutomationRun releases a direct-run reservation created by
// TryReserveAutomationRun. Calling it for a daemon-owned task is a no-op, which
// keeps the main event hook simple: every agent:run-finished event can flow
// through this method.
func (d *Daemon) ReleaseAutomationRun(taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	d.activeMu.Lock()
	_, ok := d.automationActive[taskID]
	if ok {
		delete(d.automationActive, taskID)
		delete(d.active, taskID)
	}
	d.activeMu.Unlock()
	if ok {
		d.releaseWorkerSlot()
	}
}

func (d *Daemon) acquireWorkerSlot(ctx context.Context, extraActive func() int) bool {
	for {
		d.workerLimitMu.Lock()
		extra := 0
		if extraActive != nil {
			extra = extraActive()
		}
		if d.runningWorkers+extra < d.maxWorkers {
			d.runningWorkers++
			d.workerLimitMu.Unlock()
			return true
		}
		changed := d.workerLimitChanged
		d.workerLimitMu.Unlock()

		select {
		case <-ctx.Done():
			return false
		case <-d.rootCtx.Done():
			return false
		case <-changed:
		}
	}
}

func (d *Daemon) tryAcquireWorkerSlotNow(extraActive int) bool {
	d.workerLimitMu.Lock()
	defer d.workerLimitMu.Unlock()
	if d.runningWorkers+extraActive >= d.maxWorkers {
		return false
	}
	d.runningWorkers++
	return true
}

func (d *Daemon) releaseWorkerSlot() {
	d.workerLimitMu.Lock()
	if d.runningWorkers > 0 {
		d.runningWorkers--
	}
	d.signalWorkerLimitChangedLocked()
	d.workerLimitMu.Unlock()
}

func (d *Daemon) signalWorkerLimitChangedLocked() {
	close(d.workerLimitChanged)
	d.workerLimitChanged = make(chan struct{})
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

func (d *Daemon) activationState() (uint64, context.Context, bool) {
	d.boardMu.Lock()
	defer d.boardMu.Unlock()
	if !d.activated || d.activationCtx == nil {
		return 0, nil, false
	}
	return d.generation, d.activationCtx, true
}

func (d *Daemon) activationContextFor(generation uint64) (context.Context, bool) {
	d.boardMu.Lock()
	defer d.boardMu.Unlock()
	if !d.activated || d.generation != generation || d.activationCtx == nil {
		return nil, false
	}
	return d.activationCtx, true
}

func (d *Daemon) isCurrentGeneration(generation uint64) bool {
	d.boardMu.Lock()
	defer d.boardMu.Unlock()
	return d.activated && d.generation == generation
}

// worker is the FIFO dispatcher: it reads one queued task at a time, waits for
// a live worker slot, then launches the actual run in its own goroutine. The
// dispatcher never reads task N+1 while task N is still waiting for capacity, so
// runtime max_workers shrink cannot let later tasks overtake earlier ones.
func (d *Daemon) worker(idx int) {
	defer d.workersWG.Done()
	logger := d.logger.With("dispatcher", idx)
	for {
		select {
		case <-d.rootCtx.Done():
			logger.Debug("daemon: dispatcher exited (ctx cancelled)")
			return
		case task := <-d.queue:
			d.dispatchOne(logger, task)
		}
	}
}

func (d *Daemon) dispatchOne(logger *slog.Logger, task queuedTask) {
	taskID := task.id
	runCtx, ok := d.activationContextFor(task.generation)
	if !ok {
		logger.Debug("daemon: dropping stale queued task", "task", taskID)
		d.release(taskID)
		return
	}
	if !d.acquireWorkerSlot(runCtx, d.unreservedAgentActiveCount) {
		logger.Debug("daemon: dropping queued task after worker-slot wait cancelled", "task", taskID)
		d.release(taskID)
		return
	}
	d.workersWG.Add(1)
	go func() {
		defer d.workersWG.Done()
		defer d.releaseWorkerSlot()
		d.runOne(logger, task)
	}()
}

func (d *Daemon) runOne(logger *slog.Logger, task queuedTask) {
	taskID := task.id
	defer d.release(taskID)
	runCtx, ok := d.activationContextFor(task.generation)
	if !ok {
		logger.Debug("daemon: dropping stale queued task", "task", taskID)
		return
	}
	if d.agent == nil {
		logger.Warn("daemon: no agent service; dropping", "task", taskID)
		return
	}
	if d.reconciler != nil {
		if err := d.reconciler.ReconcileTask(runCtx, taskID); err != nil {
			logger.Warn("daemon: pre-run reconciliation failed; continuing", "task", taskID, "err", err)
		}
	}
	status, err := d.agent.RunQueuedAgentSync(runCtx, taskID)
	if err != nil {
		logger.Warn("daemon: run failed", "task", taskID, "err", err)
		return
	}
	logger.Info("daemon: run finished", "task", taskID, "status", status)
	if !d.isCurrentGeneration(task.generation) {
		logger.Debug("daemon: skipping stale post-run reconciliation", "task", taskID)
		return
	}
	if d.reconciler != nil {
		if err := d.reconciler.ReconcileTask(context.Background(), taskID); err != nil {
			logger.Warn("daemon: post-run reconciliation failed", "task", taskID, "err", err)
		}
	}
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
	if d.startupGraceActive() {
		return false, nil
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
	if d.startupGraceActive() {
		return 0, nil
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

// ReconcileActive runs the optional reconciliation hook for callers that need
// deterministic board repair before scanning for queued work.
func (d *Daemon) ReconcileActive(ctx context.Context) error {
	if !d.isActivated() {
		return ErrNotActivated
	}
	if d.reconciler == nil {
		return nil
	}
	return d.reconciler.ReconcileActive(ctx)
}

// ReconcileTask runs the optional per-task reconciliation hook.
func (d *Daemon) ReconcileTask(ctx context.Context, id string) error {
	if !d.isActivated() {
		return ErrNotActivated
	}
	if d.reconciler == nil {
		return nil
	}
	return d.reconciler.ReconcileTask(ctx, id)
}
