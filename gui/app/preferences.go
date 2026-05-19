package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

// AgentTimeoutMinutesDefault is the default unattended agent-run deadline
// persisted in preferences.json. The service converts it to a duration per
// run so settings changes take effect without restarting.
const AgentTimeoutMinutesDefault = 30

// AgentTimeoutMinutesMin / AgentTimeoutMinutesMax bracket the user-tunable
// timeout range in minutes.
const (
	AgentTimeoutMinutesMin = 1
	AgentTimeoutMinutesMax = 240
)

// DefaultAgentValues is the persisted enum for the settings panel's default
// agent dropdown. "none" means leave unassigned unless the user chooses one.
var DefaultAgentValues = []string{"none", "claude", "codex"}

// AutoGroomEnabledDefault is the default for the auto-groom feature.
// Off so existing users opt in deliberately.
const AutoGroomEnabledDefault = false

// AutoGroomSettleMinutesDefault is the default settle window before a
// freshly-created or edited backlog task becomes eligible for auto-groom.
// Five minutes gives users time to attach files and refine the task.
const AutoGroomSettleMinutesDefault = 5

// AutoGroomSettleMinutesMin / AutoGroomSettleMinutesMax bracket the user-tunable
// settle window in minutes. Zero opts out of the window entirely.
const (
	AutoGroomSettleMinutesMin = 0
	AutoGroomSettleMinutesMax = 60
)

// Preferences is the persisted tuning knob set. Fields are JSON-tagged
// in snake_case so a future settings UI (M7) can serialise the same
// shape directly from a hand-edited form.
type Preferences struct {
	MaxWorkers              int    `json:"max_workers"`
	AgentTimeoutMinutes     int    `json:"agent_timeout_minutes"`
	DefaultAgent            string `json:"default_agent"`
	CLIPath                 string `json:"cli_path"`
	DisablePeriodicRecovery bool   `json:"disable_periodic_recovery"`
	AutoGroomEnabled        bool   `json:"auto_groom_enabled"`
	AutoGroomSettleMinutes  int    `json:"auto_groom_settle_minutes"`
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
	return s.updatePreferences(func(prefs *Preferences) {
		prefs.MaxWorkers = n
	})
}

// GetAgentTimeoutMinutes returns the persisted agent timeout in minutes,
// clamped to [AgentTimeoutMinutesMin, AgentTimeoutMinutesMax]. Missing/zero
// values yield AgentTimeoutMinutesDefault.
func (s *SettingsService) GetAgentTimeoutMinutes() int {
	prefs, err := s.loadPreferences()
	if err != nil {
		s.logger.Warn("preferences: read failed; using default", "err", err)
		return AgentTimeoutMinutesDefault
	}
	return clampAgentTimeoutMinutes(prefs.AgentTimeoutMinutes, s.logger)
}

// SetAgentTimeoutMinutes persists the agent timeout in minutes after
// clamping. Returns the underlying I/O error on failure.
func (s *SettingsService) SetAgentTimeoutMinutes(n int) error {
	return s.updatePreferences(func(prefs *Preferences) {
		prefs.AgentTimeoutMinutes = n
	})
}

// GetDefaultAgent returns the default agent selection. Unknown values fall
// back to "none" so hand-edited config cannot force an unsupported runner.
func (s *SettingsService) GetDefaultAgent() string {
	prefs, err := s.loadPreferences()
	if err != nil {
		s.logger.Warn("preferences: read failed; using default", "err", err)
		return "none"
	}
	return normalizeDefaultAgent(prefs.DefaultAgent, s.logger)
}

// SetDefaultAgent persists the default agent selection after normalizing it
// to the supported enum. Notifies the auto-groom coordinator (if wired) so
// it can re-evaluate the no-default-agent gate immediately.
func (s *SettingsService) SetDefaultAgent(agent string) error {
	if err := s.updatePreferences(func(prefs *Preferences) {
		prefs.DefaultAgent = agent
	}); err != nil {
		return err
	}
	if controller, ok := s.activator.(AutoGroomController); ok {
		controller.NotifyDefaultAgentChanged()
	}
	return nil
}

