package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
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
	coord     *AutoGroomCoordinator
	clock     *fakeClock
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
	agentLine := ""
	if agentName != "" {
		agentLine = "**Agent:** " + agentName + "\n"
	}
	return "# " + id + ": Triage candidate\n" +
		"\n" +
		"**Type:** feature\n" +
		"**Size:** M\n" +
		"**Branch:** —\n" +
		agentLine +
		"\n" +
		"## Goal\n\nReal goal — the missing priority is what triggers triage.\n" +
		"\n" +
		"## Acceptance Criteria\n\n- [ ] one\n- [ ] two\n" +
		"\n" +
		"## Log\n\n- 2026-05-19: Created\n"
}

func newAutoGroomFixture(t *testing.T, agentName string) *autoGroomFixture {
	return newAutoGroomFixtureWithConfig(t, agentName, "board: board\nprefix: TB\n")
}

func newAutoGroomFixtureWithConfig(t *testing.T, agentName, boardConfig string) *autoGroomFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only board flock")
	}
	if !strings.HasSuffix(boardConfig, "\n") {
		boardConfig += "\n"
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
		[]byte(boardConfig), 0o644); err != nil {
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
	budget := &fakeWorkerBudget{max: 64}
	svc := NewAgentService(AgentServiceOptions{
		Board:        board,
		Emitter:      em,
		WorkerBudget: func() AutomationWorkerBudget { return budget },
	})
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
		budget:    budget,
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
	f.stopDebounce()
	f.coord.scan(context.Background(), f.boardDir)
}

