package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"tools/tb-gui/internal/agent"
)

const (
	reconcileTransitionAutoGroomReady      = "auto-groom-ready"
	reconcileTransitionAutoImplementStart  = "auto-implement-start"
	reconcileTransitionAutoImplementSubmit = "auto-implement-submit"
	reconcileTransitionReviewFailedReady   = "review-failed-ready"
)

// StageReconciler performs soft deterministic repair for staged autonomous
// transitions. It only acts on objective markers and always delegates board
// mutations to the managed CLI wrappers on BoardService.
type StageReconciler struct {
	board  *BoardService
	logger *slog.Logger
	now    func() time.Time

	skipMu sync.Mutex
}

// NewStageReconciler constructs a daemon-compatible reconciler. A nil logger
// falls back to slog.Default.
func NewStageReconciler(board *BoardService, logger *slog.Logger) *StageReconciler {
	if logger == nil {
		logger = slog.Default()
	}
	return &StageReconciler{
		board:  board,
		logger: logger.With("component", "stage-reconciler"),
		now:    time.Now,
	}
}

// ReconcileActive scans every active column that can contain deterministic
// staged-flow fallout and applies any safe repair.
func (r *StageReconciler) ReconcileActive(ctx context.Context) error {
	if r == nil || r.board == nil {
		return nil
	}
	r.board.clearTriageCache()
	snap, err := r.board.LoadBoard(ctx)
	if err != nil {
		return err
	}
	var ids []string
	for _, bucket := range [][]Task{snap.Backlog, snap.Ready, snap.InProgress, snap.CodeReview} {
		for _, t := range bucket {
			ids = append(ids, t.ID)
		}
	}
	for _, id := range ids {
		if err := r.reconcileTask(ctx, id, snap); err != nil {
			r.logger.Debug("stage reconciliation skipped task after error", "task", id, "err", err)
		}
	}
	return nil
}

// ReconcileTask repairs a single task when a watcher event or terminal run
// identifies a likely stale transition.
func (r *StageReconciler) ReconcileTask(ctx context.Context, id string) error {
	if r == nil || r.board == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	r.board.clearTriageCache()
	snap, err := r.board.LoadBoard(ctx)
	if err != nil {
		return err
	}
	return r.reconcileTask(ctx, id, snap)
}

// OnAgentRunFinished is convenient for Wails event wiring: agent terminal
// events carry only task_id, while the reconciler re-reads the task and JSONL
// state before making any decision.
func (r *StageReconciler) OnAgentRunFinished(payload map[string]any) {
	taskID, _ := payload["task_id"].(string)
	if strings.TrimSpace(taskID) == "" {
		return
	}
	if boardDir, _ := payload["board_dir"].(string); boardDir != "" && !r.isCurrentBoard(boardDir) {
		return
	}
	go func() {
		if err := r.ReconcileTask(context.Background(), taskID); err != nil {
			r.logger.Debug("stage reconciliation after terminal event failed", "task", taskID, "err", err)
		}
	}()
}

func (r *StageReconciler) isCurrentBoard(boardDir string) bool {
	if r == nil || r.board == nil {
		return false
	}
	r.board.mu.RLock()
	defer r.board.mu.RUnlock()
	return r.board.boardDir == boardDir
}

