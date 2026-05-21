package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

type AutoReviewStatus struct {
	Enabled           bool              `json:"enabled"`
	DefaultAgent      string            `json:"default_agent"`
	NeedsDefaultAgent bool              `json:"needs_default_agent"`
	LastScanAt        string            `json:"last_scan_at,omitempty"`
	LastSkipReasons   map[string]string `json:"last_skip_reasons,omitempty"`
}

type AutoReviewCoordinator struct {
	board    *BoardService
	agent    *AgentService
	settings *SettingsService
	emitter  Emitter
	logger   *slog.Logger
	budget   AutomationWorkerBudget
	now      func() time.Time

	mu                sync.Mutex
	boardDir          string
	activated         bool
	lastNeedsDef      bool
	lastScanAt        time.Time
	lastSkip          map[string]string
	debounceTimer     *time.Timer
	startupGraceUntil time.Time
	resumeAttempts    map[string]time.Time
	closed            chan struct{}
}

type AutoReviewCoordinatorOptions struct {
	Board        *BoardService
	Agent        *AgentService
	Settings     *SettingsService
	Emitter      Emitter
	Logger       *slog.Logger
	WorkerBudget AutomationWorkerBudget
	Now          func() time.Time
}

func NewAutoReviewCoordinator(opts AutoReviewCoordinatorOptions) *AutoReviewCoordinator {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := opts.Now
	if clock == nil {
		clock = time.Now
	}
	return &AutoReviewCoordinator{
		board:          opts.Board,
		agent:          opts.Agent,
		settings:       opts.Settings,
		emitter:        opts.Emitter,
		logger:         logger.With("component", "auto-review"),
		budget:         opts.WorkerBudget,
		now:            clock,
		lastSkip:       map[string]string{},
		resumeAttempts: map[string]time.Time{},
		closed:         make(chan struct{}),
	}
}

func (c *AutoReviewCoordinator) SetSettings(s *SettingsService) {
	c.mu.Lock()
	c.settings = s
	c.mu.Unlock()
}

func (c *AutoReviewCoordinator) Activate(ctx context.Context, boardDir string) error {
	return c.ActivateWithStartupGrace(ctx, boardDir, 0)
}

func (c *AutoReviewCoordinator) ActivateWithStartupGrace(ctx context.Context, boardDir string, grace time.Duration) error {
	if strings.TrimSpace(boardDir) == "" {
		return errors.New("AutoReviewCoordinator.Activate: empty boardDir")
	}
	c.mu.Lock()
	if c.activated && c.boardDir != boardDir {
		c.logger.Warn("auto-review: activate called with a different boardDir without Deactivate",
			"old", c.boardDir, "new", boardDir)
		c.cancelTimersLocked()
	}
	c.boardDir = boardDir
	c.activated = true
	c.lastSkip = map[string]string{}
	c.resumeAttempts = map[string]time.Time{}
	c.lastNeedsDef = false
	c.startupGraceUntil = time.Time{}
	if grace > 0 {
		c.startupGraceUntil = c.now().Add(grace)
	}
	c.cancelTimersLocked()
	if c.closed == nil {
		c.closed = make(chan struct{})
	}
	c.mu.Unlock()

	c.scheduleScan()
	_ = ctx
	return nil
}

func (c *AutoReviewCoordinator) Deactivate() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activated = false
	c.boardDir = ""
	c.startupGraceUntil = time.Time{}
	c.cancelTimersLocked()
	if c.closed != nil {
		select {
		case <-c.closed:
		default:
			close(c.closed)
		}
	}
	c.closed = nil
	return nil
}

func (c *AutoReviewCoordinator) cancelTimersLocked() {
	if c.debounceTimer != nil {
		c.debounceTimer.Stop()
		c.debounceTimer = nil
	}
}

func (c *AutoReviewCoordinator) NotifyAutoReviewEnabled()   { c.scheduleScan() }
func (c *AutoReviewCoordinator) NotifyDefaultAgentChanged() { c.scheduleScan() }
func (c *AutoReviewCoordinator) NotifyWorkerBudgetChanged() { c.scheduleScan() }

func (c *AutoReviewCoordinator) OnAgentRunFinished(payload map[string]any) {
	if payload == nil {
		return
	}
	if mode, _ := payload["mode"].(string); mode == agent.ModeReview.String() {
		c.scheduleScan()
	}
}

func (c *AutoReviewCoordinator) Emit(name string, _ ...any) {
	switch {
	case name == "board:reloaded", strings.HasPrefix(name, "task:updated:"):
		c.scheduleScan()
	}
}

