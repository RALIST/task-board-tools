package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Claude reads project-local settings from <projectRoot>/.claude/settings.local.json
// (untracked by convention). We piggy-back on its statusLine hook: claude pipes
// the same `rate_limits` blob it would show in the TUI `/usage` panel to any
// configured statusLine command. Our tap writes that blob to a known file so
// the GUI can read it without re-running claude or touching the user's keychain.
//
// The tap is per-project: installed under <projectRoot>/.claude/ and ignored
// outside this project's tree. Toggling it on/off is fully reversible.

const (
	claudeDirName            = ".claude"
	claudeSettingsLocalFile  = "settings.local.json"
	claudeTapScriptFile      = "tb-gui-statusline.sh"
	claudeTapUsageFile       = "tb-gui-usage.json"
	claudeTapStatusLineMark  = "tb-gui-statusline" // marker used to identify our entry
	claudeStatusLineKey      = "statusLine"
	claudeStatusLineCmdKey   = "command"
	claudeStatusLineTypeKey  = "type"
	claudeStatusLineTypeCmd  = "command"
	claudeStatusLinePaddingS = "  " // 2-space JSON indent matches claude's own writer
)

// ClaudeUsageTapStatus reports the on-disk state of the tap for a project.
type ClaudeUsageTapStatus struct {
	// Enabled is true when the script exists AND settings.local.json's
	// statusLine.command points at it.
	Enabled bool `json:"enabled"`
	// ScriptPath is the absolute path to the tap script (always set so the UI
	// can render where things would go even when disabled).
	ScriptPath string `json:"scriptPath"`
	// SettingsPath is the absolute path to settings.local.json.
	SettingsPath string `json:"settingsPath"`
	// UsagePath is where the tap writes captured rate_limits.
	UsagePath string `json:"usagePath"`
	// Reason carries a non-empty explanation when Enabled=false (e.g. "script
	// missing", "settings.local.json points elsewhere"). Empty when Enabled.
	Reason string `json:"reason,omitempty"`
}

// claudeTapPaths returns the canonical filesystem locations for the tap files
// inside a project. projectRoot must be absolute.
func claudeTapPaths(projectRoot string) (scriptPath, settingsPath, usagePath string) {
	dir := filepath.Join(projectRoot, claudeDirName)
	scriptPath = filepath.Join(dir, claudeTapScriptFile)
	settingsPath = filepath.Join(dir, claudeSettingsLocalFile)
	usagePath = filepath.Join(dir, claudeTapUsageFile)
	return
}

// GetClaudeUsageTapStatus inspects the on-disk state of the tap for the given
// project root. Never returns an error: missing files map to Enabled=false
// with a Reason rather than a hard failure.
func GetClaudeUsageTapStatus(projectRoot string) ClaudeUsageTapStatus {
	scriptPath, settingsPath, usagePath := claudeTapPaths(projectRoot)
	status := ClaudeUsageTapStatus{
		ScriptPath:   scriptPath,
		SettingsPath: settingsPath,
		UsagePath:    usagePath,
	}
	if projectRoot == "" {
		status.Reason = "no project root"
		return status
	}

	if _, err := os.Stat(scriptPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			status.Reason = "tap script not installed"
		} else {
			status.Reason = "cannot stat tap script: " + err.Error()
		}
		return status
	}

	settings, err := readClaudeSettingsLocal(settingsPath)
	if err != nil {
		status.Reason = "cannot read settings.local.json: " + err.Error()
		return status
	}
	cmd, ok := extractStatusLineCommand(settings)
	if !ok {
		status.Reason = "settings.local.json has no statusLine.command"
		return status
	}
	if !strings.Contains(cmd, claudeTapStatusLineMark) {
		status.Reason = "settings.local.json statusLine.command points elsewhere"
		return status
	}
	status.Enabled = true
	return status
}

// EnableClaudeUsageTap installs the tap script + patches settings.local.json
// so claude's statusLine flows through our writer. Idempotent: re-running on
// an already-enabled project is a no-op apart from refreshing the script.
func EnableClaudeUsageTap(projectRoot string) (ClaudeUsageTapStatus, error) {
	if projectRoot == "" {
		return ClaudeUsageTapStatus{}, errors.New("EnableClaudeUsageTap: empty project root")
	}
	scriptPath, settingsPath, _ := claudeTapPaths(projectRoot)
	dir := filepath.Dir(scriptPath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ClaudeUsageTapStatus{}, fmt.Errorf("create %s: %w", dir, err)
	}

	// 1. Write the script atomically.
	script := buildTapScript()
	if err := writeFileAtomic(scriptPath, []byte(script), 0o755); err != nil {
		return ClaudeUsageTapStatus{}, fmt.Errorf("write script: %w", err)
	}
	// writeFileAtomic preserves the temp file's mode; re-chmod just in case
	// the renamed file lost the exec bit on this platform.
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return ClaudeUsageTapStatus{}, fmt.Errorf("chmod script: %w", err)
	}

	// 2. Patch settings.local.json — merge our statusLine, leave other keys
	//    intact.
	settings, err := readClaudeSettingsLocal(settingsPath)
	if err != nil {
		return ClaudeUsageTapStatus{}, fmt.Errorf("read settings.local.json: %w", err)
	}
	settings[claudeStatusLineKey] = map[string]any{
		claudeStatusLineTypeKey: claudeStatusLineTypeCmd,
		claudeStatusLineCmdKey:  scriptPath,
	}
	if err := writeClaudeSettingsLocal(settingsPath, settings); err != nil {
		return ClaudeUsageTapStatus{}, fmt.Errorf("write settings.local.json: %w", err)
	}

	// 3. Ensure tap artefacts are gitignored so a user-specific quota state
	//    never ends up committed.
	if err := ensureGitignoreEntries(projectRoot); err != nil {
		// Non-fatal: the tap still works, the user may just need to ignore
		// these files manually.
		_ = err
	}

	return GetClaudeUsageTapStatus(projectRoot), nil
}

