package daemon

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeBoard implements Board with an in-memory task list.
type fakeBoard struct {
	mu    sync.Mutex
	tasks map[string]AgentTask
}

func newFakeBoard(tasks ...AgentTask) *fakeBoard {
	b := &fakeBoard{tasks: map[string]AgentTask{}}
	for _, t := range tasks {
		b.tasks[t.ID] = t
	}
	return b
}

func (b *fakeBoard) ListActive(ctx context.Context) ([]AgentTask, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]AgentTask, 0, len(b.tasks))
	for _, t := range b.tasks {
		out = append(out, t)
	}
	return out, nil
}

func (b *fakeBoard) GetTask(ctx context.Context, id string) (AgentTask, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t, ok := b.tasks[id]
	if !ok {
		return AgentTask{}, errors.New("not found")
	}
	return t, nil
}

func (b *fakeBoard) set(t AgentTask) {
	b.mu.Lock()
	b.tasks[t.ID] = t
	b.mu.Unlock()
}

func (b *fakeBoard) replace(tasks ...AgentTask) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tasks = map[string]AgentTask{}
	for _, t := range tasks {
		b.tasks[t.ID] = t
	}
}

// fakeAgent records every RunQueuedAgentSync call. The optional onRun
// hook lets a test block or simulate work.
type fakeAgent struct {
	mu       sync.Mutex
	calls    []string
	starts   []time.Time
	ends     []time.Time
	onRun    func(ctx context.Context, id string) (string, error)
	hasFn    func(id string) bool
	activeFn func() []string
	counter  atomic.Int32
}

func (a *fakeAgent) RunQueuedAgentSync(ctx context.Context, id string) (string, error) {
	a.mu.Lock()
	a.calls = append(a.calls, id)
	a.starts = append(a.starts, time.Now())
	a.mu.Unlock()
	a.counter.Add(1)
	defer a.counter.Add(-1)
	var status string
	var err error
	if a.onRun != nil {
		status, err = a.onRun(ctx, id)
	} else {
		status = "success"
	}
	a.mu.Lock()
	a.ends = append(a.ends, time.Now())
	a.mu.Unlock()
	return status, err
}

func (a *fakeAgent) HasActiveRun(id string) bool {
	if a.hasFn != nil {
		return a.hasFn(id)
	}
	return false
}

func (a *fakeAgent) ActiveTaskIDs() []string {
	if a.activeFn != nil {
		return a.activeFn()
	}
	return nil
}

func (a *fakeAgent) callCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.calls)
}

func TestDaemon_NewIsIdle_NoIO(t *testing.T) {
	a := &fakeAgent{}
	d := New(Options{Agent: a, MaxWorkers: 2})
	t.Cleanup(func() { _ = d.Close() })
	time.Sleep(50 * time.Millisecond)
	if a.callCount() != 0 {
		t.Errorf("agent invoked before Activate: %d calls", a.callCount())
	}
}

func TestDaemon_Activate_RunsRecoveryThenScan(t *testing.T) {
	b := newFakeBoard(
		AgentTask{ID: "TB-1", Agent: "claude", AgentStatus: "queued"},
		AgentTask{ID: "TB-2", Agent: "claude", AgentStatus: "running"}, // not enqueued
		AgentTask{ID: "TB-3", Agent: "", AgentStatus: "queued"},        // missing agent
	)
	a := &fakeAgent{}
	rec := &fakeRecovery{}
	d := New(Options{Board: b, Agent: a, Recovery: rec, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })

	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	// Recovery ran exactly once.
	if rec.calls.Load() != 1 {
		t.Errorf("recovery calls: %d, want 1", rec.calls.Load())
	}

	// Only TB-1 was enqueued (queued+agent set).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.callCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := a.callCount(); got != 1 {
		t.Errorf("agent calls: %d, want 1", got)
	}
	a.mu.Lock()
	if a.calls[0] != "TB-1" {
		t.Errorf("first call: %q, want TB-1", a.calls[0])
	}
	a.mu.Unlock()
}

