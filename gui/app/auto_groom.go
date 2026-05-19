package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

// statusDirs mirrors the canonical kanban columns. Local copy keeps
// auto_groom self-contained without exposing internal/agent vars.
var autoGroomStatusDirs = []string{"backlog", "ready", "in-progress", "code-review", "done", "archive"}

// scanDebounce coalesces bursts of watcher events into one scan.
const scanDebounce = 250 * time.Millisecond

// settleJitter is added to the deferred rescan tick that fires when a
// task's settle window expires. Tiny but non-zero so two tasks expiring
// in lockstep don't both wake the coordinator on the same nanosecond.
const settleJitter = 50 * time.Millisecond

// AutoGroomStatus is the snapshot the Wails-bound Status() returns. The
// frontend uses it to render the no-default-agent warning and the
// per-task settle countdown without polling the JSONL files itself.
type AutoGroomStatus struct {
	Enabled            bool              `json:"enabled"`
	DefaultAgent       string            `json:"default_agent"`
	NeedsDefaultAgent  bool              `json:"needs_default_agent"`
	SettleMinutes      int               `json:"settle_minutes"`
	LastScanAt         string            `json:"last_scan_at,omitempty"`
	LastSkipReasons    map[string]string `json:"last_skip_reasons,omitempty"`
	SettleEligibleAtMs map[string]int64  `json:"settle_eligible_at_ms,omitempty"`
}

// AutoGroomCoordinator drives the TB-174 auto-groom loop: it watches the
// triage candidates returned by BoardService, dedupes them against the
// per-task .agent-state.jsonl history, respects the user-configured
// settle window, and queues `mode=groom` runs through AgentService's
// existing groom lifecycle. Post-groom success → `tb ready <ID>` only if
// triage is clean.
//
// Wiring: parallel to *daemon.Daemon. The same OnBoardOpened hook that
// activates the daemon should call Activate(boardDir); the watcher's
// TeeEmitter should fan board:reloaded / task:updated:<id> events here;
// agent:run-finished from the Wails event bus should call
// OnAgentRunFinished.
type AutoGroomCoordinator struct {
	board    *BoardService
	agent    *AgentService
	settings *SettingsService
	emitter  Emitter
	logger   *slog.Logger

	// now is the clock the coordinator uses for settle-window math. Tests
	// override it to advance a virtual clock without sleeping.
	now func() time.Time

	mu            sync.Mutex
	boardDir      string
	activated     bool
	lastNeedsDef  bool
	lastScanAt    time.Time
	lastSkip      map[string]string
	settleTargets map[string]time.Time
	settleTimers  map[string]*time.Timer
	debounceTimer *time.Timer
	// closed signals Deactivate has been called; goroutines spawned by the
	// coordinator should check this before re-arming timers.
	closed chan struct{}
}

// AutoGroomCoordinatorOptions configures NewAutoGroomCoordinator. Board,
// Agent, and Settings are required; Emitter and Logger are optional (the
// coordinator is silent without an emitter, and falls back to slog.Default).
type AutoGroomCoordinatorOptions struct {
	Board    *BoardService
	Agent    *AgentService
	Settings *SettingsService
	Emitter  Emitter
	Logger   *slog.Logger
	// Now overrides time.Now for tests. nil = production clock.
	Now func() time.Time
}

// NewAutoGroomCoordinator constructs a coordinator. It does NOT activate
// or take any board state — call Activate after the daemon has run its
// stale-recovery + startup-scan so the coordinator sees a stable view.
func NewAutoGroomCoordinator(opts AutoGroomCoordinatorOptions) *AutoGroomCoordinator {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := opts.Now
	if clock == nil {
		clock = time.Now
	}
	return &AutoGroomCoordinator{
		board:         opts.Board,
		agent:         opts.Agent,
		settings:      opts.Settings,
		emitter:       opts.Emitter,
		logger:        logger.With("component", "auto-groom"),
		now:           clock,
		lastSkip:      map[string]string{},
		settleTargets: map[string]time.Time{},
		settleTimers:  map[string]*time.Timer{},
		closed:        make(chan struct{}),
	}
}

