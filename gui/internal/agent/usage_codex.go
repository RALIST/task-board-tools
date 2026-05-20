package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// codexSessionsDir resolves the user's codex session directory. Overridable in
// tests via WithCodexSessionsDir on the collector.
func codexSessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "sessions"), nil
}

// codexRateLimitsLine matches the JSON shape we care about inside a codex
// rollout JSONL line. We only decode the subset we need so unknown / changing
// fields elsewhere don't break the parser.
type codexRateLimitsLine struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		Type       string `json:"type"`
		RateLimits *struct {
			Primary *struct {
				UsedPercent   *float64 `json:"used_percent"`
				WindowMinutes int      `json:"window_minutes"`
				ResetsAt      int64    `json:"resets_at"`
			} `json:"primary"`
			Secondary *struct {
				UsedPercent   *float64 `json:"used_percent"`
				WindowMinutes int      `json:"window_minutes"`
				ResetsAt      int64    `json:"resets_at"`
			} `json:"secondary"`
			PlanType string `json:"plan_type"`
		} `json:"rate_limits"`
	} `json:"payload"`
}

// CollectCodexUsage reads the most recent codex rollout JSONL file and returns
// the latest rate_limits snapshot. sessionsDir overrides the default location
// (used by tests). When sessionsDir is "" the default ~/.codex/sessions is
// used.
//
// Errors are intentionally folded into Usage{Available:false} rather than
// returned — the caller (UsageService) wants a value to render in either case.
func CollectCodexUsage(sessionsDir string) Usage {
	const agent = "codex"
	const source = "codex-session-jsonl"

	if sessionsDir == "" {
		d, err := codexSessionsDir()
		if err != nil {
			u := unknownUsage(agent, "could not resolve $HOME: "+err.Error())
			u.Source = source
			return u
		}
		sessionsDir = d
	}

	latest, err := findLatestCodexSession(sessionsDir)
	if err != nil {
		u := unknownUsage(agent, codexUnavailableReason(sessionsDir, err))
		u.Source = source
		return u
	}

	snapshot, err := parseLatestCodexRateLimits(latest)
	if err != nil {
		u := unknownUsage(agent, "codex session has no rate_limits entries yet")
		u.Source = source
		return u
	}
	snapshot.Source = source
	return snapshot
}

func codexUnavailableReason(sessionsDir string, err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return "codex has no sessions yet at " + sessionsDir
	}
	return "could not enumerate codex sessions: " + err.Error()
}

// findLatestCodexSession walks `~/.codex/sessions/**/rollout-*.jsonl` and
// returns the path with the most recent modtime. Returns fs.ErrNotExist when
// the directory or any rollouts are missing — the caller maps that to an
// "unknown, no sessions yet" reason.
func findLatestCodexSession(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fs.ErrNotExist
	}

	type candidate struct {
		path  string
		mtime time.Time
	}
	var cands []candidate

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip subtrees we can't read; don't fail the whole scan.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
			return nil
		}
		// Files can disappear between WalkDir's entry read and Info. Skip
		// that candidate and keep scanning the rest of the rollout tree.
		if fi, err := d.Info(); err == nil {
			cands = append(cands, candidate{path: path, mtime: fi.ModTime()})
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(cands) == 0 {
		return "", fs.ErrNotExist
	}
	sort.Slice(cands, func(i, j int) bool {
		return cands[i].mtime.After(cands[j].mtime)
	})
	return cands[0].path, nil
}

// parseLatestCodexRateLimits scans the JSONL file line-by-line and returns the
// last (most recent) rate_limits snapshot it finds. Returns an error only when
// the file is unreadable or contains no rate_limits entries.
func parseLatestCodexRateLimits(path string) (Usage, error) {
	f, err := os.Open(path)
	if err != nil {
		return Usage{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Some lines are large (tool results, agent reasoning blobs). 2 MiB is
	// generous; if a line is longer than that, we skip it — rate_limits lines
	// themselves are well under 1 KiB.
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)

	var latest codexRateLimitsLine
	var haveLatest bool
	rateLimitsToken := []byte("rate_limits")
	for scanner.Scan() {
		line := scanner.Bytes()
		// Cheap reject: every line we care about has the substring
		// "rate_limits".
		if !bytes.Contains(line, rateLimitsToken) {
			continue
		}
		var entry codexRateLimitsLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Payload.RateLimits == nil {
			continue
		}
		latest = entry
		haveLatest = true
	}
	if err := scanner.Err(); err != nil {
		return Usage{}, err
	}
	if !haveLatest {
		return Usage{}, errors.New("no rate_limits entries in codex session")
	}

	u := Usage{
		Agent:       "codex",
		Available:   true,
		Plan:        latest.Payload.RateLimits.PlanType,
		LastUpdated: time.Now().UTC(),
	}
	rl := latest.Payload.RateLimits
	if p := rl.Primary; p != nil {
		u.Primary = &UsageWindow{
			UsedPercent: p.UsedPercent,
			WindowLabel: windowLabelFromMinutes(p.WindowMinutes),
			ResetsAt:    unixTimeOrZero(p.ResetsAt),
		}
	}
	if s := rl.Secondary; s != nil {
		u.Secondary = &UsageWindow{
			UsedPercent: s.UsedPercent,
			WindowLabel: windowLabelFromMinutes(s.WindowMinutes),
			ResetsAt:    unixTimeOrZero(s.ResetsAt),
		}
	}
	return u, nil
}

// windowLabelFromMinutes converts the agent's reported window length into a
// short human-readable label. Known sizes get nice labels; anything else
// (including a zero / missing value) returns an empty string so the
// frontend can decide what to render in its place.
func windowLabelFromMinutes(m int) string {
	switch m {
	case 60:
		return "1h"
	case 300:
		return "5h"
	case 1440:
		return "daily"
	case 10080:
		return "weekly"
	}
	return ""
}

func unixTimeOrZero(ts int64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0).UTC()
}
