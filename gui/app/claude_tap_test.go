package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
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

// isolateClaudeHome points os.UserHomeDir at a fresh tempdir for the duration
// of the test so resolveTapOriginal can't accidentally read the developer's
// real ~/.claude/settings.json. Tests that want a fake global statusLine can
// pass a non-nil global map.
func isolateClaudeHome(t *testing.T, global map[string]any) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows fallback
	if global != nil {
		dir := filepath.Join(home, ".claude")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir global .claude: %v", err)
		}
		buf, _ := json.MarshalIndent(global, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), buf, 0o644); err != nil {
			t.Fatalf("write global settings.json: %v", err)
		}
	}
	return home
}

func TestEnableClaudeUsageTap_FreshProject(t *testing.T) {
	isolateClaudeHome(t, nil)
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
	isolateClaudeHome(t, nil)
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
	isolateClaudeHome(t, nil)
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
	isolateClaudeHome(t, nil)
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
	isolateClaudeHome(t, nil)
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
	isolateClaudeHome(t, nil)
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

// writeLocalStatusLine seeds <root>/.claude/settings.local.json with a
// foreign statusLine entry plus the given extras, so Enable has something
// to capture as source=local.
func writeLocalStatusLine(t *testing.T, root, cmd string) {
	t.Helper()
	dir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	prior := map[string]any{
		"statusLine": map[string]any{
			"type":    "command",
			"command": cmd,
		},
	}
	buf, _ := json.MarshalIndent(prior, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "settings.local.json"), buf, 0o644); err != nil {
		t.Fatalf("write local settings: %v", err)
	}
}

// TestEnable_CapturesLocalForeignStatusLine confirms that a pre-existing
// non-ours entry in settings.local.json is moved into the sidecar with
// source=local, and the generated script chains to it via bash -c.
func TestEnable_CapturesLocalForeignStatusLine(t *testing.T) {
	isolateClaudeHome(t, nil)
	root := t.TempDir()
	writeLocalStatusLine(t, root, "/some/where/custom.sh --flag")

	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatalf("enable: %v", err)
	}

	orig, ok := readTapOriginal(root)
	if !ok {
		t.Fatalf("sidecar not written")
	}
	if orig.Source != "local" {
		t.Errorf("orig.Source = %q, want local", orig.Source)
	}
	if orig.Command != "/some/where/custom.sh --flag" {
		t.Errorf("orig.Command = %q, want the foreign cmd", orig.Command)
	}

	script, err := os.ReadFile(filepath.Join(root, ".claude", "tb-gui-statusline.sh"))
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	if !strings.Contains(string(script), "/some/where/custom.sh --flag") {
		t.Errorf("script doesn't embed the captured original:\n%s", string(script))
	}
	if !strings.Contains(string(script), `bash -c "$ORIG_CMD"`) {
		t.Errorf("script doesn't chain via bash -c:\n%s", string(script))
	}
}

// TestEnable_CapturesGlobalStatusLine confirms that when settings.local.json
// has nothing, the global ~/.claude/settings.json statusLine is captured
// with source=global so Disable knows to just unmask it rather than write
// it into settings.local.json.
func TestEnable_CapturesGlobalStatusLine(t *testing.T) {
	isolateClaudeHome(t, map[string]any{
		"statusLine": map[string]any{
			"type":    "command",
			"command": "sh /home/me/.claude/statusline.sh",
		},
	})
	root := t.TempDir()

	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatalf("enable: %v", err)
	}

	orig, ok := readTapOriginal(root)
	if !ok {
		t.Fatalf("sidecar not written")
	}
	if orig.Source != "global" {
		t.Errorf("orig.Source = %q, want global", orig.Source)
	}
	if orig.Command != "sh /home/me/.claude/statusline.sh" {
		t.Errorf("orig.Command = %q", orig.Command)
	}
}

// TestDisable_RestoresLocalForeignStatusLine confirms the source=local path:
// Enable captured a foreign local entry; Disable must write that entry
// back into settings.local.json so the user gets their custom statusLine
// back exactly as it was.
func TestDisable_RestoresLocalForeignStatusLine(t *testing.T) {
	isolateClaudeHome(t, nil)
	root := t.TempDir()
	writeLocalStatusLine(t, root, "/some/where/custom.sh")

	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if _, err := DisableClaudeUsageTap(root); err != nil {
		t.Fatalf("disable: %v", err)
	}

	settings := readJSON(t, filepath.Join(root, ".claude", "settings.local.json"))
	sl, ok := settings["statusLine"].(map[string]any)
	if !ok {
		t.Fatalf("statusLine not restored: %+v", settings)
	}
	if sl["command"] != "/some/where/custom.sh" {
		t.Errorf("statusLine.command = %v, want the restored foreign cmd", sl["command"])
	}
	if sl["type"] != "command" {
		t.Errorf("statusLine.type = %v, want command", sl["type"])
	}
	// Sidecar is gone after disable.
	if _, err := os.Stat(filepath.Join(root, ".claude", "tb-gui-statusline-original.json")); !os.IsNotExist(err) {
		t.Errorf("sidecar should be removed after disable, err=%v", err)
	}
}