// SetSettings late-binds the SettingsService dependency. Wired from
// main after both this coordinator and the SettingsService exist (the
// SettingsService takes the coordinator as part of its BoardActivator,
// creating a construction cycle that this two-step wiring breaks).
// Safe to call multiple times.
func (c *AutoGroomCoordinator) SetSettings(s *SettingsService) {
	c.mu.Lock()
	c.settings = s
	c.mu.Unlock()
}

// Activate is the post-OpenBoard hook. It records boardDir and kicks the
// initial scan. Subsequent watcher events drive incremental scans.
//
// Safe to call repeatedly with the same boardDir (idempotent). Calling
// with a different boardDir without a Deactivate in between is a wiring
// bug — log a warning and replace.
func (c *AutoGroomCoordinator) Activate(ctx context.Context, boardDir string) error {
	if strings.TrimSpace(boardDir) == "" {
		return errors.New("AutoGroomCoordinator.Activate: empty boardDir")
	}
	c.mu.Lock()
	if c.activated && c.boardDir != boardDir {
		c.logger.Warn("auto-groom: activate called with a different boardDir without Deactivate",
			"old", c.boardDir, "new", boardDir)
		c.cancelTimersLocked()
	}
	c.boardDir = boardDir
	c.activated = true
	// Reset transient state from a prior board (skips, settle targets,
	// last needs-default flag). The edge-triggered emission in
	// transitionNeedsDefault short-circuits on prev==now, so failing to
	// reset lastNeedsDef would swallow the first emission on a fresh
	// board that happens to share the same state as the previous one.
	// LastGroomTriageHash provides the durable dedupe across activations.
	c.lastSkip = map[string]string{}
	c.settleTargets = map[string]time.Time{}
	c.lastNeedsDef = false
	c.cancelTimersLocked()
	if c.closed == nil {
		c.closed = make(chan struct{})
	}
	c.mu.Unlock()

	c.scheduleScan()
	return nil
}

// Deactivate releases any pending timers and resets the activation flag.
// Called from OpenBoard before activating against a new board.
func (c *AutoGroomCoordinator) Deactivate() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activated = false
	c.boardDir = ""
	c.cancelTimersLocked()
	if c.closed != nil {
		select {
		case <-c.closed:
			// already closed
		default:
			close(c.closed)
		}
	}
	c.closed = nil
	return nil
}

// cancelTimersLocked stops all in-flight timers. mu must be held.
func (c *AutoGroomCoordinator) cancelTimersLocked() {
	if c.debounceTimer != nil {
		c.debounceTimer.Stop()
		c.debounceTimer = nil
	}
	for id, t := range c.settleTimers {
		t.Stop()
		delete(c.settleTimers, id)
	}
}

// NotifyAutoGroomEnabled is invoked by SettingsService.SetAutoGroomEnabled
// so a freshly flipped preference triggers an immediate scan instead of
// waiting for the next watcher event.
func (c *AutoGroomCoordinator) NotifyAutoGroomEnabled() {
	c.scheduleScan()
}

// NotifyDefaultAgentChanged is invoked by SettingsService.SetDefaultAgent
// so clearing the no-default state emits the cleared event promptly.
func (c *AutoGroomCoordinator) NotifyDefaultAgentChanged() {
	c.scheduleScan()
}

// Emit satisfies the watcher.Emitter contract. The coordinator only
// cares about board:reloaded and task:updated:<id> — anything else is a
// no-op. Synchronous to keep parity with the daemon's sink semantics.
func (c *AutoGroomCoordinator) Emit(name string, _ ...any) {
	switch {
	case name == "board:reloaded", strings.HasPrefix(name, "task:updated:"):
		c.scheduleScan()
	}
}

// OnAgentRunFinished is the callback the Wails app event hook invokes
// with the payload from `agent:run-finished` (the same map AgentService
// emits at recordTerminal). For successful groom runs the coordinator
// re-checks triage; if clean it promotes the task to ready.
func (c *AutoGroomCoordinator) OnAgentRunFinished(payload map[string]any) {
	mode, _ := payload["mode"].(string)
	status, _ := payload["status"].(string)
	taskID, _ := payload["task_id"].(string)
	if mode != agent.ModeGroom.String() || status != string(agent.StatusSuccess) {
		return
	}
	if taskID == "" {
		return
	}
	// Use a fresh background context — the originating run already
	// terminated; promoting must not hang on the request that triggered
	// the event.
	go c.tryPromote(context.Background(), taskID)
}

