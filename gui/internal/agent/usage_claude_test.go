package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectClaudeUsage_NoClaudeHome(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no-claude")
	got := CollectClaudeUsage(dir, "")
	if got.Available {
		t.Fatalf("expected Available=false when claude home is missing")
	}
	// Two reasons can fire here depending on whether the test machine has
	// `claude` on PATH: "claude CLI is not on PATH" or
	// "claude config dir missing". Either is a valid "unknown" disposition;
	// we just want a non-empty reason and the right agent label.
	if got.Reason == "" {
		t.Errorf("expected a non-empty Reason")
	}
	if !strings.Contains(got.Reason, "claude") {
		t.Errorf("reason = %q, want it to mention 'claude'", got.Reason)
	}
	if got.Agent != "claude" {
		t.Errorf("agent = %q", got.Agent)
	}
}

func TestCollectClaudeUsage_ReportsStubReasonWhenHomeExists(t *testing.T) {
	dir := t.TempDir() // exists but empty
	got := CollectClaudeUsage(dir, "")
	if got.Available {
		t.Fatalf("expected Available=false in the stub state")
	}
	// Either the CLI lookup fails ("not on PATH") or the OAuth stub message
	// fires — both are valid "unknown" reasons. We just need a non-empty
	// reason and the right source tag.
	if got.Reason == "" {
		t.Errorf("expected a non-empty Reason")
	}
	if got.Source != "claude-stub" {
		t.Errorf("source = %q, want claude-stub", got.Source)
	}
}

// writeClaudeTapFile drops a synthetic statusline payload into the tap
// location for tests. Mirrors the real shape claude emits: rate_limits with
// five_hour / seven_day buckets, optional model.plan.
func writeClaudeTapFile(t *testing.T, projectRoot, body string) string {
	t.Helper()
	dir := filepath.Join(projectRoot, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "tb-gui-usage.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCollectClaudeUsage_ReadsTapFile(t *testing.T) {
	root := t.TempDir()
	body := `{
	  "rate_limits": {
	    "five_hour":  {"used_percentage": 42.7, "resets_at": 1778794200},
	    "seven_day":  {"used_percentage": 11.0, "resets_at": 1779181200}
	  },
	  "model": {"plan": "max"}
	}`
	writeClaudeTapFile(t, root, body)

	got := CollectClaudeUsage("", root)
	if !got.Available {
		t.Fatalf("expected Available=true from tap file, got %+v", got)
	}
	if got.Source != "claude-statusline-tap" {
		t.Errorf("source = %q", got.Source)
	}
	if got.Plan != "max" {
		t.Errorf("plan = %q, want max", got.Plan)
	}
	if got.Primary == nil || got.Primary.UsedPercent == nil || *got.Primary.UsedPercent != 42.7 {
		t.Errorf("primary = %+v, want 42.7", got.Primary)
	}
	if got.Primary.WindowLabel != "5h" {
		t.Errorf("primary label = %q", got.Primary.WindowLabel)
	}
	if got.Secondary == nil || got.Secondary.UsedPercent == nil || *got.Secondary.UsedPercent != 11.0 {
		t.Errorf("secondary = %+v, want 11.0", got.Secondary)
	}
	if got.Secondary.WindowLabel != "weekly" {
		t.Errorf("secondary label = %q", got.Secondary.WindowLabel)
	}
}

func TestCollectClaudeUsage_TapFileStaleFallsBack(t *testing.T) {
	root := t.TempDir()
	body := `{"rate_limits": {"five_hour": {"used_percentage": 99}}}`
	path := writeClaudeTapFile(t, root, body)
	// Backdate the file past the staleness window.
	old := time.Now().Add(-(claudeTapMaxAge + time.Hour))
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	got := CollectClaudeUsage(t.TempDir(), root) // empty claude home to force stub branch
	if got.Available {
		t.Errorf("expected Available=false for stale tap file, got %+v", got)
	}
}

func TestCollectClaudeUsage_TapFileMalformedFallsBack(t *testing.T) {
	root := t.TempDir()
	writeClaudeTapFile(t, root, "not json")
	got := CollectClaudeUsage(t.TempDir(), root)
	if got.Available {
		t.Errorf("expected Available=false for malformed tap file")
	}
}
