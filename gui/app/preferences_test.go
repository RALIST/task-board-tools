package app

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func newSettingsForPrefs(t *testing.T) (*SettingsService, string) {
	t.Helper()
	dir := t.TempDir()
	prefs := filepath.Join(dir, "preferences.json")
	s := NewSettingsService(SettingsOptions{
		Logger:      slog.Default(),
		RecentsPath: filepath.Join(dir, "recent.json"),
		PrefsPath:   prefs,
	})
	return s, prefs
}

func TestGetMaxWorkers_MissingFile(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if got := s.GetMaxWorkers(); got != MaxWorkersDefault {
		t.Errorf("missing file: got %d, want %d", got, MaxWorkersDefault)
	}
}

func TestSetMaxWorkers_RoundTrip(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetMaxWorkers(3); err != nil {
		t.Fatalf("SetMaxWorkers: %v", err)
	}
	if got := s.GetMaxWorkers(); got != 3 {
		t.Errorf("after set: got %d, want 3", got)
	}
	// Reload from disk via a fresh instance.
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetMaxWorkers(); got != 3 {
		t.Errorf("fresh read: got %d, want 3", got)
	}
}

func TestSetMaxWorkers_ClampsBelow(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if err := s.SetMaxWorkers(0); err != nil {
		t.Fatalf("SetMaxWorkers(0): %v", err)
	}
	if got := s.GetMaxWorkers(); got != MaxWorkersDefault {
		t.Errorf("0 → got %d, want %d", got, MaxWorkersDefault)
	}
}

func TestSetMaxWorkers_ClampsAbove(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if err := s.SetMaxWorkers(999); err != nil {
		t.Fatalf("SetMaxWorkers(999): %v", err)
	}
	if got := s.GetMaxWorkers(); got != MaxWorkersMax {
		t.Errorf("999 → got %d, want %d", got, MaxWorkersMax)
	}
}

func TestGetMaxWorkers_ReadTimeClampsOutOfRangeFile(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	// Externally-edited file with bad value.
	b, _ := json.Marshal(Preferences{MaxWorkers: 99})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := s.GetMaxWorkers(); got != MaxWorkersMax {
		t.Errorf("clamp on read: got %d, want %d", got, MaxWorkersMax)
	}
}

func TestGetMaxWorkers_CorruptFileFallsBackToDefault(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := s.GetMaxWorkers(); got != MaxWorkersDefault {
		t.Errorf("corrupt file: got %d, want %d", got, MaxWorkersDefault)
	}
}
