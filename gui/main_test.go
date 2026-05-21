package main

import (
	"runtime"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"

	"tools/tb-gui/internal/shell"
)

func TestWindowClosePolicyForTray(t *testing.T) {
	tests := []struct {
		name          string
		traySupported bool
		wantTerminate bool
		wantDisable   bool
	}{
		{
			name:          "tray supported",
			traySupported: true,
			wantTerminate: false,
			wantDisable:   true,
		},
		{
			name:          "no tray",
			traySupported: false,
			wantTerminate: true,
			wantDisable:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := windowClosePolicyForTray(tt.traySupported)
			if got.terminateAfterLastWindowClosed != tt.wantTerminate {
				t.Fatalf("terminateAfterLastWindowClosed = %v, want %v", got.terminateAfterLastWindowClosed, tt.wantTerminate)
			}
			if got.disableQuitOnLastWindowClosed != tt.wantDisable {
				t.Fatalf("disableQuitOnLastWindowClosed = %v, want %v", got.disableQuitOnLastWindowClosed, tt.wantDisable)
			}
		})
	}
}

func TestWindowCloseTerminationPolicyFollowsTraySupport(t *testing.T) {
	got := shouldTerminateAfterLastWindowClosed()
	want := !shell.TraySupported()
	if got != want {
		t.Fatalf("shouldTerminateAfterLastWindowClosed = %v, want %v", got, want)
	}

	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		if got {
			t.Fatalf("tray-capable platform %s should keep app alive when main window closes", runtime.GOOS)
		}
	default:
		if !got {
			t.Fatalf("non-tray platform %s should terminate when last window closes", runtime.GOOS)
		}
	}
}

func TestHandleTrayWindowClosing(t *testing.T) {
	tests := []struct {
		name            string
		traySupported   bool
		appShuttingDown bool
		wantCancelled   bool
		wantHidden      bool
	}{
		{
			name:          "tray close hides",
			traySupported: true,
			wantCancelled: true,
			wantHidden:    true,
		},
		{
			name:            "quit shutdown closes",
			traySupported:   true,
			appShuttingDown: true,
		},
		{
			name: "no tray closes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := application.NewWindowEvent()
			hidden := false
			handleTrayWindowClosing(event, tt.traySupported, tt.appShuttingDown, func() {
				hidden = true
			})
			if event.IsCancelled() != tt.wantCancelled {
				t.Fatalf("cancelled = %v, want %v", event.IsCancelled(), tt.wantCancelled)
			}
			if hidden != tt.wantHidden {
				t.Fatalf("hidden = %v, want %v", hidden, tt.wantHidden)
			}
		})
	}
}
