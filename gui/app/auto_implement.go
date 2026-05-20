package app

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/automation/epicorder"
	"tools/tb-gui/internal/cli"
)

// AutoImplementStatus is the snapshot the Wails-bound Status() returns
// (TB-180 consumes it). NeedsDefaultAgent and NeedsQuery describe the
// two enable-prerequisites. NeedsQuery now means "filter is empty" since
// TB-288 replaced the text-DSL parser with a structured filter that
// cannot fail to parse. LastSkipReasons helps the UI explain
// "skipped because TB-X is not done yet" diagnostics.
type AutoImplementStatus struct {
	Enabled           bool                `json:"enabled"`
	Query             AutoImplementFilter `json:"query"`
	DefaultAgent      string              `json:"default_agent"`
	NeedsDefaultAgent bool                `json:"needs_default_agent"`
	NeedsQuery        bool                `json:"needs_query"`
	LastScanAt        string              `json:"last_scan_at,omitempty"`
	LastSkipReasons   map[string]string   `json:"last_skip_reasons,omitempty"`
}

// AutoImplementCoordinator drives the TB-179 auto-implement loop:
// scan the ready column, match each task against the saved query,
// enforce epic ordering (TB-267) and triage gates, sort the eligible
// pool with review-failed first within priority bucket (TB-233), pull
// each candidate ready→in-progress via the canonical `tb pull` path,
// and queue an implementation-mode run through AgentService.
//
// Lifecycle and wiring parallel AutoGroomCoordinator: same Activate /
// Deactivate semantics, same watcher Emit / NotifyAutoImplementEnabled /
// NotifyAutoImplementQueryChanged / NotifyDefaultAgentChanged hooks.
type AutoImplementCoordinator struct {
	board    *BoardService
	agent    *AgentService
	settings *SettingsService
	emitter  Emitter
	logger   *slog.Logger

	now func() time.Time

	mu             sync.Mutex
	boardDir       string
	activated      bool
	lastNeedsDef   bool
	lastNeedsQry   bool
	lastScanAt     time.Time
	lastSkip       map[string]string
	debounceTimer  *time.Timer
	resumeAttempts map[string]time.Time
	closed         chan struct{}
}

// AutoImplementCoordinatorOptions configures NewAutoImplementCoordinator.
type AutoImplementCoordinatorOptions struct {
	Board    *BoardService
	Agent    *AgentService
	Settings *SettingsService
	Emitter  Emitter
	Logger   *slog.Logger
	Now      func() time.Time
}

// NewAutoImplementCoordinator constructs the coordinator. Activate must
// be called separately so the daemon's stale-recovery + startup scan
// run first.
func NewAutoImplementCoordinator(opts AutoImplementCoordinatorOptions) *AutoImplementCoordinator {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := opts.Now
	if clock == nil {
		clock = time.Now
	}
	return &AutoImplementCoordinator{
		board:          opts.Board,
		agent:          opts.Agent,
		settings:       opts.Settings,
		emitter:        opts.Emitter,
		logger:         logger.With("component", "auto-implement"),
		now:            clock,
		lastSkip:       map[string]string{},
		resumeAttempts: map[string]time.Time{},
		closed:         make(chan struct{}),
	}
}

// SetSettings late-binds the SettingsService. Wired from main after
// both this coordinator and the SettingsService exist (the
// SettingsService takes the coordinator via the BoardActivator, so the
// two-step binding breaks the construction cycle).
func (c *AutoImplementCoordinator) SetSettings(s *SettingsService) {
	c.mu.Lock()
	c.settings = s
	c.mu.Unlock()
}

// Activate is the post-OpenBoard hook. Records boardDir and kicks an
// initial scan. Subsequent watcher events drive incremental scans.
func (c *AutoImplementCoordinator) Activate(ctx context.Context, boardDir string) error {
	if strings.TrimSpace(boardDir) == "" {
		return errors.New("AutoImplementCoordinator.Activate: empty boardDir")
	}
	c.mu.Lock()
	if c.activated && c.boardDir != boardDir {
		c.logger.Warn("auto-implement: activate called with a different boardDir without Deactivate",
			"old", c.boardDir, "new", boardDir)
		c.cancelTimersLocked()
	}
	c.boardDir = boardDir
	c.activated = true
	c.lastSkip = map[string]string{}
	c.resumeAttempts = map[string]time.Time{}
	c.lastNeedsDef = false
	c.lastNeedsQry = false
	c.cancelTimersLocked()
	if c.closed == nil {
		c.closed = make(chan struct{})
	}
	c.mu.Unlock()

	c.scheduleScan()
	return nil
}