func (c *AutoReviewCoordinator) Status() AutoReviewStatus {
	enabled := c.settingsEnabled()
	defaultAgent := c.settingsDefaultAgent()

	c.mu.Lock()
	defer c.mu.Unlock()
	out := AutoReviewStatus{
		Enabled:           enabled,
		DefaultAgent:      defaultAgent,
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
	return out
}

func (c *AutoReviewCoordinator) ServiceName() string { return "AutoReviewCoordinator" }

func (c *AutoReviewCoordinator) scheduleScan() {
	c.mu.Lock()
	if !c.activated {
		c.mu.Unlock()
		return
	}
	if c.debounceTimer != nil {
		c.debounceTimer.Stop()
	}
	delay := scanDebounce
	if !c.startupGraceUntil.IsZero() {
		remaining := c.startupGraceUntil.Sub(c.now())
		if remaining > 0 {
			delay = remaining
		} else {
			c.startupGraceUntil = time.Time{}
		}
	}
	c.debounceTimer = time.AfterFunc(delay, func() {
		c.runScan(context.Background())
	})
	c.mu.Unlock()
}

func (c *AutoReviewCoordinator) runScan(ctx context.Context) {
	c.mu.Lock()
	if !c.activated {
		c.mu.Unlock()
		return
	}
	boardDir := c.boardDir
	c.mu.Unlock()
	c.scan(ctx, boardDir)
}

func (c *AutoReviewCoordinator) scan(ctx context.Context, boardDir string) {
	if c.board == nil || c.agent == nil || c.settings == nil {
		return
	}

	enabled := c.settingsEnabled()
	defaultAgent := c.settingsDefaultAgent()

	c.mu.Lock()
	c.lastScanAt = c.now()
	c.lastSkip = map[string]string{}
	c.mu.Unlock()

	if !enabled {
		c.transitionNeedsDefault(false)
		c.emit("auto-review:scan-complete", map[string]any{})
		return
	}
	if !isValidDefaultAgent(defaultAgent) {
		c.transitionNeedsDefault(true)
		c.emit("auto-review:scan-complete", map[string]any{})
		return
	}
	c.transitionNeedsDefault(false)

	snap, err := c.board.LoadBoard(ctx)
	if err != nil {
		c.logger.Warn("auto-review: LoadBoard failed", "err", err)
		c.emit("auto-review:scan-complete", map[string]any{})
		return
	}

	remainingWorkers := c.remainingWorkerCapacity()
	c.pruneResumeAttempts()
	recoveryHandled := map[string]struct{}{}
	for _, t := range snap.CodeReview {
		if t.AgentStatus != "interrupted" && t.AgentStatus != "lost" {
			continue
		}
		recoveryHandled[t.ID] = struct{}{}
		initiator, ierr := agent.LatestQueuedInitiator(boardDir, t.ID)
		if ierr != nil {
			c.logger.Debug("auto-review: LatestQueuedInitiator failed", "task", t.ID, "err", ierr)
			continue
		}
		if initiator != agent.InitiatorAutoReview {
			c.recordSkip(t.ID, "user-initiated; auto-resume skipped")
			continue
		}
		if remainingWorkers <= 0 {
			c.recordSkip(t.ID, "worker capacity full")
			continue
		}
		if c.tryAutoResume(ctx, t.ID, t.AgentStatus) {
			remainingWorkers--
		}
	}

	for _, t := range snap.CodeReview {
		if _, ok := recoveryHandled[t.ID]; ok {
			continue
		}
		if remainingWorkers <= 0 {
			c.recordSkip(t.ID, "worker capacity full")
			continue
		}
		if reason := autoReviewGateBlocker(t); reason != "" {
			if reason == "missing ReviewRef" {
				c.handoffMissingReviewRef(ctx, t.ID)
				continue
			}
			c.recordSkip(t.ID, reason)
			continue
		}
		if c.agent.HasActiveRun(t.ID) {
			c.recordSkip(t.ID, "active run")
			continue
		}
		detail, err := c.board.GetTask(ctx, t.ID)
		if err != nil {
			c.logger.Warn("auto-review: GetTask failed", "task", t.ID, "err", err)
			c.recordSkip(t.ID, "load-failed: "+err.Error())
			continue
		}
		fp := autoReviewFingerprint(detail.Metadata, detail.Body)
		if autoReviewAlreadyQueued(boardDir, t.ID, fp) {
			c.recordSkip(t.ID, "duplicate review epoch")
			continue
		}
		if _, ok := markdownSectionBody(detail.Body, "## Review Target"); !ok {
			c.emit("auto-review:missing-review-target-prose", map[string]any{
				"task_id":    t.ID,
				"review_ref": strings.TrimSpace(t.ReviewRef),
			})
		}
		if c.startCandidate(ctx, t, defaultAgent) {
			remainingWorkers--
		}
	}
	c.emit("auto-review:scan-complete", map[string]any{})
}

func autoReviewGateBlocker(t Task) string {
	switch t.AgentStatus {
	case "queued", "running", "needs-user", "cancelled", "interrupted", "lost":
		return "agent-status " + t.AgentStatus
	}
	switch t.ReviewStatus {
	case "queued", "running":
		return "review-status " + t.ReviewStatus
	}
	if strings.TrimSpace(t.ReviewRef) == "" {
		return "missing ReviewRef"
	}
	return ""
}

func (c *AutoReviewCoordinator) handoffMissingReviewRef(ctx context.Context, taskID string) {
	body := strings.Join([]string{
		"Reason: missing review target.",
		"Question/Action: set ReviewRef to a branch, PR URL, commit SHA, worktree path, or other machine-readable target.",
		"Attempted context: auto-review found this task in code-review but could not determine a safe target.",
		"Unblock condition: set ReviewRef, then clear AgentStatus with `tb edit " + taskID + " --agent-status none`.",
	}, "\n")
	if err := c.board.EditTask(ctx, taskID, EditTaskInput{
		AgentStatus:   "needs-user",
		UserAttention: body,
	}); err != nil {
		c.logger.Warn("auto-review: missing ReviewRef handoff failed", "task", taskID, "err", err)
		c.recordSkip(taskID, "missing ReviewRef; handoff failed: "+err.Error())
		return
	}
	c.recordSkip(taskID, "missing ReviewRef")
	c.emit("auto-review:needs-user", map[string]any{
		"task_id": taskID,
		"reason":  "missing ReviewRef",
	})
}

func (c *AutoReviewCoordinator) startCandidate(ctx context.Context, task Task, defaultAgent string) bool {
	agentName := strings.ToLower(strings.TrimSpace(task.Agent))
	if agentName == "" {
		c2 := c.board.snapshot()
		if c2 == nil {
			c.logger.Warn("auto-review: board has no CLI client", "task", task.ID)
			c.recordSkip(task.ID, "no CLI client")
			return false
		}
		if err := c2.Edit(ctx, task.ID, cli.EditInput{Agent: defaultAgent}); err != nil {
			c.logger.Warn("auto-review: set agent failed", "task", task.ID, "agent", defaultAgent, "err", err)
			c.recordSkip(task.ID, "set-agent-failed: "+err.Error())
			return false
		}
		agentName = defaultAgent
	}

	if _, err := c.agent.runnerFor(agentName); err != nil {
		c.logger.Warn("auto-review: agent unsupported", "task", task.ID, "agent", agentName, "err", err)
		c.recordSkip(task.ID, "agent-unsupported: "+err.Error())
		return false
	}

	runID, err := c.agent.ReviewTaskAs(ctx, task.ID, agent.InitiatorAutoReview)
	if err != nil {
		if errors.Is(err, ErrWorkerCapacityFull) {
			c.recordSkip(task.ID, "worker capacity full")
			c.emit("auto-review:worker-capacity-full", map[string]any{
				"task_id": task.ID,
				"error":   err.Error(),
			})
			return false
		}
		c.logger.Warn("auto-review: ReviewTaskAs failed", "task", task.ID, "err", err)
		c.recordSkip(task.ID, "run-failed: "+err.Error())
		c.emit("auto-review:run-failed", map[string]any{
			"task_id": task.ID,
			"error":   err.Error(),
		})
		return false
	}
	c.mu.Lock()
	delete(c.lastSkip, task.ID)
	c.mu.Unlock()
	c.emit("auto-review:queued", map[string]any{
		"task_id":    task.ID,
		"run_id":     runID,
		"agent":      agentName,
		"review_ref": strings.TrimSpace(task.ReviewRef),
	})
	return true
}

func (c *AutoReviewCoordinator) pruneResumeAttempts() {
	cutoff := c.now().Add(-2 * resumeAttemptCooldown)
	c.mu.Lock()
	for id, ts := range c.resumeAttempts {
		if ts.Before(cutoff) {
			delete(c.resumeAttempts, id)
		}
	}
	c.mu.Unlock()
}

func (c *AutoReviewCoordinator) tryAutoResume(ctx context.Context, id, status string) bool {
	if c.agent.HasActiveRun(id) {
		c.recordSkip(id, "active run")
		return false
	}
	c.mu.Lock()
	if last, ok := c.resumeAttempts[id]; ok && c.now().Sub(last) < resumeAttemptCooldown {
		c.mu.Unlock()
		c.recordSkip(id, "resume cooldown")
		return false
	}
	c.resumeAttempts[id] = c.now()
	c.mu.Unlock()

	var (
		runID string
		err   error
		op    string
	)
	switch status {
	case "interrupted":
		op = "resume"
		runID, err = c.agent.ResumeAgentAs(ctx, id, agent.InitiatorAutoReview)
	case "lost":
		op = "restart"
		runID, err = c.agent.ReviewTaskAs(ctx, id, agent.InitiatorAutoReview)
	default:
		return false
	}
	if err != nil {
		if errors.Is(err, ErrAlreadyRunning) {
			return false
		}
		if errors.Is(err, ErrWorkerCapacityFull) {
			c.recordSkip(id, "worker capacity full")
			return false
		}
		c.logger.Warn("auto-review: auto-"+op+" failed", "task", id, "err", err)
		c.recordSkip(id, op+" failed: "+err.Error())
		c.emit("auto-review:resume-failed", map[string]any{
			"task_id": id,
			"op":      op,
			"error":   err.Error(),
		})
		return false
	}
	c.mu.Lock()
	delete(c.lastSkip, id)
	delete(c.resumeAttempts, id)
	c.mu.Unlock()
	c.emit("auto-review:resumed", map[string]any{
		"task_id": id,
		"run_id":  runID,
		"op":      op,
	})
	return true
}

func autoReviewFingerprint(t Task, body string) string {
	reviewTarget, _ := markdownSectionBody(body, "## Review Target")
	return hashString(strings.Join([]string{
		t.ID,
		strings.TrimSpace(t.ReviewRef),
		reviewTarget,
		codeReviewLogEpoch(body),
	}, "\x00"))
}

func codeReviewLogEpoch(body string) string {
	logBody, ok := markdownSectionBody(body, "## Log")
	if !ok {
		return ""
	}
	lines := strings.Split(logBody, "\n")
	count := 0
	last := ""
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "code-review") {
			count++
			if last == "" {
				last = line
			}
		}
	}
	if last == "" {
		return ""
	}
	return strconv.Itoa(count) + "\x00" + last
}

