package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
)

// preferencesFile is the filename SettingsService persists tuning knobs
// to, sibling of recent.json. Separate file so the existing recent.json
// schema doesn't need a migration on every existing user.
const preferencesFile = "preferences.json"

// MaxWorkersDefault is the daemon's default semaphore capacity when the
// preferences file is missing or specifies an out-of-range value.
const MaxWorkersDefault = 1

// MaxWorkersMin / MaxWorkersMax bracket the allowed range for
// max_workers. Clamping happens on read so an externally-edited
// preferences.json with a wild value never destabilises the daemon.
const (
	MaxWorkersMin = 1
	MaxWorkersMax = 4
)

// Preferences is the persisted tuning knob set. Fields are JSON-tagged
// in snake_case so a future settings UI (M7) can serialise the same
// shape directly from a hand-edited form.
type Preferences struct {
	MaxWorkers int `json:"max_workers"`
}

// preferencesPath returns the absolute path the preferences file lives
// at. Mirrors defaultRecentsPath but for the preferences file.
func defaultPreferencesPath() string {
	return filepath.Join(filepath.Dir(defaultRecentsPath()), preferencesFile)
}

// preferencesPath returns the override-aware preferences path. Tests
// supply a tmp path via the SettingsOptions struct.
func (s *SettingsService) preferencesPath() string {
	if s.prefsPath != "" {
		return s.prefsPath
	}
	return defaultPreferencesPath()
}

// GetMaxWorkers returns the persisted max_workers value, clamped to
// [MaxWorkersMin, MaxWorkersMax]. A missing file (or a missing field)
// yields MaxWorkersDefault.
//
// Values outside the allowed range are coerced AND logged at WARN so
// the operator can fix the file; we don't fail the call — over-defending
// against bad config files breaks the GUI for no benefit.
func (s *SettingsService) GetMaxWorkers() int {
	prefs, err := s.loadPreferences()
	if err != nil {
		s.logger.Warn("preferences: read failed; using default", "err", err)
		return MaxWorkersDefault
	}
	return clampMaxWorkers(prefs.MaxWorkers, s.logger)
}

// SetMaxWorkers persists the value to disk after clamping. Returns the
// underlying I/O error on failure.
func (s *SettingsService) SetMaxWorkers(n int) error {
	prefs, err := s.loadPreferences()
	if err != nil {
		// Treat a corrupt file as empty — overwrite cleanly.
		s.logger.Warn("preferences: read for write failed; starting fresh", "err", err)
		prefs = Preferences{}
	}
	prefs.MaxWorkers = clampMaxWorkers(n, s.logger)
	return s.savePreferences(prefs)
}

func clampMaxWorkers(n int, logger *slog.Logger) int {
	if n < MaxWorkersMin {
		if n != 0 {
			logger.Warn("preferences: max_workers below min; clamping",
				"value", n, "min", MaxWorkersMin)
		}
		return MaxWorkersDefault
	}
	if n > MaxWorkersMax {
		logger.Warn("preferences: max_workers above max; clamping",
			"value", n, "max", MaxWorkersMax)
		return MaxWorkersMax
	}
	return n
}

func (s *SettingsService) loadPreferences() (Preferences, error) {
	path := s.preferencesPath()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Preferences{MaxWorkers: MaxWorkersDefault}, nil
	}
	if err != nil {
		return Preferences{}, err
	}
	var p Preferences
	if err := json.Unmarshal(b, &p); err != nil {
		return Preferences{}, err
	}
	return p, nil
}

func (s *SettingsService) savePreferences(prefs Preferences) error {
	path := s.preferencesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