// Deactivate cancels pending timers and resets activation state.
func (c *AutoImplementCoordinator) Deactivate() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activated = false
	c.boardDir = ""
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

func (c *AutoImplementCoordinator) cancelTimersLocked() {
	if c.debounceTimer != nil {
		c.debounceTimer.Stop()
		c.debounceTimer = nil
	}
}

// NotifyAutoImplementEnabled is invoked by SettingsService whenever
// auto-implement enabled toggles. Triggers an immediate scan.
func (c *AutoImplementCoordinator) NotifyAutoImplementEnabled() { c.scheduleScan() }

// NotifyAutoImplementQueryChanged is invoked by SettingsService whenever
// the saved query string changes. Triggers an immediate scan so the
// next pickup uses the new filter.
func (c *AutoImplementCoordinator) NotifyAutoImplementQueryChanged() { c.scheduleScan() }

// NotifyDefaultAgentChanged is invoked by SettingsService whenever
// default_agent changes. Triggers an immediate scan so a freshly
// supplied default agent unblocks queued candidates.
func (c *AutoImplementCoordinator) NotifyDefaultAgentChanged() { c.scheduleScan() }

// Emit satisfies the watcher.Emitter contract. Coordinator only cares
// about board:reloaded and task:updated:<id>.
func (c *AutoImplementCoordinator) Emit(name string, _ ...any) {
	switch {
	case name == "board:reloaded", strings.HasPrefix(name, "task:updated:"):
		c.scheduleScan()
	}
}

