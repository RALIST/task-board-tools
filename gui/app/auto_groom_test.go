package app

import (
	"context"
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

// autoGroomFixture wires the moving pieces: a real `tb` board with one
// triage-worthy backlog task, a fake AgentService whose runner is a
// stubRunner, a SettingsService writing to a tmp prefs file, and an
// AutoGroomCoordinator wired against all three. The clock is virtual so
// settle-window tests can advance time without sleeping.
type autoGroomFixture struct {
	t          *testing.T
	root       string
	boardDir   string
	board      *BoardService
	svc        *AgentService
	stub       *stubRunner
	settings   *SettingsService
	prefsPath  string
	emitter    *recordingEmitter
	coord      *AutoGroomCoordinator
	clock      *fakeClock
}

// fakeClock is a deterministic time source. now() is read-only after
// construction; Set advances it to a new value. Independent of the
// coordinator's internal clock so tests can control it directly.
type fakeClock struct {
	mu  sync.Mutex
	cur time.Time
}

func newFakeClock(start time.Time) *fakeClock { return &fakeClock{cur: start} }

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.cur = c.cur.Add(d)
	c.mu.Unlock()
}

// triageWorthyBody returns a backlog task body that's deliberately
// missing Priority so `tb triage` flags it.
func triageWorthyBody(id, agentName string) string {
	return "# " + id + ": Triage candidate\n" +
		"\n" +
		"**Type:** feature\n" +
		"**Size:** M\n" +
		"**Branch:** —\n" +
		"**Agent:** " + agentName + "\n" +
		"\n" +
		"## Goal\n\nReal goal — the missing priority is what triggers triage.\n" +
		"\n" +
		"## Acceptance Criteria\n\n- [ ] one\n- [ ] two\n" +
		"\n" +
		"## Log\n\n- 2026-05-19: Created\n"
}

