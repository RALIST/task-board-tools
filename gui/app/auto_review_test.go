package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

type autoReviewFixture struct {
	t         *testing.T
	root      string
	boardDir  string
	board     *BoardService
	svc       *AgentService
	stub      *stubRunner
	settings  *SettingsService
	prefsPath string
	emitter   *recordingEmitter
	budget    *fakeWorkerBudget
	coord     *AutoReviewCoordinator
}

type reviewTaskSpec struct {
	readyTaskSpec
	ReviewRef       string
	ReviewTarget    string
	AgentStatus     string
	ImplementStatus string
	ReviewStatus    string
}

func reviewTaskBody(spec reviewTaskSpec) string {
	body := readyTaskBody(spec.readyTaskSpec)
	lines := []string{"**Branch:** —"}
	if spec.ReviewRef != "" {
		lines = append(lines, "**ReviewRef:** "+spec.ReviewRef)
	}
	if spec.AgentStatus != "" {
		lines = append(lines, "**AgentStatus:** "+spec.AgentStatus)
	}
	if spec.ImplementStatus != "" {
		lines = append(lines, "**ImplementedBy:** claude", "**ImplementStatus:** "+spec.ImplementStatus)
	}
	if spec.ReviewStatus != "" {
		lines = append(lines, "**ReviewedBy:** claude", "**ReviewStatus:** "+spec.ReviewStatus)
	}
	body = strings.Replace(body, "**Branch:** —", strings.Join(lines, "\n"), 1)
	if spec.ReviewTarget != "" {
		body = strings.Replace(body, "## Related Tasks", "## Review Target\n\n"+spec.ReviewTarget+"\n\n## Related Tasks", 1)
	}
	body = strings.Replace(body, "- 2026-05-20: Created\n", "- 2026-05-20: Created\n- 2026-05-21: Submitted to code-review\n", 1)
	return body
}