func (r *StageReconciler) reconcileTask(ctx context.Context, id string, snap BoardSnapshot) error {
	detail, err := r.board.GetTask(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	t := detail.Metadata
	if isReconcileProtectedStatus(t.AgentStatus) {
		return nil
	}

	boardDir, err := r.board.resolveBoardDir(ctx)
	if err != nil {
		return err
	}
	run, hasRun, err := latestReconcileRun(boardDir, t.ID)
	if err != nil {
		r.logger.Debug("stage reconciliation: cannot read run history", "task", t.ID, "err", err)
	}

	if err := r.reconcileAutoGroom(ctx, boardDir, t, run, hasRun, snap); err != nil {
		return err
	}
	if err := r.reconcileAutoImplementStart(ctx, boardDir, t, run, hasRun, snap); err != nil {
		return err
	}
	if err := r.reconcileAutoImplementSubmit(ctx, boardDir, t, run, hasRun, snap); err != nil {
		return err
	}
	return r.reconcileReviewFailed(ctx, boardDir, detail, snap)
}

func (r *StageReconciler) reconcileAutoGroom(ctx context.Context, boardDir string, t Task, run reconcileRun, hasRun bool, snap BoardSnapshot) error {
	if !hasRun || t.Status != "backlog" {
		return nil
	}
	if !run.Finished || run.Mode != agent.ModeGroom || run.Status != agent.StatusSuccess || run.Initiator != agent.InitiatorAutoGroom {
		return nil
	}
	triage, err := r.board.Triage(ctx)
	if err != nil {
		return err
	}
	if reasons, stillTriage := triage[t.ID]; stillTriage {
		fp := reconcileFingerprint("triage", t.ID, run.RunID, strings.Join(reasons, "\n"))
		r.recordSkip(boardDir, t.ID, reconcileTransitionAutoGroomReady, fp, "post-groom still needs triage: "+strings.Join(reasons, ", "))
		return nil
	}
	fp := reconcileFingerprint("ready", t.ID, run.RunID, string(run.Status), wipFingerprint(snap, "ready"))
	return r.attemptRepair(ctx, boardDir, t.ID, reconcileTransitionAutoGroomReady, fp, "ready", func() error {
		if autoGroomReadyWIPFull(snap) {
			return errors.New(autoGroomReadyWIPFullReason)
		}
		return r.board.ReadyTaskStrictWIP(ctx, t.ID)
	})
}

func (r *StageReconciler) reconcileAutoImplementStart(ctx context.Context, boardDir string, t Task, run reconcileRun, hasRun bool, snap BoardSnapshot) error {
	if !hasRun || t.Status != "ready" {
		return nil
	}
	if run.Finished || run.Mode != agent.ModeImplement || run.Initiator != agent.InitiatorAutoImplement {
		return nil
	}
	if t.AgentStatus != "queued" && t.AgentStatus != "running" {
		return nil
	}
	fp := reconcileFingerprint("pull", t.ID, run.RunID, t.AgentStatus, wipFingerprint(snap, "in-progress"))
	return r.attemptRepair(ctx, boardDir, t.ID, reconcileTransitionAutoImplementStart, fp, "pull", func() error {
		if blocked, reason := wipStrictBlocked(snap, "in-progress"); blocked {
			return errors.New(reason)
		}
		return r.board.PullTask(ctx, t.ID)
	})
}

func (r *StageReconciler) reconcileAutoImplementSubmit(ctx context.Context, boardDir string, t Task, run reconcileRun, hasRun bool, snap BoardSnapshot) error {
	if !hasRun || t.Status != "in-progress" {
		return nil
	}
	if !run.Finished || run.Mode != agent.ModeImplement || run.Status != agent.StatusSuccess || run.Initiator != agent.InitiatorAutoImplement {
		return nil
	}
	reviewRef := strings.TrimSpace(t.ReviewRef)
	fp := reconcileFingerprint("submit", t.ID, run.RunID, reviewRef, wipFingerprint(snap, "code-review"))
	if reviewRef == "" {
		r.recordSkip(boardDir, t.ID, reconcileTransitionAutoImplementSubmit, fp, "missing ReviewRef; set `tb edit "+t.ID+" --review-ref <branch|PR|commit>` before submitting")
		return nil
	}
	return r.attemptRepair(ctx, boardDir, t.ID, reconcileTransitionAutoImplementSubmit, fp, "review submit", func() error {
		if blocked, reason := wipStrictBlocked(snap, "code-review"); blocked {
			return errors.New(reason)
		}
		return r.board.SubmitReview(ctx, t.ID)
	})
}

func (r *StageReconciler) reconcileReviewFailed(ctx context.Context, boardDir string, detail TaskDetail, snap BoardSnapshot) error {
	t := detail.Metadata
	if t.Status != "code-review" || !taskHasTag(t, "review-failed") {
		return nil
	}
	findings, ok := markdownSectionBody(detail.Body, "## Review Findings")
	if !ok || strings.TrimSpace(findings) == "" {
		return nil
	}
	fp := reconcileFingerprint("review-failed", t.ID, hashString(findings), wipFingerprint(snap, "ready"))
	return r.attemptRepair(ctx, boardDir, t.ID, reconcileTransitionReviewFailedReady, fp, "review fail", func() error {
		if blocked, reason := wipStrictBlocked(snap, "ready"); blocked {
			return errors.New(reason)
		}
		return r.board.FailReview(ctx, t.ID, findings)
	})
}

func (r *StageReconciler) attemptRepair(ctx context.Context, boardDir, taskID, transition, fingerprint, action string, fn func() error) error {
	_ = ctx
	if r.hasCurrentSkip(boardDir, taskID, transition, fingerprint) {
		return nil
	}
	if err := fn(); err != nil {
		r.recordSkip(boardDir, taskID, transition, fingerprint, err.Error())
		r.logger.Info("stage reconciliation repair skipped", "task", taskID, "transition", transition, "action", action, "err", err)
		return nil
	}
	if err := r.clearSkip(boardDir, taskID, transition); err != nil {
		r.logger.Debug("stage reconciliation could not clear skip marker", "task", taskID, "transition", transition, "err", err)
	}
	return nil
}

type reconcileRun struct {
	RunID     string
	Mode      agent.Mode
	Initiator string
	Status    agent.Status
	Queued    bool
	Started   bool
	Finished  bool
}

func latestReconcileRun(boardDir, taskID string) (reconcileRun, bool, error) {
	path := agent.StatePath(boardDir, taskID)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return reconcileRun{}, false, nil
	}
	if err != nil {
		return reconcileRun{}, false, err
	}
	type state struct {
		reconcileRun
	}
	runs := map[string]*state{}
	order := []string{}
	for _, raw := range strings.Split(string(data), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var ev agent.Event
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			continue
		}
		if ev.RunID == "" {
			continue
		}
		st, ok := runs[ev.RunID]
		if !ok {
			st = &state{reconcileRun: reconcileRun{RunID: ev.RunID}}
			runs[ev.RunID] = st
			order = append(order, ev.RunID)
		}
		if ev.Mode != "" {
			st.Mode = parseReconcileMode(ev.Mode)
		}
		switch ev.Event {
		case agent.EvQueued:
			st.Queued = true
			st.Initiator = ev.Initiator
		case agent.EvStarted:
			st.Started = true
		case agent.EvFinished:
			st.Finished = true
			st.Status = ev.Status
		}
	}
	if len(order) == 0 {
		return reconcileRun{}, false, nil
	}
	latest := runs[order[len(order)-1]].reconcileRun
	return latest, true, nil
}