// DisableClaudeUsageTap removes our statusLine entry from settings.local.json
// and deletes the tap script. The captured tb-gui-usage.json is left in place
// so the header keeps showing the last-known value until the next refresh —
// the user can delete it manually if they want a clean slate.
func DisableClaudeUsageTap(projectRoot string) (ClaudeUsageTapStatus, error) {
	if projectRoot == "" {
		return ClaudeUsageTapStatus{}, errors.New("DisableClaudeUsageTap: empty project root")
	}
	scriptPath, settingsPath, _ := claudeTapPaths(projectRoot)

	settings, err := readClaudeSettingsLocal(settingsPath)
	if err != nil {
		return ClaudeUsageTapStatus{}, fmt.Errorf("read settings.local.json: %w", err)
	}
	if cmd, ok := extractStatusLineCommand(settings); ok && strings.Contains(cmd, claudeTapStatusLineMark) {
		delete(settings, claudeStatusLineKey)
		if err := writeClaudeSettingsLocal(settingsPath, settings); err != nil {
			return ClaudeUsageTapStatus{}, fmt.Errorf("write settings.local.json: %w", err)
		}
	}

	if err := os.Remove(scriptPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ClaudeUsageTapStatus{}, fmt.Errorf("remove script: %w", err)
	}
	return GetClaudeUsageTapStatus(projectRoot), nil
}

// buildTapScript renders the bash script that claude executes on every
// statusline update. The script reads stdin (claude's JSON payload), atomically
// writes it next to itself, and echoes a single-character status so the
// statusline bar isn't visibly empty.
//
// The "tb-gui-statusline" comment is the marker GetClaudeUsageTapStatus uses
// to recognise our install — do not rename it without updating the constant.
func buildTapScript() string {
	return `#!/usr/bin/env bash
# tb-gui-statusline — auto-generated by tb-gui to tap claude's /usage data.
# Reads claude's statusline JSON on stdin; persists rate_limits next door.
# Safe to delete; tb-gui will reinstall on next Settings toggle.
set -u
DIR="$(cd "$(dirname "$0")" && pwd)"
PAYLOAD="$(cat)"
TMP="$DIR/tb-gui-usage.json.tmp.$$"
printf '%s' "$PAYLOAD" > "$TMP" && mv -f "$TMP" "$DIR/tb-gui-usage.json"
# Echo a minimal statusline so claude doesn't render a blank bar. The user can
# disable the tap from tb-gui Settings if they want their own statusline back.
printf 'tb-gui tap\n'
`
}

// readClaudeSettingsLocal returns the contents of settings.local.json as a
// generic JSON object. A missing file is normal (claude treats it as
// optional); the function returns an empty map in that case. Malformed JSON
// is preserved as an error so the install path doesn't accidentally clobber
// user content.
func readClaudeSettingsLocal(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return map[string]any{}, nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		obj = map[string]any{}
	}
	return obj, nil
}

// writeClaudeSettingsLocal serialises the merged settings back to disk
// atomically. We use claude's own 2-space indent so a hand-eyeballed diff
// stays readable.
func writeClaudeSettingsLocal(path string, settings map[string]any) error {
	if len(settings) == 0 {
		// Nothing left to persist — remove the file rather than leaving an
		// empty {} stub.
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(settings, "", claudeStatusLinePaddingS)
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	return writeFileAtomic(path, buf, 0o644)
}

// extractStatusLineCommand digs the "command" out of a {"statusLine": {...}}
// blob. Returns ok=false when the field is absent or has the wrong shape so
// the caller can distinguish "no entry" from "entry but pointing elsewhere".
func extractStatusLineCommand(settings map[string]any) (string, bool) {
	raw, ok := settings[claudeStatusLineKey]
	if !ok {
		return "", false
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return "", false
	}
	cmd, ok := obj[claudeStatusLineCmdKey].(string)
	if !ok {
		return "", false
	}
	return cmd, true
}

// ensureGitignoreEntries appends the tap-related ignores to the project's
// .gitignore so user-specific quota state never gets committed. Idempotent:
// existing entries are detected by exact-line match.
func ensureGitignoreEntries(projectRoot string) error {
	path := filepath.Join(projectRoot, ".gitignore")
	wanted := []string{
		"/.claude/" + claudeSettingsLocalFile,
		"/.claude/" + claudeTapScriptFile,
		"/.claude/" + claudeTapUsageFile,
	}

	var existing string
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	lines := strings.Split(existing, "\n")
	have := make(map[string]bool, len(lines))
	for _, l := range lines {
		have[strings.TrimSpace(l)] = true
	}

	var add []string
	for _, w := range wanted {
		if !have[w] {
			add = append(add, w)
		}
	}
	if len(add) == 0 {
		return nil
	}

	var buf strings.Builder
	buf.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		buf.WriteByte('\n')
	}
	if existing != "" {
		buf.WriteByte('\n')
	}
	buf.WriteString("# tb-gui claude usage tap (auto-added)\n")
	for _, w := range add {
		buf.WriteString(w)
		buf.WriteByte('\n')
	}
	return writeFileAtomic(path, []byte(buf.String()), 0o644)
}