func newAutoReviewFixture(t *testing.T, runnerName string, codeReview []reviewTaskSpec, others map[string][]reviewTaskSpec) *autoReviewFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only board flock")
	}
	tbBinary := buildTbForIntegration(t)

	root := t.TempDir()
	boardDir := filepath.Join(root, "board")
	for _, d := range []string{"backlog", "ready", "in-progress", "code-review", "done", "archive"} {
		if err := os.MkdirAll(filepath.Join(boardDir, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte("board: board\nprefix: TB\n"), 0o644); err != nil {
		t.Fatalf(".tb.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("999\n"), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}

	writeTask := func(status string, spec reviewTaskSpec) {
		path := filepath.Join(boardDir, status, spec.ID+".md")
		if err := os.WriteFile(path, []byte(reviewTaskBody(spec)), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	for _, spec := range codeReview {
		writeTask("code-review", spec)
	}
	for status, specs := range others {
		for _, spec := range specs {
			writeTask(status, spec)
		}
	}

	c, err := cli.NewClient(cli.Options{BinaryPath: tbBinary, Cwd: root})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	board := NewBoardService()
	board.setClient(c)
	board.setBoardDir(boardDir)

	em := newRecordingEmitter()
	budget := &fakeWorkerBudget{max: 64}
	stub := &stubRunner{name: runnerName, stdoutLines: []string{"reviewing"}, exitCode: 0}
	svc := NewAgentService(AgentServiceOptions{
		Board:        board,
		Emitter:      em,
		WorkerBudget: func() AutomationWorkerBudget { return budget },
	})
	svc.setRunnerFactory(func(name string) (agent.Runner, error) {
		switch strings.TrimSpace(name) {
		case AgentClaude, AgentCodex:
			return stub, nil
		default:
			return nil, fmt.Errorf("%w: %q", ErrAgentNotSupported, name)
		}
	})

	prefs := filepath.Join(t.TempDir(), "preferences.json")
	settings := NewSettingsService(SettingsOptions{
		Logger:      slog.Default(),
		RecentsPath: filepath.Join(t.TempDir(), "recent.json"),
		PrefsPath:   prefs,
	})
	coord := NewAutoReviewCoordinator(AutoReviewCoordinatorOptions{
		Board:        board,
		Agent:        svc,
		Settings:     settings,
		Emitter:      em,
		Logger:       slog.Default(),
		WorkerBudget: budget,
	})

	t.Cleanup(func() {
		_ = coord.Deactivate()
		svc.mu.Lock()
		runs := make([]*activeRun, 0, len(svc.active))
		for _, ar := range svc.active {
			runs = append(runs, ar)
		}
		svc.mu.Unlock()
		for _, ar := range runs {
			ar.Cancel()
			<-ar.Done
		}
	})

	return &autoReviewFixture{
		t:         t,
		root:      root,
		boardDir:  boardDir,
		board:     board,
		svc:       svc,
		stub:      stub,
		settings:  settings,
		prefsPath: prefs,
		emitter:   em,
		budget:    budget,
		coord:     coord,
	}
}

func (f *autoReviewFixture) activate(t *testing.T) {
	t.Helper()
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
}

func (f *autoReviewFixture) runScanSync() {
	f.t.Helper()
	f.coord.mu.Lock()
	if f.coord.debounceTimer != nil {
		f.coord.debounceTimer.Stop()
		f.coord.debounceTimer = nil
	}
	f.coord.mu.Unlock()
	f.coord.scan(context.Background(), f.boardDir)
}

func (f *autoReviewFixture) waitForActiveDrained(timeout time.Duration) {
	f.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		f.svc.mu.Lock()
		empty := len(f.svc.active) == 0
		f.svc.mu.Unlock()
		if empty {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.t.Fatalf("active runs did not drain within %v", timeout)
}

func (f *autoReviewFixture) enableAutoReview(t *testing.T, defaultAgent string) {
	t.Helper()
	if err := f.settings.SetDefaultAgent(defaultAgent); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := f.settings.SetAutoReviewEnabled(true); err != nil {
		t.Fatalf("SetAutoReviewEnabled: %v", err)
	}
}

func (f *autoReviewFixture) queuedTaskIDs() []string {
	out := []string{}
	for _, e := range f.emitter.snapshot() {
		if e.Name != "auto-review:queued" || len(e.Payload) == 0 {
			continue
		}
		if payload, ok := e.Payload[0].(map[string]any); ok {
			if id, ok := payload["task_id"].(string); ok {
				out = append(out, id)
			}
		}
	}
	return out
}

func (f *autoReviewFixture) emittedTaskIDs(name string) []string {
	out := []string{}
	for _, e := range f.emitter.snapshot() {
		if e.Name != name || len(e.Payload) == 0 {
			continue
		}
		if payload, ok := e.Payload[0].(map[string]any); ok {
			if id, ok := payload["task_id"].(string); ok {
				out = append(out, id)
			}
		}
	}
	return out
}

func appendQueuedReviewEvent(t *testing.T, boardDir, taskID, initiator string) {
	t.Helper()
	if err := agent.AppendEvent(boardDir, taskID, agent.Event{
		TS:        time.Now().UTC().Format(time.RFC3339),
		RunID:     agent.GenerateRunID(),
		TaskID:    taskID,
		Event:     agent.EvQueued,
		Agent:     "claude",
		Mode:      agent.ModeReview.String(),
		Initiator: initiator,
	}); err != nil {
		t.Fatalf("append queued event: %v", err)
	}
}

func appendQueuedReviewFingerprintEvent(t *testing.T, boardDir, taskID, fingerprint string) {
	t.Helper()
	if err := agent.AppendEvent(boardDir, taskID, agent.Event{
		TS:        time.Now().UTC().Format(time.RFC3339),
		RunID:     agent.GenerateRunID(),
		TaskID:    taskID,
		Event:     agent.EvQueued,
		Agent:     "claude",
		Mode:      agent.ModeReview.String(),
		Initiator: agent.InitiatorAutoReview,
		Target:    fingerprint,
	}); err != nil {
		t.Fatalf("append queued fingerprint event: %v", err)
	}
}

func TestAutoReviewCoordinator_DisabledNoEnqueue(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:tb-100"}},
		nil,
	)
	f.activate(t)
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected no enqueue while disabled; got %v", got)
	}
}

func TestAutoReviewCoordinator_NoDefaultEmits(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100"}, ReviewRef: "branch:tb-100"}},
		nil,
	)
	if err := os.WriteFile(f.prefsPath, []byte(`{"default_agent":"none","auto_review_enabled":true}`), 0o644); err != nil {
		t.Fatalf("write prefs: %v", err)
	}

	f.activate(t)
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected no enqueue without default agent; got %v", got)
	}
	count := 0
	for _, e := range f.emitter.snapshot() {
		if e.Name == "auto-review:needs-default-agent" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 needs-default-agent event; got %d", count)
	}
}

func TestAutoReviewCoordinator_EnqueuesEligibleCodeReviewTask(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:tb-100", ReviewTarget: "Inspect branch tb-100."}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()

	queued := f.queuedTaskIDs()
	if len(queued) != 1 || queued[0] != "TB-100" {
		t.Fatalf("expected TB-100 queued exactly once; got %v", queued)
	}
	f.waitForActiveDrained(5 * time.Second)
	if f.stub.input().Mode != agent.ModeReview {
		t.Fatalf("runner mode = %s, want review", f.stub.input().Mode)
	}
	events := readEvents(t, f.boardDir, "TB-100")
	var found bool
	for _, ev := range events {
		if ev.Event == agent.EvQueued && ev.Mode == agent.ModeReview.String() && ev.Initiator == agent.InitiatorAutoReview {
			found = true
			if ev.Target == "" {
				t.Fatalf("queued auto-review event missing review fingerprint: %+v", ev)
			}
		}
	}
	if !found {
		t.Fatalf("queued auto-review event not found in %+v", events)
	}
}

func TestAutoReviewCoordinator_ReviewRefWithoutProseStillQueues(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:tb-100"}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 1 || got[0] != "TB-100" {
		t.Fatalf("expected ReviewRef-only task queued; got %v", got)
	}
}

func TestAutoReviewCoordinator_ImplementSuccessStatusDoesNotBlock(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{
			readyTaskSpec:   readyTaskSpec{ID: "TB-100", Agent: "claude"},
			ReviewRef:       "branch:tb-100",
			AgentStatus:     "success",
			ImplementStatus: "success",
		}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 1 || got[0] != "TB-100" {
		t.Fatalf("expected implement-success task queued; got %v", got)
	}
}

func TestAutoReviewCoordinator_MissingReviewRefNeedsUser(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()

	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected missing ReviewRef not queued; got %v", got)
	}
	detail, err := f.board.GetTask(context.Background(), "TB-100")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if detail.Metadata.AgentStatus != "needs-user" {
		t.Fatalf("AgentStatus = %q, want needs-user", detail.Metadata.AgentStatus)
	}
	if !strings.Contains(detail.Body, "## User Attention") || !strings.Contains(detail.Body, "missing review target") {
		t.Fatalf("missing User Attention handoff:\n%s", detail.Body)
	}
}

