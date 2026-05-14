package app

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
)

type fakeUsageCollector struct {
	calls atomic.Int32
	mu    sync.Mutex
	out   []agent.Usage
}

func (f *fakeUsageCollector) set(out []agent.Usage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.out = out
}

func (f *fakeUsageCollector) Collect(_ context.Context) []agent.Usage {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	dup := make([]agent.Usage, len(f.out))
	copy(dup, f.out)
	return dup
}

type fakeEmitter struct {
	mu     sync.Mutex
	events []string
}

func (e *fakeEmitter) Emit(name string, _ ...any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, name)
}

func (e *fakeEmitter) names() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	dup := make([]string, len(e.events))
	copy(dup, e.events)
	return dup
}

func waitForSeed(t *testing.T, svc *UsageService) {
	t.Helper()
	select {
	case <-svc.SeedDone():
	case <-time.After(2 * time.Second):
		t.Fatal("usage service seed did not complete in time")
	}
}

func TestUsageService_SeedsCacheOnConstruction(t *testing.T) {
	pct := 12.5
	c := &fakeUsageCollector{}
	c.set([]agent.Usage{
		{Agent: "codex", Available: true, Primary: &agent.UsageWindow{UsedPercent: &pct}},
		{Agent: "claude", Available: false, Reason: "no oauth"},
	})

	svc := NewUsageService(UsageServiceOptions{Collector: c, Interval: time.Hour})
	svc.Start(t.Context())
	defer svc.Close()
	waitForSeed(t, svc)
	got := svc.GetAgentUsage()
	if len(got) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(got))
	}
	if got[0].Agent != "claude" || got[1].Agent != "codex" {
		t.Errorf("expected sorted by agent name, got %q/%q", got[0].Agent, got[1].Agent)
	}
	if c.calls.Load() != 1 {
		t.Errorf("expected exactly one seed call, got %d", c.calls.Load())
	}
}

func TestUsageService_RefreshEmitsEvent(t *testing.T) {
	c := &fakeUsageCollector{}
	c.set([]agent.Usage{{Agent: "codex", Available: true}})
	em := &fakeEmitter{}
	svc := NewUsageService(UsageServiceOptions{Collector: c, Emitter: em, Interval: time.Hour})
	svc.Start(t.Context())
	defer svc.Close()
	waitForSeed(t, svc)

	// The seed already emitted once. Reset and verify the manual refresh
	// emits exactly one more event.
	em.mu.Lock()
	em.events = nil
	em.mu.Unlock()

	got := svc.RefreshAgentUsage(context.Background())
	if len(got) != 1 || got[0].Agent != "codex" {
		t.Fatalf("expected codex snapshot, got %+v", got)
	}
	if names := em.names(); len(names) != 1 || names[0] != UsageEvent {
		t.Errorf("expected single %q emission, got %v", UsageEvent, names)
	}
}

func TestUsageService_PreservesPriorAgentOnEmptyCollect(t *testing.T) {
	pct := 7.0
	c := &fakeUsageCollector{}
	c.set([]agent.Usage{{Agent: "codex", Available: true, Primary: &agent.UsageWindow{UsedPercent: &pct}}})
	svc := NewUsageService(UsageServiceOptions{Collector: c, Interval: time.Hour})
	svc.Start(t.Context())
	defer svc.Close()
	waitForSeed(t, svc)

	// Subsequent collect returns nothing (e.g. transient FS error). We expect
	// the previous "codex" entry to remain — falling back to an empty map
	// would flicker the UI to "unknown" and is exactly what the spec
	// forbids.
	c.set(nil)
	svc.RefreshAgentUsage(context.Background())

	got := svc.GetAgentUsage()
	if len(got) != 1 || got[0].Agent != "codex" {
		t.Fatalf("expected prior codex snapshot preserved, got %+v", got)
	}
}