func (f *autoGroomFixture) stopDebounce() {
	f.t.Helper()
	f.coord.mu.Lock()
	if f.coord.debounceTimer != nil {
		f.coord.debounceTimer.Stop()
		f.coord.debounceTimer = nil
	}
	f.coord.mu.Unlock()
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

func (f *autoGroomFixture) writeBacklogTask(id, agentName string) {
	f.t.Helper()
	path := filepath.Join(f.boardDir, "backlog", id+".md")
	if err := os.WriteFile(path, []byte(triageWorthyBody(id, agentName)), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
	f.board.clearTriageCache()
}

func (f *autoGroomFixture) writeTask(status string, spec readyTaskSpec) {
	f.t.Helper()
	path := filepath.Join(f.boardDir, status, spec.ID+".md")
	if err := os.WriteFile(path, []byte(readyTaskBody(spec)), 0o644); err != nil {
		f.t.Fatalf("write %s: %v", path, err)
	}
	f.board.clearTriageCache()
}

func (f *autoGroomFixture) queuedTaskIDs() []string {
	f.t.Helper()
	out := []string{}
	for _, e := range f.emitter.snapshot() {
		if e.Name != "auto-groom:queued" || len(e.Payload) == 0 {
			continue
		}
		if payload, ok := e.Payload[0].(map[string]any); ok {
			if id, ok := payload["task_id"].(string); ok {
				out = append(out, id)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (f *autoGroomFixture) enableAutoGroom(t *testing.T, defaultAgent string) {
	t.Helper()
	if err := f.settings.SetDefaultAgent(defaultAgent); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("SetAutoGroomEnabled: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(0); err != nil {
		t.Fatalf("SetAutoGroomSettleMinutes: %v", err)
	}
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

func TestAutoGroomCoordinator_LimitsStartsToWorkerBudget(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	f.writeBacklogTask("TB-2", "claude")
	f.writeBacklogTask("TB-3", "claude")
	if err := f.settings.SetMaxWorkers(2); err != nil {
		t.Fatalf("SetMaxWorkers: %v", err)
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	f.enableAutoGroom(t, "claude")

	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	queued := f.queuedTaskIDs()
	if len(queued) != 2 {
		t.Fatalf("queued = %v, want exactly 2 tasks due worker budget", queued)
	}
	status := f.coord.Status()
	skipped := []string{}
	for _, id := range []string{"TB-1", "TB-2", "TB-3"} {
		if status.LastSkipReasons[id] == "worker capacity full" {
			skipped = append(skipped, id)
		}
	}
	if got, want := len(skipped), 1; got != want {
		t.Fatalf("worker-capacity skipped = %v, want exactly one skipped task", skipped)
	}
	if len(queued) == 2 && len(skipped) == 1 {
		all := append(append([]string{}, queued...), skipped...)
		sort.Strings(all)
		if want := []string{"TB-1", "TB-2", "TB-3"}; !reflect.DeepEqual(all, want) {
			t.Fatalf("queued+skipped tasks = %v, want %v", all, want)
		}
	}
}

func TestAutoGroomCoordinator_ActiveRunReducesWorkerBudget(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	f.writeBacklogTask("TB-2", "claude")
	f.coord.budget = &fakeWorkerBudget{max: 2, active: []string{"TB-999"}}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	f.enableAutoGroom(t, "claude")

	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	queued := f.queuedTaskIDs()
	if len(queued) != 1 {
		t.Fatalf("queued = %v, want exactly 1 task because one worker is already active", queued)
	}
	status := f.coord.Status()
	skipped := []string{}
	for _, id := range []string{"TB-1", "TB-2"} {
		if status.LastSkipReasons[id] == "worker capacity full" {
			skipped = append(skipped, id)
		}
	}
	if got, want := len(skipped), 1; got != want {
		t.Fatalf("worker-capacity skipped = %v, want exactly one skipped task", skipped)
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

func TestAutoGroomCoordinator_WarnModeReadyWIPPreflightSkipsBeforeQueue(t *testing.T) {
	f := newAutoGroomFixtureWithConfig(t, "claude",
		"board: board\nprefix: TB\nwip_limit_ready: 1\nwip_enforcement: warn\n",
	)
	f.writeBacklogTask("TB-1", "")
	f.writeTask("ready", readyTaskSpec{ID: "TB-99", Agent: "claude"})
	f.enableAutoGroom(t, "claude")
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}

	f.runScanSync()

	if got := f.countEmits("auto-groom:queued"); got != 0 {
		t.Fatalf("auto-groom:queued = %d, want 0", got)
	}
	if _, err := os.Stat(filepath.Join(f.boardDir, "backlog", "TB-1.md")); err != nil {
		t.Fatalf("TB-1 should remain in backlog: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(f.boardDir, "backlog", "TB-1.md"))
	if err != nil {
		t.Fatalf("read TB-1: %v", err)
	}
	for _, banned := range []string{"**Agent:**", "**AgentStatus:**"} {
		if strings.Contains(string(body), banned) {
			t.Fatalf("WIP preflight should not write %s before queueing:\n%s", banned, body)
		}
	}
	if _, err := os.Stat(agent.StatePath(f.boardDir, "TB-1")); !os.IsNotExist(err) {
		t.Fatalf("WIP preflight should not create JSONL state, err=%v", err)
	}
	status := f.coord.Status()
	if reason, ok := status.LastSkipReasons["TB-1"]; !ok || reason != "ready WIP limit full" {
		t.Fatalf("skip reason = %q (have=%v), want ready WIP limit full", reason, ok)
	}
}

func TestAutoGroomCoordinator_StartupGraceDelaysActivationScan(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	f.enableAutoGroom(t, "claude")
	if err := f.coord.ActivateWithStartupGrace(context.Background(), f.boardDir, 80*time.Millisecond); err != nil {
		t.Fatalf("ActivateWithStartupGrace: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if got := f.countEmits("auto-groom:queued"); got != 0 {
		t.Fatalf("queued during startup grace = %d, want 0", got)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.waitForActiveDrained(5 * time.Second)
		if got := countEmitsByName(f.emitter, "auto-groom:queued", "TB-1"); got == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("auto-groom did not queue after grace; events=%+v", f.emitter.snapshot())
}

func TestAutoGroomCoordinator_ZeroReadyWIPLimitDoesNotBlockQueue(t *testing.T) {
	f := newAutoGroomFixtureWithConfig(t, "claude",
		"board: board\nprefix: TB\nwip_limit_ready: 0\nwip_enforcement: warn\n",
	)
	f.writeTask("ready", readyTaskSpec{ID: "TB-99", Agent: "claude"})
	f.enableAutoGroom(t, "claude")
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}

	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	if got := countEmitsByName(f.emitter, "auto-groom:queued", "TB-1"); got != 1 {
		t.Fatalf("auto-groom:queued count = %d, want 1\nevents: %+v", got, f.emitter.snapshot())
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

func TestAutoGroomCoordinator_PostGroomReadyWIPFullSkipsPromotion(t *testing.T) {
	f := newAutoGroomFixtureWithConfig(t, "claude",
		"board: board\nprefix: TB\nwip_limit_ready: 1\nwip_enforcement: warn\n",
	)
	f.writeTask("ready", readyTaskSpec{ID: "TB-99", Agent: "claude"})
	f.enableAutoGroom(t, "claude")
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}
	f.stopDebounce()
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
	f.board.clearTriageCache()

	f.coord.OnAgentRunFinished(map[string]any{
		"mode":    agent.ModeGroom.String(),
		"status":  string(agent.StatusSuccess),
		"task_id": "TB-1",
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, err := f.board.GetTask(context.Background(), "TB-1")
		if err == nil && task.Metadata.Status != "backlog" {
			break
		}
		if countEmitsByName(f.emitter, "auto-groom:guarded-skip", "TB-1") > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	task, err := f.board.GetTask(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.Metadata.Status != "backlog" {
		t.Fatalf("ready WIP full should leave task in backlog, got %q", task.Metadata.Status)
	}
	if got := countEmitsByName(f.emitter, "auto-groom:guarded-skip", "TB-1"); got != 1 {
		t.Fatalf("guarded-skip emits = %d, want 1\nevents: %+v", got, f.emitter.snapshot())
	}
	if got := countEmitsByName(f.emitter, "auto-groom:promote-failed", "TB-1"); got != 0 {
		t.Fatalf("promote-failed emits = %d, want 0", got)
	}
	status := f.coord.Status()
	if reason, ok := status.LastSkipReasons["TB-1"]; !ok || reason != "ready WIP limit full" {
		t.Fatalf("skip reason = %q (have=%v), want ready WIP limit full", reason, ok)
	}
}

func TestAutoGroomCoordinator_StrictWIPRaceFallsBackToReadyFailure(t *testing.T) {
	f := newAutoGroomFixtureWithConfig(t, "claude",
		"board: board\nprefix: TB\nwip_limit_ready: 1\nwip_enforcement: strict\n",
	)
	f.enableAutoGroom(t, "claude")
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("activate: %v", err)
	}
	f.stopDebounce()
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
	f.board.clearTriageCache()
	f.coord.beforeReadyForTesting = func(taskID string) {
		if taskID != "TB-1" {
			return
		}
		f.writeTask("ready", readyTaskSpec{ID: "TB-99", Agent: "claude"})
	}

	f.coord.OnAgentRunFinished(map[string]any{
		"mode":    agent.ModeGroom.String(),
		"status":  string(agent.StatusSuccess),
		"task_id": "TB-1",
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if countEmitsByName(f.emitter, "auto-groom:promote-failed", "TB-1") > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(f.boardDir, "backlog", "TB-1.md")); err != nil {
		t.Fatalf("TB-1 should remain backlog after strict ready race: %v", err)
	}
	if got := countEmitsByName(f.emitter, "auto-groom:promote-failed", "TB-1"); got != 1 {
		t.Fatalf("promote-failed emits = %d, want 1\nevents: %+v", got, f.emitter.snapshot())
	}
	if got := countEmitsByName(f.emitter, "auto-groom:queued", "TB-1"); got != 0 {
		t.Fatalf("queued emits = %d, want 0", got)
	}
	status := f.coord.Status()
	if reason, ok := status.LastSkipReasons["TB-1"]; !ok || !strings.HasPrefix(reason, "promote-failed:") {
		t.Fatalf("skip reason = %q (have=%v), want promote-failed", reason, ok)
	}
}

func TestAutoGroomCoordinator_OnAgentRunFinishedIgnoresStaleBoard(t *testing.T) {
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
	f.board.clearTriageCache()

	f.coord.OnAgentRunFinished(map[string]any{
		"mode":      agent.ModeGroom.String(),
		"status":    string(agent.StatusSuccess),
		"task_id":   "TB-1",
		"board_dir": filepath.Join(f.root, "other-board"),
	})
	time.Sleep(50 * time.Millisecond)

	task, err := f.board.GetTask(context.Background(), "TB-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.Metadata.Status != "backlog" {
		t.Fatalf("stale-board terminal event promoted task to %q", task.Metadata.Status)
	}
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

// TestAutoGroomCoordinator_ResumesInterruptedBacklogTask verifies the
// resume sweep auto-resumes a task left `interrupted` in backlog (the
// daemon's recovery target for crashed groom-mode runs).
func TestAutoGroomCoordinator_ResumesInterruptedBacklogTask(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	c := f.board.snapshot()
	if c == nil {
		t.Fatalf("board has no CLI client")
	}
	seedInterruptedTask(t, f.boardDir, c, "TB-1", "claude", agent.InitiatorAutoGroom, "interrupted")
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("SetAutoGroomEnabled: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(0); err != nil {
		t.Fatalf("SetAutoGroomSettleMinutes: %v", err)
	}
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	if got := countEmitsByName(f.emitter, "auto-groom:resumed", "TB-1"); got != 1 {
		t.Fatalf("auto-groom:resumed count = %d, want 1\nevents: %+v",
			got, f.emitter.snapshot())
	}
	// And: no fresh groom should have been queued for the same task.
	if got := countEmitsByName(f.emitter, "auto-groom:queued", "TB-1"); got != 0 {
		t.Fatalf("auto-groom:queued count = %d, want 0 (resume must replace fresh-groom)\nevents: %+v",
			got, f.emitter.snapshot())
	}
	if got := f.stub.input().Mode; got != agent.ModeResume {
		t.Errorf("stub RunInput.Mode = %q, want %q", got, agent.ModeResume)
	}
}

func TestAutoGroomCoordinator_ResumeSweepRespectsWorkerBudget(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	f.writeBacklogTask("TB-2", "claude")
	f.writeBacklogTask("TB-3", "claude")
	if err := f.settings.SetMaxWorkers(2); err != nil {
		t.Fatalf("SetMaxWorkers: %v", err)
	}
	c := f.board.snapshot()
	if c == nil {
		t.Fatalf("board has no CLI client")
	}
	for _, id := range []string{"TB-1", "TB-2", "TB-3"} {
		seedInterruptedTask(t, f.boardDir, c, id, "claude", agent.InitiatorAutoGroom, "interrupted")
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	f.enableAutoGroom(t, "claude")

	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	resumed := countEmitsByName(f.emitter, "auto-groom:resumed", "")
	if resumed != 2 {
		t.Fatalf("auto-groom:resumed count = %d, want 2 due worker budget\nevents: %+v",
			resumed, f.emitter.snapshot())
	}
	status := f.coord.Status()
	skipped := []string{}
	for _, id := range []string{"TB-1", "TB-2", "TB-3"} {
		if status.LastSkipReasons[id] == "worker capacity full" {
			skipped = append(skipped, id)
		}
	}
	if got, want := len(skipped), 1; got != want {
		t.Fatalf("worker-capacity skipped = %v, want exactly one skipped task", skipped)
	}
}

// TestAutoGroomCoordinator_SkipsLostTask verifies the defensive `lost`
// skip: a task whose recovery couldn't capture a session_id is left
// alone (no fresh groom, no resume attempt — there's nothing to resume).
func TestAutoGroomCoordinator_SkipsLostTask(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	c := f.board.snapshot()
	if c == nil {
		t.Fatalf("board has no CLI client")
	}
	if err := c.Edit(context.Background(), "TB-1", cli.EditInput{AgentStatus: "lost"}); err != nil {
		t.Fatalf("seed lost: %v", err)
	}
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("SetAutoGroomEnabled: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(0); err != nil {
		t.Fatalf("SetAutoGroomSettleMinutes: %v", err)
	}
	f.runScanSync()

	if got := countEmitsByName(f.emitter, "auto-groom:resumed", "TB-1"); got != 0 {
		t.Errorf("auto-groom:resumed count = %d, want 0 (lost must not resume)", got)
	}
	if got := countEmitsByName(f.emitter, "auto-groom:queued", "TB-1"); got != 0 {
		t.Errorf("auto-groom:queued count = %d, want 0 (lost must not start a fresh groom)", got)
	}
}

// TestAutoGroomCoordinator_SkipsUserInitiatedInterrupted verifies the
// initiator filter: an interrupted run a user originally triggered is
// not auto-resumed.
func TestAutoGroomCoordinator_SkipsUserInitiatedInterrupted(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	c := f.board.snapshot()
	if c == nil {
		t.Fatalf("board has no CLI client")
	}
	// Empty initiator marks the run as user-triggered.
	seedInterruptedTask(t, f.boardDir, c, "TB-1", "claude", "", "interrupted")
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("SetAutoGroomEnabled: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(0); err != nil {
		t.Fatalf("SetAutoGroomSettleMinutes: %v", err)
	}
	f.runScanSync()

	if got := countEmitsByName(f.emitter, "auto-groom:resumed", "TB-1"); got != 0 {
		t.Errorf("auto-groom:resumed count = %d, want 0 (user-initiated must not be auto-resumed)", got)
	}
}

// TestAutoGroomCoordinator_RestartsLostAutoRun verifies a `lost` task
// the coordinator originally queued is auto-restarted via a fresh
// StartGroomWithTriageHashAs call (no session continuity).
func TestAutoGroomCoordinator_RestartsLostAutoRun(t *testing.T) {
	f := newAutoGroomFixture(t, "claude")
	c := f.board.snapshot()
	if c == nil {
		t.Fatalf("board has no CLI client")
	}
	seedInterruptedTask(t, f.boardDir, c, "TB-1", "claude", agent.InitiatorAutoGroom, "lost")
	if err := f.coord.Activate(context.Background(), f.boardDir); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := f.settings.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := f.settings.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("SetAutoGroomEnabled: %v", err)
	}
	if err := f.settings.SetAutoGroomSettleMinutes(0); err != nil {
		t.Fatalf("SetAutoGroomSettleMinutes: %v", err)
	}
	f.runScanSync()
	f.waitForActiveDrained(5 * time.Second)

	if got := countEmitsByName(f.emitter, "auto-groom:resumed", "TB-1"); got != 1 {
		t.Fatalf("auto-groom:resumed count = %d, want 1\nevents: %+v",
			got, f.emitter.snapshot())
	}
	// Fresh restart: stub sees ModeGroom (not ModeResume).
	if got := f.stub.input().Mode; got != agent.ModeGroom {
		t.Errorf("stub RunInput.Mode = %q, want %q (lost restart is a fresh groom)",
			got, agent.ModeGroom)
	}
}
