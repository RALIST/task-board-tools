package app

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"tools/tb-gui/internal/agent"
)

// UsageRefreshInterval is how often UsageService re-collects per-agent quota
// usage in the background. Five minutes balances staleness against per-call
// IO (codex JSONL walks) and matches the cache TTL of the upstream agent
// data (5h window changes slowly).
const UsageRefreshInterval = 5 * time.Minute

// usageRefreshTimeout caps a single refresh so a stuck filesystem (slow
// network home, codex sessions tree with thousands of stat-able entries)
// can never freeze the Wails RPC goroutine or the periodic ticker.
const usageRefreshTimeout = 8 * time.Second

// UsageEvent is the Wails event UsageService emits after a successful refresh.
// The frontend store subscribes to this name to live-update.
const UsageEvent = "agent-usage:updated"

// UsageCollector is the contract UsageService needs to obtain a per-agent
// snapshot. The production implementation calls agent.CollectCodexUsage /
// agent.CollectClaudeUsage; tests pass an in-memory implementation.
type UsageCollector interface {
	Collect(ctx context.Context) []agent.Usage
}

// ProjectRootProvider returns the currently-open board's project root. The
// claude collector uses it to find the project-scoped statusline tap output;
// when no board is open, the empty string is returned and the collector
// falls back to the "unknown — no project open" state.
type ProjectRootProvider func() string

// defaultUsageCollector is the production collector. It walks the user's
// configured agent CLIs and produces one snapshot per supported agent.
type defaultUsageCollector struct {
	projectRoot ProjectRootProvider
}

// Collect implements UsageCollector. Both calls are pure filesystem reads, so
// we run them sequentially — parallelising buys nothing material here.
func (c defaultUsageCollector) Collect(ctx context.Context) []agent.Usage {
	if err := ctx.Err(); err != nil {
		return nil
	}
	root := ""
	if c.projectRoot != nil {
		root = c.projectRoot()
	}
	return []agent.Usage{
		agent.CollectCodexUsage(""),
		agent.CollectClaudeUsage("", root),
	}
}

// UsageService caches per-agent quota usage and refreshes it on a schedule
// and on demand. It is a separate Wails service so the frontend can bind to
// GetAgentUsage / RefreshAgentUsage without going through AgentService (whose
// per-run lifecycle is unrelated).
type UsageService struct {
	collector UsageCollector
	emitter   Emitter
	logger    *slog.Logger
	interval  time.Duration

	mu        sync.RWMutex
	snapshots map[string]agent.Usage

	stopOnce sync.Once
	stopCh   chan struct{}
	// seedDone closes once the asynchronous startup refresh completes (success
	// or failure). Tests block on it; production callers can ignore it.
	seedDone chan struct{}
}

// UsageServiceOptions wires construction-time dependencies. All fields are
// optional — zero values produce a service that runs the default collector
// with the default refresh interval and a no-op emitter.
type UsageServiceOptions struct {
	Collector   UsageCollector
	Emitter     Emitter
	Logger      *slog.Logger
	Interval    time.Duration
	ProjectRoot ProjectRootProvider
}

// NewUsageService builds a UsageService and seeds its cache so the first
// frontend call to GetAgentUsage returns rendered values immediately rather
// than an empty map.
func NewUsageService(opts UsageServiceOptions) *UsageService {
	svc := &UsageService{
		collector: opts.Collector,
		emitter:   opts.Emitter,
		logger:    opts.Logger,
		interval:  opts.Interval,
		snapshots: make(map[string]agent.Usage),
		stopCh:    make(chan struct{}),
		seedDone:  make(chan struct{}),
	}
	if svc.collector == nil {
		svc.collector = defaultUsageCollector{projectRoot: opts.ProjectRoot}
	}
	if svc.logger == nil {
		svc.logger = slog.Default()
	}
	if svc.interval <= 0 {
		svc.interval = UsageRefreshInterval
	}
	// The seed runs on Start(ctx), not here — at construction time the
	// emitter's underlying *application.App may not exist yet (main.go
	// passes the service into application.New, and only sets the app
	// reference afterwards), so an emit fired during NewUsageService would
	// be silently dropped before the frontend can subscribe.
	return svc
}

// SeedDone returns a channel that closes once the asynchronous startup
// refresh has completed. Exposed for tests; production callers should rely on
// GetAgentUsage / the agent-usage:updated event instead.
func (s *UsageService) SeedDone() <-chan struct{} { return s.seedDone }

// ServiceName satisfies Wails service registration; the frontend binds to
// `UsageService.GetAgentUsage` / `UsageService.RefreshAgentUsage`.
func (s *UsageService) ServiceName() string { return "UsageService" }

// GetAgentUsage returns the current per-agent usage snapshots, sorted by
// agent name for stable rendering. Safe to call from any goroutine.
func (s *UsageService) GetAgentUsage() []agent.Usage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]agent.Usage, 0, len(s.snapshots))
	for _, u := range s.snapshots {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Agent < out[j].Agent })
	return out
}

// RefreshAgentUsage forces a refresh and emits the update event when it
// changes any cached value. Bound to the frontend "refresh" button. The
// refresh runs under a bounded timeout so a slow filesystem can't pin the
// Wails RPC goroutine.
func (s *UsageService) RefreshAgentUsage(ctx context.Context) []agent.Usage {
	if ctx == nil {
		ctx = context.Background()
	}
	refreshCtx, cancel := context.WithTimeout(ctx, usageRefreshTimeout)
	defer cancel()
	s.refresh(refreshCtx, true)
	return s.GetAgentUsage()
}

// Start performs an initial seed refresh and begins the background loop.
// Returns immediately; the loop stops on Close or when ctx is cancelled.
// The seed runs here (not in the constructor) so the emitter's underlying
// *application.App is already live and the seed's UsageEvent reaches the
// frontend instead of being dropped.
func (s *UsageService) Start(ctx context.Context) {
	go func() {
		defer close(s.seedDone)
		seedCtx, cancel := context.WithTimeout(ctx, usageRefreshTimeout)
		defer cancel()
		s.refresh(seedCtx, true)
	}()
	go s.run(ctx)
}

// Close stops the background refresh loop. Safe to call multiple times.
func (s *UsageService) Close() {
	s.stopOnce.Do(func() { close(s.stopCh) })
}

func (s *UsageService) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			refreshCtx, cancel := context.WithTimeout(ctx, usageRefreshTimeout)
			s.refresh(refreshCtx, true)
			cancel()
		}
	}
}

// refresh runs the collector and updates the cache. emit=true publishes a
// Wails event after a successful update so the frontend can re-render
// without polling. emit=false is used for the constructor seed (no listeners
// yet) and tests.
func (s *UsageService) refresh(ctx context.Context, emit bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	snapshots := s.collector.Collect(ctx)
	if ctx.Err() != nil {
		return
	}

	s.mu.Lock()
	for _, u := range snapshots {
		if u.Agent == "" {
			continue
		}
		s.snapshots[u.Agent] = u
	}
	s.mu.Unlock()

	if emit && s.emitter != nil {
		s.emitter.Emit(UsageEvent, s.GetAgentUsage())
	}
}