func autoReviewAlreadyQueued(boardDir, taskID, fingerprint string) bool {
	paths, err := agent.ResolveArtifactPaths(boardDir, taskID)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(paths.StatePath)
	if err != nil {
		return false
	}
	matchingQueued := map[string]struct{}{}
	progressedRuns := map[string]struct{}{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev agent.Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Event == agent.EvQueued &&
			ev.Mode == agent.ModeReview.String() &&
			ev.Initiator == agent.InitiatorAutoReview &&
			ev.Target == fingerprint {
			matchingQueued[ev.RunID] = struct{}{}
			continue
		}
		if ev.Event == agent.EvStarted || ev.Event == agent.EvFinished {
			progressedRuns[ev.RunID] = struct{}{}
		}
	}
	for runID := range matchingQueued {
		if _, ok := progressedRuns[runID]; ok {
			return true
		}
	}
	return false
}

func (c *AutoReviewCoordinator) recordSkip(taskID, reason string) {
	c.mu.Lock()
	c.lastSkip[taskID] = reason
	c.mu.Unlock()
}

func (c *AutoReviewCoordinator) transitionNeedsDefault(now bool) {
	c.mu.Lock()
	prev := c.lastNeedsDef
	c.lastNeedsDef = now
	c.mu.Unlock()
	if prev == now {
		return
	}
	if now {
		c.emit("auto-review:needs-default-agent", map[string]any{})
	} else {
		c.emit("auto-review:default-agent-cleared", map[string]any{})
	}
}

func (c *AutoReviewCoordinator) emit(name string, payload any) {
	if c.emitter == nil {
		return
	}
	c.emitter.Emit(name, payload)
}

func (c *AutoReviewCoordinator) settingsEnabled() bool {
	if c.settings == nil {
		return false
	}
	return c.settings.GetAutoReviewEnabled()
}

func (c *AutoReviewCoordinator) settingsDefaultAgent() string {
	if c.settings == nil {
		return "none"
	}
	return c.settings.GetDefaultAgent()
}

func (c *AutoReviewCoordinator) remainingWorkerCapacity() int {
	return remainingAutomationWorkerCapacity(c.budget, c.agent, c.settings)
}
