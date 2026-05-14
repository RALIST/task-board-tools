package app

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"tools/tb-gui/internal/agent"
)

// TestClaudeTap_EndToEnd installs the tap into a temp project, pipes a real
// claude statusline JSON shape through the script, then runs the collector
// against the project and asserts it returns the expected percents.
//
// This proves the script + reader contract without needing a live claude
// session: the script is just a bash filter, and we feed it the same JSON
// claude would emit.
func TestClaudeTap_EndToEnd(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	root := t.TempDir()
	status, err := EnableClaudeUsageTap(root)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("expected Enabled=true after install: %+v", status)
	}

	// Execute the script with a realistic claude statusline payload on stdin.
	// This mirrors how claude invokes it.
	payload := `{"sessionId":"abc","model":{"plan":"max","display_name":"Sonnet"},"workspace":{"current_dir":"/tmp"},"rate_limits":{"five_hour":{"used_percentage":17.4,"resets_at":1778794200},"seven_day":{"used_percentage":42.0,"resets_at":1779181200}}}`
	cmd := exec.Command("bash", status.ScriptPath)
	cmd.Stdin = bytes.NewBufferString(payload)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script run: %v (out=%q)", err, string(out))
	}
	if !bytes.Contains(out, []byte("tb-gui")) {
		t.Errorf("script should echo a statusline string; got %q", string(out))
	}

	// Tap file should exist next to the script with the same payload.
	written, err := os.ReadFile(status.UsagePath)
	if err != nil {
		t.Fatalf("usage file not written: %v", err)
	}
	if !bytes.Contains(written, []byte("17.4")) {
		t.Errorf("usage file missing five_hour value: %s", string(written))
	}

	// Collector reads it back with the right shape.
	usage := agent.CollectClaudeUsage("", root)
	if !usage.Available {
		t.Fatalf("expected collector Available=true after tap fed, got %+v", usage)
	}
	if usage.Source != "claude-statusline-tap" {
		t.Errorf("source = %q", usage.Source)
	}
	if usage.Plan != "max" {
		t.Errorf("plan = %q, want max", usage.Plan)
	}
	if usage.Primary == nil || *usage.Primary.UsedPercent != 17.4 {
		t.Errorf("primary = %+v", usage.Primary)
	}
	if usage.Secondary == nil || *usage.Secondary.UsedPercent != 42.0 {
		t.Errorf("secondary = %+v", usage.Secondary)
	}

	// Round-trip: disabling removes the tap; subsequent collector call falls
	// back to the stub.
	if _, err := DisableClaudeUsageTap(root); err != nil {
		t.Fatalf("disable: %v", err)
	}
	// Touch a fresh stale file to ensure the disable removed the script and
	// the cached usage isn't picked up after a long gap.
	if _, err := os.Stat(filepath.Join(root, ".claude", "tb-gui-statusline.sh")); !os.IsNotExist(err) {
		t.Errorf("script should be removed: %v", err)
	}
	// Backdate the usage file so the staleness branch kicks in too.
	old := time.Now().Add(-(48 * time.Hour))
	_ = os.Chtimes(status.UsagePath, old, old)

	after := agent.CollectClaudeUsage(t.TempDir(), root)
	if after.Available {
		t.Errorf("expected Available=false after disable+stale, got %+v", after)
	}
}