func TestAutoReviewCoordinator_AssignedAgentUsed(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:tb-100"}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "codex")
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	detail, err := f.board.GetTask(context.Background(), "TB-100")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if detail.Metadata.Agent != "claude" {
		t.Fatalf("Agent = %q, want explicit claude preserved", detail.Metadata.Agent)
	}
}

func TestAutoReviewCoordinator_DefaultAgentFallback(t *testing.T) {
	f := newAutoReviewFixture(t, "codex",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100"}, ReviewRef: "branch:tb-100"}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "codex")
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	detail, err := f.board.GetTask(context.Background(), "TB-100")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if detail.Metadata.Agent != "codex" {
		t.Fatalf("Agent = %q, want default codex written", detail.Metadata.Agent)
	}
}

func TestAutoReviewCoordinator_DurableDedupeAcrossRestart(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:tb-100", ReviewTarget: "Inspect branch."}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	restarted := NewAutoReviewCoordinator(AutoReviewCoordinatorOptions{
		Board:        f.board,
		Agent:        f.svc,
		Settings:     f.settings,
		Emitter:      f.emitter,
		Logger:       slog.Default(),
		WorkerBudget: f.budget,
	})
	if err := restarted.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("restart Activate: %v", err)
	}
	restarted.scan(context.Background(), f.boardDir)
	if got := f.queuedTaskIDs(); len(got) != 1 {
		t.Fatalf("expected one queued event across restart; got %d (%v)", len(got), got)
	}
}

