package app

import (
	"context"
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

func TestPreferences_MissingFileReturnsDefaults(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if got := s.GetMaxWorkers(); got != MaxWorkersDefault {
		t.Errorf("max_workers: got %d, want %d", got, MaxWorkersDefault)
	}
	if got := s.GetAgentTimeoutMinutes(); got != AgentTimeoutMinutesDefault {
		t.Errorf("agent_timeout_minutes: got %d, want %d", got, AgentTimeoutMinutesDefault)
	}
	if got := s.GetDefaultAgent(); got != "none" {
		t.Errorf("default_agent: got %q, want none", got)
	}
	if got := s.GetCLIPath(); got != "" {
		t.Errorf("cli_path: got %q, want empty", got)
	}
	if got := s.GetPeriodicRecoveryEnabled(); !got {
		t.Errorf("periodic_recovery_enabled: got false, want true")
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

func TestSetAgentTimeoutMinutes_RoundTrip(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetAgentTimeoutMinutes(45); err != nil {
		t.Fatalf("SetAgentTimeoutMinutes: %v", err)
	}
	if got := s.GetAgentTimeoutMinutes(); got != 45 {
		t.Errorf("after set: got %d, want 45", got)
	}
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetAgentTimeoutMinutes(); got != 45 {
		t.Errorf("fresh read: got %d, want 45", got)
	}
}

func TestSetDefaultAgent_RoundTrip(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetDefaultAgent("Codex"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if got := s.GetDefaultAgent(); got != "codex" {
		t.Errorf("after set: got %q, want codex", got)
	}
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetDefaultAgent(); got != "codex" {
		t.Errorf("fresh read: got %q, want codex", got)
	}
}

func TestSetCLIPath_RoundTrip(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	want := stubTbBinary(t)
	if err := s.SetCLIPath(want); err != nil {
		t.Fatalf("SetCLIPath: %v", err)
	}
	if got := s.GetCLIPath(); got != want {
		t.Errorf("after set: got %q, want %q", got, want)
	}
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetCLIPath(); got != want {
		t.Errorf("fresh read: got %q, want %q", got, want)
	}
}

func TestSetPeriodicRecoveryEnabled_RoundTrip(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetPeriodicRecoveryEnabled(false); err != nil {
		t.Fatalf("SetPeriodicRecoveryEnabled: %v", err)
	}
	if got := s.GetPeriodicRecoveryEnabled(); got {
		t.Errorf("after set: got true, want false")
	}
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetPeriodicRecoveryEnabled(); got {
		t.Errorf("fresh read: got true, want false")
	}
}

type fakePeriodicRecoveryActivator struct {
	enabledCalls []bool
}

func (f *fakePeriodicRecoveryActivator) Activate(ctx context.Context, boardDir string) error {
	return nil
}

func (f *fakePeriodicRecoveryActivator) Deactivate() error {
	return nil
}

func (f *fakePeriodicRecoveryActivator) SetPeriodicRecoveryEnabled(enabled bool) {
	f.enabledCalls = append(f.enabledCalls, enabled)
}

func TestSetPeriodicRecoveryEnabled_UpdatesActivatorRuntime(t *testing.T) {
	activator := &fakePeriodicRecoveryActivator{}
	dir := t.TempDir()
	s := NewSettingsService(SettingsOptions{
		Logger:      slog.Default(),
		RecentsPath: filepath.Join(dir, "recent.json"),
		PrefsPath:   filepath.Join(dir, "preferences.json"),
		Activator:   activator,
	})

	if err := s.SetPeriodicRecoveryEnabled(false); err != nil {
		t.Fatalf("SetPeriodicRecoveryEnabled(false): %v", err)
	}
	if err := s.SetPeriodicRecoveryEnabled(true); err != nil {
		t.Fatalf("SetPeriodicRecoveryEnabled(true): %v", err)
	}

	if got, want := activator.enabledCalls, []bool{false, true}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("runtime toggle calls = %v, want %v", got, want)
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

func TestSetAgentTimeoutMinutes_ZeroUsesDefault(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if err := s.SetAgentTimeoutMinutes(0); err != nil {
		t.Fatalf("SetAgentTimeoutMinutes(0): %v", err)
	}
	if got := s.GetAgentTimeoutMinutes(); got != AgentTimeoutMinutesDefault {
		t.Errorf("0 → got %d, want %d", got, AgentTimeoutMinutesDefault)
	}
}

func TestGetAgentTimeoutMinutes_ReadTimeClampsAbove(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	b, _ := json.Marshal(Preferences{AgentTimeoutMinutes: 99999})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := s.GetAgentTimeoutMinutes(); got != AgentTimeoutMinutesMax {
		t.Errorf("clamp on read: got %d, want %d", got, AgentTimeoutMinutesMax)
	}
}

func TestGetDefaultAgent_ReadTimeUnknownFallsBackToNone(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	b, _ := json.Marshal(Preferences{DefaultAgent: "foo"})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := s.GetDefaultAgent(); got != "none" {
		t.Errorf("unknown default_agent: got %q, want none", got)
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