func (c *AutoGroomCoordinator) tryPromote(ctx context.Context, taskID string) {
	if c.board == nil {
		return
	}
	reasons, err := c.board.Triage(ctx)
	if err != nil {
		c.logger.Warn("auto-groom: triage re-check failed", "task", taskID, "err", err)
		return
	}
	if _, stillTriage := reasons[taskID]; stillTriage {
		c.recordSkip(taskID, "post-groom still needs triage")
		c.emit("auto-groom:guarded-skip", map[string]any{
			"task_id": taskID,
			"reasons": reasons[taskID],
		})
		return
	}
	if err := c.board.ReadyTask(ctx, taskID); err != nil {
		c.logger.Warn("auto-groom: tb ready failed", "task", taskID, "err", err)
		c.emit("auto-groom:promote-failed", map[string]any{
			"task_id": taskID,
			"error":   err.Error(),
		})
		return
	}
	c.logger.Info("auto-groom: promoted to ready", "task", taskID)
}

// Status returns the current coordinator snapshot for the frontend.
// Settings (which hit disk via SettingsService) are read BEFORE taking
// the coordinator mutex so a polling frontend never serialises three
// disk reads behind the scan-mutating lock.
func (c *AutoGroomCoordinator) Status() AutoGroomStatus {
	enabled := c.settingsEnabled()
	defaultAgent := c.settingsDefaultAgent()
	settle := c.settingsSettleMinutes()

	c.mu.Lock()
	defer c.mu.Unlock()
	out := AutoGroomStatus{
		Enabled:           enabled,
		DefaultAgent:      defaultAgent,
		SettleMinutes:     settle,
		NeedsDefaultAgent: c.lastNeedsDef,
	}
	if !c.lastScanAt.IsZero() {
		out.LastScanAt = c.lastScanAt.UTC().Format(time.RFC3339)
	}
	if len(c.lastSkip) > 0 {
		out.LastSkipReasons = make(map[string]string, len(c.lastSkip))
		for k, v := range c.lastSkip {
			out.LastSkipReasons[k] = v
		}
	}
	if len(c.settleTargets) > 0 {
		out.SettleEligibleAtMs = make(map[string]int64, len(c.settleTargets))
		for k, v := range c.settleTargets {
			out.SettleEligibleAtMs[k] = v.UnixMilli()
		}
	}
	return out
}

// ServiceName satisfies the Wails service contract so Status() can be
// bound for frontend consumption.
func (c *AutoGroomCoordinator) ServiceName() string { return "AutoGroomCoordinator" }

// scheduleScan coalesces multiple triggers into a single scan within
// scanDebounce. Safe to call concurrently from watcher goroutines.
func (c *AutoGroomCoordinator) scheduleScan() {
	c.mu.Lock()
	if !c.activated {
		c.mu.Unlock()
		return
	}
	if c.debounceTimer != nil {
		c.debounceTimer.Stop()
	}
	c.debounceTimer = time.AfterFunc(scanDebounce, func() {
		c.runScan(context.Background())
	})
	c.mu.Unlock()
}

// runScan is the entry the debounce/timer goroutines call.
func (c *AutoGroomCoordinator) runScan(ctx context.Context) {
	c.mu.Lock()
	if !c.activated {
		c.mu.Unlock()
		return
	}
	boardDir := c.boardDir
	c.mu.Unlock()
	c.scan(ctx, boardDir)
}

