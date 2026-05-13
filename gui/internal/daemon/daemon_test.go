package daemon

import (
	"context"
	"errors"
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

// fakeAgent records every RunQueuedAgentSync call. The optional onRun
// hook lets a test block or simulate work.
type fakeAgent struct {
	mu      sync.Mutex
	calls   []string
	starts  []time.Time
	ends    []time.Time
	onRun   func(ctx context.Context, id string) (string, error)
	hasFn   func(id string) bool
	counter atomic.Int32
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

type fakeRecovery struct {
	calls atomic.Int32
	err   error
}

func (r *fakeRecovery) RecoverStale(ctx context.Context, boardDir string) error {
	r.calls.Add(1)
	return r.err
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