func TestAutoReviewCoordinator_StaleQueuedFingerprintDoesNotDedupe(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:tb-100", ReviewTarget: "Inspect branch."}},
		nil,
	)
	detail, err := f.board.GetTask(context.Background(), "TB-100")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	appendQueuedReviewFingerprintEvent(t, f.boardDir, "TB-100", autoReviewFingerprint(detail.Metadata, detail.Body))

	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()

	if got := f.queuedTaskIDs(); len(got) != 1 || got[0] != "TB-100" {
		t.Fatalf("expected stale queued fingerprint not to dedupe; got %v", got)
	}
}

func TestAutoReviewCoordinator_RepeatedCodeReviewLogLineRetries(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:tb-100", ReviewTarget: "Inspect branch."}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	path := filepath.Join(f.boardDir, "code-review", "TB-100.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read task: %v", err)
	}
	updated := strings.Replace(string(data),
		"- 2026-05-21: Submitted to code-review\n",
		"- 2026-05-21: Submitted to code-review\n- 2026-05-21: Submitted to code-review\n",
		1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	if got := f.queuedTaskIDs(); len(got) != 2 {
		t.Fatalf("expected second queue after repeated submit log; got %d (%v)", len(got), got)
	}
}

func TestAutoReviewCoordinator_ReviewRefChangeRetries(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:old"}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	if err := f.board.EditTask(context.Background(), "TB-100", EditTaskInput{ReviewRef: "branch:new"}); err != nil {
		t.Fatalf("change ReviewRef: %v", err)
	}
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	if got := f.queuedTaskIDs(); len(got) != 2 {
		t.Fatalf("expected two queued events after ReviewRef change; got %d (%v)", len(got), got)
	}
}

func TestAutoReviewCoordinator_WrongColumnSkipped(t *testing.T) {
	f := newAutoReviewFixture(t, "claude", nil, map[string][]reviewTaskSpec{
		"ready": {{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:tb-100"}},
	})
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected ready task skipped; got %v", got)
	}
}

func TestAutoReviewCoordinator_NeedsUserSkipped(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"}, ReviewRef: "branch:tb-100", AgentStatus: "needs-user"}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected needs-user task skipped; got %v", got)
	}
}

func TestAutoReviewCoordinator_UnsupportedExplicitAgentSkipped(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "gemini"}, ReviewRef: "branch:tb-100"}},
		nil,
	)
	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected unsupported agent skipped; got %v", got)
	}
	status := f.coord.Status()
	if !strings.Contains(status.LastSkipReasons["TB-100"], "agent-unsupported") {
		t.Fatalf("skip reason = %q, want agent-unsupported", status.LastSkipReasons["TB-100"])
	}
}

func TestAutoReviewCoordinator_LostAutoReviewRestarts(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{
			readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"},
			ReviewRef:     "branch:tb-100",
			AgentStatus:   "lost",
		}},
		nil,
	)
	appendQueuedReviewEvent(t, f.boardDir, "TB-100", agent.InitiatorAutoReview)

	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()

	if got := f.emittedTaskIDs("auto-review:resumed"); len(got) != 1 || got[0] != "TB-100" {
		t.Fatalf("expected lost auto-review restarted once; got %v", got)
	}
}

func TestAutoReviewCoordinator_UserLostReviewNotAutoResumed(t *testing.T) {
	f := newAutoReviewFixture(t, "claude",
		[]reviewTaskSpec{{
			readyTaskSpec: readyTaskSpec{ID: "TB-100", Agent: "claude"},
			ReviewRef:     "branch:tb-100",
			AgentStatus:   "lost",
		}},
		nil,
	)
	appendQueuedReviewEvent(t, f.boardDir, "TB-100", "")

	f.activate(t)
	f.enableAutoReview(t, "claude")
	f.runScanSync()

	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected user-owned lost review skipped; got %v", got)
	}
	status := f.coord.Status()
	if !strings.Contains(status.LastSkipReasons["TB-100"], "user-initiated") {
		t.Fatalf("skip reason = %q, want user-initiated", status.LastSkipReasons["TB-100"])
	}
}