// scan is the body of one coordinator pass. Exported on the type only so
// tests can drive it deterministically without going through the
// debounce timer.
func (c *AutoGroomCoordinator) scan(ctx context.Context, boardDir string) {
	if c.board == nil || c.agent == nil || c.settings == nil {
		return
	}

	enabled := c.settingsEnabled()
	defaultAgent := c.settingsDefaultAgent()
	settle := c.settingsSettleMinutes()

	c.mu.Lock()
	c.lastScanAt = c.now()
	c.mu.Unlock()

	if !enabled {
		c.transitionNeedsDefault(false)
		return
	}
	if !isValidDefaultAgent(defaultAgent) {
		c.transitionNeedsDefault(true)
		return
	}
	c.transitionNeedsDefault(false)

	reasons, err := c.board.Triage(ctx)
	if err != nil {
		c.logger.Warn("auto-groom: triage failed", "err", err)
		return
	}

	// Clear stale per-task state for tasks that are no longer in triage
	// (either promoted or removed). The durable dedupe in
	// LastGroomTriageHash already covers cross-restart skip behavior, so
	// we don't need to retain in-memory entries for gone-from-triage IDs.
	c.mu.Lock()
	for id := range c.lastSkip {
		if _, stillReason := reasons[id]; !stillReason {
			delete(c.lastSkip, id)
		}
	}
	for id, t := range c.settleTimers {
		if _, stillReason := reasons[id]; !stillReason {
			t.Stop()
			delete(c.settleTimers, id)
			delete(c.settleTargets, id)
		}
	}
	c.mu.Unlock()

	for id, taskReasons := range reasons {
		c.evaluate(ctx, boardDir, id, taskReasons, defaultAgent, settle)
	}
}

// evaluate decides whether to queue a single candidate. Encapsulated so
// the test harness can also invoke this directly via scan().
func (c *AutoGroomCoordinator) evaluate(ctx context.Context, boardDir, id string, reasons []string, defaultAgent string, settle int) {
	task, err := c.board.GetTask(ctx, id)
	if err != nil {
		c.logger.Debug("auto-groom: GetTask failed", "task", id, "err", err)
		return
	}
	if task.Metadata.Status != "backlog" {
		c.recordSkip(id, "not in backlog")
		return
	}
	switch task.Metadata.AgentStatus {
	case "queued", "running", "needs-user":
		c.recordSkip(id, "agent-status "+task.Metadata.AgentStatus)
		return
	}
	if c.agent.HasActiveRun(id) {
		c.recordSkip(id, "active run")
		return
	}

	// Settle window — guard freshly created/edited tasks so attachments
	// and follow-up notes can land before automation kicks in.
	if settle > 0 {
		eligibleAt, ok, err := c.taskEligibleAt(boardDir, id, settle)
		if err != nil {
			c.logger.Debug("auto-groom: stat task failed", "task", id, "err", err)
			return
		}
		if ok && c.now().Before(eligibleAt) {
			c.armSettleTimer(boardDir, id, eligibleAt)
			c.recordSkip(id, "settle")
			return
		}
	}

	// Durable dedupe — skip an unchanged task that already completed an
	// auto-groom pass for the same triage state.
	hash := computeTriageHash(reasons)
	if prior, ok, err := agent.LastGroomTriageHash(boardDir, id); err != nil {
		c.logger.Debug("auto-groom: read groom history failed", "task", id, "err", err)
	} else if ok && prior == hash {
		c.recordSkip(id, "dedupe")
		return
	}

	// Ensure an Agent is set without overwriting an explicit assignment.
	if strings.TrimSpace(task.Metadata.Agent) == "" {
		c2 := c.board.snapshot()
		if c2 == nil {
			c.logger.Warn("auto-groom: board has no CLI client", "task", id)
			return
		}
		if err := c2.Edit(ctx, id, cli.EditInput{Agent: defaultAgent}); err != nil {
			c.logger.Warn("auto-groom: set agent failed", "task", id, "agent", defaultAgent, "err", err)
			return
		}
	}

	runID, err := c.agent.StartGroomWithTriageHash(ctx, id, hash)
	if err != nil {
		c.logger.Warn("auto-groom: StartGroomWithTriageHash failed", "task", id, "err", err)
		return
	}
	c.mu.Lock()
	delete(c.lastSkip, id)
	c.mu.Unlock()
	c.emit("auto-groom:queued", map[string]any{
		"task_id":      id,
		"run_id":       runID,
		"triage_hash":  hash,
	})
}

