package agent

import (
	"encoding/json"
	"os"
	"testing"
)

// TestCollectUsage_Smoke runs both collectors against the user's real home
// directory when the env var TBGUI_SMOKE_USAGE is set. Useful for verifying
// the parser works end-to-end on a real installation; skipped in CI / normal
// runs so it never fails on machines without sessions / installed agents.
func TestCollectUsage_Smoke(t *testing.T) {
	if os.Getenv("TBGUI_SMOKE_USAGE") == "" {
		t.Skip("set TBGUI_SMOKE_USAGE=1 to run smoke checks against the real home dir")
	}
	codex := CollectCodexUsage("")
	out, _ := json.MarshalIndent(codex, "", "  ")
	t.Logf("codex usage: %s", string(out))
	// Pass an empty projectRoot here — the smoke test only proves codex parses
	// against a real installation. Use TestClaudeTap_EndToEnd for the claude
	// path; it builds a complete tap install from scratch.
	claude := CollectClaudeUsage("", "")
	out2, _ := json.MarshalIndent(claude, "", "  ")
	t.Logf("claude usage: %s", string(out2))
}
