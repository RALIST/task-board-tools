package agent

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// claudeTapUsageRelPath is the file the project-scoped statusline tap
// (installed by gui/app/claude_tap.go) writes claude's `/usage` payload to.
// The path is relative to the active board's project root.
const claudeTapUsageRelPath = ".claude/tb-gui-usage.json"

// claudeTapMaxAge is the staleness threshold for the tap file. The tap is
// rewritten on every claude statusline refresh (~every few seconds while
// claude is running); anything older than this is presented as "unknown"
// rather than as a fresh value, so a long-idle claude doesn't show
// misleading numbers.
const claudeTapMaxAge = 24 * time.Hour

// claudeTapPayload is the subset of claude's statusline JSON we care about.
// Only the percent + reset fields are decoded; the full schema is much larger
// (model, workspace, sessionId, ...) and unstable, so we keep the parser
// narrow.
//
// ResetsAt is a Unix epoch seconds integer in the real payload — not an
// RFC3339 string. Keeping the field as int64 mirrors codex's `resets_at`
// shape, so both collectors funnel through the same time conversion helper.
type claudeTapPayload struct {
	RateLimits *struct {
		FiveHour *struct {
			UsedPercentage *float64 `json:"used_percentage"`
			ResetsAt       int64    `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay *struct {
			UsedPercentage *float64 `json:"used_percentage"`
			ResetsAt       int64    `json:"resets_at"`
		} `json:"seven_day"`
	} `json:"rate_limits"`
	Model *struct {
		// Claude's statusline payload doesn't expose the plan name; we keep
		// the field so a future build can populate it without a parser change.
		Plan string `json:"plan"`
	} `json:"model"`
}

// CollectClaudeUsage returns a usage snapshot for the `claude` CLI.
//
// Resolution order:
//
//  1. If projectRoot is non-empty and `<projectRoot>/.claude/tb-gui-usage.json`
//     exists and is fresh, parse it. This file is written by the statusline
//     tap installed via Settings → "Enable claude usage tap".
//  2. Otherwise fall back to a stub Usage{Available:false}, with the Reason
//     pinpointing what's missing (PATH, config dir, tap install).
//
// claudeHome overrides ~/.claude for tests; pass "" in production. projectRoot
// is the currently-open board's project root, used to look for the tap output.
func CollectClaudeUsage(claudeHome, projectRoot string) Usage {
	const agent = "claude"

	if projectRoot != "" {
		if u, ok := readClaudeTapFile(filepath.Join(projectRoot, claudeTapUsageRelPath)); ok {
			return u
		}
	}

	const source = "claude-stub"
	if _, err := exec.LookPath("claude"); err != nil {
		u := unknownUsage(agent, "claude CLI is not on PATH")
		u.Source = source
		return u
	}

	resolved := claudeHome
	if resolved == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			u := unknownUsage(agent, "could not resolve $HOME: "+err.Error())
			u.Source = source
			return u
		}
		resolved = filepath.Join(home, ".claude")
	}
	if _, err := os.Stat(resolved); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			u := unknownUsage(agent, "claude config dir missing at "+resolved)
			u.Source = source
			return u
		}
		u := unknownUsage(agent, "could not stat "+resolved+": "+err.Error())
		u.Source = source
		return u
	}
	reason := "enable the claude usage tap in Settings to populate this"
	if projectRoot == "" {
		reason = "no project open; claude usage tap needs a board to attach to"
	}
	u := unknownUsage(agent, reason)
	u.Source = source
	return u
}

// readClaudeTapFile parses the tap output. Returns ok=false when the file is
// missing, stale, malformed, or has no rate_limits — the caller falls back to
// the stub branch in that case.
func readClaudeTapFile(path string) (Usage, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return Usage{}, false
	}
	if time.Since(info.ModTime()) > claudeTapMaxAge {
		return Usage{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Usage{}, false
	}
	var payload claudeTapPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return Usage{}, false
	}
	if payload.RateLimits == nil {
		return Usage{}, false
	}

	u := Usage{
		Agent:       "claude",
		Available:   true,
		Source:      "claude-statusline-tap",
		LastUpdated: info.ModTime().UTC(),
	}
	if payload.Model != nil {
		u.Plan = payload.Model.Plan
	}
	if w := payload.RateLimits.FiveHour; w != nil && w.UsedPercentage != nil {
		u.Primary = &UsageWindow{
			UsedPercent: w.UsedPercentage,
			WindowLabel: "5h",
			ResetsAt:    unixTimeOrZero(w.ResetsAt),
		}
	}
	if w := payload.RateLimits.SevenDay; w != nil && w.UsedPercentage != nil {
		u.Secondary = &UsageWindow{
			UsedPercent: w.UsedPercentage,
			WindowLabel: "weekly",
			ResetsAt:    unixTimeOrZero(w.ResetsAt),
		}
	}
	// If both windows came back nil-percent the tap saw an unusable payload —
	// treat as not-yet-available so the header shows "unknown" rather than an
	// empty chip.
	if u.Primary == nil && u.Secondary == nil {
		return Usage{}, false
	}
	return u, true
}

