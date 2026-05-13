package main

import (
	"context"

	tbapp "tools/tb-gui/app"
	"tools/tb-gui/internal/daemon"
)

// boardAdapter implements daemon.Board on top of *app.BoardService.
// Stays in package main so daemon doesn't depend on app and app doesn't
// depend on daemon — the cycle would otherwise form via the
// SettingsService/BoardActivator hook.
type boardAdapter struct {
	b *tbapp.BoardService
}

func (a *boardAdapter) ListActive(ctx context.Context) ([]daemon.AgentTask, error) {
	snap, err := a.b.LoadBoard(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]daemon.AgentTask, 0, len(snap.Backlog)+len(snap.InProgress)+len(snap.Done))
	for _, bucket := range [][]tbapp.Task{snap.Backlog, snap.InProgress, snap.Done} {
		for _, t := range bucket {
			out = append(out, daemon.AgentTask{
				ID:          t.ID,
				Agent:       t.Agent,
				AgentStatus: t.AgentStatus,
			})
		}
	}
	return out, nil
}

func (a *boardAdapter) GetTask(ctx context.Context, id string) (daemon.AgentTask, error) {
	d, err := a.b.GetTask(ctx, id)
	if err != nil {
		return daemon.AgentTask{}, err
	}
	return daemon.AgentTask{
		ID:          d.Metadata.ID,
		Agent:       d.Metadata.Agent,
		AgentStatus: d.Metadata.AgentStatus,
	}, nil
}

// agentAdapter implements daemon.Agent on top of *app.AgentService.
type agentAdapter struct {
	s *tbapp.AgentService
}

func (a *agentAdapter) RunQueuedAgentSync(ctx context.Context, id string) (string, error) {
	return a.s.RunQueuedAgentSync(ctx, id)
}

func (a *agentAdapter) HasActiveRun(id string) bool { return a.s.HasActiveRun(id) }

// teeShim adapts a daemon.TeeEmitter to the watcher.Emitter interface,
// which uses a concrete type rather than an interface for the chained
// emitter. The wrapping is purely structural.
type teeShim struct {
	tee daemon.TeeEmitter
}

func (t teeShim) Emit(name string, data ...any) {
	t.tee.Emit(name, data...)
}