// Status returns the current coordinator snapshot for the frontend.
func (c *AutoImplementCoordinator) Status() AutoImplementStatus {
	enabled := c.settingsEnabled()
	q := c.settingsQuery()
	defaultAgent := c.settingsDefaultAgent()

	c.mu.Lock()
	defer c.mu.Unlock()
	out := AutoImplementStatus{
		Enabled:           enabled,
		Query:             q,
		DefaultAgent:      defaultAgent,
		NeedsDefaultAgent: c.lastNeedsDef,
		NeedsQuery:        c.lastNeedsQry,
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

// ServiceName satisfies the Wails service contract so Status() is
// bindable for the frontend (TB-180).
func (c *AutoImplementCoordinator) ServiceName() string { return "AutoImplementCoordinator" }

// scheduleScan coalesces multiple triggers into one scan within
// scanDebounce. The constant lives in auto_groom.go for both
// coordinators (no need for a separate knob).
func (c *AutoImplementCoordinator) scheduleScan() {
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

func (c *AutoImplementCoordinator) runScan(ctx context.Context) {
	c.mu.Lock()
	if !c.activated {
		c.mu.Unlock()
		return
	}
	boardDir := c.boardDir
	c.mu.Unlock()
	c.scan(ctx, boardDir)
}

// scan is the body of one coordinator pass.
func (c *AutoImplementCoordinator) scan(ctx context.Context, boardDir string) {
	if c.board == nil || c.agent == nil || c.settings == nil {
		return
	}

	enabled := c.settingsEnabled()
	q := c.settingsQuery()
	defaultAgent := c.settingsDefaultAgent()

	c.mu.Lock()
	c.lastScanAt = c.now()
	// Reset the per-scan skip ledger so stale reasons from a prior scan
	// don't outlive a state change that fixed them.
	c.lastSkip = map[string]string{}
	c.mu.Unlock()

	if !enabled {
		c.transitionNeedsDefault(false)
		c.transitionNeedsQuery(false)
		c.emit("auto-implement:scan-complete", map[string]any{})
		return
	}

	if q.IsEmpty() {
		c.transitionNeedsQuery(true)
		c.transitionNeedsDefault(false)
		c.emit("auto-implement:scan-complete", map[string]any{})
		return
	}
	c.transitionNeedsQuery(false)

	if !isValidDefaultAgent(defaultAgent) {
		c.transitionNeedsDefault(true)
		c.emit("auto-implement:scan-complete", map[string]any{})
		return
	}
	c.transitionNeedsDefault(false)

	snap, err := c.board.LoadBoard(ctx)
	if err != nil {
		c.logger.Warn("auto-implement: LoadBoard failed", "err", err)
		c.emit("auto-implement:scan-complete", map[string]any{})
		return
	}

	// Candidate pool comes from `tb ls --status ready` with the persisted
	// filter applied CLI-side (TB-289 / TB-288). The local gates below
	// (agent-status, triage, active-run, epic-order) still run on the
	// pre-filtered set. snap above is kept for the resume sweep + sibling
	// pool because those need every column, not just ready.
	readyCandidates, err := c.board.ListWithFilter(ctx, "ready", q)
	if err != nil {
		c.logger.Warn("auto-implement: ListWithFilter failed", "err", err)
		c.emit("auto-implement:scan-complete", map[string]any{})
		return
	}

	// Resume sweep: in-progress tasks left `interrupted` or `lost` by
	// stale-recovery (daemon crash) get reanimated before we hunt for
	// fresh candidates. The initiator filter scopes this to runs that
	// the auto-implement coordinator originally queued — user-triggered
	// runs that crashed are left for the user to deal with via the GUI.
	// ResumeAgent/RunAgent flip AgentStatus to "queued" atomically, so
	// the candidate pass below won't see them as resumable on this scan
	// or the next.
	c.pruneResumeAttempts()
	for _, t := range snap.InProgress {
		if t.AgentStatus != "interrupted" && t.AgentStatus != "lost" {
			continue
		}
		initiator, ierr := agent.LatestQueuedInitiator(boardDir, t.ID)
		if ierr != nil {
			c.logger.Debug("auto-implement: LatestQueuedInitiator failed", "task", t.ID, "err", ierr)
			continue
		}
		if initiator != agent.InitiatorAutoImplement {
			c.recordSkip(t.ID, "user-initiated; auto-resume skipped")
			continue
		}
		c.tryAutoResume(ctx, t.ID, t.AgentStatus)
	}

	triage, err := c.board.Triage(ctx)
	if err != nil {
		c.logger.Warn("auto-implement: Triage failed", "err", err)
		c.emit("auto-implement:scan-complete", map[string]any{})
		return
	}

	// Build the epicorder sibling pool once. Includes every task across
	// all visible columns so epicorder.EligibleForEpicOrder can resolve
	// same-parent siblings regardless of which column they're in.
	siblings := buildEpicorderSiblings(snap)

	// Candidate pass: filter ready tasks against eligibility gates.
	type candidate struct {
		task         Task
		isReviewFail bool
	}
	var candidates []candidate
	for _, t := range readyCandidates {
		if t.AgentStatus != "" {
			c.recordSkip(t.ID, "agent-status "+t.AgentStatus)
			continue
		}
		if _, flagged := triage[t.ID]; flagged {
			c.recordSkip(t.ID, "triage flagged")
			continue
		}
		if c.agent.HasActiveRun(t.ID) {
			c.recordSkip(t.ID, "active run")
			continue
		}
		// Epic-order gate (TB-267): a child cannot auto-run until every
		// earlier sibling is closed.
		cand := epicTaskFromBoardTask(t)
		res := epicorder.EligibleForEpicOrder(cand, siblings)
		if !res.Eligible {
			c.recordSkip(t.ID, "epic-order: "+res.Reason)
			c.emit("auto-implement:epic-order-skip", map[string]any{
				"task_id":    t.ID,
				"blocker_id": res.BlockerID,
				"reason":     res.Reason,
			})
			continue
		}
		candidates = append(candidates, candidate{
			task:         t,
			isReviewFail: tagListContains(t.Tags, "review-failed"),
		})
	}

	// TB-233 sort: priority desc, then review-failed first within
	// priority bucket, then numeric id asc (oldest first).
	sort.SliceStable(candidates, func(i, j int) bool {
		pi := priorityRank(candidates[i].task.Priority)
		pj := priorityRank(candidates[j].task.Priority)
		if pi != pj {
			return pi < pj
		}
		if candidates[i].isReviewFail != candidates[j].isReviewFail {
			return candidates[i].isReviewFail
		}
		ni, _ := epicorder.ParseNumeric(candidates[i].task.ID)
		nj, _ := epicorder.ParseNumeric(candidates[j].task.ID)
		return ni < nj
	})

	for _, cand := range candidates {
		c.startCandidate(ctx, boardDir, cand.task, defaultAgent)
	}
	c.emit("auto-implement:scan-complete", map[string]any{})
}

// startCandidate runs the per-task move-then-queue pipeline. Records
// skip reasons (with diagnostics) on every failure path so the
// frontend can surface what happened.
func (c *AutoImplementCoordinator) startCandidate(ctx context.Context, boardDir string, task Task, defaultAgent string) {
	_ = boardDir

	// Set the Agent first if blank, before the canonical move. The
	// daemon's pickup path requires a non-blank Agent and so does
	// AgentService.startAgentRun.
	agentName := strings.ToLower(strings.TrimSpace(task.Agent))
	if agentName == "" {
		c2 := c.board.snapshot()
		if c2 == nil {
			c.logger.Warn("auto-implement: board has no CLI client", "task", task.ID)
			c.recordSkip(task.ID, "no CLI client")
			return
		}
		if err := c2.Edit(ctx, task.ID, cli.EditInput{Agent: defaultAgent}); err != nil {
			c.logger.Warn("auto-implement: set agent failed", "task", task.ID, "agent", defaultAgent, "err", err)
			c.recordSkip(task.ID, "set-agent-failed: "+err.Error())
			return
		}
		agentName = defaultAgent
	}

	// Canonical pull: respects WIP limits per CLAUDE.md. If WIP strict
	// blocks the move, the underlying tb CLI returns a non-zero exit
	// and our wrapper surfaces an error. Record skip + emit so the
	// frontend can render the WIP-blocked diagnostic.
	if err := c.board.PullTask(ctx, task.ID); err != nil {
		c.logger.Info("auto-implement: pull failed", "task", task.ID, "err", err)
		c.recordSkip(task.ID, "pull-failed: "+err.Error())
		c.emit("auto-implement:pull-failed", map[string]any{
			"task_id": task.ID,
			"error":   err.Error(),
		})
		return
	}

	runID, err := c.agent.RunAgentAs(ctx, task.ID, agent.InitiatorAutoImplement)
	if err != nil {
		c.logger.Warn("auto-implement: RunAgentAs failed", "task", task.ID, "err", err)
		c.recordSkip(task.ID, "run-failed: "+err.Error())
		c.emit("auto-implement:run-failed", map[string]any{
			"task_id": task.ID,
			"error":   err.Error(),
		})
		return
	}
	c.mu.Lock()
	delete(c.lastSkip, task.ID)
	c.mu.Unlock()
	c.emit("auto-implement:queued", map[string]any{
		"task_id": task.ID,
		"run_id":  runID,
		"agent":   agentName,
	})
}

// pruneResumeAttempts ages out cooldown entries past their useful life so
// the map can't grow indefinitely across many distinct tasks that each
// failed resume once and then left the interrupted state.
func (c *AutoImplementCoordinator) pruneResumeAttempts() {
	cutoff := c.now().Add(-2 * resumeAttemptCooldown)
	c.mu.Lock()
	for id, ts := range c.resumeAttempts {
		if ts.Before(cutoff) {
			delete(c.resumeAttempts, id)
		}
	}
	c.mu.Unlock()
}

// tryResume calls AgentService.ResumeAgent with a per-task cooldown so a
// persistently failing resume (e.g., session no longer resumable in the
// agent CLI) doesn't get retried on every watcher-driven scan.
// tryAutoResume reanimates a daemon-owned crashed run. interrupted →
// session-resume via ResumeAgentAs; lost → fresh implement run via
// RunAgentAs (no session continuity; the task is already in-progress so
// no pull is needed). Per-task cooldown prevents tight loops on
// persistent failures.
func (c *AutoImplementCoordinator) tryAutoResume(ctx context.Context, id, status string) {
	if c.agent.HasActiveRun(id) {
		c.recordSkip(id, "active run")
		return
	}
	c.mu.Lock()
	if last, ok := c.resumeAttempts[id]; ok && c.now().Sub(last) < resumeAttemptCooldown {
		c.mu.Unlock()
		c.recordSkip(id, "resume cooldown")
		return
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
		runID, err = c.agent.ResumeAgentAs(ctx, id, agent.InitiatorAutoImplement)
	case "lost":
		// No session continuity; the task is already in in-progress so no
		// pull is needed. Re-queues a fresh implement run that will see
		// whatever the previous (now-dead) run left in the worktree.
		op = "restart"
		runID, err = c.agent.RunAgentAs(ctx, id, agent.InitiatorAutoImplement)
	default:
		return
	}
	if err != nil {
		if errors.Is(err, ErrAlreadyRunning) {
			return
		}
		c.logger.Warn("auto-implement: auto-"+op+" failed", "task", id, "err", err)
		c.recordSkip(id, op+" failed: "+err.Error())
		c.emit("auto-implement:resume-failed", map[string]any{
			"task_id": id,
			"op":      op,
			"error":   err.Error(),
		})
		return
	}
	c.mu.Lock()
	delete(c.lastSkip, id)
	delete(c.resumeAttempts, id)
	c.mu.Unlock()
	c.logger.Info("auto-implement: auto-"+op+" of crashed run",
		"task", id, "run_id", runID, "previous_status", status)
	c.emit("auto-implement:resumed", map[string]any{
		"task_id": id,
		"run_id":  runID,
		"op":      op,
	})
}

func (c *AutoImplementCoordinator) recordSkip(taskID, reason string) {
	c.mu.Lock()
	c.lastSkip[taskID] = reason
	c.mu.Unlock()
}

func (c *AutoImplementCoordinator) transitionNeedsDefault(now bool) {
	c.mu.Lock()
	prev := c.lastNeedsDef
	c.lastNeedsDef = now
	c.mu.Unlock()
	if prev == now {
		return
	}
	if now {
		c.emit("auto-implement:needs-default-agent", map[string]any{})
	} else {
		c.emit("auto-implement:default-agent-cleared", map[string]any{})
	}
}

func (c *AutoImplementCoordinator) transitionNeedsQuery(now bool) {
	c.mu.Lock()
	prev := c.lastNeedsQry
	c.lastNeedsQry = now
	c.mu.Unlock()
	if prev == now {
		return
	}
	if now {
		c.emit("auto-implement:needs-query", map[string]any{})
	} else {
		c.emit("auto-implement:query-cleared", map[string]any{})
	}
}

func (c *AutoImplementCoordinator) emit(name string, payload any) {
	if c.emitter == nil {
		return
	}
	c.emitter.Emit(name, payload)
}

func (c *AutoImplementCoordinator) settingsEnabled() bool {
	if c.settings == nil {
		return false
	}
	return c.settings.GetAutoImplementEnabled()
}

func (c *AutoImplementCoordinator) settingsQuery() AutoImplementFilter {
	if c.settings == nil {
		return AutoImplementFilter{}
	}
	return c.settings.GetAutoImplementQuery()
}

func (c *AutoImplementCoordinator) settingsDefaultAgent() string {
	if c.settings == nil {
		return "none"
	}
	return c.settings.GetDefaultAgent()
}

// buildEpicorderSiblings flattens every visible task across all
// canonical columns into the epicorder.Task projection so a single
// EligibleForEpicOrder call can resolve same-parent siblings without
// re-querying the board per task.
func buildEpicorderSiblings(snap BoardSnapshot) []epicorder.Task {
	buckets := [][]Task{snap.Backlog, snap.Ready, snap.InProgress, snap.CodeReview, snap.Done, snap.Archive}
	total := 0
	for _, b := range buckets {
		total += len(b)
	}
	out := make([]epicorder.Task, 0, total)
	for _, bucket := range buckets {
		for _, t := range bucket {
			out = append(out, epicTaskFromBoardTask(t))
		}
	}
	return out
}

func epicTaskFromBoardTask(t Task) epicorder.Task {
	n, _ := epicorder.ParseNumeric(t.ID)
	return epicorder.Task{
		ID:          t.ID,
		Numeric:     n,
		Parent:      t.Parent,
		Status:      t.Status,
		AgentStatus: t.AgentStatus,
		Tags:        t.Tags,
	}
}


// priorityRank maps a priority string to an int where lower is more
// urgent. Unknown / empty priorities sort lowest (largest rank) so they
// trail explicit priorities, matching the CLI's resolveStatusFilter
// sort discipline.
func priorityRank(p string) int {
	switch strings.ToUpper(strings.TrimSpace(p)) {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	default:
		return 4
	}
}

func tagListContains(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}
