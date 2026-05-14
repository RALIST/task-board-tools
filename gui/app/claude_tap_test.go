package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return obj
}

func TestEnableClaudeUsageTap_FreshProject(t *testing.T) {
	root := t.TempDir()
	status, err := EnableClaudeUsageTap(root)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("expected Enabled=true, got %+v", status)
	}
	// Script exists and is executable.
	info, err := os.Stat(filepath.Join(root, ".claude", "tb-gui-statusline.sh"))
	if err != nil {
		t.Fatalf("stat script: %v", err)
	}
	if info.Mode()&0o100 == 0 {
		t.Errorf("script not executable: mode=%v", info.Mode())
	}
	// settings.local.json has our statusLine entry pointing at the script.
	settings := readJSON(t, filepath.Join(root, ".claude", "settings.local.json"))
	sl, ok := settings["statusLine"].(map[string]any)
	if !ok {
		t.Fatalf("statusLine missing or wrong type: %+v", settings)
	}
	if sl["type"] != "command" {
		t.Errorf("statusLine.type = %v, want command", sl["type"])
	}
	cmd, _ := sl["command"].(string)
	if !strings.HasSuffix(cmd, "tb-gui-statusline.sh") {
		t.Errorf("statusLine.command = %q, want path ending in tb-gui-statusline.sh", cmd)
	}
}

func TestEnableClaudeUsageTap_PreservesOtherSettings(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	prior := map[string]any{
		"permissions": map[string]any{"deny": []any{"Bash(rm)"}},
		"env":         map[string]any{"FOO": "bar"},
	}
	buf, _ := json.MarshalIndent(prior, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "settings.local.json"), buf, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatalf("enable: %v", err)
	}
	settings := readJSON(t, filepath.Join(dir, "settings.local.json"))
	if _, ok := settings["permissions"]; !ok {
		t.Errorf("permissions key dropped after enable")
	}
	if env, ok := settings["env"].(map[string]any); !ok || env["FOO"] != "bar" {
		t.Errorf("env not preserved: %+v", settings["env"])
	}
}

func TestDisableClaudeUsageTap_RemovesOurEntryOnly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-existing custom statusLine that ISN'T ours.
	prior := map[string]any{
		"statusLine": map[string]any{
			"type":    "command",
			"command": "/somewhere/else/custom.sh",
		},
		"env": map[string]any{"FOO": "bar"},
	}
	buf, _ := json.MarshalIndent(prior, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "settings.local.json"), buf, 0o644); err != nil {
		t.Fatal(err)
	}

	// Disable on a project where the tap was never enabled should leave the
	// foreign statusLine alone and not crash.
	status, err := DisableClaudeUsageTap(root)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if status.Enabled {
		t.Errorf("expected Enabled=false after disable, got %+v", status)
	}
	settings := readJSON(t, filepath.Join(dir, "settings.local.json"))
	sl, ok := settings["statusLine"].(map[string]any)
	if !ok {
		t.Fatalf("foreign statusLine was removed — should have been preserved")
	}
	if sl["command"] != "/somewhere/else/custom.sh" {
		t.Errorf("foreign statusLine clobbered: %v", sl["command"])
	}
}

func TestEnableThenDisable_RoundTrip(t *testing.T) {
	root := t.TempDir()
	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if status := GetClaudeUsageTapStatus(root); !status.Enabled {
		t.Fatalf("expected Enabled after enable")
	}
	if _, err := DisableClaudeUsageTap(root); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if status := GetClaudeUsageTapStatus(root); status.Enabled {
		t.Errorf("expected Disabled after disable, got %+v", status)
	}
	// Script gone.
	if _, err := os.Stat(filepath.Join(root, ".claude", "tb-gui-statusline.sh")); !os.IsNotExist(err) {
		t.Errorf("script should be removed, err=%v", err)
	}
	// settings.local.json gone (empty after our entry removed).
	if _, err := os.Stat(filepath.Join(root, ".claude", "settings.local.json")); !os.IsNotExist(err) {
		t.Errorf("empty settings.local.json should be removed, err=%v", err)
	}
}

func TestGetClaudeUsageTapStatus_PointsElsewhere(t *testing.T) {
	root := t.TempDir()
	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatalf("enable: %v", err)
	}
	// Tamper: overwrite settings.local.json so statusLine.command points at
	// a different script. Our detector should report Enabled=false with a
	// reason that names the mismatch.
	settingsPath := filepath.Join(root, ".claude", "settings.local.json")
	tampered := map[string]any{
		"statusLine": map[string]any{
			"type":    "command",
			"command": "/somewhere/else/other.sh",
		},
	}
	buf, _ := json.MarshalIndent(tampered, "", "  ")
	if err := os.WriteFile(settingsPath, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	status := GetClaudeUsageTapStatus(root)
	if status.Enabled {
		t.Errorf("expected Enabled=false when statusLine points elsewhere")
	}
	if !strings.Contains(status.Reason, "elsewhere") {
		t.Errorf("reason should mention mismatch, got %q", status.Reason)
	}
}

func TestEnsureGitignore_AppendsOnce(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/foo\n/bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(first), "tb-gui-usage.json") {
		t.Errorf("gitignore missing usage entry: %s", string(first))
	}
	// Re-enable: should not duplicate.
	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if strings.Count(string(second), "tb-gui-usage.json") != 1 {
		t.Errorf("gitignore duplicated entry on second enable: %s", string(second))
	}
}