func newAutoGroomFixture(t *testing.T, agentName string) *autoGroomFixture {
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
	if err := os.WriteFile(filepath.Join(boardDir, ".next-id"), []byte("2\n"), 0o644); err != nil {
		t.Fatalf(".next-id: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "backlog", "TB-1.md"),
		[]byte(triageWorthyBody("TB-1", agentName)), 0o644); err != nil {
		t.Fatalf("task md: %v", err)
	}

	c, err := cli.NewClient(cli.Options{BinaryPath: tbBinary, Cwd: root})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	stub := &stubRunner{
		name:        agentName,
		stdoutLines: []string{"groomed"},
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

	clock := newFakeClock(time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC))
	coord := NewAutoGroomCoordinator(AutoGroomCoordinatorOptions{
		Board:    board,
		Agent:    svc,
		Settings: settings,
		Emitter:  em,
		Logger:   slog.Default(),
		Now:      clock.now,
	})

	t.Cleanup(func() {
		_ = coord.Deactivate()
		// Snapshot the active set under the lock, then cancel + wait
		// OUTSIDE the lock. recordTerminal also takes svc.mu (to delete
		// from svc.active), so blocking on ar.Done while holding the
		// mutex would deadlock with any in-flight run.
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

	f := &autoGroomFixture{
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
		clock:     clock,
	}
	return f
}

// runScanSync invokes the coordinator scan directly, bypassing the
// debounce timer so the test stays deterministic. Activate's
// scheduleScan() arms a 250ms timer; we cancel it here so it can't
// race the synchronous scan we're about to run.
func (f *autoGroomFixture) runScanSync() {
	f.t.Helper()
	f.coord.mu.Lock()
	if f.coord.debounceTimer != nil {
		f.coord.debounceTimer.Stop()
		f.coord.debounceTimer = nil
	}
	f.coord.mu.Unlock()
	f.coord.scan(context.Background(), f.boardDir)
}

func (f *autoGroomFixture) countEmits(name string) int {
	f.t.Helper()
	n := 0
	for _, e := range f.emitter.snapshot() {
		if e.Name == name {
			n++
		}
	}
	return n
}

// waitForActiveDrained waits until svc.active is empty or fails the test.
func (f *autoGroomFixture) waitForActiveDrained(timeout time.Duration) {
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

// TestAutoGroomCoordinator_SettleSkipEmitsScanComplete pins the contract
// that a scan which records nothing but settle-window skips still emits
// `auto-groom:scan-complete` so the frontend autoGroomStore refetches
// and the Card pill / drawer countdown appears. Without this, the
// dominant "create a new backlog task, see the waiting pill" UX broke
// because no other event fired during a settle-only scan.
func TestAutoGroomCoordinator_SettleSkipEmitsScanComplete(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("default agent: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(10); err != nil {
		t.Fatalf("settle 10: %v", err)
	}

	// Force the task mtime to match the fake clock so settle math kicks in.
	taskPath := filepath.Join(f.boardDir, "backlog", "TB-1.md")
	now := f.clock.now()
	if err := os.Chtimes(taskPath, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}
	f.runScanSync()

	// No groom queue event; only the scan-complete signal.
	if got := f.countEmits("auto-groom:queued"); got != 0 {
		t.Errorf("settle-skip scan queued runs: got %d, want 0", got)
	}
	if got := f.countEmits("auto-groom:scan-complete"); got != 1 {
		t.Errorf("auto-groom:scan-complete: got %d, want 1", got)
	}
	if reason := f.coord.Status().LastSkipReasons["TB-1"]; reason != "settle" {
		t.Errorf("Status skip reason: got %q, want \"settle\"", reason)
	}
}

func TestAutoGroomCoordinator_DisabledNoEnqueue(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}
	f.runScanSync()

	if got := f.countEmits("auto-groom:queued"); got != 0 {
		t.Errorf("disabled: got %d auto-groom:queued emits, want 0", got)
	}
	if got := f.countEmits("agent:run-queued"); got != 0 {
		t.Errorf("disabled: got %d agent:run-queued emits, want 0", got)
	}
}

func TestAutoGroomCoordinator_NoDefaultEmitsOnce(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	// default_agent stays at "none" (the SettingsService default).
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}

	f.runScanSync()
	f.runScanSync() // second scan in the same state — must NOT re-emit.

	if got := f.countEmits("auto-groom:needs-default-agent"); got != 1 {
		t.Errorf("needs-default emits: got %d, want 1 (edge-triggered)", got)
	}
	if got := f.countEmits("auto-groom:queued"); got != 0 {
		t.Errorf("no default: got %d enqueues, want 0", got)
	}

	// Flip the default to a real agent → cleared event fires exactly once.
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	f.runScanSync()
	if got := f.countEmits("auto-groom:default-agent-cleared"); got != 1 {
		t.Errorf("cleared emits: got %d, want 1", got)
	}
}

func TestAutoGroomCoordinator_QueuesTriageCandidate(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("default agent: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(0); err != nil {
		t.Fatalf("settle 0: %v", err)
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}

	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	if got := f.countEmits("auto-groom:queued"); got != 1 {
		t.Errorf("auto-groom:queued: got %d, want 1", got)
	}
	events := readEvents(t, f.boardDir, "TB-1")
	var sawHash bool
	for _, ev := range events {
		if ev.Event == agent.EvQueued && ev.Mode == agent.ModeGroom.String() && ev.TriageHash != "" {
			sawHash = true
		}
	}
	if !sawHash {
		t.Errorf("no queued event with triage_hash for TB-1; events=%+v", events)
	}
}

func TestAutoGroomCoordinator_DedupeByHash(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("default agent: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(0); err != nil {
		t.Fatalf("settle 0: %v", err)
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}

	// First scan: queues + completes a groom run.
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	firstQueued := f.countEmits("auto-groom:queued")
	if firstQueued != 1 {
		t.Fatalf("first scan: got %d queued, want 1", firstQueued)
	}

	// Reset AgentStatus so the next scan isn't gated by "success".
	c := f.board.snapshot()
	if c == nil {
		t.Fatal("no board client")
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "none"}); err != nil {
		t.Fatalf("clear status: %v", err)
	}

	// Second scan: same triage reasons → durable dedupe must skip.
	f.runScanSync()
	if got := f.countEmits("auto-groom:queued"); got != firstQueued {
		t.Errorf("dedupe: got %d queued, want %d (no re-enqueue)", got, firstQueued)
	}
}

func TestAutoGroomCoordinator_SettleWindowDefersUntilExpiry(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("default agent: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(10); err != nil {
		t.Fatalf("settle 10: %v", err)
	}

	// Set the task file mtime to now in the fake clock's frame: any
	// real mtime is "before" the fake clock since the fixture creates
	// files on the wall clock. To make the settle window actually
	// matter, advance the file mtime to match the fake clock's now.
	taskPath := filepath.Join(f.boardDir, "backlog", "TB-1.md")
	now := f.clock.now()
	if err := os.Chtimes(taskPath, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}

	f.runScanSync()
	if got := f.countEmits("auto-groom:queued"); got != 0 {
		t.Errorf("within settle window: got %d queued, want 0", got)
	}
	st := f.coord.Status()
	if st.LastSkipReasons["TB-1"] != "settle" {
		t.Errorf("skip reason: got %q, want \"settle\"", st.LastSkipReasons["TB-1"])
	}

	// Advance the fake clock past the settle window — the coordinator
	// must now queue the candidate on the next scan.
	f.clock.advance(11 * time.Minute)
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	if got := f.countEmits("auto-groom:queued"); got != 1 {
		t.Errorf("after settle: got %d queued, want 1", got)
	}
}

func TestAutoGroomCoordinator_OnAgentRunFinishedPromotesWhenClean(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("default agent: %v", err)
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}

	// Mutate the task so it satisfies the triage gate (`tb ready` would
	// have rejected it without this). Triage no longer flags it.
	c := f.board.snapshot()
	if c == nil {
		t.Fatal("no board client")
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{
		Priority: "P1",
		Module:   "core",
	}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	// Production wires `task:updated:<id>` watcher events through
	// BoardWatcherSink → clearTriageCache. Tests don't run the
	// watcher, so invalidate the cache directly so Triage re-runs the
	// CLI against the freshly edited file.
	f.board.clearTriageCache()

	f.coord.OnAgentRunFinished(map[string]any{
		"mode":    agent.ModeGroom.String(),
		"status":  string(agent.StatusSuccess),
		"task_id": "TB-1",
	})

	// Poll until the task moves to ready (the goroutine inside the
	// coordinator does the work).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		t2, err := f.board.GetTask(context.Background(), "TB-1")
		if err == nil && t2.Metadata.Status == "ready" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t2, _ := f.board.GetTask(context.Background(), "TB-1")
	t.Fatalf("task did not promote to ready; current status=%q", t2.Metadata.Status)
}

func TestAutoGroomCoordinator_OnAgentRunFinishedGuardedSkipWhenStillTriaged(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("default agent: %v", err)
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}

	// Task is still triage-worthy (no priority). Promote attempt must
	// emit guarded-skip and NOT move the task.
	f.coord.OnAgentRunFinished(map[string]any{
		"mode":    agent.ModeGroom.String(),
		"status":  string(agent.StatusSuccess),
		"task_id": "TB-1",
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f.countEmits("auto-groom:guarded-skip") > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := f.countEmits("auto-groom:guarded-skip"); got == 0 {
		t.Errorf("guarded-skip emits: got 0, want >=1")
	}

	task, err := f.board.GetTask(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.Metadata.Status != "backlog" {
		t.Errorf("still-triaged task moved out of backlog: status=%q", task.Metadata.Status)
	}
}

// TestAutoGroomCoordinator_HashIsStable pins the dedupe primitive so a
// reordering of reasons (or future additions to BoardService.Triage)
// don't silently invalidate the on-disk hash. Stays in this package so
// a hash-format change ripples through the test surface intentionally.
func TestAutoGroomCoordinator_HashIsStable(t *testing.T) {
	a := computeTriageHash([]string{"no priority", "no goal"})
	b := computeTriageHash([]string{"no goal", "no priority"})
	if a != b || a == "" {
		t.Errorf("hash not deterministic under reorder: a=%q b=%q", a, b)
	}
	c := computeTriageHash([]string{"no priority"})
	if c == a {
		t.Errorf("hash collision across different reason sets: %q", c)
	}
}

// TestAutoGroomCoordinator_DeactivateStopsFurtherWork verifies a
// Deactivate prevents subsequent scheduleScan invocations from doing
// any work. Without this, a stray watcher event could fire scans
// against a stale boardDir after board switch.
func TestAutoGroomCoordinator_DeactivateStopsFurtherWork(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("default agent: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(0); err != nil {
		t.Fatalf("settle 0: %v", err)
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := f.coord.Deactivate(); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	f.coord.scheduleScan()
	time.Sleep(2 * scanDebounce)
	if got := f.countEmits("auto-groom:queued"); got != 0 {
		t.Errorf("post-deactivate emits: got %d, want 0", got)
	}
}

// TestAutoGroomCoordinator_StatusReflectsState exercises the Wails-
// bound Status snapshot for the no-default warning + last-scan
// timestamp. Keeps the JSON shape stable for the frontend.
func TestAutoGroomCoordinator_StatusReflectsState(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}
	f.runScanSync()

	st := f.coord.Status()
	if !st.Enabled {
		t.Errorf("Enabled: got false, want true")
	}
	if st.DefaultAgent != "none" {
		t.Errorf("DefaultAgent: got %q, want none", st.DefaultAgent)
	}
	if !st.NeedsDefaultAgent {
		t.Errorf("NeedsDefaultAgent: got false, want true")
	}
	if !strings.HasPrefix(st.LastScanAt, "2026-") {
		t.Errorf("LastScanAt: got %q, want RFC3339-ish", st.LastScanAt)
	}
}