// taskEligibleAt returns when the task becomes eligible for auto-groom,
// using the task file (folder form: TaskDir; file form: <status>/<ID>.md)
// mtime + the configured settle minutes. Any structured `tb edit` or
// attach bumps the mtime, so editing or attaching a file restarts the
// settle window — the deliberate UX. ok=false when the task can't be
// located (caller treats that as "skip the settle gate" so a missing
// task doesn't permanently block scanning).
func (c *AutoGroomCoordinator) taskEligibleAt(boardDir, taskID string, settle int) (time.Time, bool, error) {
	paths, err := agent.ResolveArtifactPaths(boardDir, taskID)
	if err != nil {
		return time.Time{}, false, err
	}
	var statPath string
	if paths.Layout == agent.ArtifactLayoutFolder && paths.TaskDir != "" {
		statPath = paths.TaskDir
	} else {
		for _, status := range autoGroomStatusDirs {
			candidate := filepath.Join(boardDir, status, taskID+".md")
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				statPath = candidate
				break
			}
		}
	}
	if statPath == "" {
		return time.Time{}, false, nil
	}
	info, err := os.Stat(statPath)
	if err != nil {
		return time.Time{}, false, err
	}
	eligibleAt := info.ModTime().Add(time.Duration(settle) * time.Minute)
	return eligibleAt, true, nil
}

// armSettleTimer arms (or re-arms) the deferred rescan tick for taskID
// so the coordinator wakes up the moment the settle window expires —
// even if no further watcher event fires.
func (c *AutoGroomCoordinator) armSettleTimer(boardDir, id string, eligibleAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.activated || c.boardDir != boardDir {
		return
	}
	// Idempotent re-arm: cancel the old timer; record the new target.
	if t, ok := c.settleTimers[id]; ok {
		t.Stop()
	}
	c.settleTargets[id] = eligibleAt
	delay := eligibleAt.Sub(c.now()) + settleJitter
	if delay < 0 {
		delay = settleJitter
	}
	c.settleTimers[id] = time.AfterFunc(delay, func() {
		c.mu.Lock()
		if t, ok := c.settleTimers[id]; ok && t != nil {
			delete(c.settleTimers, id)
			delete(c.settleTargets, id)
		}
		c.mu.Unlock()
		c.runScan(context.Background())
	})
}

// recordSkip pins the most recent skip reason for taskID. Used by
// Status() so the frontend can render context-aware UI.
func (c *AutoGroomCoordinator) recordSkip(taskID, reason string) {
	c.mu.Lock()
	c.lastSkip[taskID] = reason
	c.mu.Unlock()
}

// transitionNeedsDefault is the edge-trigger guard: emits the Wails
// event ONLY when the no-default-agent state actually flips. Steady-
// state scans are silent.
func (c *AutoGroomCoordinator) transitionNeedsDefault(now bool) {
	c.mu.Lock()
	prev := c.lastNeedsDef
	c.lastNeedsDef = now
	c.mu.Unlock()
	if prev == now {
		return
	}
	if now {
		c.emit("auto-groom:needs-default-agent", map[string]any{})
	} else {
		c.emit("auto-groom:default-agent-cleared", map[string]any{})
	}
}

func (c *AutoGroomCoordinator) emit(name string, payload any) {
	if c.emitter == nil {
		return
	}
	c.emitter.Emit(name, payload)
}

func (c *AutoGroomCoordinator) settingsEnabled() bool {
	if c.settings == nil {
		return false
	}
	return c.settings.GetAutoGroomEnabled()
}

func (c *AutoGroomCoordinator) settingsDefaultAgent() string {
	if c.settings == nil {
		return "none"
	}
	return c.settings.GetDefaultAgent()
}

func (c *AutoGroomCoordinator) settingsSettleMinutes() int {
	if c.settings == nil {
		return AutoGroomSettleMinutesDefault
	}
	return c.settings.GetAutoGroomSettleMinutes()
}

// computeTriageHash returns the durable dedupe fingerprint for a triage
// reason set. Deterministic across the lifetime of the board: the same
// reason set produces the same hash, so an unchanged task is never
// auto-groomed twice.
func computeTriageHash(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	sorted := make([]string, len(reasons))
	copy(sorted, reasons)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}

// isValidDefaultAgent gates the "no default" emission. Mirrors the
// DefaultAgentValues whitelist minus "none".
func isValidDefaultAgent(agent string) bool {
	return agent == "claude" || agent == "codex"
}