// GetCLIPath returns the persisted tb binary path override. Empty means the
// CLI client should use PATH lookup.
func (s *SettingsService) GetCLIPath() string {
	prefs, err := s.loadPreferences()
	if err != nil {
		s.logger.Warn("preferences: read failed; using default", "err", err)
		return ""
	}
	return prefs.CLIPath
}

// GetPeriodicRecoveryEnabled returns whether the daemon should run the
// steady-state stale-recovery ticker. Missing preferences default to enabled;
// the persisted field is a disable flag so old files keep the safer behavior.
func (s *SettingsService) GetPeriodicRecoveryEnabled() bool {
	prefs, err := s.loadPreferences()
	if err != nil {
		s.logger.Warn("preferences: read failed; using default", "err", err)
		return true
	}
	return !prefs.DisablePeriodicRecovery
}

// SetPeriodicRecoveryEnabled persists the steady-state recovery toggle.
// Startup-time recovery still runs even when this is disabled.
func (s *SettingsService) SetPeriodicRecoveryEnabled(enabled bool) error {
	if err := s.updatePreferences(func(prefs *Preferences) {
		prefs.DisablePeriodicRecovery = !enabled
	}); err != nil {
		return err
	}
	if controller, ok := s.activator.(PeriodicRecoveryController); ok {
		controller.SetPeriodicRecoveryEnabled(enabled)
	}
	return nil
}

// GetAutoGroomEnabled returns whether the GUI auto-groom coordinator may
// start grooming runs for triage-flagged backlog tasks. Missing preferences
// default to false (opt-in feature).
func (s *SettingsService) GetAutoGroomEnabled() bool {
	prefs, err := s.loadPreferences()
	if err != nil {
		s.logger.Warn("preferences: read failed; using default", "err", err)
		return AutoGroomEnabledDefault
	}
	return prefs.AutoGroomEnabled
}

// SetAutoGroomEnabled persists the auto-groom on/off toggle. Notifies
// the auto-groom coordinator (if wired) so a freshly flipped preference
// triggers an immediate scan instead of waiting for the next watcher
// event.
func (s *SettingsService) SetAutoGroomEnabled(enabled bool) error {
	if err := s.updatePreferences(func(prefs *Preferences) {
		prefs.AutoGroomEnabled = enabled
	}); err != nil {
		return err
	}
	if controller, ok := s.activator.(AutoGroomController); ok {
		controller.NotifyAutoGroomEnabled()
	}
	return nil
}

// GetAutoGroomSettleMinutes returns the configured settle window before a
// freshly created or edited backlog task becomes eligible for auto-groom.
// Clamped to [AutoGroomSettleMinutesMin, AutoGroomSettleMinutesMax]; missing
// values yield AutoGroomSettleMinutesDefault.
func (s *SettingsService) GetAutoGroomSettleMinutes() int {
	prefs, err := s.loadPreferences()
	if err != nil {
		s.logger.Warn("preferences: read failed; using default", "err", err)
		return AutoGroomSettleMinutesDefault
	}
	return clampAutoGroomSettleMinutes(prefs.AutoGroomSettleMinutes, s.logger)
}

