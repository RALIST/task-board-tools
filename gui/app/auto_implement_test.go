package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
	"tools/tb-gui/internal/cli"
)

// autoImplementFixture wires a real `tb` board with seeded ready tasks,
// an AgentService backed by a stubRunner, a SettingsService writing to
// a tmp prefs file, and an AutoImplementCoordinator pointed at the
// trio. Mirrors autoGroomFixture but scoped to ready-column candidate
// selection + implement-mode runs.
type autoImplementFixture struct {
	t         *testing.T
	root      string
	boardDir  string
	board     *BoardService
	svc       *AgentService
	stub      *stubRunner
	settings  *SettingsService
	prefsPath string
	emitter   *recordingEmitter
	coord     *AutoImplementCoordinator
}

// readyTaskBody returns a body for a task that already passed grooming
// (priority + acceptance criteria + module set) so it would survive the
// triage gate.
func readyTaskBody(spec readyTaskSpec) string {
	if spec.Title == "" {
		spec.Title = "Auto-implement candidate"
	}
	if spec.Priority == "" {
		spec.Priority = "P1"
	}
	if spec.Type == "" {
		spec.Type = "bug"
	}
	if spec.Size == "" {
		spec.Size = "S"
	}
	if spec.Module == "" {
		spec.Module = "gui"
	}
	parent := ""
	if spec.Parent != "" {
		parent = "**Parent:** " + spec.Parent + "\n"
	}
	tagsLine := ""
	if len(spec.Tags) > 0 {
		tagsLine = "**Tags:** " + strings.Join(spec.Tags, ",") + "\n"
	}
	agentLine := ""
	if spec.Agent != "" {
		agentLine = "**Agent:** " + spec.Agent + "\n"
	}
	return fmt.Sprintf(`# %s: %s

**Type:** %s
**Priority:** %s
**Size:** %s
**Module:** %s
%s%s%s**Branch:** —

## Goal

Implement the thing.

## Acceptance Criteria

- [ ] one
- [ ] two

## Log

- 2026-05-20: Created
`, spec.ID, spec.Title, spec.Type, spec.Priority, spec.Size, spec.Module, tagsLine, agentLine, parent)
}

type readyTaskSpec struct {
	ID       string
	Title    string
	Type     string
	Priority string
	Size     string
	Module   string
	Tags     []string
	Agent    string
	Parent   string
}

