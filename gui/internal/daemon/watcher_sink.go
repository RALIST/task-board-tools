package daemon

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// sinkBufferDefault is the depth of the in-process event channel
// between the watcher's emitter and the daemon's reader goroutine. A
// buffer absorbs short bursts; a drop is recoverable because the
// active-set dedup + next event re-triggers.
const sinkBufferDefault = 256

// EventSink is a watcher.Emitter implementation that forwards events
// to the daemon. Construct it via NewEventSink, chain it with the
// Wails app emitter via TeeEmitter, and pass the result to
// watcher.New.
//
// The sink does not block the watcher: events are non-blocking sent
// to an internal channel; if it's full, a WARN is logged and the
// event is dropped. The reader goroutine reconciles via
// (a) GetTask for task:updated:<id> and (b) ListActive for
// board:reloaded — both routes use Daemon.Enqueue with active-set
// dedup so a missed event is recovered by the next one.
type EventSink struct {
	d      *Daemon
	logger *slog.Logger
	events chan sinkEvent
	closed chan struct{}
	once   sync.Once
}

type sinkEvent struct {
	name string
	at   time.Time
}

// NewEventSink returns a sink wired to the given daemon. Call Start to
// spawn the reader goroutine.
func NewEventSink(d *Daemon, logger *slog.Logger) *EventSink {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventSink{
		d:      d,
		logger: logger.With("component", "daemon-sink"),
		events: make(chan sinkEvent, sinkBufferDefault),
		closed: make(chan struct{}),
	}
}

// Emit satisfies watcher.Emitter. It is called on the watcher's
// goroutine; we must not block. The data slice from the watcher is
// ignored — the event name itself carries all the routing data we
// need.
func (s *EventSink) Emit(name string, data ...any) {
	if !s.d.isActivated() {
		return
	}
	select {
	case s.events <- sinkEvent{name: name, at: time.Now()}:
	default:
		s.logger.Warn("daemon: sink overflow; dropping event", "event", name)
	}
}

// Start spawns the reader goroutine that drains events and enqueues
// ready tasks into the daemon. Returns immediately. Idempotent.
func (s *EventSink) Start(ctx context.Context) {
	go s.loop(ctx)
}

// Close releases the sink's reader goroutine. Safe to call multiple
// times.
func (s *EventSink) Close() {
	s.once.Do(func() {
		close(s.closed)
	})
}

func (s *EventSink) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closed:
			return
		case ev := <-s.events:
			s.handle(ctx, ev.name)
		}
	}
}

func (s *EventSink) handle(ctx context.Context, name string) {
	switch {
	case strings.HasPrefix(name, "task:updated:"):
		id := strings.TrimPrefix(name, "task:updated:")
		if id == "" {
			return
		}
		if err := s.d.ReconcileTask(ctx, id); err != nil {
			s.logger.Debug("daemon: task reconciliation failed", "task", id, "err", err)
		}
		if _, err := s.d.EnqueueIfReady(ctx, id); err != nil {
			s.logger.Debug("daemon: enqueue-if-ready failed", "task", id, "err", err)
		}
	case name == "board:reloaded":
		// Atomic-rename CLI edits route through this event. Re-scan
		// the active board and enqueue any newly-queued tasks. The
		// active-set dedup makes repeated scans cheap.
		if err := s.d.ReconcileActive(ctx); err != nil {
			s.logger.Debug("daemon: active reconciliation failed", "err", err)
		}
		if n, err := s.d.RescanActive(ctx); err != nil {
			s.logger.Debug("daemon: rescan failed", "err", err)
		} else if n > 0 {
			s.logger.Info("daemon: rescan enqueued tasks", "count", n)
		}
	}
}

// TeeEmitter forwards Emit to two emitters in sequence. Used in main.go
// to fan watcher events out to both the Wails app event bus AND the
// daemon's sink.
type TeeEmitter struct {
	A WatcherEmitter
	B WatcherEmitter
}

// WatcherEmitter is the contract shared with internal/watcher.Emitter
// — declared here so this package doesn't need to import watcher (which
// would create a cycle if watcher ever needed daemon types).
type WatcherEmitter interface {
	Emit(name string, data ...any)
}

// Emit forwards to A then B. A nil emitter is a no-op.
func (t TeeEmitter) Emit(name string, data ...any) {
	if t.A != nil {
		t.A.Emit(name, data...)
	}
	if t.B != nil {
		t.B.Emit(name, data...)
	}
}
