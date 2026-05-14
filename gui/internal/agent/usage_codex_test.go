package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// codexSessionFixture is the real shape of a `payload.type=="token_count"`
// line as observed in ~/.codex/sessions/.../rollout-*.jsonl on codex-cli
// 0.130.0. We freeze a copy here so the parser is tested against the actual
// on-disk format, not a paraphrase.
const codexSessionFixture = `{"timestamp":"2026-05-14T16:59:01.000Z","type":"event_msg","payload":{"type":"session_start"}}
{"timestamp":"2026-05-14T17:00:13.145Z","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":{"limit_id":"codex","limit_name":null,"primary":{"used_percent":0.0,"window_minutes":300,"resets_at":1778779125},"secondary":{"used_percent":16.0,"window_minutes":10080,"resets_at":1779262512},"credits":null,"plan_type":"prolite","rate_limit_reached_type":null}}}
{"timestamp":"2026-05-14T17:00:19.313Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":25649,"output_tokens":283,"total_tokens":25932}},"rate_limits":{"limit_id":"codex","limit_name":null,"primary":{"used_percent":12.5,"window_minutes":300,"resets_at":1778779125},"secondary":{"used_percent":17.3,"window_minutes":10080,"resets_at":1779262512},"credits":null,"plan_type":"prolite","rate_limit_reached_type":null}}}
`

func writeCodexSession(t *testing.T, dir string, mtime time.Time, body string) string {
	t.Helper()
	day := filepath.Join(dir, "2026", "05", "14")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "rollout-" + mtime.Format("2006-01-02T15-04-05") + ".jsonl"
	path := filepath.Join(day, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCollectCodexUsage_LatestEntry(t *testing.T) {
	dir := t.TempDir()
	writeCodexSession(t, dir, time.Now().Add(-time.Hour), codexSessionFixture)

	got := CollectCodexUsage(dir)
	if !got.Available {
		t.Fatalf("expected Available=true, got reason=%q", got.Reason)
	}
	if got.Agent != "codex" {
		t.Errorf("agent = %q, want codex", got.Agent)
	}
	if got.Source != "codex-session-jsonl" {
		t.Errorf("source = %q", got.Source)
	}
	if got.Plan != "prolite" {
		t.Errorf("plan = %q, want prolite", got.Plan)
	}
	if got.Primary == nil || got.Primary.UsedPercent == nil || *got.Primary.UsedPercent != 12.5 {
		t.Errorf("primary used_percent = %+v, want 12.5", got.Primary)
	}
	if got.Primary.WindowLabel != "5h" {
		t.Errorf("primary window label = %q, want 5h", got.Primary.WindowLabel)
	}
	if got.Secondary == nil || got.Secondary.UsedPercent == nil || *got.Secondary.UsedPercent != 17.3 {
		t.Errorf("secondary used_percent = %+v, want 17.3", got.Secondary)
	}
	if got.Secondary.WindowLabel != "weekly" {
		t.Errorf("secondary window label = %q, want weekly", got.Secondary.WindowLabel)
	}
	if got.Primary.ResetsAt.IsZero() {
		t.Errorf("primary resets_at is zero")
	}
}

func TestCollectCodexUsage_PicksNewestSession(t *testing.T) {
	dir := t.TempDir()
	// Older session — only one rate_limits entry, used_percent=99.
	oldBody := `{"timestamp":"2026-05-13T00:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","primary":{"used_percent":99.0,"window_minutes":300,"resets_at":1778779125},"secondary":{"used_percent":99.0,"window_minutes":10080,"resets_at":1779262512},"plan_type":"prolite"}}}` + "\n"
	writeCodexSession(t, dir, time.Now().Add(-24*time.Hour), oldBody)
	// Newer session with a different value.
	newBody := `{"timestamp":"2026-05-14T00:00:00Z","type":"event_msg","payload":{"type":"token_count","rate_limits":{"limit_id":"codex","primary":{"used_percent":1.2,"window_minutes":300,"resets_at":1778779125},"secondary":{"used_percent":3.4,"window_minutes":10080,"resets_at":1779262512},"plan_type":"prolite"}}}` + "\n"
	writeCodexSession(t, dir, time.Now(), newBody)

	got := CollectCodexUsage(dir)
	if !got.Available {
		t.Fatalf("expected Available=true, reason=%q", got.Reason)
	}
	if got.Primary == nil || *got.Primary.UsedPercent != 1.2 {
		t.Errorf("expected newer session primary=1.2, got %+v", got.Primary)
	}
}

func TestCollectCodexUsage_NoSessionsDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	got := CollectCodexUsage(dir)
	if got.Available {
		t.Fatalf("expected Available=false for missing dir")
	}
	if got.Reason == "" {
		t.Errorf("expected a reason for unavailable usage")
	}
	if got.Source != "codex-session-jsonl" {
		t.Errorf("source = %q", got.Source)
	}
}

func TestCollectCodexUsage_NoRateLimitsEntries(t *testing.T) {
	dir := t.TempDir()
	// A session with no token_count event ever.
	body := `{"timestamp":"2026-05-14T16:59:01.000Z","type":"event_msg","payload":{"type":"session_start"}}` + "\n"
	writeCodexSession(t, dir, time.Now(), body)

	got := CollectCodexUsage(dir)
	if got.Available {
		t.Fatalf("expected Available=false when no rate_limits present")
	}
	if got.Reason == "" {
		t.Errorf("expected a reason explaining why")
	}
}

func TestCollectCodexUsage_MalformedLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	body := "{not json\n" +
		codexSessionFixture +
		"{not json either}\n"
	writeCodexSession(t, dir, time.Now(), body)

	got := CollectCodexUsage(dir)
	if !got.Available {
		t.Fatalf("expected Available=true even with malformed lines around the data: %q", got.Reason)
	}
	if *got.Primary.UsedPercent != 12.5 {
		t.Errorf("expected the well-formed line to win, got %v", *got.Primary.UsedPercent)
	}
}

func TestWindowLabelFromMinutes(t *testing.T) {
	cases := map[int]string{
		0:     "",
		60:    "1h",
		300:   "5h",
		1440:  "daily",
		10080: "weekly",
		7:     "",
	}
	for m, want := range cases {
		if got := windowLabelFromMinutes(m); got != want {
			t.Errorf("windowLabelFromMinutes(%d) = %q, want %q", m, got, want)
		}
	}
}
