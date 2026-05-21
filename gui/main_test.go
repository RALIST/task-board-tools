package main

import (
	"runtime"
	"testing"

	"tools/tb-gui/internal/shell"
)

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
