package app

import (
	"context"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// TestOpenBoard_FiresBoardOpenedHook verifies the regression covered by the
// TB-232 follow-up: UsageService's seed runs before any board is open, so the
// claude usage chip caches a stale "no project open" snapshot. OpenBoard now
// fires a hook (production: UsageService.RefreshAgentUsage) so the chip
// repopulates the moment a board is committed — no waiting for the 5-min
// ticker, no manual ↻ click.
func TestOpenBoard_FiresBoardOpenedHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shell stub")
	}
	tb := buildTbForIntegration(t)
	root := fixtureBoard(t, "TB")
	recents := filepath.Join(t.TempDir(), "recent.json")

	var fired atomic.Int32
	done := make(chan struct{}, 1)
	svc := NewSettingsService(SettingsOptions{
		Board:       NewBoardService(),
		Watcher:     &fakeSwitcher{},
		CLIPath:     tb,
		RecentsPath: recents,
	})
	svc.SetBoardOpenedHook(func(ctx context.Context) {
		fired.Add(1)
		select {
		case done <- struct{}{}:
		default:
		}
	})

	if err := svc.OpenBoard(context.Background(), root); err != nil {
		t.Fatalf("OpenBoard: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("BoardOpenedHook never fired after OpenBoard")
	}
	if got := fired.Load(); got != 1 {
		t.Fatalf("hook fire count: got %d want 1", got)
	}
}

// TestOpenBoard_HookContextDetachedFromCaller verifies the hook does not
// inherit the OpenBoard ctx. The Wails RPC ctx is cancelled the moment
// OpenBoard returns; if the hook saw that ctx its refresh would be cancelled
// immediately and the chip would still show "unknown".
func TestOpenBoard_HookContextDetachedFromCaller(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shell stub")
	}
	tb := buildTbForIntegration(t)
	root := fixtureBoard(t, "TB")
	recents := filepath.Join(t.TempDir(), "recent.json")

	ctxErrCh := make(chan error, 1)
	svc := NewSettingsService(SettingsOptions{
		Board:       NewBoardService(),
		Watcher:     &fakeSwitcher{},
		CLIPath:     tb,
		RecentsPath: recents,
	})
	svc.SetBoardOpenedHook(func(ctx context.Context) {
		// Briefly wait so the OpenBoard ctx (which we'll cancel below)
		// would have time to propagate IF the hook used it.
		select {
		case <-ctx.Done():
			ctxErrCh <- ctx.Err()
		case <-time.After(100 * time.Millisecond):
			ctxErrCh <- nil
		}
	})

	callerCtx, cancelCaller := context.WithCancel(context.Background())
	if err := svc.OpenBoard(callerCtx, root); err != nil {
		t.Fatalf("OpenBoard: %v", err)
	}
	cancelCaller() // simulate the RPC returning to the frontend

	select {
	case err := <-ctxErrCh:
		if err != nil {
			t.Fatalf("hook ctx leaked from caller: got %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("hook did not complete in time")
	}
}