// TestDisable_UnmasksGlobalStatusLine confirms the source=global path:
// the global entry was masked while Enabled; Disable just removes our
// local entry so the global one is no longer masked. We must NOT copy
// the global command into settings.local.json — that would duplicate
// and pin a stale snapshot.
func TestDisable_UnmasksGlobalStatusLine(t *testing.T) {
	isolateClaudeHome(t, map[string]any{
		"statusLine": map[string]any{
			"type":    "command",
			"command": "sh /home/me/.claude/statusline.sh",
		},
	})
	root := t.TempDir()

	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if _, err := DisableClaudeUsageTap(root); err != nil {
		t.Fatalf("disable: %v", err)
	}

	// settings.local.json should not exist (empty after removal) or have
	// no statusLine entry. Either is correct.
	path := filepath.Join(root, ".claude", "settings.local.json")
	if data, err := os.ReadFile(path); err == nil {
		var obj map[string]any
		_ = json.Unmarshal(data, &obj)
		if _, has := obj["statusLine"]; has {
			t.Errorf("settings.local.json should not carry statusLine after disable: %s", string(data))
		}
	}
}

// TestEnable_ReEnablePreservesSidecar confirms idempotency: the second
// Enable sees our entry in settings.local.json (containing the marker),
// must not "capture" itself, and must keep the previously-captured local
// original recorded in the sidecar.
func TestEnable_ReEnablePreservesSidecar(t *testing.T) {
	isolateClaudeHome(t, nil)
	root := t.TempDir()
	writeLocalStatusLine(t, root, "/keep/me.sh")

	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatalf("enable 1: %v", err)
	}
	first, ok := readTapOriginal(root)
	if !ok || first.Command != "/keep/me.sh" || first.Source != "local" {
		t.Fatalf("first capture wrong: %+v", first)
	}

	if _, err := EnableClaudeUsageTap(root); err != nil {
		t.Fatalf("enable 2: %v", err)
	}
	second, ok := readTapOriginal(root)
	if !ok {
		t.Fatalf("sidecar dropped on re-enable")
	}
	if second.Command != "/keep/me.sh" || second.Source != "local" {
		t.Errorf("re-enable clobbered captured original: %+v", second)
	}
}

// TestTapScript_ChainsForOriginalCommand exercises the generated script
// end-to-end: it feeds JSON on stdin, asserts the usage file is written,
// AND asserts the stdout reflects the chained command (a stub script
// that prints a unique sentinel). This is the core requirement: save
// the JSON AND keep the user's nice display.
func TestTapScript_ChainsForOriginalCommand(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	isolateClaudeHome(t, nil)
	root := t.TempDir()

	// Stub user statusLine that just prints a sentinel line (proxy for
	// the user's nice colored statusline).
	stub := filepath.Join(t.TempDir(), "stub.sh")
	stubContent := "#!/usr/bin/env bash\ncat >/dev/null\nprintf 'CHAINED-OK\\n'\n"
	if err := os.WriteFile(stub, []byte(stubContent), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	writeLocalStatusLine(t, root, stub)

	status, err := EnableClaudeUsageTap(root)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}

	payload := `{"rate_limits":{"five_hour":{"used_percentage":1.0}}}`
	cmd := exec.Command("bash", status.ScriptPath)
	cmd.Stdin = bytes.NewBufferString(payload)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run script: %v (out=%q)", err, string(out))
	}
	if !bytes.Contains(out, []byte("CHAINED-OK")) {
		t.Errorf("chained stub output not on stdout: %q", string(out))
	}
	if bytes.Contains(out, []byte("tb-gui tap")) {
		t.Errorf("fallback line leaked when chain was present: %q", string(out))
	}
	// JSON still saved.
	saved, err := os.ReadFile(status.UsagePath)
	if err != nil {
		t.Fatalf("usage file missing: %v", err)
	}
	if !bytes.Contains(saved, []byte("five_hour")) {
		t.Errorf("usage file didn't capture payload: %s", string(saved))
	}
}

// TestTapScript_ChainSurvivesSingleQuotes proves the embedded original
// command is shell-quote-safe — a command containing single quotes
// (which is what `'\''`-escaping is for) still runs correctly.
func TestTapScript_ChainSurvivesSingleQuotes(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	isolateClaudeHome(t, nil)
	root := t.TempDir()
	// printf 'it''s fine\n' uses an apostrophe-laden literal to mimic a
	// hostile-but-legitimate user statusline command.
	writeLocalStatusLine(t, root, `bash -c "cat >/dev/null; printf 'it'\''s fine\n'"`)

	status, err := EnableClaudeUsageTap(root)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}

	cmd := exec.Command("bash", status.ScriptPath)
	cmd.Stdin = bytes.NewBufferString(`{}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run script: %v (out=%q)", err, string(out))
	}
	if !bytes.Contains(out, []byte("it's fine")) {
		t.Errorf("quoted chain command lost its quoting: %q", string(out))
	}
}

// TestShellSingleQuote_EscapesEmbeddedQuotes is a pure unit test for the
// escaping helper.
func TestShellSingleQuote_EscapesEmbeddedQuotes(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"abc", "'abc'"},
		{"a'b", `'a'\''b'`},
		{"'", `''\'''`},
	}
	for _, tc := range cases {
		got := shellSingleQuote(tc.in)
		if got != tc.want {
			t.Errorf("shellSingleQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