func parseReconcileMode(mode string) agent.Mode {
	switch agent.Mode(mode) {
	case agent.ModeGroom:
		return agent.ModeGroom
	case agent.ModeReview:
		return agent.ModeReview
	case agent.ModeResume:
		return agent.ModeResume
	default:
		return agent.ModeImplement
	}
}

func isReconcileProtectedStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "needs-user", "cancelled", "interrupted", "lost":
		return true
	default:
		return false
	}
}

func taskHasTag(t Task, tag string) bool {
	for _, got := range t.Tags {
		if got == tag {
			return true
		}
	}
	return false
}

func markdownSectionBody(content, heading string) (string, bool) {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "## ") {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n")), true
}

func wipStrictBlocked(snap BoardSnapshot, status string) (bool, string) {
	if snap.WipEnforcement != "strict" {
		return false, ""
	}
	limit, ok := snap.WipLimits[status]
	if !ok || limit <= 0 {
		return false, ""
	}
	count := snap.WipCounts[status]
	if count < limit {
		return false, ""
	}
	return true, fmt.Sprintf("WIP limit reached for %s (%d/%d) - strict mode blocks this move", status, count, limit)
}

func wipFingerprint(snap BoardSnapshot, status string) string {
	return fmt.Sprintf("wip:%s:%s:%d:%d", status, snap.WipEnforcement, snap.WipCounts[status], snap.WipLimits[status])
}

