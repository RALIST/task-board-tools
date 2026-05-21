package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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

type maxWorkersRuntimeActivator struct {
	values []int
}

func (a *maxWorkersRuntimeActivator) Activate(context.Context, string) error { return nil }
func (a *maxWorkersRuntimeActivator) Deactivate() error                      { return nil }
func (a *maxWorkersRuntimeActivator) SetMaxWorkers(n int) {
	a.values = append(a.values, n)
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
	if got := s.GetAutoGroomEnabled(); got != AutoGroomEnabledDefault {
		t.Errorf("auto_groom_enabled: got %v, want %v", got, AutoGroomEnabledDefault)
	}
	if got := s.GetAutoGroomSettleMinutes(); got != AutoGroomSettleMinutesDefault {
		t.Errorf("auto_groom_settle_minutes: got %d, want %d", got, AutoGroomSettleMinutesDefault)
	}
	if got := s.GetAutomationStartupGraceSeconds(); got != AutomationStartupGraceSecondsDefault {
		t.Errorf("automation_startup_grace_seconds: got %d, want %d", got, AutomationStartupGraceSecondsDefault)
	}
	if got := s.GetAutoReviewEnabled(); got != AutoReviewEnabledDefault {
		t.Errorf("auto_review_enabled: got %v, want %v", got, AutoReviewEnabledDefault)
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

func TestSetAutomationStartupGraceSeconds_RoundTripAndClamp(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetAutomationStartupGraceSeconds(45); err != nil {
		t.Fatalf("SetAutomationStartupGraceSeconds: %v", err)
	}
	if got := s.GetAutomationStartupGraceSeconds(); got != 45 {
		t.Fatalf("after set: got %d, want 45", got)
	}

	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetAutomationStartupGraceSeconds(); got != 45 {
		t.Fatalf("fresh read: got %d, want 45", got)
	}

	if err := s.SetAutomationStartupGraceSeconds(999); err != nil {
		t.Fatalf("SetAutomationStartupGraceSeconds high: %v", err)
	}
	if got := s.GetAutomationStartupGraceSeconds(); got != AutomationStartupGraceSecondsMax {
		t.Fatalf("high clamp: got %d, want %d", got, AutomationStartupGraceSecondsMax)
	}
	if err := s.SetAutomationStartupGraceSeconds(-5); err != nil {
		t.Fatalf("SetAutomationStartupGraceSeconds low: %v", err)
	}
	if got := s.GetAutomationStartupGraceSeconds(); got != AutomationStartupGraceSecondsMin {
		t.Fatalf("low clamp: got %d, want %d", got, AutomationStartupGraceSecondsMin)
	}
}

func TestSetMaxWorkers_NotifiesRuntimeController(t *testing.T) {
	dir := t.TempDir()
	activator := &maxWorkersRuntimeActivator{}
	s := NewSettingsService(SettingsOptions{
		Logger:      slog.Default(),
		RecentsPath: filepath.Join(dir, "recent.json"),
		PrefsPath:   filepath.Join(dir, "preferences.json"),
		Activator:   activator,
	})

	if err := s.SetMaxWorkers(2); err != nil {
		t.Fatalf("SetMaxWorkers: %v", err)
	}
	if len(activator.values) != 1 || activator.values[0] != 2 {
		t.Fatalf("runtime notifications: got %#v, want [2]", activator.values)
	}
}

func TestMaxWorkersMaxTracksRuntimeCPUCount(t *testing.T) {
	want := runtime.NumCPU()
	if want < 1 {
		want = 1
	}
	if MaxWorkersMax != want {
		t.Fatalf("MaxWorkersMax = %d, want runtime.NumCPU minimum 1 (%d)", MaxWorkersMax, want)
	}
}

func TestSetMaxWorkers_RoundTripsCPUCount(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	want := runtime.NumCPU()
	if want < 1 {
		want = 1
	}
	if err := s.SetMaxWorkers(want); err != nil {
		t.Fatalf("SetMaxWorkers(%d): %v", want, err)
	}
	if got := s.GetMaxWorkers(); got != want {
		t.Fatalf("after set: got %d, want %d", got, want)
	}
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetMaxWorkers(); got != want {
		t.Fatalf("fresh read: got %d, want %d", got, want)
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

func TestSetAutoGroomEnabled_RoundTrip(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetAutoGroomEnabled(true); err != nil {
		t.Fatalf("SetAutoGroomEnabled: %v", err)
	}
	if got := s.GetAutoGroomEnabled(); !got {
		t.Errorf("after set: got false, want true")
	}
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetAutoGroomEnabled(); !got {
		t.Errorf("fresh read: got false, want true")
	}
}

// TB-178 / TB-288: auto-implement preferences round-trip with the
// structured filter. The text-DSL parser was deleted in TB-288.
func acFixtureFilter() AutoImplementFilter {
	return AutoImplementFilter{
		Types:   []string{"bug"},
		Sizes:   []string{"S"},
		Modules: []string{"gui"},
	}
}

func TestSetAutoImplementQuery_RoundTrip(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	want := acFixtureFilter()
	if err := s.SetAutoImplementQuery(want); err != nil {
		t.Fatalf("SetAutoImplementQuery: %v", err)
	}
	if got := s.GetAutoImplementQuery(); !reflect.DeepEqual(got, want) {
		t.Errorf("after set: got %#v, want %#v", got, want)
	}
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetAutoImplementQuery(); !reflect.DeepEqual(got, want) {
		t.Errorf("fresh read: got %#v", got)
	}
}

func TestSetAutoImplementQuery_NormalizesValues(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	in := AutoImplementFilter{
		Search: "  router ",
		Types:  []string{" bug ", "", "improvement"},
		Tags:   []string{",", "macos", " "},
	}
	if err := s.SetAutoImplementQuery(in); err != nil {
		t.Fatalf("SetAutoImplementQuery: %v", err)
	}
	got := s.GetAutoImplementQuery()
	if got.Search != "router" {
		t.Errorf("Search not trimmed: %q", got.Search)
	}
	if !reflect.DeepEqual(got.Types, []string{"bug", "improvement"}) {
		t.Errorf("Types not normalized: %#v", got.Types)
	}
	if !reflect.DeepEqual(got.Tags, []string{",", "macos"}) {
		// Note: a lone "," is preserved because cleanStringSlice only
		// drops fully-whitespace segments. Tag values with commas would
		// be split by `tb ls`, but we don't validate against that here
		// — that's the user's responsibility via the FilterBar UI.
		t.Errorf("Tags not normalized: %#v", got.Tags)
	}
}

func TestSetAutoImplementEnabled_RequiresDefaultAgent(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	// default_agent stays at "none" — enable must fail.
	if err := s.SetAutoImplementQuery(acFixtureFilter()); err != nil {
		t.Fatalf("SetAutoImplementQuery: %v", err)
	}
	if err := s.SetAutoImplementEnabled(true); err == nil {
		t.Fatalf("expected enable rejection without default_agent")
	}
	if got := s.GetAutoImplementEnabled(); got {
		t.Errorf("preferences mutated despite validation failure")
	}
}

func TestSetAutoImplementEnabled_RequiresNonEmptyQuery(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if err := s.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := s.SetAutoImplementEnabled(true); err == nil {
		t.Fatalf("expected enable rejection with empty filter")
	}
	if s.GetAutoImplementEnabled() {
		t.Errorf("preferences mutated despite validation failure")
	}
}

func TestSetAutoImplementEnabled_AcceptsValidPrereqs(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if err := s.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := s.SetAutoImplementQuery(acFixtureFilter()); err != nil {
		t.Fatalf("SetAutoImplementQuery: %v", err)
	}
	if err := s.SetAutoImplementEnabled(true); err != nil {
		t.Fatalf("SetAutoImplementEnabled: %v", err)
	}
	if !s.GetAutoImplementEnabled() {
		t.Errorf("auto-implement should be enabled")
	}
}

func TestSetAutoImplementQuery_BlocksBlankWhileEnabled(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if err := s.SetDefaultAgent("codex"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := s.SetAutoImplementQuery(acFixtureFilter()); err != nil {
		t.Fatalf("SetAutoImplementQuery: %v", err)
	}
	if err := s.SetAutoImplementEnabled(true); err != nil {
		t.Fatalf("SetAutoImplementEnabled: %v", err)
	}
	if err := s.SetAutoImplementQuery(AutoImplementFilter{}); err == nil {
		t.Errorf("expected empty filter rejection while enabled")
	}
	if got := s.GetAutoImplementQuery(); got.IsEmpty() {
		t.Errorf("filter mutated despite validation failure: %#v", got)
	}
}

// TestSetAutoImplementEnabled_RevalidatesInsideWrite pins the TOCTOU
// fix: even if a fresh validation read sees a valid state, the write
// itself must re-validate so a between-read flip of default_agent on
// disk cannot land enabled=true with default_agent=none.
func TestSetAutoImplementEnabled_RevalidatesInsideWrite(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := s.SetAutoImplementQuery(acFixtureFilter()); err != nil {
		t.Fatalf("SetAutoImplementQuery: %v", err)
	}
	external := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if err := external.SetDefaultAgent("none"); err != nil {
		t.Fatalf("external SetDefaultAgent: %v", err)
	}
	if err := s.SetAutoImplementEnabled(true); err == nil {
		t.Fatalf("expected enable rejection after external default_agent flip")
	}
	if s.GetAutoImplementEnabled() {
		t.Errorf("auto-implement enabled despite stale prereqs")
	}
}

func TestSetAutoReviewEnabled_RequiresDefaultAgent(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	err := s.SetAutoReviewEnabled(true)
	if !errors.Is(err, ErrAutoReviewDefaultAgentRequired) {
		t.Fatalf("SetAutoReviewEnabled error = %v, want ErrAutoReviewDefaultAgentRequired", err)
	}
	if got := s.GetAutoReviewEnabled(); got {
		t.Errorf("preferences mutated despite validation failure")
	}
}

func TestSetAutoReviewEnabled_AcceptsValidDefaultAgent(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetDefaultAgent("claude"); err != nil {
		t.Fatalf("SetDefaultAgent: %v", err)
	}
	if err := s.SetAutoReviewEnabled(true); err != nil {
		t.Fatalf("SetAutoReviewEnabled(true): %v", err)
	}
	if got := s.GetAutoReviewEnabled(); !got {
		t.Errorf("auto-review should be enabled")
	}
	sEnabled := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := sEnabled.GetAutoReviewEnabled(); !got {
		t.Errorf("fresh read after enable: got false, want true")
	}
	if err := s.SetAutoReviewEnabled(false); err != nil {
		t.Fatalf("SetAutoReviewEnabled(false): %v", err)
	}
	if got := s.GetAutoReviewEnabled(); got {
		t.Errorf("auto-review should be disabled")
	}
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetAutoReviewEnabled(); got {
		t.Errorf("fresh read: got true, want false")
	}
}

func TestSetAutoReviewEnabled_ValidationLeavesFileUnchanged(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := s.SetMaxWorkers(3); err != nil {
		t.Fatalf("SetMaxWorkers: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	err = s.SetAutoReviewEnabled(true)
	if !errors.Is(err, ErrAutoReviewDefaultAgentRequired) {
		t.Fatalf("SetAutoReviewEnabled error = %v, want ErrAutoReviewDefaultAgentRequired", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("preferences changed despite validation failure:\nbefore=%s\nafter=%s", before, after)
	}
}

// Legacy migration: a preferences.json carrying the pre-TB-288 text DSL
// string for auto_implement_query loads cleanly, the field resets to an
// empty filter, and subsequent normal use works.
func TestLoadPreferences_LegacyStringAutoImplementQueryMigratesToEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")
	legacy := []byte(`{"auto_implement_query":"bug, S size, gui"}`)
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("write legacy prefs: %v", err)
	}
	s := NewSettingsService(SettingsOptions{Logger: slog.Default(), PrefsPath: path})
	got := s.GetAutoImplementQuery()
	if !got.IsEmpty() {
		t.Errorf("legacy string did not reset to empty filter: %#v", got)
	}
}

func TestGetAutoImplement_MissingFileReturnsDefaults(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if got := s.GetAutoImplementEnabled(); got != AutoImplementEnabledDefault {
		t.Errorf("missing file: enabled = %v, want default %v", got, AutoImplementEnabledDefault)
	}
	if got := s.GetAutoImplementQuery(); !got.IsEmpty() {
		t.Errorf("missing file: query = %#v, want empty filter", got)
	}
}

func TestSetAutoGroomSettleMinutes_RoundTrip(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	// 0 is a legitimate opt-out value and must survive the round trip,
	// distinct from the missing-key default of AutoGroomSettleMinutesDefault.
	if err := s.SetAutoGroomSettleMinutes(0); err != nil {
		t.Fatalf("SetAutoGroomSettleMinutes(0): %v", err)
	}
	if got := s.GetAutoGroomSettleMinutes(); got != 0 {
		t.Errorf("after set 0: got %d, want 0", got)
	}
	if err := s.SetAutoGroomSettleMinutes(30); err != nil {
		t.Fatalf("SetAutoGroomSettleMinutes(30): %v", err)
	}
	if got := s.GetAutoGroomSettleMinutes(); got != 30 {
		t.Errorf("after set 30: got %d, want 30", got)
	}
	s2 := NewSettingsService(SettingsOptions{
		Logger:    slog.Default(),
		PrefsPath: path,
	})
	if got := s2.GetAutoGroomSettleMinutes(); got != 30 {
		t.Errorf("fresh read: got %d, want 30", got)
	}
}

func TestSetAutoGroomSettleMinutes_ClampsAbove(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if err := s.SetAutoGroomSettleMinutes(999); err != nil {
		t.Fatalf("SetAutoGroomSettleMinutes(999): %v", err)
	}
	if got := s.GetAutoGroomSettleMinutes(); got != AutoGroomSettleMinutesMax {
		t.Errorf("999 → got %d, want %d", got, AutoGroomSettleMinutesMax)
	}
}

func TestSetAutoGroomSettleMinutes_ClampsBelow(t *testing.T) {
	s, _ := newSettingsForPrefs(t)
	if err := s.SetAutoGroomSettleMinutes(-5); err != nil {
		t.Fatalf("SetAutoGroomSettleMinutes(-5): %v", err)
	}
	if got := s.GetAutoGroomSettleMinutes(); got != AutoGroomSettleMinutesMin {
		t.Errorf("-5 → got %d, want %d", got, AutoGroomSettleMinutesMin)
	}
}

func TestGetAutoGroomSettleMinutes_ReadTimeClampsOutOfRangeFile(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	b, _ := json.Marshal(Preferences{AutoGroomSettleMinutes: 9999})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := s.GetAutoGroomSettleMinutes(); got != AutoGroomSettleMinutesMax {
		t.Errorf("clamp on read: got %d, want %d", got, AutoGroomSettleMinutesMax)
	}
}

// TestGetAutoGroomSettleMinutes_PartialFileFallsBackToDefault covers the
// upgrade path: a preferences.json written before this field existed has
// no `auto_groom_settle_minutes` key. A naive `json.Unmarshal` would leave
// the int at zero — which is a *legitimate* user opt-out value, so the
// loader must distinguish "key absent" from "key=0". The second-pass
// raw-map check in loadPreferences enforces this.
func TestGetAutoGroomSettleMinutes_PartialFileFallsBackToDefault(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	// Only one unrelated key — `auto_groom_settle_minutes` is absent.
	if err := os.WriteFile(path, []byte(`{"max_workers":2}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := s.GetAutoGroomSettleMinutes(); got != AutoGroomSettleMinutesDefault {
		t.Errorf("partial file (absent key): got %d, want %d (default)",
			got, AutoGroomSettleMinutesDefault)
	}
	// Sanity: an explicit 0 in the file MUST still be honored as opt-out.
	if err := os.WriteFile(path, []byte(`{"auto_groom_settle_minutes":0}`), 0o644); err != nil {
		t.Fatalf("write explicit-0: %v", err)
	}
	if got := s.GetAutoGroomSettleMinutes(); got != 0 {
		t.Errorf("explicit 0 in file: got %d, want 0", got)
	}
}

// TestGetAutoGroomEnabled_PartialFileReturnsFalse covers the same upgrade
// path for the boolean — absent key and explicit false both correctly map
// to the same off-by-default semantics, so no ambiguity exists, but the
// test pins the contract.
func TestGetAutoGroomEnabled_PartialFileReturnsFalse(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := os.WriteFile(path, []byte(`{"max_workers":2}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := s.GetAutoGroomEnabled(); got {
		t.Errorf("partial file: got true, want false")
	}
}

// TestGetAutoGroom_CorruptFileFallsBackToDefaults mirrors the existing
// corrupt-file coverage for max_workers, ensuring the new fields don't
// regress the established behavior (corrupt file = warn + use defaults).
func TestGetAutoGroom_CorruptFileFallsBackToDefaults(t *testing.T) {
	s, path := newSettingsForPrefs(t)
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := s.GetAutoGroomEnabled(); got != AutoGroomEnabledDefault {
		t.Errorf("corrupt file: GetAutoGroomEnabled got %v, want %v", got, AutoGroomEnabledDefault)
	}
	if got := s.GetAutoGroomSettleMinutes(); got != AutoGroomSettleMinutesDefault {
		t.Errorf("corrupt file: GetAutoGroomSettleMinutes got %d, want %d",
			got, AutoGroomSettleMinutesDefault)
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