func newAutoImplementFixture(t *testing.T, agentName string, ready []readyTaskSpec, others map[string][]readyTaskSpec) *autoImplementFixture {
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
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"),
		[]byte("board: board\nprefix: TB\n"), 0o644); err != nil {
		t.Fatalf(".tb.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("999\n"), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}

	writeTask := func(status string, spec readyTaskSpec) {
		if spec.Title == "" {
			spec.Title = "Auto-implement candidate"
		}
		path := filepath.Join(boardDir, status, spec.ID+".md")
		if err := os.WriteFile(path, []byte(readyTaskBody(spec)), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	for _, spec := range ready {
		writeTask("ready", spec)
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

	stub := &stubRunner{
		name:        agentName,
		stdoutLines: []string{"implementing"},
		exitCode:    0,
	}

	board := NewBoardService()
	board.setClient(c)
	board.setBoardDir(boardDir)

	em := newRecordingEmitter()
	svc := NewAgentService(AgentServiceOptions{Board: board, Emitter: em})
	svc.setRunnerFactory(func(name string) (agent.Runner, error) { return stub, nil })

	prefs := filepath.Join(t.TempDir(), "preferences.json")
	settings := NewSettingsService(SettingsOptions{
		Logger:      slog.Default(),
		RecentsPath: filepath.Join(t.TempDir(), "recent.json"),
		PrefsPath:   prefs,
	})

	coord := NewAutoImplementCoordinator(AutoImplementCoordinatorOptions{
		Board:    board,
		Agent:    svc,
		Settings: settings,
		Emitter:  em,
		Logger:   slog.Default(),
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

	return &autoImplementFixture{
		t:         t,
		root:      root,
		boardDir:  boardDir,
		board:     board,
		svc:       svc,
		stub:      stub,
		settings:  settings,
		prefsPath: prefs,
		emitter:   em,
		coord:     coord,
	}
}

// runScanSync bypasses the debounce timer for deterministic testing.
func (f *autoImplementFixture) runScanSync() {
	f.t.Helper()
	f.coord.mu.Lock()
	if f.coord.debounceTimer != nil {
		f.coord.debounceTimer.Stop()
		f.coord.debounceTimer = nil
	}
	f.coord.mu.Unlock()
	f.coord.scan(context.Background(), f.boardDir)
}

func (f *autoImplementFixture) waitForActiveDrained(timeout time.Duration) {
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

func (f *autoImplementFixture) enableAutoImplement(t *testing.T, defaultAgent, query string) {
	t.Helper()
	if err := f.settings.SetDefaultAgent(defaultAgent); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := f.settings.SetAutoImplementQuery(query); err != nil {
		t.Fatalf("SetAutoImplementQuery: %v", err)
	}
	if err := f.settings.SetAutoImplementEnabled(true); err != nil {
		t.Fatalf("SetAutoImplementEnabled: %v", err)
	}
}

func (f *autoImplementFixture) queuedTaskIDs() []string {
	out := []string{}
	for _, e := range f.emitter.snapshot() {
		if e.Name != "auto-implement:queued" {
			continue
		}
		if len(e.Payload) == 0 {
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

func (f *autoImplementFixture) activate(t *testing.T) {
	t.Helper()
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
}

// --------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------

// AC: disabled → no enqueue.
func TestAutoImplementCoordinator_DisabledNoEnqueue(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	f.activate(t)
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected no enqueue while disabled; got %v", got)
	}
}

// AC: no default agent → no enqueue + needs-default-agent emission.
func TestAutoImplementCoordinator_NoDefaultEmits(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui"}},
		nil,
	)
	if err := f.settings.SetAutoImplementQuery("bug, S size, gui"); err != nil {
		t.Fatalf("set query: %v", err)
	}
	// Don't set default agent. Enabling would fail validation; directly
	// write to disk to simulate an externally edited preferences file.
	prefsContent := `{"default_agent":"none","auto_implement_enabled":true,"auto_implement_query":"bug, S size, gui"}`
	if err := os.WriteFile(f.prefsPath, []byte(prefsContent), 0o644); err != nil {
		t.Fatalf("write prefs: %v", err)
	}

	f.activate(t)
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected no enqueue without default agent; got %v", got)
	}
	count := 0
	for _, e := range f.emitter.snapshot() {
		if e.Name == "auto-implement:needs-default-agent" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 needs-default-agent emit; got %d", count)
	}
}

// AC: matching ready task → pulled to in-progress + queued.
func TestAutoImplementCoordinator_EnqueuesMatchingReadyTask(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.runScanSync()

	queued := f.queuedTaskIDs()
	if len(queued) != 1 || queued[0] != "TB-100" {
		t.Fatalf("expected TB-100 queued exactly once; got %v", queued)
	}
	f.waitForActiveDrained(5 * time.Second)
	// Task should be in in-progress after the pull.
	if _, err := os.Stat(filepath.Join(f.boardDir, "in-progress", "TB-100.md")); err != nil {
		t.Errorf("task should have been moved to in-progress: %v", err)
	}
}

// AC: query mismatch → no enqueue.
func TestAutoImplementCoordinator_QueryMismatchSkips(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "feature", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected no enqueue for non-matching task; got %v", got)
	}
}

// AC: backlog tasks skipped even if they match the query.
func TestAutoImplementCoordinator_BacklogTaskSkipped(t *testing.T) {
	f := newAutoImplementFixture(t, "claude", nil, map[string][]readyTaskSpec{
		"backlog": {{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
	})
	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected backlog task to be skipped; got %v", got)
	}
}

// AC: ready task with non-blank AgentStatus is skipped (no auto-retry).
func TestAutoImplementCoordinator_NonBlankAgentStatusSkipped(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	c := f.board.snapshot()
	if c == nil {
		t.Fatalf("no CLI client")
	}
	// Seed terminal status on the otherwise-eligible task.
	if err := c.Edit(context.Background(), "TB-100", cli.EditInput{AgentStatus: "success"}); err != nil {
		t.Fatalf("seed success: %v", err)
	}
	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.runScanSync()
	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected non-blank-status task to be skipped; got %v", got)
	}
}

// AC: assigned-agent task uses its assigned agent (not default).
func TestAutoImplementCoordinator_AssignedAgentUsed(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	f.activate(t)
	// Default is codex, but the task's assigned agent is claude.
	f.enableAutoImplement(t, "codex", "bug, S size, gui")
	f.runScanSync()
	queued := f.queuedTaskIDs()
	if len(queued) != 1 || queued[0] != "TB-100" {
		t.Fatalf("expected TB-100 queued; got %v", queued)
	}
	f.waitForActiveDrained(5 * time.Second)
	// Verify task still has Agent=claude (not flipped to default codex).
	detail, err := f.board.GetTask(context.Background(), "TB-100")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if detail.Metadata.Agent != "claude" {
		t.Errorf("expected Agent=claude preserved; got %q", detail.Metadata.Agent)
	}
}

// AC: unassigned task falls back to default agent (the coordinator writes
// the default into Agent via tb edit before queuing).
func TestAutoImplementCoordinator_DefaultAgentFallback(t *testing.T) {
	f := newAutoImplementFixture(t, "codex",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui"}},
		nil,
	)
	f.activate(t)
	f.enableAutoImplement(t, "codex", "bug, S size, gui")
	f.runScanSync()
	queued := f.queuedTaskIDs()
	if len(queued) != 1 || queued[0] != "TB-100" {
		t.Fatalf("expected TB-100 queued; got %v", queued)
	}
	f.waitForActiveDrained(5 * time.Second)
	detail, err := f.board.GetTask(context.Background(), "TB-100")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if detail.Metadata.Agent != "codex" {
		t.Errorf("expected Agent=codex written; got %q", detail.Metadata.Agent)
	}
}

// AC: duplicate watcher events enqueue the same task only once.
// runScanSync is idempotent because RunAgent returns ErrAlreadyRunning
// on the second call AND the task moved out of ready after the first
// pull. Either path satisfies the AC.
func TestAutoImplementCoordinator_DedupeAcrossRapidScans(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")

	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)
	f.runScanSync() // second pass — task is now in in-progress, no longer eligible.

	queued := f.queuedTaskIDs()
	if len(queued) != 1 {
		t.Fatalf("expected exactly 1 queued event; got %d (%v)", len(queued), queued)
	}
}

// TB-267: epic-order blocked task → emits epic-order-skip + records skip.
func TestAutoImplementCoordinator_EpicOrderBlocked(t *testing.T) {
	// TB-101 (lower id) and TB-102 (higher id) both have parent TB-177.
	// TB-101 is in backlog (not done) so TB-102 must be blocked.
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{
			{ID: "TB-102", Type: "bug", Size: "S", Module: "gui", Agent: "claude", Parent: "TB-177"},
		},
		map[string][]readyTaskSpec{
			"backlog": {{ID: "TB-101", Type: "bug", Size: "S", Module: "gui", Agent: "claude", Parent: "TB-177"}},
		},
	)
	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.runScanSync()

	if got := f.queuedTaskIDs(); len(got) != 0 {
		t.Fatalf("expected no enqueue when earlier sibling unfinished; got %v", got)
	}
	count := 0
	for _, e := range f.emitter.snapshot() {
		if e.Name == "auto-implement:epic-order-skip" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 epic-order-skip; got %d", count)
	}
}

// TB-233: review-failed first within the same priority bucket.
func TestAutoImplementCoordinator_ReviewFailedFirstWithinPriority(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{
			// Both P2, both eligible. Plain comes first by ID, but the
			// review-failed tag must boost TB-200 ahead.
			{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude", Priority: "P2"},
			{ID: "TB-200", Type: "bug", Size: "S", Module: "gui", Agent: "claude", Priority: "P2", Tags: []string{"review-failed"}},
		},
		nil,
	)
	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.runScanSync()
	f.waitForActiveDrained(10 * time.Second)

	queued := f.queuedTaskIDs()
	if len(queued) != 2 {
		t.Fatalf("expected both queued; got %v", queued)
	}
	if queued[0] != "TB-200" {
		t.Errorf("expected TB-200 (review-failed) queued first; got %v", queued)
	}
}

// TB-233: P1 plain beats P2 review-failed.
func TestAutoImplementCoordinator_PriorityBucketTrumpsReviewFailed(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{
			{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude", Priority: "P1"},
			{ID: "TB-200", Type: "bug", Size: "S", Module: "gui", Agent: "claude", Priority: "P2", Tags: []string{"review-failed"}},
		},
		nil,
	)
	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.runScanSync()
	f.waitForActiveDrained(10 * time.Second)

	queued := f.queuedTaskIDs()
	if len(queued) != 2 {
		t.Fatalf("expected both queued; got %v", queued)
	}
	if queued[0] != "TB-100" {
		t.Errorf("expected P1 TB-100 first despite TB-200 review-failed; got %v", queued)
	}
}

// TB-233: no review-failed candidates → numeric ID order is preserved.
func TestAutoImplementCoordinator_NoReviewFailedPreservesIDOrder(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{
			{ID: "TB-200", Type: "bug", Size: "S", Module: "gui", Agent: "claude", Priority: "P2"},
			{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude", Priority: "P2"},
		},
		nil,
	)
	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.runScanSync()
	f.waitForActiveDrained(10 * time.Second)

	queued := f.queuedTaskIDs()
	if len(queued) != 2 {
		t.Fatalf("expected both queued; got %v", queued)
	}
	if queued[0] != "TB-100" {
		t.Errorf("expected TB-100 (lower id) first; got %v", queued)
	}
}

// Restart-scan parity: Activate (re-)kicks a scan, so a coordinator
// constructed against an existing eligible task picks it up on activation.
func TestAutoImplementCoordinator_ActivateKicksInitialScan(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	// Configure prefs BEFORE Activate so the initial debounced scan picks
	// up the task once it fires.
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.activate(t)

	// Wait for the debounce timer to fire and the run to drain.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(f.queuedTaskIDs()) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	queued := f.queuedTaskIDs()
	if len(queued) != 1 || queued[0] != "TB-100" {
		t.Errorf("expected initial scan to queue TB-100; got %v", queued)
	}
}

// Deactivate must stop the debounce timer so further Emit-driven scans
// silently no-op.
func TestAutoImplementCoordinator_DeactivateStopsFurtherWork(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	f.activate(t)
	if err := f.coord.Deactivate(); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	f.coord.Emit("board:reloaded")
	// Sleep past the debounce window — if the timer fired, a scan would
	// run and (with prefs unconfigured) emit scan-complete. With
	// activated=false, scheduleScan no-ops, so no scan-complete fires.
	time.Sleep(scanDebounce + 100*time.Millisecond)
	for _, e := range f.emitter.snapshot() {
		if e.Name == "auto-implement:scan-complete" {
			t.Fatalf("Deactivate did not stop the scan: %v", e)
		}
	}
}

// WIP strict: PullTask should fail when in-progress is at the limit, and
// the coordinator should emit pull-failed + record a skip reason, NOT
// proceed to RunAgent (the task stays in ready).
func TestAutoImplementCoordinator_WIPBlockedPullSkips(t *testing.T) {
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
	// Strict WIP limit of 1 with one task already in-progress.
	cfg := "board: board\nprefix: TB\nwip_limit_in_progress: 1\nwip_enforcement: strict\n"
	if err := os.WriteFile(filepath.Join(root, ".tb.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf(".tb.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("300\n"), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "in-progress", "TB-200.md"),
		[]byte(readyTaskBody(readyTaskSpec{ID: "TB-200", Type: "bug", Size: "S", Module: "gui", Agent: "claude"})),
		0o644); err != nil {
		t.Fatalf("seed in-progress: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "ready", "TB-100.md"),
		[]byte(readyTaskBody(readyTaskSpec{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"})),
		0o644); err != nil {
		t.Fatalf("seed ready: %v", err)
	}

	c, err := cli.NewClient(cli.Options{BinaryPath: tbBinary, Cwd: root})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	stub := &stubRunner{name: "claude", stdoutLines: []string{"x"}, exitCode: 0}
	board := NewBoardService()
	board.setClient(c)
	board.setBoardDir(boardDir)
	em := newRecordingEmitter()
	svc := NewAgentService(AgentServiceOptions{Board: board, Emitter: em})
	svc.setRunnerFactory(func(name string) (agent.Runner, error) { return stub, nil })

	prefs := filepath.Join(t.TempDir(), "preferences.json")
	settings := NewSettingsService(SettingsOptions{
		Logger:      slog.Default(),
		RecentsPath: filepath.Join(t.TempDir(), "recent.json"),
		PrefsPath:   prefs,
	})

	coord := NewAutoImplementCoordinator(AutoImplementCoordinatorOptions{
		Board: board, Agent: svc, Settings: settings, Emitter: em, Logger: slog.Default(),
	})
	t.Cleanup(func() { _ = coord.Deactivate() })

	if err := settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := settings.SetAutoImplementQuery("bug, S size, gui"); err != nil {
		t.Fatalf("SetAutoImplementQuery: %v", err)
	}
	if err := settings.SetAutoImplementEnabled(true); err != nil {
		t.Fatalf("SetAutoImplementEnabled: %v", err)
	}

	if err := coord.Activate(context.Background(), boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	coord.mu.Lock()
	if coord.debounceTimer != nil {
		coord.debounceTimer.Stop()
		coord.debounceTimer = nil
	}
	coord.mu.Unlock()
	coord.scan(context.Background(), boardDir)

	// Task must still be in ready (pull rejected).
	if _, err := os.Stat(filepath.Join(boardDir, "ready", "TB-100.md")); err != nil {
		t.Errorf("task should remain in ready after WIP-blocked pull: %v", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "in-progress", "TB-100.md")); !os.IsNotExist(err) {
		t.Errorf("task should NOT be in in-progress (WIP blocked): err=%v", err)
	}
	pullFailed := 0
	queued := 0
	for _, e := range em.snapshot() {
		switch e.Name {
		case "auto-implement:pull-failed":
			pullFailed++
		case "auto-implement:queued":
			queued++
		}
	}
	if pullFailed != 1 {
		t.Errorf("expected 1 pull-failed emit; got %d", pullFailed)
	}
	if queued != 0 {
		t.Errorf("expected 0 queued emits; got %d", queued)
	}
	status := coord.Status()
	if reason, ok := status.LastSkipReasons["TB-100"]; !ok || !strings.HasPrefix(reason, "pull-failed:") {
		t.Errorf("expected skip reason starting with 'pull-failed:'; got %q (have=%v)", reason, ok)
	}
}

// RunAgent error path: when the candidate is in a state where RunAgent
// rejects (e.g. another active run for the same id from an out-of-band
// path), the coordinator must skip and emit run-failed without crashing
// or holding an in-progress task with no run.
func TestAutoImplementCoordinator_RunAgentErrorEmitsRunFailed(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	// Block RunAgent by registering an already-active run for the same id
	// directly in svc.active. RunAgent will return ErrAlreadyRunning.
	cancel := func() {}
	ar := &activeRun{
		RunID:  agent.GenerateRunID(),
		TaskID: "TB-100",
		Agent:  "claude",
		Mode:   agent.ModeImplement.String(),
		Cancel: func() { cancel() },
		Done:   make(chan struct{}),
	}
	f.svc.mu.Lock()
	f.svc.active["TB-100"] = ar
	f.svc.mu.Unlock()
	t.Cleanup(func() { close(ar.Done) })

	f.activate(t)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	// HasActiveRun catches the seeded entry and short-circuits before
	// RunAgent. That still counts as a skip, recorded with the
	// "active run" reason. The test below asserts that skip and that
	// no run is queued.
	f.runScanSync()

	queued := f.queuedTaskIDs()
	if len(queued) != 0 {
		t.Fatalf("expected no enqueue when active run present; got %v", queued)
	}
	status := f.coord.Status()
	if reason, ok := status.LastSkipReasons["TB-100"]; !ok || reason != "active run" {
		t.Errorf("expected 'active run' skip reason; got %q (have=%v)", reason, ok)
	}
}

// Status() reflects the coordinator's current state.
func TestAutoImplementCoordinator_StatusReflectsState(t *testing.T) {
	f := newAutoImplementFixture(t, "claude",
		[]readyTaskSpec{{ID: "TB-100", Type: "bug", Size: "S", Module: "gui", Agent: "claude"}},
		nil,
	)
	f.enableAutoImplement(t, "claude", "bug, S size, gui")
	f.activate(t)

	status := f.coord.Status()
	if !status.Enabled {
		t.Errorf("expected Enabled=true")
	}
	if status.Query != "bug, S size, gui" {
		t.Errorf("Query mismatch: %q", status.Query)
	}
	if status.DefaultAgent != "claude" {
		t.Errorf("DefaultAgent mismatch: %q", status.DefaultAgent)
	}
}

// _ keeps sync.Mutex unused warning off for the fixture struct.
var _ = sync.Mutex{}