func TestDaemon_StartupGraceDelaysOnlyQueueScan(t *testing.T) {
	b := newFakeBoard(AgentTask{ID: "TB-1", Agent: "claude", AgentStatus: "queued"})
	a := &fakeAgent{}
	rec := &fakeRecovery{}
	d := New(Options{Board: b, Agent: a, Recovery: rec, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })

	if err := d.ActivateWithStartupGrace(context.Background(), "/tmp/fake", 80*time.Millisecond); err != nil {
		t.Fatalf("ActivateWithStartupGrace: %v", err)
	}
	if rec.calls.Load() != 1 {
		t.Fatalf("recovery calls = %d, want immediate 1", rec.calls.Load())
	}
	time.Sleep(30 * time.Millisecond)
	if got := a.callCount(); got != 0 {
		t.Fatalf("startup scan ran during grace: calls=%d", got)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.callCount() == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("startup scan did not run after grace; calls=%d", a.callCount())
}

func TestDaemon_StartupGraceCoalescesWatcherRescanAndCancelsOldBoard(t *testing.T) {
	b := newFakeBoard(AgentTask{ID: "TB-A", Agent: "claude", AgentStatus: "queued"})
	a := &fakeAgent{}
	d := New(Options{Board: b, Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })

	if err := d.ActivateWithStartupGrace(context.Background(), "/tmp/fake-a", 80*time.Millisecond); err != nil {
		t.Fatalf("ActivateWithStartupGrace A: %v", err)
	}
	if n, err := d.RescanActive(context.Background()); err != nil || n != 0 {
		t.Fatalf("rescan during grace = %d, %v; want 0, nil", n, err)
	}
	if err := d.Deactivate(); err != nil {
		t.Fatalf("Deactivate A: %v", err)
	}
	b.replace(AgentTask{ID: "TB-B", Agent: "claude", AgentStatus: "queued"})
	if err := d.Activate(context.Background(), "/tmp/fake-b"); err != nil {
		t.Fatalf("Activate B: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.callCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	a.mu.Lock()
	defer a.mu.Unlock()
	if got := a.calls; len(got) != 1 || got[0] != "TB-B" {
		t.Fatalf("calls = %v, want exactly [TB-B]", got)
	}
}

type fakeRecovery struct {
	calls atomic.Int32
	err   error
}

func (r *fakeRecovery) RecoverStale(ctx context.Context, boardDir string) error {
	r.calls.Add(1)
	return r.err
}

type fakeReconciler struct {
	mu         sync.Mutex
	active     int
	taskCalls  []string
	activeHook func()
	taskHook   func(string)
}

func (r *fakeReconciler) ReconcileActive(ctx context.Context) error {
	r.mu.Lock()
	r.active++
	hook := r.activeHook
	r.mu.Unlock()
	if hook != nil {
		hook()
	}
	return nil
}

func (r *fakeReconciler) ReconcileTask(ctx context.Context, id string) error {
	r.mu.Lock()
	r.taskCalls = append(r.taskCalls, id)
	hook := r.taskHook
	r.mu.Unlock()
	if hook != nil {
		hook(id)
	}
	return nil
}

func (r *fakeReconciler) activeCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active
}

func (r *fakeReconciler) taskCount(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, got := range r.taskCalls {
		if got == id {
			n++
		}
	}
	return n
}

type fakeUntrackedRecovery struct {
	fakeRecovery
	untrackedCalls atomic.Int32
}

func (r *fakeUntrackedRecovery) RecoverStaleUntracked(ctx context.Context, boardDir string) error {
	r.untrackedCalls.Add(1)
	return r.err
}

func TestDaemon_SetPeriodicRecoveryEnabledTogglesActiveTicker(t *testing.T) {
	rec := &fakeUntrackedRecovery{}
	d := New(Options{
		Board:                    newFakeBoard(),
		Agent:                    &fakeAgent{},
		Recovery:                 rec,
		MaxWorkers:               1,
		PeriodicRecoveryInterval: 10 * time.Millisecond,
		DisablePeriodicRecovery:  true,
	})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if got := rec.untrackedCalls.Load(); got != 0 {
		t.Fatalf("periodic recovery ran while disabled: %d calls", got)
	}

	d.SetPeriodicRecoveryEnabled(true)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && rec.untrackedCalls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if got := rec.untrackedCalls.Load(); got == 0 {
		t.Fatalf("periodic recovery did not start after enabling")
	}

	d.SetPeriodicRecoveryEnabled(false)
	afterDisable := rec.untrackedCalls.Load()
	time.Sleep(30 * time.Millisecond)
	if got := rec.untrackedCalls.Load(); got != afterDisable {
		t.Fatalf("periodic recovery kept running after disable: before=%d after=%d", afterDisable, got)
	}
}

func TestDaemon_ActivateRunsReconciliationBeforeStartupScan(t *testing.T) {
	b := newFakeBoard(AgentTask{ID: "TB-1", Agent: "claude", AgentStatus: "queued"})
	a := &fakeAgent{}
	rec := &fakeReconciler{
		activeHook: func() {
			b.set(AgentTask{ID: "TB-1", Agent: "claude", AgentStatus: "running"})
		},
	}
	d := New(Options{Board: b, Agent: a, Reconciler: rec, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })

	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if got := rec.activeCount(); got != 1 {
		t.Fatalf("reconcile active calls = %d, want 1", got)
	}
	if got := a.callCount(); got != 0 {
		t.Fatalf("startup scan ran before reconciliation effect; agent calls = %d, want 0", got)
	}
}

func TestDaemon_RunOneReconcilesBeforeAndAfterTerminalRun(t *testing.T) {
	a := &fakeAgent{}
	rec := &fakeReconciler{}
	d := New(Options{Board: newFakeBoard(), Agent: a, Reconciler: rec, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	generation, _, ok := d.activationState()
	if !ok {
		t.Fatalf("activation state missing")
	}
	d.runOne(slog.Default(), queuedTask{id: "TB-2", generation: generation})

	if got := a.callCount(); got != 1 {
		t.Fatalf("agent calls = %d, want 1", got)
	}
	if got := rec.taskCount("TB-2"); got != 2 {
		t.Fatalf("task reconciliation calls = %d, want pre-run and post-run", got)
	}
}

func TestEventSinkBoardReloadReconcilesBeforeRescan(t *testing.T) {
	b := newFakeBoard()
	a := &fakeAgent{}
	rec := &fakeReconciler{
		activeHook: func() {
			b.set(AgentTask{ID: "TB-3", Agent: "claude", AgentStatus: "running"})
		},
	}
	d := New(Options{Board: b, Agent: a, Reconciler: rec, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	b.set(AgentTask{ID: "TB-3", Agent: "claude", AgentStatus: "queued"})
	sink := NewEventSink(d, nil)
	sink.handle(context.Background(), "board:reloaded")
	time.Sleep(50 * time.Millisecond)

	if got := rec.activeCount(); got < 2 {
		t.Fatalf("reconcile active calls = %d, want activation + board reload", got)
	}
	if got := a.callCount(); got != 0 {
		t.Fatalf("rescan should observe reconciled non-queued task; agent calls = %d, want 0", got)
	}
}

func TestDaemon_Enqueue_BeforeActivate_Errors(t *testing.T) {
	d := New(Options{Agent: &fakeAgent{}})
	t.Cleanup(func() { _ = d.Close() })
	_, err := d.Enqueue("TB-1")
	if !errors.Is(err, ErrNotActivated) {
		t.Errorf("want ErrNotActivated, got %v", err)
	}
}

func TestDaemon_Enqueue_DedupsConcurrent(t *testing.T) {
	gate := make(chan struct{})
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			<-gate
			return "success", nil
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() {
		close(gate)
		_ = d.Close()
	})
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	// Hammer the same task ID 100x concurrently.
	var wg sync.WaitGroup
	var enqueueCount atomic.Int32
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := d.Enqueue("TB-X"); ok {
				enqueueCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := enqueueCount.Load(); got != 1 {
		t.Errorf("dedup: %d enqueues, want 1", got)
	}
}

func TestDaemon_Enqueue_AfterCompletionAllowsReEnqueue(t *testing.T) {
	a := &fakeAgent{} // default onRun=nil → returns "success" immediately
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	if ok, _ := d.Enqueue("TB-1"); !ok {
		t.Fatalf("first enqueue should succeed")
	}

	// Wait for the worker to drain it.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !d.IsActive("TB-1") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if d.IsActive("TB-1") {
		t.Fatalf("active flag still set after worker run")
	}

	if ok, _ := d.Enqueue("TB-1"); !ok {
		t.Fatalf("re-enqueue after completion should succeed")
	}
}

func TestDaemon_Enqueue_CrossChecksHasActiveRun(t *testing.T) {
	a := &fakeAgent{hasFn: func(id string) bool { return id == "TB-2" }}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	if ok, _ := d.Enqueue("TB-2"); ok {
		t.Errorf("HasActiveRun=true should block enqueue")
	}
	if ok, _ := d.Enqueue("TB-3"); !ok {
		t.Errorf("HasActiveRun=false should allow enqueue")
	}
}

func TestDaemon_WorkerPool_Capacity1_Serializes(t *testing.T) {
	var concurrentMax atomic.Int32
	var concurrent atomic.Int32
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			cur := concurrent.Add(1)
			for {
				peak := concurrentMax.Load()
				if cur <= peak || concurrentMax.CompareAndSwap(peak, cur) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			concurrent.Add(-1)
			return "success", nil
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	for _, id := range []string{"A", "B", "C"} {
		if ok, err := d.Enqueue(id); err != nil || !ok {
			t.Fatalf("enqueue %s: ok=%v err=%v", id, ok, err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.callCount() == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.callCount() != 3 {
		t.Fatalf("only %d runs completed", a.callCount())
	}
	if got := concurrentMax.Load(); got > 1 {
		t.Errorf("max concurrent: %d, want 1", got)
	}
}

func TestDaemon_WorkerPool_Capacity2_Overlaps(t *testing.T) {
	var concurrentMax atomic.Int32
	var concurrent atomic.Int32
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			cur := concurrent.Add(1)
			for {
				peak := concurrentMax.Load()
				if cur <= peak || concurrentMax.CompareAndSwap(peak, cur) {
					break
				}
			}
			time.Sleep(80 * time.Millisecond)
			concurrent.Add(-1)
			return "success", nil
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 2})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	for _, id := range []string{"A", "B", "C", "D"} {
		if ok, _ := d.Enqueue(id); !ok {
			t.Fatalf("enqueue %s rejected", id)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if a.callCount() == 4 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.callCount() != 4 {
		t.Fatalf("only %d runs completed", a.callCount())
	}
	if got := concurrentMax.Load(); got != 2 {
		t.Errorf("max concurrent: %d, want 2", got)
	}
}

func TestDaemon_SetMaxWorkers_ReducesRuntimeConcurrency(t *testing.T) {
	var concurrentMax atomic.Int32
	var concurrent atomic.Int32
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			cur := concurrent.Add(1)
			for {
				peak := concurrentMax.Load()
				if cur <= peak || concurrentMax.CompareAndSwap(peak, cur) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			concurrent.Add(-1)
			return "success", nil
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 2})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	d.SetMaxWorkers(1)
	if got := d.MaxWorkers(); got != 1 {
		t.Fatalf("MaxWorkers after set: got %d, want 1", got)
	}

	for _, id := range []string{"A", "B", "C"} {
		if ok, err := d.Enqueue(id); err != nil || !ok {
			t.Fatalf("enqueue %s: ok=%v err=%v", id, ok, err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.callCount() == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.callCount() != 3 {
		t.Fatalf("only %d runs completed", a.callCount())
	}
	if got := concurrentMax.Load(); got > 1 {
		t.Errorf("max concurrent: %d, want <= 1", got)
	}
}

func TestDaemon_SetMaxWorkers_IncreasesRuntimeConcurrency(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("host max_workers limit is below 2")
	}
	started := make(chan string, 2)
	release := make(chan struct{})
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			started <- id
			select {
			case <-release:
				return "success", nil
			case <-ctx.Done():
				return "cancelled", ctx.Err()
			}
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() {
		close(release)
		_ = d.Close()
	})
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	for _, id := range []string{"A", "B"} {
		if ok, err := d.Enqueue(id); err != nil || !ok {
			t.Fatalf("enqueue %s: ok=%v err=%v", id, ok, err)
		}
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatalf("first run never started")
	}
	select {
	case id := <-started:
		t.Fatalf("second run %s started before max_workers increased", id)
	case <-time.After(50 * time.Millisecond):
	}

	d.SetMaxWorkers(2)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatalf("second run did not start after max_workers increased")
	}
}

func TestDaemon_AutomationReservationConsumesWorkerSlot(t *testing.T) {
	started := make(chan string, 1)
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			started <- id
			return "success", nil
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	if !d.TryReserveAutomationRun("AUTO-1") {
		t.Fatalf("automation reservation rejected")
	}
	if d.TryReserveAutomationRun("AUTO-1") {
		t.Fatalf("duplicate automation reservation for same task should be rejected")
	}
	if got := d.ActiveTaskIDs(); len(got) != 1 || got[0] != "AUTO-1" {
		t.Fatalf("active after reservation: got %v, want [AUTO-1]", got)
	}
	if d.TryReserveAutomationRun("AUTO-2") {
		t.Fatalf("second automation reservation should be rejected at max_workers=1")
	}
	if ok, err := d.Enqueue("TB-1"); err != nil || !ok {
		t.Fatalf("enqueue while automation slot held: ok=%v err=%v", ok, err)
	}
	select {
	case id := <-started:
		t.Fatalf("daemon run %s started while automation slot was reserved", id)
	case <-time.After(50 * time.Millisecond):
	}

	d.ReleaseAutomationRun("AUTO-1")
	select {
	case id := <-started:
		if id != "TB-1" {
			t.Fatalf("started task: got %s, want TB-1", id)
		}
	case <-time.After(time.Second):
		t.Fatalf("daemon run did not start after automation slot release")
	}
}

func TestDaemon_AutomationReservationCountsAgentServiceActiveRuns(t *testing.T) {
	a := &fakeAgent{
		activeFn: func() []string { return []string{"MANUAL-1"} },
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	if d.TryReserveAutomationRun("AUTO-1") {
		t.Fatalf("automation reservation should be rejected while AgentService has another active run")
	}
}

func TestDaemon_DispatcherWaitsForAgentServiceActiveRun(t *testing.T) {
	started := make(chan string, 1)
	active := atomic.Bool{}
	active.Store(true)
	a := &fakeAgent{
		activeFn: func() []string {
			if active.Load() {
				return []string{"RECOVERED-1"}
			}
			return nil
		},
		onRun: func(ctx context.Context, id string) (string, error) {
			started <- id
			return "success", nil
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if ok, err := d.Enqueue("TB-1"); err != nil || !ok {
		t.Fatalf("enqueue: ok=%v err=%v", ok, err)
	}
	select {
	case id := <-started:
		t.Fatalf("daemon run %s started while AgentService active run consumed capacity", id)
	case <-time.After(50 * time.Millisecond):
	}

	active.Store(false)
	d.NotifyAgentActiveChanged()
	select {
	case id := <-started:
		if id != "TB-1" {
			t.Fatalf("started task: got %s, want TB-1", id)
		}
	case <-time.After(time.Second):
		t.Fatalf("daemon run did not start after AgentService active run cleared")
	}
}

func TestDaemon_Close_CancelsCtxAndDrains(t *testing.T) {
	gate := make(chan struct{})
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			select {
			case <-gate:
				return "success", nil
			case <-ctx.Done():
				return "cancelled", ctx.Err()
			}
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if ok, _ := d.Enqueue("TB-9"); !ok {
		t.Fatalf("enqueue rejected")
	}
	time.Sleep(50 * time.Millisecond)

	// Close should cancel the ctx the worker is using; the fakeAgent
	// observes that and returns. Close returns within ~5s.
	start := time.Now()
	if err := d.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 6*time.Second {
		t.Errorf("Close took too long: %v", elapsed)
	}

	close(gate)
}

func TestDaemon_DeactivateCancelsActiveRunWithBoardSwitchCause(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan error, 1)
	var once sync.Once
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			once.Do(func() { close(started) })
			<-ctx.Done()
			cancelled <- context.Cause(ctx)
			return "cancelled", ctx.Err()
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Activate(context.Background(), "/tmp/fake-a"); err != nil {
		t.Fatalf("Activate A: %v", err)
	}
	if ok, err := d.Enqueue("TB-7"); err != nil || !ok {
		t.Fatalf("enqueue: ok=%v err=%v", ok, err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatalf("run never started")
	}

	if err := d.Deactivate(); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	select {
	case cause := <-cancelled:
		if cause == nil || cause.Error() != "board switch" {
			t.Fatalf("cancel cause = %v, want board switch", cause)
		}
	default:
		t.Fatalf("active run did not observe board-switch cancellation")
	}
	if d.IsActive("TB-7") {
		t.Fatalf("active flag should be cleared after Deactivate")
	}
	if err := d.Activate(context.Background(), "/tmp/fake-b"); err != nil {
		t.Fatalf("Activate B after Deactivate: %v", err)
	}
}

func TestDaemon_DeactivateDropsQueuedOldBoardWork(t *testing.T) {
	started := make(chan string, 4)
	releaseFirst := make(chan struct{})
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			started <- id
			if id == "TB-1" {
				select {
				case <-releaseFirst:
					return "success", nil
				case <-ctx.Done():
					return "cancelled", ctx.Err()
				}
			}
			return "success", nil
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() {
		close(releaseFirst)
		_ = d.Close()
	})
	if err := d.Activate(context.Background(), "/tmp/fake-a"); err != nil {
		t.Fatalf("Activate A: %v", err)
	}
	if ok, err := d.Enqueue("TB-1"); err != nil || !ok {
		t.Fatalf("enqueue TB-1: ok=%v err=%v", ok, err)
	}
	if ok, err := d.Enqueue("TB-2"); err != nil || !ok {
		t.Fatalf("enqueue TB-2: ok=%v err=%v", ok, err)
	}
	select {
	case got := <-started:
		if got != "TB-1" {
			t.Fatalf("first run = %s, want TB-1", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("first run never started")
	}

	if err := d.Deactivate(); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if err := d.Activate(context.Background(), "/tmp/fake-b"); err != nil {
		t.Fatalf("Activate B: %v", err)
	}

	select {
	case got := <-started:
		t.Fatalf("queued old-board task ran after board switch: %s", got)
	case <-time.After(75 * time.Millisecond):
	}
	if ok, err := d.Enqueue("TB-3"); err != nil || !ok {
		t.Fatalf("enqueue TB-3 after switch: ok=%v err=%v", ok, err)
	}
	select {
	case got := <-started:
		if got != "TB-3" {
			t.Fatalf("new-board run = %s, want TB-3", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("new-board run never started")
	}
}

// TestDaemon_Close_RacesEnqueueWithoutPanic exercises the
// Close-during-Enqueue scenario the reviewer flagged: many Enqueue
// callers fire while Close drives rootCancel + grace timeout. The
// daemon must not panic on send-on-closed-channel — workers exit via
// rootCtx, the queue stays open. Run with -race to surface any data
// race that survives.
func TestDaemon_Close_RacesEnqueueWithoutPanic(t *testing.T) {
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			select {
			case <-ctx.Done():
				return "cancelled", ctx.Err()
			case <-time.After(10 * time.Millisecond):
				return "success", nil
			}
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 2, QueueBuffer: 8})
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	closeStarted := make(chan struct{})
	closeDone := make(chan error, 1)
	go func() {
		close(closeStarted)
		closeDone <- d.Close()
	}()

	// Hammer Enqueue while Close is racing through rootCancel.
	<-closeStarted
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Enqueue panicked under race: %v", r)
				}
			}()
			id := "X-" + string(rune('A'+n%26))
			_, _ = d.Enqueue(id)
		}(i)
	}
	wg.Wait()
	<-closeDone
}

func TestDaemon_DeactivateClearsActiveSet(t *testing.T) {
	a := &fakeAgent{
		onRun: func(ctx context.Context, id string) (string, error) {
			<-ctx.Done()
			return "cancelled", ctx.Err()
		},
	}
	d := New(Options{Board: newFakeBoard(), Agent: a, MaxWorkers: 1})
	t.Cleanup(func() {
		_ = d.Close()
	})
	if err := d.Activate(context.Background(), "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if ok, _ := d.Enqueue("TB-7"); !ok {
		t.Fatalf("enqueue rejected")
	}
	if !d.IsActive("TB-7") {
		t.Errorf("expected TB-7 active before Deactivate")
	}
	if err := d.Deactivate(); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if d.IsActive("TB-7") {
		t.Errorf("active flag should be cleared after Deactivate")
	}
}