// SetAutoGroomSettleMinutes persists the settle window after clamping.
func (s *SettingsService) SetAutoGroomSettleMinutes(n int) error {
	return s.updatePreferences(func(prefs *Preferences) {
		prefs.AutoGroomSettleMinutes = n
	})
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

func clampAgentTimeoutMinutes(n int, logger *slog.Logger) int {
	if n == 0 {
		return AgentTimeoutMinutesDefault
	}
	if n < AgentTimeoutMinutesMin {
		logger.Warn("preferences: agent_timeout_minutes below min; clamping",
			"value", n, "min", AgentTimeoutMinutesMin)
		return AgentTimeoutMinutesMin
	}
	if n > AgentTimeoutMinutesMax {
		logger.Warn("preferences: agent_timeout_minutes above max; clamping",
			"value", n, "max", AgentTimeoutMinutesMax)
		return AgentTimeoutMinutesMax
	}
	return n
}

// clampAutoGroomSettleMinutes coerces the persisted value into the allowed
// [Min, Max] range. Negative or missing-on-an-existing-file values fall
// back to the default; the field is a non-pointer int so we cannot
// distinguish "absent" from "explicit 0" at this layer. Zero is treated
// as an explicit opt-out and passes through unchanged.
func clampAutoGroomSettleMinutes(n int, logger *slog.Logger) int {
	if n < AutoGroomSettleMinutesMin {
		logger.Warn("preferences: auto_groom_settle_minutes below min; clamping",
			"value", n, "min", AutoGroomSettleMinutesMin)
		return AutoGroomSettleMinutesMin
	}
	if n > AutoGroomSettleMinutesMax {
		logger.Warn("preferences: auto_groom_settle_minutes above max; clamping",
			"value", n, "max", AutoGroomSettleMinutesMax)
		return AutoGroomSettleMinutesMax
	}
	return n
}

func normalizeDefaultAgent(agent string, logger *slog.Logger) string {
	trimmed := strings.ToLower(strings.TrimSpace(agent))
	if trimmed == "" {
		return "none"
	}
	for _, allowed := range DefaultAgentValues {
		if trimmed == allowed {
			return trimmed
		}
	}
	logger.Warn("preferences: default_agent unsupported; using none",
		"value", agent, "allowed", DefaultAgentValues)
	return "none"
}

func defaultPreferences() Preferences {
	return Preferences{
		MaxWorkers:             MaxWorkersDefault,
		AgentTimeoutMinutes:    AgentTimeoutMinutesDefault,
		DefaultAgent:           "none",
		AutoGroomEnabled:       AutoGroomEnabledDefault,
		AutoGroomSettleMinutes: AutoGroomSettleMinutesDefault,
	}
}

func normalizePreferences(prefs Preferences, logger *slog.Logger) Preferences {
	prefs.MaxWorkers = clampMaxWorkers(prefs.MaxWorkers, logger)
	prefs.AgentTimeoutMinutes = clampAgentTimeoutMinutes(prefs.AgentTimeoutMinutes, logger)
	prefs.DefaultAgent = normalizeDefaultAgent(prefs.DefaultAgent, logger)
	prefs.AutoGroomSettleMinutes = clampAutoGroomSettleMinutes(prefs.AutoGroomSettleMinutes, logger)
	return prefs
}

func (s *SettingsService) updatePreferences(mut func(*Preferences)) error {
	prefs, err := s.loadPreferences()
	if err != nil {
		// Treat a corrupt file as empty — overwrite cleanly.
		s.logger.Warn("preferences: read for write failed; starting fresh", "err", err)
		prefs = defaultPreferences()
	}
	prefs = normalizePreferences(prefs, s.logger)
	mut(&prefs)
	prefs = normalizePreferences(prefs, s.logger)
	return s.savePreferences(prefs)
}

func (s *SettingsService) loadPreferences() (Preferences, error) {
	path := s.preferencesPath()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultPreferences(), nil
	}
	if err != nil {
		return Preferences{}, err
	}
	var p Preferences
	if err := json.Unmarshal(b, &p); err != nil {
		return Preferences{}, err
	}
	// Second pass: for fields where the zero value is a legitimate user
	// choice distinct from "use default", check key presence so an older
	// preferences.json without the key falls back to the default rather
	// than silently adopting the int zero. Currently only
	// AutoGroomSettleMinutes (0 means "no delay", default is 5).
	var raw map[string]json.RawMessage
	if jsonErr := json.Unmarshal(b, &raw); jsonErr == nil {
		if _, present := raw["auto_groom_settle_minutes"]; !present {
			p.AutoGroomSettleMinutes = AutoGroomSettleMinutesDefault
		}
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
