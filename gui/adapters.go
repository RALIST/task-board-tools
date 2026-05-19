package main

import (
	"context"
	"errors"

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

// boardActivator is the composite that drives the daemon plus every
// coordinator from a single SettingsService.Activator slot (TB-174 +
// TB-179). Activation is sequential: the daemon runs stale-recovery
// and the startup queue scan first; then the coordinators begin their
// scan loops against a post-reconciled view.
//
// Implements:
//   - app.BoardActivator (required by SettingsService).
//   - app.PeriodicRecoveryController (forwarded to the daemon so the
//     runtime preference toggle reaches the ticker).
//   - app.AutoGroomController (forwarded to the auto-groom coordinator
//     so SetAutoGroomEnabled / SetDefaultAgent kick fresh scans).
//   - app.AutoImplementController (forwarded to the auto-implement
//     coordinator so SetAutoImplementEnabled / SetAutoImplementQuery /
//     SetDefaultAgent kick fresh scans).
type boardActivator struct {
	daemon        *daemon.Daemon
	autoGroom     *tbapp.AutoGroomCoordinator
	autoImplement *tbapp.AutoImplementCoordinator
}

func (a *boardActivator) Activate(ctx context.Context, boardDir string) error {
	if err := a.daemon.Activate(ctx, boardDir); err != nil {
		return err
	}
	if err := a.autoGroom.Activate(ctx, boardDir); err != nil {
		return err
	}
	return a.autoImplement.Activate(ctx, boardDir)
}

func (a *boardActivator) Deactivate() error {
	// Stop the coordinators first so any in-flight timers don't fire a
	// scan against a stale boardDir while the daemon tears down. Errors
	// are joined so callers see all failures — keeps the contract
	// honest if any Deactivate gains a real error path.
	implErr := a.autoImplement.Deactivate()
	coordErr := a.autoGroom.Deactivate()
	daemonErr := a.daemon.Deactivate()
	return errors.Join(implErr, coordErr, daemonErr)
}

func (a *boardActivator) SetPeriodicRecoveryEnabled(enabled bool) {
	a.daemon.SetPeriodicRecoveryEnabled(enabled)
}

func (a *boardActivator) NotifyAutoGroomEnabled() {
	a.autoGroom.NotifyAutoGroomEnabled()
}

func (a *boardActivator) NotifyDefaultAgentChanged() {
	a.autoGroom.NotifyDefaultAgentChanged()
	a.autoImplement.NotifyDefaultAgentChanged()
}

func (a *boardActivator) NotifyAutoImplementEnabled() {
	a.autoImplement.NotifyAutoImplementEnabled()
}

func (a *boardActivator) NotifyAutoImplementQueryChanged() {
	a.autoImplement.NotifyAutoImplementQueryChanged()
}
