package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventSink_TaskUpdated_Enqueues(t *testing.T) {
	a := &fakeAgent{}
	b := newFakeBoard(AgentTask{ID: "TB-1", Agent: "claude", AgentStatus: "queued"})
	d := New(Options{Board: b, Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })

	sink := NewEventSink(d, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	sink.Start(ctx)

	if err := d.Activate(ctx, "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	sink.Emit("task:updated:TB-1", "TB-1")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.callCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.callCount() < 1 {
		t.Errorf("task:updated did not produce a run")
	}
}

func TestEventSink_BoardReloaded_Rescans(t *testing.T) {
	a := &fakeAgent{}
	b := newFakeBoard()
	d := New(Options{Board: b, Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })

	sink := NewEventSink(d, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	sink.Start(ctx)
	if err := d.Activate(ctx, "/tmp/fake"); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	// CLI edit lands AFTER activation — append the queued task.
	b.set(AgentTask{ID: "TB-77", Agent: "claude", AgentStatus: "queued"})
	sink.Emit("board:reloaded")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a.callCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if a.callCount() < 1 {
		t.Errorf("board:reloaded did not rescan-and-enqueue")
	}
}

func TestEventSink_DropsWhenNotActivated(t *testing.T) {
	a := &fakeAgent{}
	b := newFakeBoard(AgentTask{ID: "TB-1", Agent: "claude", AgentStatus: "queued"})
	d := New(Options{Board: b, Agent: a, MaxWorkers: 1})
	t.Cleanup(func() { _ = d.Close() })

	sink := NewEventSink(d, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	sink.Start(ctx)

	// No Activate. Emit should be a silent no-op.
	sink.Emit("task:updated:TB-1", "TB-1")
	time.Sleep(50 * time.Millisecond)
	if a.callCount() != 0 {
		t.Errorf("sink should drop events when daemon not activated")
	}
}

func TestTeeEmitter_FansOut(t *testing.T) {
	var aCount, bCount atomic.Int32
	a := emitterFn(func() { aCount.Add(1) })
	b := emitterFn(func() { bCount.Add(1) })
	tee := TeeEmitter{A: a, B: b}
	tee.Emit("x")
	tee.Emit("y")
	if aCount.Load() != 2 || bCount.Load() != 2 {
		t.Errorf("tee fan-out failed: a=%d b=%d", aCount.Load(), bCount.Load())
	}
}

type emitterFn func()

func (f emitterFn) Emit(name string, data ...any) { f() }