func reconcileFingerprint(parts ...string) string {
	return hashString(strings.Join(parts, "\x00"))
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

type reconcileSkipFile struct {
	Entries map[string]reconcileSkipEntry `json:"entries"`
}

type reconcileSkipEntry struct {
	TaskID      string `json:"task_id"`
	Transition  string `json:"transition"`
	Fingerprint string `json:"fingerprint"`
	Reason      string `json:"reason"`
	Attempts    int    `json:"attempts"`
	UpdatedAt   string `json:"updated_at"`
}

func reconcileSkipPath(boardDir string) string {
	return filepath.Join(boardDir, ".agent-state", "reconcile-skips.json")
}

func reconcileSkipKey(taskID, transition string) string {
	return taskID + "|" + transition
}

func (r *StageReconciler) hasCurrentSkip(boardDir, taskID, transition, fingerprint string) bool {
	r.skipMu.Lock()
	defer r.skipMu.Unlock()
	payload, err := readReconcileSkipFile(boardDir)
	if err != nil {
		r.logger.Debug("stage reconciliation could not read skip file", "err", err)
		return false
	}
	entry, ok := payload.Entries[reconcileSkipKey(taskID, transition)]
	return ok && entry.Fingerprint == fingerprint
}

func (r *StageReconciler) recordSkip(boardDir, taskID, transition, fingerprint, reason string) {
	r.skipMu.Lock()
	defer r.skipMu.Unlock()
	payload, err := readReconcileSkipFile(boardDir)
	if err != nil {
		r.logger.Debug("stage reconciliation could not read skip file before write", "err", err)
		payload = reconcileSkipFile{Entries: map[string]reconcileSkipEntry{}}
	}
	if payload.Entries == nil {
		payload.Entries = map[string]reconcileSkipEntry{}
	}
	key := reconcileSkipKey(taskID, transition)
	if existing, ok := payload.Entries[key]; ok && existing.Fingerprint == fingerprint {
		return
	}
	attempts := 1
	if existing, ok := payload.Entries[key]; ok {
		attempts = existing.Attempts + 1
	}
	payload.Entries[key] = reconcileSkipEntry{
		TaskID:      taskID,
		Transition:  transition,
		Fingerprint: fingerprint,
		Reason:      reason,
		Attempts:    attempts,
		UpdatedAt:   r.now().UTC().Format(time.RFC3339),
	}
	if err := writeReconcileSkipFile(boardDir, payload); err != nil {
		r.logger.Warn("stage reconciliation could not write skip marker", "task", taskID, "transition", transition, "err", err)
	}
}

func (r *StageReconciler) clearSkip(boardDir, taskID, transition string) error {
	r.skipMu.Lock()
	defer r.skipMu.Unlock()
	payload, err := readReconcileSkipFile(boardDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	key := reconcileSkipKey(taskID, transition)
	if _, ok := payload.Entries[key]; !ok {
		return nil
	}
	delete(payload.Entries, key)
	return writeReconcileSkipFile(boardDir, payload)
}

func readReconcileSkipFile(boardDir string) (reconcileSkipFile, error) {
	path := reconcileSkipPath(boardDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return reconcileSkipFile{Entries: map[string]reconcileSkipEntry{}}, nil
	}
	if err != nil {
		return reconcileSkipFile{}, err
	}
	var payload reconcileSkipFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return reconcileSkipFile{}, err
	}
	if payload.Entries == nil {
		payload.Entries = map[string]reconcileSkipEntry{}
	}
	return payload, nil
}

func writeReconcileSkipFile(boardDir string, payload reconcileSkipFile) error {
	path := reconcileSkipPath(boardDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	keys := make([]string, 0, len(payload.Entries))
	for key := range payload.Entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := reconcileSkipFile{Entries: map[string]reconcileSkipEntry{}}
	for _, key := range keys {
		ordered.Entries[key] = payload.Entries[key]
	}
	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
